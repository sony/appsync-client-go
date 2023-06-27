package appsync

import (
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
func WithIAMAuthorization(signer sdkv1_v4.Signer, region, host string) ClientOption {
	return func(c *Client) {
		c.iamAuthV1 = &struct {
			signer sdkv1_v4.Signer
			region string
			host   string
		}{signer, region, host}
	}
}

// WithIAMAuthorizationV2 returns a ClientOption configured with the given sdk v2 signature version 4 signer.
func WithIAMAuthorizationV2(signer sdkv2_v4.Signer, region, host string) ClientOption {
	return func(c *Client) {
		c.iamAuthV2 = &struct {
			signer sdkv2_v4.Signer
			region string
			host   string
		}{signer, region, host}
	}
}
