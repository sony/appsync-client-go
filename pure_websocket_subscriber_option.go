package appsync

import (
	"net/url"
	"strings"

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

// WithIAMV1 returns a PureWebSocketSubscriberOption configured with the sdk v1 signature version 4 signer, the region and the url for the AWS AppSync GraphQL endpoint.
func WithIAMV1(signer *sdkv1_v4.Signer, region, url string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.sigv4 = &_signer{signer, region, url, nil}
	}
}

// WithIAMV2 returns a PureWebSocketSubscriberOption configured with the sdk v2 signature version 4 signer, the credentials, the region and the url for the AWS AppSync GraphQL endpoint.
func WithIAMV2(signer *sdkv2_v4.Signer, creds aws.Credentials, region, url string) PureWebSocketSubscriberOption {
	return func(p *PureWebSocketSubscriber) {
		p.sigv4 = &_signer{signer, region, url, &creds}
	}
}
