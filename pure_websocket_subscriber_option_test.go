package appsync

import (
	"strings"
	"testing"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/sony/appsync-client-go/graphql"
)

var (
	realtimeEndpoint = "wss://example1234567890000.appsync-realtime-api.us-east-1.amazonaws.com/graphql"
	request          = graphql.PostRequest{}
	onReceive        = func(*graphql.Response) {}
	onConnectionLost = func(error) {}
)

func TestWithAPIKey(t *testing.T) {
	type args struct {
		host   string
		apiKey string
	}
	tests := []struct {
		name string
		args args
		want PureWebSocketSubscriberOption
	}{
		{
			name: "WithAPIKey Success",
			args: args{
				host:   "example1234567890000.appsync-api.us-east-1.amazonaws.com",
				apiKey: "apikey",
			},
		},
		{
			name: "WithAPIKey with sanitize Success",
			args: args{
				host:   "https://example1234567890000.appsync-api.us-east-1.amazonaws.com/graphql",
				apiKey: "apikey",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewPureWebSocketSubscriber(realtimeEndpoint, request, onReceive, onConnectionLost)
			if len(s.header) != 0 {
				t.Fatal(s.header)
			}
			opt := WithAPIKey(tt.args.host, tt.args.apiKey)
			opt(s)
			if len(s.header) != 2 {
				t.Fatal(s.header)
			}
			if s.header.Get("host") != sanitize(tt.args.host) {
				t.Errorf("got: %s, want: %s", s.header.Get("host"), tt.args.host)
			}
			if s.header.Get("x-api-key") != tt.args.apiKey {
				t.Errorf("got: %s, want: %s", s.header.Get("x-api-key"), tt.args.apiKey)
			}
		})
	}
}

func TestWithOIDC(t *testing.T) {
	type args struct {
		host string
		jwt  string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "WithOIDC Success",
			args: args{
				host: "example1234567890000.appsync-api.us-east-1.amazonaws.com",
				jwt:  "jwt",
			},
		},
		{
			name: "WithOIDC with sanitize Success",
			args: args{
				host: "https://example1234567890000.appsync-api.us-east-1.amazonaws.com/graphql",
				jwt:  "jwt",
			},
		},
		{
			name: "WithOIDC with trim Bearer Success",
			args: args{
				host: "https://example1234567890000.appsync-api.us-east-1.amazonaws.com/graphql",
				jwt:  "Bearer jwt",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewPureWebSocketSubscriber(realtimeEndpoint, request, onReceive, onConnectionLost)
			if len(s.header) != 0 {
				t.Fatal(s.header)
			}
			opt := WithOIDC(tt.args.host, tt.args.jwt)
			opt(s)
			if len(s.header) != 2 {
				t.Fatal(s.header)
			}
			if s.header.Get("host") != sanitize(tt.args.host) {
				t.Errorf("got: %s, want: %s", s.header.Get("host"), tt.args.host)
			}
			if s.header.Get("authorization") != strings.TrimPrefix(tt.args.jwt, "Bearer ") {
				t.Errorf("got: %s, want: %s", s.header.Get("authorization"), strings.TrimPrefix(tt.args.jwt, "Bearer "))
			}
		})
	}
}

func TestWithIAM(t *testing.T) {
	type args struct {
		signer *v4.Signer
		region string
		host   string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "WithIAM Success",
			args: args{
				signer: &v4.Signer{},
				region: "region",
				host:   "host",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewPureWebSocketSubscriber(realtimeEndpoint, request, onReceive, onConnectionLost)
			if s.iamAuth != nil {
				t.Fatal(s.iamAuth)
			}
			opt := WithIAM(tt.args.signer, tt.args.region, tt.args.host)
			opt(s)
			if s.iamAuth.signer != tt.args.signer {
				t.Errorf("got: %v, want: %v", s.iamAuth.signer, tt.args.signer)
			}
			if s.iamAuth.region != tt.args.region {
				t.Errorf("got: %s, want: %s", s.iamAuth.region, tt.args.region)
			}
			if s.iamAuth.host != tt.args.host {
				t.Errorf("got: %s, want: %s", s.iamAuth.host, s.iamAuth.host)
			}
		})
	}
}
