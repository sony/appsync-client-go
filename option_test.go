package appsync

import (
	"testing"
)

var (
	testSubscriberID = "subscriberID"
)

func TestSubscriberID(t *testing.T) {
	client := NewClient(&testGraphQLAPI{})
	if len(client.subscriberID) != 0 {
		t.Fatal(client.subscriberID)
	}

	opt := WithSubscriberID(testSubscriberID)
	opt(client)
	if client.subscriberID != testSubscriberID {
		t.Fatal(client.subscriberID)
	}
}
