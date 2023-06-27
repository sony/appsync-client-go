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
	res, err := client.Post(graphql.PostRequest{
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

	log.Println("subscription subscribeToEventComments()")
	subscription := fmt.Sprintf(`
subscription {
	subscribeToEventComments(eventId: "%s"){
		eventId
		commentId
		content
		createdAt
	}
}`, ev.ID)

	var s subscriber
	subreq := graphql.PostRequest{
		Query: subscription,
	}

	ch := make(chan *graphql.Response)
	defer close(ch)

	switch *protocol {
	case "mqtt":
		res, err = client.Post(subreq)
		if err != nil {
			log.Fatalln(err)
		}
		_, _ = pp.Println(res)

		ext, err := appsync.NewExtensions(res)
		if err != nil {
			log.Fatalln(err)
		}

		s = appsync.NewSubscriber(*ext,
			func(r *graphql.Response) { ch <- r },
			func(err error) { log.Println(err) })
	case "graphql-ws":
		realtime := strings.Replace(strings.Replace(*url, "https", "wss", 1), "appsync-api", "appsync-realtime-api", 1)
		s = appsync.NewPureWebSocketSubscriber(realtime, subreq,
			func(r *graphql.Response) { ch <- r },
			func(err error) { log.Println(err) },
			sOpt,
		)
	default:
		log.Fatalln("unsupported protocol: " + *protocol)
	}

	if err := s.Start(); err != nil {
		log.Fatalln(err)
	}
	defer s.Stop()

	log.Println("mutation commentOnEvent()")
	mutation = fmt.Sprintf(`
mutation {
	commentOnEvent(eventId: "%s", content: "content", createdAt: "%s"){
		eventId
		commentId
		content
		createdAt
    }
}`, ev.ID, time.Now().String())
	res, err = client.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		log.Fatalln(err)
	}
	_, _ = pp.Println(res)

	msg, ok := <-ch
	if !ok {
		log.Fatal("ch has been closed.")
	}
	log.Println("comment received")
	_, _ = pp.Println(msg)

	log.Println("mutation deleteEvent()")
	mutation = fmt.Sprintf(`
mutation {
	deleteEvent(id: "%s"){
            id
            name
			when
			where
			description
    }
}`, ev.ID)
	res, err = client.Post(graphql.PostRequest{
		Query: mutation,
	})
	if err != nil {
		log.Fatalln(err)
	}
	_, _ = pp.Println(res)
}
