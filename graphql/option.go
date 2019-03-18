package graphql

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

// ClientOption represents options for a generic GraphQL client.
type ClientOption interface {
	Apply(*Client)
}

//WithAPIKey returns a ClientOption configured with the given API key
func WithAPIKey(apiKey string) ClientOption {
	return withAPIKey{apiKey}
}

type withAPIKey struct {
	apiKey string
}

func (w withAPIKey) Apply(c *Client) {
	c.header.Set("X-Api-Key", w.apiKey)
}

//WithCredential returns a ClientOption configured with the given credential
func WithCredential(credential string) ClientOption {
	return withCredential{credential}
}

type withCredential struct {
	credential string
}

func (w withCredential) Apply(c *Client) {
	c.header.Set("Authorization", w.credential)
}

//WithHTTPProxy returns a ClientOption configured with the given http proxy
func WithHTTPProxy(proxy string) ClientOption {
	return withHTTPProxy{proxy}
}

type withHTTPProxy struct {
	proxy string
}

func (w withHTTPProxy) Apply(c *Client) {
	proxy, err := url.Parse(w.proxy)
	if err != nil {
		log.Println(err)
		return
	}
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = http.ProxyURL(proxy)
	}
}

//WithTimeout returns a ClientOption configured with the given timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return withTimeout{timeout}
}

type withTimeout struct {
	timeout time.Duration
}

func (w withTimeout) Apply(c *Client) {
	c.timeout = w.timeout
}

//WithMaxElapsedTime returns a ClientOption configured with the given maxElapsedTime
func WithMaxElapsedTime(maxElapsedTime time.Duration) ClientOption {
	return withMaxElapsedTime{maxElapsedTime}
}

type withMaxElapsedTime struct {
	maxElapsedTime time.Duration
}

func (w withMaxElapsedTime) Apply(c *Client) {
	c.maxElapsedTime = w.maxElapsedTime
}
