package appsync

import (
	"context"
	"net/http"

	"github.com/sony/appsync-client-go/graphql"
)

// GraphQLClient is the interface to access GraphQL server.
type GraphQLClient interface {
	Post(header http.Header, request graphql.PostRequest) (*graphql.Response, error)
	PostAsync(header http.Header, request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error)
}

type graphQLClient struct {
	client *graphql.Client
}

// NewGraphQLClient returns a GraphQLClient interface
func NewGraphQLClient(client *graphql.Client) GraphQLClient {
	return &graphQLClient{client}
}

func (d *graphQLClient) Post(header http.Header, request graphql.PostRequest) (*graphql.Response, error) {
	return d.client.Post(header, request)
}

func (d *graphQLClient) PostAsync(header http.Header, request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	return d.client.PostAsync(header, request, callback)
}
