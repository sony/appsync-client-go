package appsync_test

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mec07/appsync-client-go/internal/appsynctest"

	appsync "github.com/mec07/appsync-client-go"
	"github.com/mec07/appsync-client-go/graphql"
	"github.com/mec07/awstokens"
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
		log.Fatal(err)
	}

	data := new(string)
	if err := response.DataAs(data); err != nil {
		log.Fatalln(err, response)
	}
	fmt.Println(*data)

	// Output:
	// Hello, AppSync!
}

func ExampleClient_Post_query_with_tokens() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	query := `query Message() { message }`

	validToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		ExpiresAt: time.Now().Add(time.Hour).UTC().Unix(),
	}).SignedString([]byte("secret signing key"))
	if err != nil {
		log.Fatal(err)
	}
	config := awstokens.Config{
		AccessToken:  validToken,
		IDToken:      "id_token",
		RefreshToken: "refresh_token",
	}
	auth, err := awstokens.NewAuth(config)
	if err != nil {
		log.Fatal(err)
	}
	client := appsync.NewClient(
		appsync.NewGraphQLClient(graphql.NewClient(server.URL)),
		appsync.WithTokenAuthorization(auth),
	)
	response, err := client.Post(graphql.PostRequest{
		Query: query,
	})
	if err != nil {
		log.Fatal(err)
	}

	data := new(string)
	if err := response.DataAs(data); err != nil {
		log.Fatalln(err, response)
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
		log.Fatal(err)
	}

	data := new(string)
	if err := response.DataAs(data); err != nil {
		log.Fatalln(err, response)
	}
	fmt.Println(*data)

	// Output:
	// Hi, AppSync!
}

func ExampleClient_Post_subscription() {
	server := appsynctest.NewAppSyncEchoServer()
	defer server.Close()

	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(server.URL)))
	subscription := `subscription SubscribeToEcho() { subscribeToEcho }`
	response, err := client.Post(graphql.PostRequest{
		Query: subscription,
	})
	if err != nil {
		log.Fatal(err)
	}

	ext, err := appsync.NewExtensions(response)
	if err != nil {
		log.Fatalln(err)
	}

	ch := make(chan *graphql.Response)
	subscriber := appsync.NewSubscriber(*ext,
		func(r *graphql.Response) { ch <- r },
		func(err error) { log.Println(err) },
	)

	if err := subscriber.Start(); err != nil {
		log.Fatalln(err)
	}
	defer subscriber.Stop()

	mutation := `mutation Echo($message: String!) { echo(message: $message) }`
	variables := json.RawMessage(fmt.Sprintf(`{ "message": "%s" }`, "Hi, AppSync!"))
	_, err = client.Post(graphql.PostRequest{
		Query:     mutation,
		Variables: &variables,
	})
	if err != nil {
		log.Fatal(err)
	}

	response = <-ch
	data := new(string)
	if err := response.DataAs(data); err != nil {
		log.Fatalln(err, response)
	}
	fmt.Println(*data)

	// Output:
	// Hi, AppSync!
}
