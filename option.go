package appsync

import v4 "github.com/aws/aws-sdk-go/aws/signer/v4"

// ClientOption represents options for an AppSync client.
type ClientOption func(*Client)

// iamAuth contains the information required for IAM authorisation
type iamAuth struct {
	signer v4.Signer
	region string
	host   string
}

// WithSubscriberID returns a ClientOption configured with the given AppSync subscriber ID
func WithSubscriberID(subscriberID string) ClientOption {
	return func(c *Client) {
		c.subscriberID = subscriberID
	}
}

// WithIAMAuthorization returns a ClientOption configured with the given signature version 4 signer.
func WithIAMAuthorization(signer v4.Signer, region, host string) ClientOption {
	return func(c *Client) {
		c.iamAuth = &iamAuth{
			signer: signer,
			region: region,
			host:   host,
		}
	}
}

// WithTokenAuthorization uses tokens to authorize the request. We recommend
// using github.com/mec07/awstokens.Auth to satisfy this interface.
func WithTokenAuthorization(auth AuthTokenGetter) ClientOption {
	return func(c *Client) {
		c.auth = auth
	}
}
