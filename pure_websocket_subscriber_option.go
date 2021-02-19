package appsync

import (
	"net/url"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// PureWebSocketSubscriberOption represents options for an PureWebSocketSubscriber.
type PureWebSocketSubscriberOption func(*PureWebSocketSubscriber)

func sanitize(host string) string {
	if u, err := url.ParseRequestURI(host); err == nil {
		return u.Host
	}
	return host
}

//WithAPIKey returns a PureWebSocketSubscriberOption configured with the host for the AWS AppSync GraphQL endpoint and API key
func WithAPIKey(host, apiKey string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.header.Set("host", sanitize(host))
		p.header.Set("X-Api-Key", apiKey)
	}
}

//WithOIDC returns a PureWebSocketSubscriberOption configured with the host for the AWS AppSync GraphQL endpoint and JWT Access Token.
func WithOIDC(host, jwt string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.header.Set("host", sanitize(host))
		p.header.Set("Authorization", jwt)
	}
}

// WithIAM returns a PureWebSocketSubscriberOption configured with the signature version 4 signer, the region and the host for the AWS AppSync GraphQL endpoint.
func WithIAM(signer *v4.Signer, region, host string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.iamAuth = &struct {
			signer *v4.Signer
			region string
			host   string
		}{signer, region, host}
	}
}
