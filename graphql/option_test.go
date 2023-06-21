package graphql

import (
	"net/http"
	"testing"
	"time"
)

var (
	testEndpoint       = "dummy"
	testAPIKey         = "apiKey"
	testCredential     = "credential"
	testProxy          = "proxy"
	testTimeout        = 1 * time.Second
	testMaxElapsedTime = 2 * time.Second
)

func TestWithAPIKey(t *testing.T) {
	client := NewClient(testEndpoint)
	opt := WithAPIKey(testAPIKey)
	if _, ok := client.header["X-Api-Key"]; ok {
		t.Fatal(client.header)
	}

	opt(client)
	if _, ok := client.header["X-Api-Key"]; !ok {
		t.Fatal(client.header)
	}
	if client.header.Get("X-Api-Key") != testAPIKey {
		t.Fatal(client.header)
	}
}

func TestWithCredential(t *testing.T) {
	client := NewClient(testEndpoint)
	opt := WithCredential(testCredential)
	if _, ok := client.header["Authorization"]; ok {
		t.Fatal(client.header)
	}

	opt(client)
	if _, ok := client.header["Authorization"]; !ok {
		t.Fatal(client.header)
	}
	if client.header.Get("Authorization") != testCredential {
		t.Fatal(client.header)
	}
}

func TestWithHTTPProxy(t *testing.T) {
	client := NewClient(testEndpoint)
	opt := WithHTTPProxy(testProxy)

	pre, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client.http.Transport is invalid")
	}
	req, err := http.NewRequest("GET", "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	url, err := pre.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if url != nil {
		t.Fatal(url)
	}

	opt(client)

	post, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client.http.Transport is invalid")
	}
	url, err = post.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if url == nil {
		t.Fatal(url)
	}
	if url.String() != testProxy {
		t.Fatal(url.String())
	}
}

func TestWithTimeout(t *testing.T) {
	client := NewClient(testEndpoint)
	opt := WithTimeout(testTimeout)
	if client.timeout != 30*time.Second {
		t.Fatal(client.timeout)
	}

	opt(client)
	if client.timeout != testTimeout {
		t.Fatal(client.timeout)
	}
}

func TestWithMaxElapsedTime(t *testing.T) {
	client := NewClient(testEndpoint)
	opt := WithMaxElapsedTime(testMaxElapsedTime)
	if client.maxElapsedTime != 20*time.Second {
		t.Fatal(client.maxElapsedTime)
	}

	opt(client)
	if client.maxElapsedTime != testMaxElapsedTime {
		t.Fatal(client.maxElapsedTime)
	}
}

func TestWithHTTPHeader(t *testing.T) {
	client := NewClient(testEndpoint)

	h := http.Header{}
	h.Add("custom", "first")
	h.Add("custom", "second")

	opt := WithHTTPHeader(h)
	if _, ok := client.header[http.CanonicalHeaderKey("custom")]; ok {
		t.Fatal(client.header)
	}

	opt(client)
	if _, ok := client.header[http.CanonicalHeaderKey("custom")]; !ok {
		t.Fatal(client.header)
	}
	if client.header.Get("custom") != "first" {
		t.Fatal(client.header)
	}
	if client.header[http.CanonicalHeaderKey("custom")][0] != "first" {
		t.Fatal(client.header)
	}
	if client.header[http.CanonicalHeaderKey("custom")][1] != "second" {
		t.Fatal(client.header)
	}

	opt = WithHTTPHeader(http.Header{"custom": []string{"third"}})
	opt(client)
	if _, ok := client.header[http.CanonicalHeaderKey("custom")]; !ok {
		t.Fatal(client.header)
	}
	if client.header.Get("custom") != "first" {
		t.Fatal(client.header)
	}
	if client.header[http.CanonicalHeaderKey("custom")][0] != "first" {
		t.Fatal(client.header)
	}
	if client.header[http.CanonicalHeaderKey("custom")][1] != "second" {
		t.Fatal(client.header)
	}
	if client.header[http.CanonicalHeaderKey("custom")][2] != "third" {
		t.Fatal(client.header)
	}

}
