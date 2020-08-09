package main

import (
	"flag"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
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

type eventConnection struct {
	Items     []event `json:"items"`
	NextToken string  `json:"nextToken"`
}

func main() {
	log.SetFlags(log.Llongfile)
	var (
		region = flag.String("region", "", "AppSync API region")
		url    = flag.String("url", "", "AppSync API URL")
	)
	flag.Parse()

	query := `
query ListEvents {
    listEvents {
        items {
            id
            name
        }
    }
}
`
	sess := session.Must(session.NewSession())
	signer := v4.NewSigner(sess.Config.Credentials)
	client := appsync.NewClient(appsync.NewGraphQLClient(graphql.NewClient(*url)),
		appsync.WithIAMAuthorization(*signer, *region, *url))
	res, err := client.Post(graphql.PostRequest{
		Query: query,
	})
	if err != nil {
		log.Fatalln(err)
	}
	pp.Println(res)
	data := new(eventConnection)
	if err := res.DataAs(data); err != nil {
		log.Fatalln(err, res)
	}
	pp.Println(*data)
}
