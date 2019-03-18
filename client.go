package appsync

import (
	"context"
	"net/http"

	"github.com/sony/appsync-client-go/graphql"
)

// Client is the AppSync GraphQL API client
type Client struct {
	graphQLAPI   GraphQLClient
	subscriberID string
}

// NewClient returns a Client instance.
func NewClient(graphql GraphQLClient, opts ...ClientOption) *Client {
	c := &Client{graphQLAPI: graphql}
	for _, opt := range opts {
		opt.Apply(c)
	}
	return c
}

//Post is a synchronous AppSync GraphQL POST request.
func (c *Client) Post(request graphql.PostRequest) (*graphql.Response, error) {
	header := http.Header{}
	if request.IsSubscription() && len(c.subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", c.subscriberID)
	}
	return c.graphQLAPI.Post(header, request)
}

//PostAsync is an asynchronous AppSync GraphQL POST request.
func (c *Client) PostAsync(request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	header := http.Header{}
	if request.IsSubscription() && len(c.subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", c.subscriberID)
	}
	return c.graphQLAPI.PostAsync(header, request, callback)
}
