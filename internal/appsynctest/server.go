package appsynctest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	graphqlgo "github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sony/appsync-client-go/graphql"
)

var (
	mqttEchoTopic = "echo"
	schema        = `
schema {
	query: Query
	mutation: Mutation
	subscription: Subscription
}

type Query {
	message: String!
}

type Mutation {
	echo(message: String!): String!
}

type Subscription {
	subscribeToEcho: String!
}
`
	initialMessage = "Hello, AppSync!"

	gqlwsconnack = `
{
	"type": "connection_ack",
	"payload" : {
		"connectionTimeoutMs": 300000
	}
}
`
	gqlwsstartackfmt = `
{
	"type": "start_ack",
	"id" : "%s"
}
`
	gqlwsdatafmt = `
{
	"type": "data",
	"id" : "%s",
	"payload": %s
}
`
	gqlwscompletefmt = `
{
	"type": "complete",
	"id" : "%s"
}
`
)

type mqttPublisher struct {
	w                http.ResponseWriter
	mqttSessions     mqttSessions
	grapqhWsSessions grapqhWsSessions
}

func (m *mqttPublisher) Header() http.Header {
	return m.w.Header()
}

func (m *mqttPublisher) Write(payload []byte) (int, error) {
	if len(m.mqttSessions) != 0 {
		go func() {
			pub := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			pub.TopicName = mqttEchoTopic
			pub.Payload = payload
			for s := range m.mqttSessions {
				writer, err := s.NextWriter(websocket.BinaryMessage)
				if err != nil {
					log.Println(err)
					continue
				}
				if err := pub.Write(writer); err != nil {
					log.Println(err)
					continue
				}
				if err := writer.Close(); err != nil {
					log.Println(err)
					continue
				}
			}
		}()
	}
	if len(m.grapqhWsSessions) != 0 {
		go func() {
			for s := range m.grapqhWsSessions {
				data := json.RawMessage(fmt.Sprintf(gqlwsdatafmt, "id", string(payload)))
				if err := s.WriteJSON(data); err != nil {
					log.Println(err)
					continue
				}
			}
		}()
	}
	return m.w.Write(payload)
}

func (m *mqttPublisher) WriteHeader(statusCode int) {
	m.w.WriteHeader(statusCode)
}

type echoResolver struct {
	message string
}

func (e *echoResolver) Message() string {
	return e.message
}

func (e *echoResolver) Echo(args struct{ Message string }) string {
	e.message = args.Message
	return e.message
}

func (e *echoResolver) SubscribeToEcho() string {
	return e.message
}

func mqttWsSession(ws *websocket.Conn, onConnected func(ws *websocket.Conn), onDisconnected func(ws *websocket.Conn)) {
	defer func() {
		if err := ws.Close(); err != nil {
			log.Println(err)
		}
	}()

	for {
		mt, r, err := ws.NextReader()
		if err != nil {
			log.Println(err)
			return
		}

		cp, err := packets.ReadPacket(r)
		if err != nil {
			log.Println(err)
			return
		}

		var ack packets.ControlPacket
		switch cp.(type) {
		case *packets.ConnectPacket:
			ack = packets.NewControlPacket(packets.Connack)
			onConnected(ws)
		case *packets.SubscribePacket:
			ack = packets.NewControlPacket(packets.Suback)
			ack.(*packets.SubackPacket).MessageID = cp.(*packets.SubscribePacket).MessageID
		case *packets.UnsubscribePacket:
			ack = packets.NewControlPacket(packets.Unsuback)
			ack.(*packets.UnsubackPacket).MessageID = cp.(*packets.UnsubscribePacket).MessageID
		case *packets.DisconnectPacket:
			onDisconnected(ws)
			return
		}
		if ack == nil {
			continue
		}

		writer, err := ws.NextWriter(mt)
		if err != nil {
			log.Println(err)
			return
		}
		if err := ack.Write(writer); err != nil {
			log.Println(err)
			return
		}
		if err := writer.Close(); err != nil {
			log.Println(err)
			return
		}
	}
}

