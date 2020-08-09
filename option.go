package appsync

import v4 "github.com/aws/aws-sdk-go/aws/signer/v4"

// ClientOption represents options for an AppSync client.
type ClientOption func(*Client)

// WithSubscriberID returns a ClientOption configured with the given AppSync subscriber ID
func WithSubscriberID(subscriberID string) ClientOption {
	return func(c *Client) {
		c.subscriberID = subscriberID
	}
}

// WithIAMAuthorization returns a ClientOption configured with the given signature version 4 signer.
func WithIAMAuthorization(signer v4.Signer, region, host string) ClientOption {
	return func(c *Client) {
		c.iamAuth = &struct {
			signer v4.Signer
			region string
			host   string
		}{signer, region, host}
	}
}
