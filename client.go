package appsync

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/sony/appsync-client-go/graphql"
)

type v4Signer interface {
	Sign(payload []byte) (http.Header, error)
}

// Client is the AppSync GraphQL API client
type Client struct {
	graphQLAPI   GraphQLClient
	subscriberID string
	v4Signer     v4Signer
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

	if c.v4Signer == nil {
		return header, nil
	}

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	h, err := c.v4Signer.Sign(jsonBytes)
	if err != nil {
		log.Println(err)
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
		log.Println(err)
		return nil, err
	}
	return c.graphQLAPI.Post(header, request)
}

// PostAsync is an asynchronous AppSync GraphQL POST request.
func (c *Client) PostAsync(request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	header, err := c.setupHeaders(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	cb := func(g *graphql.Response, err error) {
		c.sleepIfNeeded(request)
		callback(g, err)
	}
	return c.graphQLAPI.PostAsync(header, request, cb)
}
