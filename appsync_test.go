package appsync

import (
	"context"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/sony/appsync-client-go/graphql"
)

type ret struct {
	response *graphql.Response
	err      error
}

type testGraphQLAPI struct {
	header  http.Header
	request graphql.PostRequest
}

func (t *testGraphQLAPI) Post(header http.Header, request graphql.PostRequest) (*graphql.Response, error) {
	t.header = header
	t.request = request
	return &testResponse, nil
}

func (t *testGraphQLAPI) PostAsync(header http.Header, request graphql.PostRequest, callback func(*graphql.Response, error)) (context.CancelFunc, error) {
	t.header = header
	t.request = request
	go func() {
		callback(&testResponse, nil)
	}()
	return func() {}, nil
}

func (t *testGraphQLAPI) GetHTTPClient() *http.Client {
	return http.DefaultClient
}

func (t *testGraphQLAPI) GetPostedHeader() http.Header {
	return t.header
}

func (t *testGraphQLAPI) GetPostedPostRequest() graphql.PostRequest {
	return t.request
}

func TestMain(m *testing.M) {
	log.SetFlags(log.Lshortfile)
	os.Exit(m.Run())
}
