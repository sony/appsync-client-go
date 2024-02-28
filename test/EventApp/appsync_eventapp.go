package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	sdkv2_v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkv1_v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/k0kubun/pp"
	appsync "github.com/sony/appsync-client-go"
	"github.com/sony/appsync-client-go/graphql"
)

type event struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Where       *string `json:"where"`
	When        *string `json:"when"`
	Description *string `json:"description"`
}

type subscriber interface {
	Start() error
	Stop()
}

func main() {

	handle := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})

	slog.SetDefault(slog.New(handle))

	var (
		region     = flag.String("region", "", "AppSync API region")
		url        = flag.String("url", "", "AppSync API URL")
		protocol   = flag.String("protocol", "graphql-ws", "AppSync Subscription protocol(mqtt, graphql-ws)")
		sdkVersion = flag.String("sdkVersion", "v2", "AWS SDK Version(v1, v2)")
	)
	flag.Parse()

	opt := func(*appsync.Client) {}
	sOpt := func(*appsync.PureWebSocketSubscriber) {}
	switch *sdkVersion {
	case "v1":
		sess := session.Must(session.NewSession())
		signer := sdkv1_v4.NewSigner(sess.Config.Credentials)
		opt = appsync.WithIAMAuthorizationV1(signer, *region, *url)
		sOpt = appsync.WithIAMV1(signer, *region, *url)
	case "v2":
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
		signer := sdkv2_v4.NewSigner()
		opt = appsync.WithIAMAuthorizationV2(signer, creds, *region, *url)
		sOpt = appsync.WithIAMV2(signer, creds, *region, *url)
	}
	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(*url)), opt)

	slog.Info("mutation createEvent()")
	event := createEvent(client)

	slog.Info("subscription subscribeToEventComments()")
	ch := make(chan *graphql.Response)
	defer close(ch)
	var s subscriber
	switch *protocol {
	case "mqtt":
		s = mqttSubscribeToEventComments(client, event, ch)
	case "graphql-ws":
		s = wsSubscribeToEventComments(*url, sOpt, event, ch)
	default:
		slog.Error("unsupported protocol", "protocol", *protocol)
		os.Exit(1)
	}
	if err := s.Start(); err != nil {
		slog.Error("unable to start subscriber", "error", err)
		os.Exit(1)
	}
	defer s.Stop()

	slog.Info("mutation commentOnEvent()")
	commentOnEvent(client, event)
	msg, ok := <-ch
	if !ok {
		slog.Error("ch has been closed.")
		os.Exit(1)
	}
	slog.Info("comment received")
	_, _ = pp.Println(msg)

	slog.Info("mutation deleteEvent()")
	deleteEvent(client, event)
}

func createEvent(c *appsync.Client) *event {
	mutation := `
mutation {
	createEvent(name: "name", when: "when", where: "where", description: "description") {
        id
        name
		when
		where
		description
    }
}`
	res, err := c.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		slog.Error("unable to create postRequest", "error", err)
		os.Exit(1)
	}
	_, _ = pp.Println(res)

	ev := new(event)
	if err := res.DataAs(ev); err != nil {
		slog.Error("unable to process event", "error", err)
		os.Exit(1)
	}
	return ev
}

func mqttSubscribeToEventComments(c *appsync.Client, e *event, ch chan *graphql.Response) subscriber {
	subscription := fmt.Sprintf(`
subscription {
	subscribeToEventComments(eventId: "%s"){
		eventId
		commentId
		content
		createdAt
	}
}`, e.ID)
	subReq := graphql.PostRequest{
		Query: subscription,
	}
	res, err := c.Post(subReq)
	if err != nil {
		slog.Error("unable to subscribe", "error", err)
		os.Exit(1)
	}
	_, _ = pp.Println(res)

	ext, err := appsync.NewExtensions(res)
	if err != nil {
		slog.Error("unable to process extensions", "error", err)
		os.Exit(1)
	}

	return appsync.NewSubscriber(*ext,
		func(r *graphql.Response) { ch <- r },
		func(err error) {
			slog.Error("unable to create new subscriber", "error", err)
			os.Exit(1)
		})

}

func wsSubscribeToEventComments(url string, opt appsync.PureWebSocketSubscriberOption, e *event, ch chan *graphql.Response) subscriber {
	subscription := fmt.Sprintf(`
subscription {
	subscribeToEventComments(eventId: "%s"){
		eventId
		commentId
		content
		createdAt
	}
}`, e.ID)
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

func commentOnEvent(c *appsync.Client, e *event) {
	mutation := fmt.Sprintf(`
mutation {
	commentOnEvent(eventId: "%s", content: "content", createdAt: "%s"){
		eventId
		commentId
		content
		createdAt
    }
}`, e.ID, time.Now().String())
	res, err := c.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		slog.Error("unable to comment on event", "error", err)
		os.Exit(1)
	}
	_, _ = pp.Println(res)
}

func deleteEvent(c *appsync.Client, e *event) {
	mutation := fmt.Sprintf(`
mutation {
	deleteEvent(id: "%s"){
            id
            name
			when
			where
			description
    }
}`, e.ID)
	res, err := c.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		slog.Error("unable to delete event", "error", err)
		os.Exit(1)
	}
	_, _ = pp.Println(res)
}
