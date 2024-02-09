package appsync_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/sony/appsync-client-go/internal/appsynctest"

	appsync "github.com/sony/appsync-client-go"
	"github.com/sony/appsync-client-go/graphql"
)

func ExampleClient_Post_query() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	query := `query Message() { message }`
	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(server.URL)))
	response, err := client.Post(graphql.PostRequest{
		Query: query,
	})
	if err != nil {
		slog.Error("unable to post query", "error", err)
		os.Exit(1)
	}

	data := new(string)
	if err := response.DataAs(data); err != nil {
		slog.Error("unable to process data", "error", err, "response", response)
		os.Exit(1)
	}
	fmt.Println(*data)

	// Output:
	// Hello, AppSync!
}

func ExampleClient_Post_mutation() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(server.URL)))
	mutation := `mutation Echo($message: String!) { echo(message: $message) }`
	variables := json.RawMessage(fmt.Sprintf(`{ "message": "%s"	}`, "Hi, AppSync!"))
	response, err := client.Post(graphql.PostRequest{
		Query:     mutation,
		Variables: &variables,
	})
	if err != nil {
		slog.Error("unable to post mutation", "error", err)
		os.Exit(1)
	}

	data := new(string)
	if err := response.DataAs(data); err != nil {
		slog.Error("unable to process data", "error", err, "response", response)
		os.Exit(1)
	}
	fmt.Println(*data)

	// Output:
	// Hi, AppSync!
}

func ExampleClient_mqtt_subscription() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(server.URL)))
	subscription := `subscription SubscribeToEcho() { subscribeToEcho }`
	response, err := client.Post(graphql.PostRequest{
		Query: subscription,
	})
	if err != nil {
		slog.Error("unable to post subscription", "error", err)
		os.Exit(1)
	}

	ext, err := appsync.NewExtensions(response)
	if err != nil {
		slog.Error("unable to process extensions", "error", err)
		os.Exit(1)
	}

	ch := make(chan *graphql.Response)
	subscriber := appsync.NewSubscriber(*ext,
		func(r *graphql.Response) { ch <- r },
		func(err error) {
			slog.Warn("unable to create new subscriber", "error", err)
		},
	)

	if err := subscriber.Start(); err != nil {
		slog.Error("unable to start subscriber", "error", err)
		os.Exit(1)
	}
	defer subscriber.Stop()

	mutation := `mutation Echo($message: String!) { echo(message: $message) }`
	variables := json.RawMessage(fmt.Sprintf(`{ "message": "%s" }`, "Hi, AppSync!"))
	_, err = client.Post(graphql.PostRequest{
		Query:     mutation,
		Variables: &variables,
	})
	if err != nil {
		slog.Error("unable to post mutation", "error", err)
		os.Exit(1)
	}

	response = <-ch
	data := new(string)
	if err := response.DataAs(data); err != nil {
		slog.Error("unable to process data", "error", err, "response", response)
		os.Exit(1)
	}
	fmt.Println(*data)

	// Output:
	// Hi, AppSync!
}

func ExampleClient_graphqlws_subscription() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(server.URL)))
	subscription := `subscription SubscribeToEcho() { subscribeToEcho }`

	ch := make(chan *graphql.Response)
	subscriber := appsync.NewPureWebSocketSubscriber(
		strings.Replace(server.URL, "http", "ws", 1),
		graphql.PostRequest{
			Query: subscription,
		},
		func(r *graphql.Response) { ch <- r },
		func(err error) {
			slog.Warn("unable to create new pure websocket subscriber  ", "error", err)
		},
	)

	if err := subscriber.Start(); err != nil {
		slog.Error("unable to start subscriber", "error", err)
		os.Exit(1)
	}
	defer subscriber.Stop()

	mutation := `mutation Echo($message: String!) { echo(message: $message) }`
	variables := json.RawMessage(fmt.Sprintf(`{ "message": "%s" }`, "Hi, AppSync!"))
	_, err := client.Post(graphql.PostRequest{
		Query:     mutation,
		Variables: &variables,
	})
	if err != nil {
		slog.Error("unable to post mutation", "error", err)
		os.Exit(1)
	}

	response := <-ch
	data := new(string)
	if err := response.DataAs(data); err != nil {
		slog.Error("unable to process data", "error", err, "response", response)
		os.Exit(1)
	}
	fmt.Println(*data)

	// Output:
	// Hi, AppSync!
}
