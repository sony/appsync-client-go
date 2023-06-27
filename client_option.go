package appsync

import (
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
func WithIAMAuthorizationV1(signer *sdkv1_v4.Signer, region, url string) ClientOption {
	return func(c *Client) {
		c.signer = &_signer{signer, region, url, nil}
	}
}

// WithIAMAuthorizationV2 returns a ClientOption configured with the given sdk v2 signature version 4 signer.
func WithIAMAuthorizationV2(signer *sdkv2_v4.Signer, creds *aws.Credentials, region, url string) ClientOption {
	return func(c *Client) {
		c.signer = &_signer{signer, region, url, creds}
	}
}
