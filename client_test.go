package appsync

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mec07/appsync-client-go/graphql"
)

var (
	testData     = json.RawMessage(`{"test": "data"}`)
	testResponse = graphql.Response{
		Data:       testData,
		Errors:     nil,
		Extensions: nil,
	}
	testDeviceID = "deviceID"
)

func TestNewClient(t *testing.T) {
	client := NewClient(&testGraphQLAPI{})
	if client == nil {
		t.Fatal("NewClient returns nil")
	}
}

func TestQuery(t *testing.T) {
	api := &testGraphQLAPI{}
	client := NewClient(api, WithSubscriberID(testDeviceID))
	res, err := client.Post(graphql.PostRequest{
		Query: "query ",
	})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if res == nil {
		t.Fatal("Post returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	raw, ok := res.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if res.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *res.Errors)
	}
	if res.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *res.Extensions)
	}
}

func TestQueryAsync(t *testing.T) {
	ch := make(chan ret, 1)

	api := &testGraphQLAPI{}
	client := NewClient(api, WithSubscriberID(testDeviceID))
	cancel, err := client.PostAsync(graphql.PostRequest{Query: "query "}, func(r *graphql.Response, err error) { ch <- ret{r, err} })
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if cancel == nil {
		t.Fatal("PostAsync returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	ret := <-ch
	raw, ok := ret.response.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if ret.response.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *ret.response.Errors)
	}
	if ret.response.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *ret.response.Extensions)
	}
}

func TestMutation(t *testing.T) {
	api := &testGraphQLAPI{}
	client := NewClient(api, WithSubscriberID(testDeviceID))
	res, err := client.Post(graphql.PostRequest{
		Query: "mutation ",
	})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if res == nil {
		t.Fatal("Post returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	raw, ok := res.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if res.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *res.Errors)
	}
	if res.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *res.Extensions)
	}
}

func TestMutationAsync(t *testing.T) {
	ch := make(chan ret, 1)

	api := &testGraphQLAPI{}
	client := NewClient(api, WithSubscriberID(testDeviceID))
	cancel, err := client.PostAsync(graphql.PostRequest{Query: "mutation "}, func(r *graphql.Response, err error) { ch <- ret{r, err} })
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if cancel == nil {
		t.Fatal("PostAsync returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	ret := <-ch
	raw, ok := ret.response.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if ret.response.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *ret.response.Errors)
	}
	if ret.response.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *ret.response.Extensions)
	}
}

func TestSubscription(t *testing.T) {
	api := &testGraphQLAPI{}
	client := NewClient(api)
	if client == nil {
		t.Fatal("NewClient returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	client = NewClient(api, WithSubscriberID(testDeviceID))
	res, err := client.Post(graphql.PostRequest{
		Query: "subscription ",
	})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if res == nil {
		t.Fatal("Post returns nil")
	}

	if _, ok := api.GetPostedHeader()["X-Amz-Subscriber-Id"]; !ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	raw, ok := res.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", res.Data)
	}
	if res.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *res.Errors)
	}
	if res.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *res.Extensions)
	}
}

func TestSubscriptionAsync(t *testing.T) {
	ch := make(chan ret, 1)

	api := &testGraphQLAPI{}
	client := NewClient(api)
	if client == nil {
		t.Fatal("NewClient returns nil")
	}
	if _, ok := api.GetPostedHeader()["x-amz-subscriber-id"]; ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	client = NewClient(api, WithSubscriberID(testDeviceID))
	cancel, err := client.PostAsync(graphql.PostRequest{Query: "subscription "}, func(r *graphql.Response, err error) { ch <- ret{r, err} })
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if cancel == nil {
		t.Fatal("Post returns nil")
	}

	if _, ok := api.GetPostedHeader()["X-Amz-Subscriber-Id"]; !ok {
		t.Fatalf("GetPostedHeader error: %+v", api)
	}

	ret := <-ch
	raw, ok := ret.response.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if !bytes.Equal(raw, testData) {
		t.Fatalf("Data error: %+v", ret.response.Data)
	}
	if ret.response.Errors != testResponse.Errors {
		t.Fatalf("Errors error: %+v", *ret.response.Errors)
	}
	if ret.response.Extensions != testResponse.Extensions {
		t.Fatalf("Extensions error: %+v", *ret.response.Extensions)
	}
}
