package appsync

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/appsync-client-go/graphql"
)

// Client is the AppSync GraphQL API client
type Client struct {
	graphQLAPI   GraphQLClient
	subscriberID string
	signer       sigv4
}

// NewClient returns a Client instance.
func NewClient(graphql GraphQLClient, opts ...ClientOption) *Client {
	c := &Client{graphQLAPI: graphql}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) sleepIfNeeded(request graphql.PostRequest) {
	if request.IsSubscription() {
		// Here be dragons.
		time.Sleep(2 * time.Second)
	}
}

func (c *Client) setupHeaders(request graphql.PostRequest) (http.Header, error) {
	header := http.Header{}
	if request.IsSubscription() && len(c.subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", c.subscriberID)
	}

	if c.signer == nil {
		return header, nil
	}

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		slog.Error("unable to marshal request", "error", err, "request", request)
		return nil, err
	}
	h, err := c.signer.signHTTP(jsonBytes)
	if err != nil {
		slog.Error("unable to sign request", "error", err, "request", request)
		return nil, err
	}
	for k, vv := range h {
		for _, v := range vv {
			header.Add(k, v)
		}
	}
	return header, nil
}

// Post is a synchronous AppSync GraphQL POST request.
func (c *Client) Post(request graphql.PostRequest) (*graphql.Response, error) {
	defer c.sleepIfNeeded(request)
	header, err := c.setupHeaders(request)
	if err != nil {
		slog.Error("unable to setup headers", "error", err, "request", request)
		return nil, err
	}
	return c.graphQLAPI.Post(header, request)
}

// PostAsync is an asynchronous AppSync GraphQL POST request.
func (c *Client) PostAsync(request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	header, err := c.setupHeaders(request)
	if err != nil {
		slog.Error("unable to setup headers", "error", err, "request", request)
		return nil, err
	}
	cb := func(g *graphql.Response, err error) {
		c.sleepIfNeeded(request)
		callback(g, err)
	}
	return c.graphQLAPI.PostAsync(header, request, cb)
}
