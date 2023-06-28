package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	log.SetFlags(log.Llongfile)
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
			log.Fatalln(err)
		}
		creds, err := cfg.Credentials.Retrieve(ctx)
		if err != nil {
			log.Fatalln(err)
		}
		signer := sdkv2_v4.NewSigner()
		opt = appsync.WithIAMAuthorizationV2(signer, creds, *region, *url)
		sOpt = appsync.WithIAMV2(signer, creds, *region, *url)
	}
	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(*url)), opt)

	log.Println("mutation createEvent()")
	event := createEvent(client)

	log.Println("subscription subscribeToEventComments()")
	ch := make(chan *graphql.Response)
	defer close(ch)
	var s subscriber
	switch *protocol {
	case "mqtt":
		s = mqttSubscribeToEventComments(client, event, ch)
	case "graphql-ws":
		s = wsSubscribeToEventComments(*url, sOpt, event, ch)
	default:
		log.Fatalln("unsupported protocol: " + *protocol)
	}
	if err := s.Start(); err != nil {
		log.Fatalln(err)
	}
	defer s.Stop()

	log.Println("mutation commentOnEvent()")
	commentOnEvent(client, event)
	msg, ok := <-ch
	if !ok {
		log.Fatal("ch has been closed.")
	}
	log.Println("comment received")
	_, _ = pp.Println(msg)

	log.Println("mutation deleteEvent()")
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
		log.Fatalln(err)
	}
	_, _ = pp.Println(res)

	ev := new(event)
	if err := res.DataAs(ev); err != nil {
		log.Fatalln(err)
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
	subreq := graphql.PostRequest{
		Query: subscription,
	}
	res, err := c.Post(subreq)
	if err != nil {
		log.Fatalln(err)
	}
	_, _ = pp.Println(res)

	ext, err := appsync.NewExtensions(res)
	if err != nil {
		log.Fatalln(err)
	}

	return appsync.NewSubscriber(*ext,
		func(r *graphql.Response) { ch <- r },
		func(err error) { log.Println(err) })

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
		func(err error) { log.Println(err) },
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
		log.Fatalln(err)
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
		log.Fatalln(err)
	}
	_, _ = pp.Println(res)
}
