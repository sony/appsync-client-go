package graphql

import (
	"testing"
)

func TestIsQuery(t *testing.T) {
	request := PostRequest{}
	if request.IsQuery() {
		t.Fatalf("%+v", request)
	}
	request.Query = "query() foo bar baz"
	if !request.IsQuery() {
		t.Fatalf("%+v", request)
	}
}

func TestIsMutation(t *testing.T) {
	request := PostRequest{}
	if request.IsMutation() {
		t.Fatalf("%+v", request)
	}
	request.Query = "mutation() foo bar baz"
	if !request.IsMutation() {
		t.Fatalf("%+v", request)
	}
}

func TestIsSubscription(t *testing.T) {
	request := PostRequest{}
	if request.IsSubscription() {
		t.Fatalf("%+v", request)
	}
	request.Query = "subscription() foo bar baz"
	if !request.IsSubscription() {
		t.Fatalf("%+v", request)
	}
}
