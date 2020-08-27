package appsync

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mec07/appsync-client-go/graphql"
)

// Client is the AppSync GraphQL API client
type Client struct {
	sync.RWMutex
	graphQLAPI   GraphQLClient
	subscriberID string
	iamAuth      *iamAuth
	tokensInfo   *TokensInfo
	useIDToken   bool
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
	iamAuth := c.getIAMAuth()
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	req, err := http.NewRequest("POST", iamAuth.host, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = iamAuth.signer.Sign(req, bytes.NewReader(jsonBytes), "appsync", iamAuth.region, time.Now())
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return req.Header, nil
}

// Post is a synchronous AppSync GraphQL POST request.
func (c *Client) Post(request graphql.PostRequest) (*graphql.Response, error) {
	defer c.sleepIfNeeded(request)

	header, err := c.createHeader(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return c.graphQLAPI.Post(header, request)
}

// PostAsync is an asynchronous AppSync GraphQL POST request.
func (c *Client) PostAsync(request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	header, err := c.createHeader(request)
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

// UpdateTokens lets the user update the tokens. This is necessary because the
// refresh token will eventually expire.
func (c *Client) UpdateTokens(tokensInfo TokensInfo) {
	c.Lock()
	defer c.Unlock()

	c.tokensInfo = &tokensInfo
}

func (c *Client) createHeader(request graphql.PostRequest) (http.Header, error) {
	header := http.Header{}
	subscriberID := c.getSubscriberID()
	if request.IsSubscription() && len(subscriberID) > 0 {
		header.Set("x-amz-subscriber-id", subscriberID)
	}

	if c.iamAuth != nil {
		h, err := c.signRequest(request)
		if err != nil {
			log.Println(err)
			return header, err
		}
		for k, v := range h {
			header[k] = v
		}
	}

	if c.tokensInfo != nil {
		tokensInfo := c.getTokensInfo()
		// TODO: check for expiry time and refresh tokens
		authVal := tokensInfo.AccessToken
		if c.shouldUseIDToken() {
			authVal = tokensInfo.IDToken
		}
		header.Set("Authorization", authVal)
	}

	return header, nil
}

func (c *Client) getSubscriberID() string {
	c.RLock()
	defer c.RUnlock()

	return c.subscriberID
}

func (c *Client) getIAMAuth() iamAuth {
	c.RLock()
	defer c.RUnlock()

	if c.iamAuth == nil {
		return iamAuth{}
	}
	return *c.iamAuth
}

func (c *Client) getTokensInfo() TokensInfo {
	c.RLock()
	defer c.RUnlock()

	if c.tokensInfo == nil {
		return TokensInfo{}
	}
	return *c.tokensInfo
}

func (c *Client) shouldUseIDToken() bool {
	c.RLock()
	defer c.RUnlock()

	return c.useIDToken
}
