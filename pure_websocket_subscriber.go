package appsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sony/appsync-client-go/graphql"
)

type message struct {
	Type string `json:"type"`
}

type connectionAckMessage struct {
	message
	Payload struct {
		ConnectionTimeoutMs int64 `json:"connectionTimeoutMs"`
	} `json:"payload"`
}

type startMessage struct {
	message
	ID      string                          `json:"id"`
	Payload subscriptionRegistrationPayload `json:"payload"`
}
type subscriptionRegistrationPayload struct {
	Data       string                                    `json:"data"`
	Extensions subscriptionRegistrationPayloadExtensions `json:"extensions"`
}
type subscriptionRegistrationPayloadExtensions struct {
	Authorization map[string]string `json:"authorization"`
}
type startAckMessage struct {
	message
	ID string `json:"id"`
}

type processingDataMessage struct {
	message
	ID      string                `json:"id"`
	Payload processingDataPayload `json:"payload"`
}
type processingDataPayload struct {
	Data interface{} `json:"data"`
}

type stopMessage struct {
	message
	ID string `json:"id"`
}
type completeMessage struct {
	message
	ID string `json:"id"`
}

type errorMessage struct {
	message
	ID      string       `json:"id"`
	Payload errorPayload `json:"payload"`
}
type errorPayload struct {
	Errors []struct {
		ErrorType string `json:"errorType"`
		Message   string `json:"message"`
	} `json:"errors"`
}

var (
	connectionInitMsg = message{Type: "connection_init"}
)

// PureWebSocketSubscriber has pure WebSocket connections and subscription information.
type PureWebSocketSubscriber struct {
	realtimeEndpoint string
	request          graphql.PostRequest
	header           http.Header
	iamAuth          *struct {
		signer *v4.Signer
		region string
		host   string
	}
	cancel context.CancelFunc
	op     *realtimeWebSocketOperation
}

// NewPureWebSocketSubscriber returns a PureWebSocketSubscriber instance.
func NewPureWebSocketSubscriber(realtimeEndpoint string, request graphql.PostRequest,
	onReceive func(response *graphql.Response),
	onConnectionLost func(err error),
	opts ...PureWebSocketSubscriberOption) *PureWebSocketSubscriber {
	ctx, cancel := context.WithCancel(context.Background())
	p := PureWebSocketSubscriber{
		realtimeEndpoint: realtimeEndpoint,
		request:          request,
		header:           http.Header{},
		iamAuth:          nil,
		cancel:           cancel,
		op:               newRealtimeWebSocketOperation(ctx, onReceive, onConnectionLost),
	}
	for _, opt := range opts {
		opt(&p)
	}
	return &p
}

// Start starts a new subscription.
func (p *PureWebSocketSubscriber) Start() error {
	bpayload := []byte("{}")
	header := map[string]string{}
	if p.iamAuth != nil {
		var err error
		header, err = signRequest(p.iamAuth.signer, p.iamAuth.host+"/connect", p.iamAuth.region, bpayload)
		if err != nil {
			log.Println(err)
			return err
		}
	} else {
		for k, v := range p.header {
			header[k] = v[0]
		}
	}

	bheader, err := json.Marshal(header)
	if err != nil {
		log.Println(err)
		return err
	}

	if err := p.op.connect(p.realtimeEndpoint, bheader, bpayload); err != nil {
		return err
	}

	if err := p.op.connectionInit(); err != nil {
		return err
	}

	brequest, err := json.Marshal(p.request)
	if err != nil {
		log.Println(err)
		return err
	}
	authz := map[string]string{}
	if p.iamAuth != nil {
		var err error
		authz, err = signRequest(p.iamAuth.signer, p.iamAuth.host, p.iamAuth.region, brequest)
		if err != nil {
			return err
		}
	} else {
		for k, v := range p.header {
			authz[k] = v[0]
		}
	}
	if err := p.op.start(brequest, authz); err != nil {
		return err
	}

	return nil
}

