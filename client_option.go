package appsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdkv2_v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	sdkv1_v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// ClientOption represents options for an AppSync client.
type ClientOption func(*Client)

// WithSubscriberID returns a ClientOption configured with the given AppSync subscriber ID
func WithSubscriberID(subscriberID string) ClientOption {
	return func(c *Client) {
		c.subscriberID = subscriberID
	}
}

// WithIAMAuthorization returns a ClientOption configured with the given sdk v1 signature version 4 signer.
//
// Deprecated: for backward compatibility.
func WithIAMAuthorization(signer sdkv1_v4.Signer, region, host string) ClientOption {
	return WithIAMAuthorizationV1(&signer, region, host)
}

// WithIAMAuthorizationV1 returns a ClientOption configured with the given sdk v1 signature version 4 signer.
func WithIAMAuthorizationV1(signer *sdkv1_v4.Signer, region, host string) ClientOption {
	return func(c *Client) {
		c.v4Signer = &v1Signer{signer, region, host}
	}
}

type v1Signer struct {
	signer *sdkv1_v4.Signer
	region string
	host   string
}

func (v1 *v1Signer) Sign(body []byte) (http.Header, error) {
	req, err := http.NewRequest("POST", v1.host, bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = v1.signer.Sign(req, bytes.NewReader(body), "appsync", v1.region, time.Now())
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return req.Header, nil
}

// WithIAMAuthorizationV2 returns a ClientOption configured with the given sdk v2 signature version 4 signer.
func WithIAMAuthorizationV2(signer *sdkv2_v4.Signer, ctx context.Context, creds aws.Credentials, region, host string) ClientOption {
	return func(c *Client) {
		c.v4Signer = &v2Signer{signer, ctx, creds, region, host}
	}
}

type v2Signer struct {
	signer *sdkv2_v4.Signer
	ctx    context.Context
	creds  aws.Credentials
	region string
	host   string
}

func (v2 *v2Signer) Sign(body []byte) (http.Header, error) {
	req, err := http.NewRequest("POST", v2.host, bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	hash := sha256.Sum256(body)
	if err := v2.signer.SignHTTP(v2.ctx, v2.creds, req, hex.EncodeToString(hash[:]), "appsync", v2.region, time.Now()); err != nil {
		log.Println(err)
		return nil, err
	}

	return req.Header, nil
}