func graphQLWsSession(ws *websocket.Conn, onConnected func(ws *websocket.Conn), onDisconnected func(ws *websocket.Conn)) {
	defer func() {
		if err := ws.Close(); err != nil {
			log.Println(err)
		}
	}()

	for {
		msg := map[string]interface{}{}
		if err := ws.ReadJSON(&msg); err != nil {
			return
		}

		var ack json.RawMessage
		switch msg["type"].(string) {
		case "connection_init":
			ack = json.RawMessage(gqlwsconnack)
			onConnected(ws)
		case "start":
			ack = json.RawMessage(fmt.Sprintf(gqlwsstartackfmt, msg["id"].(string)))
		case "stop":
			ack = json.RawMessage(fmt.Sprintf(gqlwscompletefmt, msg["id"].(string)))
			onDisconnected(ws)
		}
		if err := ws.WriteJSON(ack); err != nil {
			log.Println(err)
			return
		}
	}
}

func newQueryHandlerFunc(h relay.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}
}

func newMutationHandlerFunc(h relay.Handler, mqtt mqttSessions, graphqlws grapqhWsSessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(&mqttPublisher{w, mqtt, graphqlws}, r)
	}
}

func newSubscriptionHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ext interface{} = map[string]interface{}{
			"subscription": map[string]interface{}{
				"version": "1.0.0",
				"mqttConnections": []map[string]interface{}{
					{
						"url": fmt.Sprintf("ws://%s", r.Host),
						"topics": []string{
							mqttEchoTopic,
						},
						"client": uuid.New().String(),
					},
				},
				"newSubscriptions": map[string]interface{}{
					"subscribeToEcho": map[string]interface{}{
						"topic":      mqttEchoTopic,
						"expireTime": nil,
					},
				},
			},
		}
		resp := graphql.Response{Extensions: &ext}
		b, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err = w.Write(b); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func isMqttWs(r *http.Request) bool {
	return r.Method == http.MethodGet && r.Header.Get("Upgrade") == "websocket" &&
		r.Header.Get("Sec-Websocket-Protocol") == "mqtt"
}

func isGraphQLWs(r *http.Request) bool {
	return r.Method == http.MethodGet && r.Header.Get("Upgrade") == "websocket" &&
		r.Header.Get("Sec-Websocket-Protocol") == "graphql-ws"
}

func newMqttWsHandlerFunc(onConnected func(ws *websocket.Conn), onDisconnected func(ws *websocket.Conn)) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		go mqttWsSession(ws, onConnected, onDisconnected)
	}
}

type mqttSessions map[*websocket.Conn]bool
type grapqhWsSessions map[*websocket.Conn]bool

func newGraphQLWsHandlerFunc(onConnected func(ws *websocket.Conn), onDisconnected func(ws *websocket.Conn)) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		go graphQLWsSession(ws, onConnected, onDisconnected)
	}
}

func newAppSyncEchoHandlerFunc(initialMessage string) http.HandlerFunc {
	s := graphqlgo.MustParseSchema(schema, &echoResolver{initialMessage})
	handler := relay.Handler{Schema: s}
	mqttSessions := make(mqttSessions)
	grapqhWsSessions := make(grapqhWsSessions)
	query := newQueryHandlerFunc(handler)
	mutation := newMutationHandlerFunc(handler, mqttSessions, grapqhWsSessions)
	subscription := newSubscriptionHandlerFunc()
	mqttws := newMqttWsHandlerFunc(
		func(ws *websocket.Conn) { mqttSessions[ws] = true },
		func(ws *websocket.Conn) { delete(mqttSessions, ws) },
	)
	graphqlws := newGraphQLWsHandlerFunc(
		func(ws *websocket.Conn) { grapqhWsSessions[ws] = true },
		func(ws *websocket.Conn) { delete(grapqhWsSessions, ws) },
	)
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Reset
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		if isMqttWs(r) {
			mqttws.ServeHTTP(w, r)
			return
		}

		if isGraphQLWs(r) {
			graphqlws.ServeHTTP(w, r)
			return
		}

		req := new(graphql.PostRequest)
		if err := json.Unmarshal(body, req); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.IsQuery() {
			query.ServeHTTP(w, r)
			return
		}
		if req.IsMutation() {
			mutation.ServeHTTP(w, r)
			return
		}
		if req.IsSubscription() {
			subscription.ServeHTTP(w, r)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// NewAppSyncEchoServer starts and returns an appsync echo server instance.
func NewAppSyncEchoServer() *httptest.Server {
	return httptest.NewServer(newAppSyncEchoHandlerFunc(initialMessage))
}