func signRequest(signer *v4.Signer, url, region string, data []byte) (map[string]string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	req.Header.Add("accept", "application/json, text/javascript")
	req.Header.Add("content-encoding", "amz-1.0")
	req.Header.Add("content-type", "application/json; charset=UTF-8")

	_, err = signer.Sign(req, bytes.NewReader(data), "appsync", region, time.Now())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	headers := map[string]string{
		"accept":           req.Header.Get("accept"),
		"content-encoding": req.Header.Get("content-encoding"),
		"content-type":     req.Header.Get("content-type"),
		"host":             req.Host,
		"x-amz-date":       req.Header.Get("x-amz-date"),
		"Authorization":    req.Header.Get("Authorization"),
	}

	token := req.Header.Get("X-Amz-Security-Token")
	if token != "" {
		headers["X-Amz-Security-Token"] = token
	}

	return headers, nil
}

// Stop ends the subscription.
func (p *PureWebSocketSubscriber) Stop() {
	p.op.stop()
	p.op.disconnect()
}

// Abort ends the subscription forcibly.
func (p *PureWebSocketSubscriber) Abort() {
	p.cancel()
	p.op.subscriptionID = ""
	p.Stop()
}

const defaultTimeout = time.Duration(300000) * time.Millisecond

type realtimeWebSocketOperation struct {
	ctx              context.Context
	onReceive        func(response *graphql.Response)
	onConnectionLost func(err error)

	ws                *websocket.Conn
	connectionTimeout time.Duration
	subscriptionID    string
	connackCh         chan connectionAckMessage
	startackCh        chan startAckMessage
	completeCh        chan completeMessage
}

func newRealtimeWebSocketOperation(ctx context.Context, onReceive func(response *graphql.Response),
	onConnectionLost func(err error)) *realtimeWebSocketOperation {
	return &realtimeWebSocketOperation{ctx, onReceive, onConnectionLost, nil, 0, "", nil, nil, nil}
}

func (r *realtimeWebSocketOperation) readLoop() {
	defer close(r.connackCh)
	defer close(r.startackCh)
	defer close(r.completeCh)

	if err := r.ws.SetReadDeadline(time.Now().Add(defaultTimeout)); err != nil {
		log.Println(err)
		return
	}
	for {
		handlers := map[string]func(b []byte) (finish bool){
			"connection_ack": r.onConnected,
			"ka":             r.onKeepAlive,
			"start_ack":      r.onStarted,
			"data":           r.onData,
			"complete":       r.onStopped,
			"error":          r.onError,
		}

		_, payload, err := r.ws.ReadMessage()
		if err != nil {
			log.Println(err)
			if strings.Contains(err.Error(), "i/o timeout") {
				r.onConnectionLost(err)
			}
			return
		}

		msg := new(message)
		if err := json.Unmarshal(payload, msg); err != nil {
			log.Println(err)
			return
		}

		handler, ok := handlers[msg.Type]
		if !ok {
			log.Println("invalid message received")
			continue
		}
		if handler(payload) {
			return
		}
	}
}

func (r *realtimeWebSocketOperation) connect(realtimeEndpoint string, header, payload []byte) error {
	if r.ws != nil {
		return errors.New("already connected")
	}

	b64h := base64.StdEncoding.EncodeToString(header)
	b64p := base64.StdEncoding.EncodeToString(payload)
	endpoint := fmt.Sprintf("%s?header=%s&payload=%s", realtimeEndpoint, b64h, b64p)

	if err := backoff.Retry(func() error {
		ws, _, err := websocket.DefaultDialer.DialContext(r.ctx, endpoint, http.Header{"sec-websocket-protocol": []string{"graphql-ws"}})
		if err != nil {
			log.Print(err)
			return err
		}
		r.ws = ws
		return nil
	}, backoff.WithContext(backoff.NewExponentialBackOff(), r.ctx)); err != nil {
		log.Println(err)
		return err
	}

	r.connackCh = make(chan connectionAckMessage, 1)
	r.startackCh = make(chan startAckMessage, 1)
	r.completeCh = make(chan completeMessage, 1)

	go r.readLoop()
	return nil
}

