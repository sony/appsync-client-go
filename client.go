package appsync

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	sdkv2_v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	sdkv1_v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/sony/appsync-client-go/graphql"
)

// Client is the AppSync GraphQL API client
type Client struct {
	graphQLAPI   GraphQLClient
	subscriberID string
	iamAuthV1    *struct {
		signer sdkv1_v4.Signer
		region string
		host   string
	}
	iamAuthV2 *struct {
		signer sdkv2_v4.Signer
		region string
		host   string
	}
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

func (c *Client) signRequest(request graphql.PostRequest) (http.Header, error) {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	req, err := http.NewRequest("POST", c.iamAuthV1.host, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = c.iamAuthV1.signer.Sign(req, bytes.NewReader(jsonBytes), "appsync", c.iamAuthV1.region, time.Now())
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return req.Header, nil
}

// Post is a synchronous AppSync GraphQL POST request.
func (c *Client) Post(request graphql.PostRequest) (*graphql.Response, error) {
	defer c.sleepIfNeeded(request)
	header := http.Header{}
	if request.IsSubscription() && len(c.subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", c.subscriberID)
	}

	if c.iamAuthV1 != nil {
		h, err := c.signRequest(request)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		for k, v := range h {
			header[k] = v
		}
	}
	return c.graphQLAPI.Post(header, request)
}

// PostAsync is an asynchronous AppSync GraphQL POST request.
func (c *Client) PostAsync(request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	header := http.Header{}
	if request.IsSubscription() && len(c.subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", c.subscriberID)
	}
	cb := func(g *graphql.Response, err error) {
		c.sleepIfNeeded(request)
		callback(g, err)
	}
	if c.iamAuthV1 != nil {
		h, err := c.signRequest(request)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		for k, v := range h {
			header[k] = v
		}
	}
	return c.graphQLAPI.PostAsync(header, request, cb)
}
