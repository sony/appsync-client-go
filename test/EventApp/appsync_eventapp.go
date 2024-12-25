package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	appsync "github.com/sony/appsync-client-go"
	"github.com/sony/appsync-client-go/graphql"
)

type channel struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func main() {
	handle := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})

	slog.SetDefault(slog.New(handle))

	var (
		region = flag.String("region", "", "AppSync API region")
		url    = flag.String("url", "", "AppSync API URL")
	)
	flag.Parse()

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("unable to load default config", "error", err)
		os.Exit(1)
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		slog.Error("unable to retrieve credentials", "error", err)
		os.Exit(1)
	}
	signer := v4.NewSigner()
	opt := appsync.WithIAMAuthorizationV2(signer, creds, *region, *url)
	sOpt := appsync.WithIAMV2(signer, creds, *region, *url)

	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(*url)), opt)

	name := "name"
	data := `{\"key\": \"value\"}`

	ch := make(chan *graphql.Response)
	defer close(ch)

	s := subscribe(*url, sOpt, name, ch)

	slog.Info("start subscribe")
	if err := s.Start(); err != nil {
		slog.Error("unable to start subscriber", "error", err)
		os.Exit(1)
	}
	slog.Info("publish", "response", publish(client, channel{Name: name, Data: data}))
	slog.Info("subscribe", "response", <-ch)
	slog.Info("stop subscribe")
	defer s.Stop()
}

func publish(c *appsync.Client, ch channel) *graphql.Response {
	mutation := fmt.Sprintf(`
mutation {
	publish(name: "%s", data: "%s") {
        name
		data
    }
}`, ch.Name, ch.Data)
	res, err := c.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		slog.Error("unable to create postRequest", "error", err)
		os.Exit(1)
	}
	return res
}

func subscribe(url string, opt appsync.PureWebSocketSubscriberOption, name string, ch chan *graphql.Response) *appsync.PureWebSocketSubscriber {
	subscription := fmt.Sprintf(`
subscription {
	subscribe(name: "%s"){
		name
		data
	}
}`, name)
	subreq := graphql.PostRequest{
		Query: subscription,
	}
	realtime := strings.Replace(strings.Replace(url, "https", "wss", 1), "appsync-api", "appsync-realtime-api", 1)
	return appsync.NewPureWebSocketSubscriber(realtime, subreq,
		func(r *graphql.Response) { ch <- r },
		func(err error) {
			slog.Error("unable to create new prue websocket subscriber", "error", err)
			os.Exit(1)
		},
		opt,
	)
}