func (r *realtimeWebSocketOperation) onConnected(payload []byte) bool {
	connack := new(connectionAckMessage)
	if err := json.Unmarshal(payload, connack); err != nil {
		log.Println(err)
		return true
	}
	r.connackCh <- *connack
	return false
}

func (r *realtimeWebSocketOperation) connectionInit() error {
	if r.connectionTimeout != 0 {
		return errors.New("already connection initialized")
	}

	init, err := json.Marshal(connectionInitMsg)
	if err != nil {
		log.Println(err)
		return err
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, init); err != nil {
		log.Println(err)
		return err
	}
	connack, ok := <-r.connackCh
	if !ok {
		return errors.New("connection failed")
	}

	r.connectionTimeout = time.Duration(connack.Payload.ConnectionTimeoutMs) * time.Millisecond
	return nil
}

func (r *realtimeWebSocketOperation) onKeepAlive([]byte) bool {
	timeout := defaultTimeout
	if r.connectionTimeout != 0 {
		timeout = r.connectionTimeout
	}
	if err := r.ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		log.Println(err)
		return true
	}
	return false
}

func (r *realtimeWebSocketOperation) start(request []byte, authorization map[string]string) error {
	if len(r.subscriptionID) != 0 {
		return errors.New("already started")
	}

	start := startMessage{
		message: message{"start"},
		ID:      uuid.New().String(),
		Payload: subscriptionRegistrationPayload{
			Data: string(request),
			Extensions: subscriptionRegistrationPayloadExtensions{
				Authorization: authorization,
			},
		},
	}

	b, err := json.Marshal(start)
	if err != nil {
		log.Println(err)
		return err
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println(err)
	}
	startack, ok := <-r.startackCh
	if !ok {
		return errors.New("subscription registration failed")
	}
	r.subscriptionID = startack.ID

	return nil
}

func (r *realtimeWebSocketOperation) onStarted(payload []byte) bool {
	startack := new(startAckMessage)
	if err := json.Unmarshal(payload, startack); err != nil {
		log.Println(err)
		return true
	}
	r.startackCh <- *startack
	return false
}

func (r *realtimeWebSocketOperation) onData(payload []byte) bool {
	data := new(processingDataMessage)
	if err := json.Unmarshal(payload, data); err != nil {
		log.Println(err)
		return true
	}
	r.onReceive(&graphql.Response{
		Data: data.Payload.Data,
	})
	return false
}

func (r *realtimeWebSocketOperation) stop() {
	if len(r.subscriptionID) == 0 {
		return
	}

	stop := stopMessage{message{"stop"}, r.subscriptionID}
	b, err := json.Marshal(stop)
	if err != nil {
		log.Println(err)
		return
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println(err)
		return
	}
	if _, ok := <-r.completeCh; !ok {
		log.Println("unsubscribe failed")
	}
	r.subscriptionID = ""
}

func (r *realtimeWebSocketOperation) onStopped(payload []byte) bool {
	complete := new(completeMessage)
	if err := json.Unmarshal(payload, complete); err != nil {
		log.Println(err)
		return true
	}
	r.completeCh <- *complete
	return true
}

func (r *realtimeWebSocketOperation) disconnect() {
	if r.ws == nil {
		return
	}

	if err := r.ws.Close(); err != nil {
		log.Println(err)
	}
	r.connectionTimeout = 0
	r.ws = nil
}

func (r *realtimeWebSocketOperation) onError(payload []byte) bool {
	em := new(errorMessage)
	if err := json.Unmarshal(payload, em); err != nil {
		log.Println(err)
		return true
	}
	errors := make([]interface{}, len(em.Payload.Errors))
	for i, e := range em.Payload.Errors {
		errors[i] = e
	}
	r.onReceive(&graphql.Response{
		Errors: &errors,
	})
	return true
}
