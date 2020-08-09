package graphql

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

// ClientOption represents options for a generic GraphQL client.
type ClientOption func(*Client)

//WithAPIKey returns a ClientOption configured with the given API key
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.header.Set("X-Api-Key", apiKey)
	}
}

//WithCredential returns a ClientOption configured with the given credential
func WithCredential(credential string) ClientOption {
	return func(c *Client) {
		c.header.Set("Authorization", credential)
	}
}

//WithHTTPProxy returns a ClientOption configured with the given http proxy
func WithHTTPProxy(proxy string) ClientOption {
	return func(c *Client) {
		proxy, err := url.Parse(proxy)
		if err != nil {
			log.Println(err)
			return
		}
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			t.Proxy = http.ProxyURL(proxy)
		}
	}
}

//WithTimeout returns a ClientOption configured with the given timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

//WithMaxElapsedTime returns a ClientOption configured with the given maxElapsedTime
func WithMaxElapsedTime(maxElapsedTime time.Duration) ClientOption {
	return func(c *Client) {
		c.maxElapsedTime = maxElapsedTime
	}
}
