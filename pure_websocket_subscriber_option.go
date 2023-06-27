package appsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdkv2_v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	sdkv1_v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// PureWebSocketSubscriberOption represents options for an PureWebSocketSubscriber.
type PureWebSocketSubscriberOption func(*PureWebSocketSubscriber)

func sanitize(host string) string {
	if u, err := url.ParseRequestURI(host); err == nil {
		return u.Host
	}
	return host
}

// WithAPIKey returns a PureWebSocketSubscriberOption configured with the host for the AWS AppSync GraphQL endpoint and API key
func WithAPIKey(host, apiKey string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.header.Set("host", sanitize(host))
		p.header.Set("X-Api-Key", apiKey)
	}
}

// WithOIDC returns a PureWebSocketSubscriberOption configured with the host for the AWS AppSync GraphQL endpoint and JWT Access Token.
func WithOIDC(host, jwt string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.header.Set("host", sanitize(host))
		p.header.Set("Authorization", strings.TrimPrefix(jwt, "Bearer "))
	}
}

// WithIAM returns a PureWebSocketSubscriberOption configured with the signature version 4 signer, the region and the host for the AWS AppSync GraphQL endpoint.
//
// Deprecated: for backward compatibility.
func WithIAM(signer *sdkv1_v4.Signer, region, host string) PureWebSocketSubscriberOption {
	return WithIAMV1(signer, region, host)
}

// WithIAMV1 returns a PureWebSocketSubscriberOption configured with the signature version 4 signer, the region and the host for the AWS AppSync GraphQL endpoint.
func WithIAMV1(signer *sdkv1_v4.Signer, region, url string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.iamAuth = &struct {
			signer pureWebSocketV4Signer
			region string
			url    string
		}{&pwsV1Signer{signer}, region, url}
	}
}

type pwsV1Signer struct {
	signer *sdkv1_v4.Signer
}

func (v1 pwsV1Signer) Sign(region, url string, payload []byte) (map[string]string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	req.Header.Add("accept", "application/json, text/javascript")
	req.Header.Add("content-encoding", "amz-1.0")
	req.Header.Add("content-type", "application/json; charset=UTF-8")

	_, err = v1.signer.Sign(req, bytes.NewReader(payload), "appsync", region, time.Now())
	if err != nil {
		log.Println(err)
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
}

// WithIAMV2 returns a PureWebSocketSubscriberOption configured with the signature version 4 signer, the region and the host for the AWS AppSync GraphQL endpoint.
func WithIAMV2(signer *sdkv2_v4.Signer, ctx context.Context, creds aws.Credentials, region, url string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.iamAuth = &struct {
			signer pureWebSocketV4Signer
			region string
			url    string
		}{&pwsV2Signer{signer, ctx, creds}, region, url}
	}
}

type pwsV2Signer struct {
	signer *sdkv2_v4.Signer
	ctx    context.Context
	creds  aws.Credentials
}

func (v2 pwsV2Signer) Sign(region, url string, payload []byte) (map[string]string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	req.Header.Add("accept", "application/json, text/javascript")
	req.Header.Add("content-encoding", "amz-1.0")
	req.Header.Add("content-type", "application/json; charset=UTF-8")

	hash := sha256.Sum256(payload)
	if err := v2.signer.SignHTTP(v2.ctx, v2.creds, req, hex.EncodeToString(hash[:]), "appsync", region, time.Now()); err != nil {
		log.Println(err)
		return nil, err
	}

	return map[string]string{
		"accept":               req.Header.Get("accept"),
		"content-encoding":     req.Header.Get("content-encoding"),
		"content-length":       strconv.FormatInt(req.ContentLength, 10),
		"content-type":         req.Header.Get("content-type"),
		"host":                 req.Host,
		"x-amz-date":           req.Header.Get("x-amz-date"),
		"X-Amz-Security-Token": req.Header.Get("X-Amz-Security-Token"),
		"Authorization":        req.Header.Get("Authorization"),
	}, nil
}
