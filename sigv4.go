package appsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdkv2_v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	sdkv1_v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

type sigv4 interface {
	signHTTP(payload []byte) (http.Header, error)
	signWS(payload []byte) (map[string]string, error)
}

type _signer struct {
	sdkSigner any
	region    string
	url       string

	creds *aws.Credentials
}

func (s *_signer) signHTTP(payload []byte) (http.Header, error) {
	slog.Debug("signing http request", "payload", string(payload))
	req, err := http.NewRequest("POST", s.url, bytes.NewBuffer(payload))
	if err != nil {
		slog.Error("error creating signing request", "error", err)
		return nil, err
	}
	switch signer := s.sdkSigner.(type) {
	case *sdkv1_v4.Signer:
		slog.Debug("signing request using sdk v1")
		_, err = signer.Sign(req, bytes.NewReader(payload), "appsync", s.region, time.Now())
		if err != nil {
			slog.Error("error signing request using sdk v1", "error", err)
			return nil, err
		}
	case *sdkv2_v4.Signer:
		slog.Debug("signing request using sdk v2")
		hash := sha256.Sum256(payload)
		if err := signer.SignHTTP(context.TODO(), *s.creds, req, hex.EncodeToString(hash[:]), "appsync", s.region, time.Now()); err != nil {
			slog.Error("error signing request using sdk v2", "error", err)
			return nil, err
		}
	default:
		return http.Header{}, errors.New("unsupported signer")
	}
	return req.Header, nil
}

func (s *_signer) signWS(payload []byte) (map[string]string, error) {
	slog.Debug("signing ws", "payload", string(payload))
	url := s.url
	if bytes.Equal(payload, []byte("{}")) {
		url = url + "/connect"
	}
	slog.Debug("signing ws url", "url", url)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		slog.Error("error creating request", "error", err)
		return nil, err
	}

	req.Header.Add("accept", "application/json, text/javascript")
	req.Header.Add("content-encoding", "amz-1.0")
	req.Header.Add("content-type", "application/json; charset=UTF-8")

	switch signer := s.sdkSigner.(type) {
	case *sdkv1_v4.Signer:
		slog.Debug("signing ws using sdk v1")
		_, err = signer.Sign(req, bytes.NewReader(payload), "appsync", s.region, time.Now())
		if err != nil {
			slog.Error("error signing request", "error", err)
			return nil, err
		}
		return map[string]string{
			"accept":               req.Header.Get("accept"),
			"content-encoding":     req.Header.Get("content-encoding"),
			"content-type":         req.Header.Get("content-type"),
			"host":                 req.Host,
			"x-amz-date":           req.Header.Get("x-amz-date"),
			"Authorization":        req.Header.Get("Authorization"),
			"X-Amz-Security-Token": req.Header.Get("X-Amz-Security-Token"),
		}, nil
	case *sdkv2_v4.Signer:
		slog.Debug("signing ws using sdk v2")
		hash := sha256.Sum256(payload)
		if err := signer.SignHTTP(context.TODO(), *s.creds, req, hex.EncodeToString(hash[:]), "appsync", s.region, time.Now()); err != nil {
			slog.Error("error signing request", "error", err)
			return nil, err
		}

		headers := map[string]string{
			"accept":           req.Header.Get("accept"),
			"content-encoding": req.Header.Get("content-encoding"),
			"content-type":     req.Header.Get("content-type"),
			"content-length":   strconv.FormatInt(req.ContentLength, 10),
			"host":             req.Host,
			"x-amz-date":       req.Header.Get("x-amz-date"),
			"Authorization":    req.Header.Get("Authorization"),
		}
		token := req.Header.Get("X-Amz-Security-Token")
		if token != "" {
			headers["X-Amz-Security-Token"] = token
		}
		slog.Debug("signed ws headers", "headers", headers)
		return headers, nil
	}
	return map[string]string{}, errors.New("unsupported signer")
}
