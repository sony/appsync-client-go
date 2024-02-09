package appsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

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
	sigv4            sigv4
	cancel           context.CancelFunc
	op               *realtimeWebSocketOperation
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
		cancel:           cancel,
		op:               newRealtimeWebSocketOperation(ctx, onReceive, onConnectionLost),
	}
	for _, opt := range opts {
		opt(&p)
	}
	return &p
}

func (p *PureWebSocketSubscriber) setupHeaders(payload []byte) (map[string]string, error) {
	if p.sigv4 == nil {
		headers := map[string]string{}
		for k := range p.header {
			headers[k] = p.header.Get(k)
		}
		return headers, nil
	}

	headers, err := p.sigv4.signWS(payload)
	if err != nil {
		slog.Error("error signing WS headers", "error", err)
		return nil, err
	}

	return headers, nil
}

// Start starts a new subscription.
func (p *PureWebSocketSubscriber) Start() error {
	bpayload := []byte("{}")
	header, err := p.setupHeaders(bpayload)
	if err != nil {
		slog.Error("error setting up headers", "error", err)
		return err
	}
	bheader, err := json.Marshal(header)
	if err != nil {
		slog.Error("error marshalling headers during Start", "error", err, "header", header)
		return err
	}
	if err := p.op.connect(p.realtimeEndpoint, bheader, bpayload); err != nil {
		slog.ErrorContext(p.op.ctx, "error connecting to websocket", "error", err, "realtimeEndpoint", p.realtimeEndpoint, "header", bheader, "payload", bpayload)
		return err
	}

	if err := p.op.connectionInit(); err != nil {
		slog.ErrorContext(p.op.ctx, "error initializing connection", "error", err)
		return err
	}

	brequest, err := json.Marshal(p.request)
	if err != nil {
		slog.ErrorContext(p.op.ctx, "error marshalling request", "error", err, "request", p.request)
		return err
	}
	authz, err := p.setupHeaders(brequest)
	if err != nil {
		slog.ErrorContext(p.op.ctx, "error setting up headers", "error", err)
		return err
	}
	if err := p.op.start(brequest, authz); err != nil {
		slog.ErrorContext(p.op.ctx, "error starting subscription", "error", err)
		return err
	}

	return nil
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
		slog.Error("error setting read deadline", "error", err)
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
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				slog.Warn("connection timeout")
			} else {
				slog.ErrorContext(r.ctx, "error reading message", "error", err)
			}
			r.onConnectionLost(err)
			return
		}

		msg := new(message)
		if err := json.Unmarshal(payload, msg); err != nil {
			slog.ErrorContext(r.ctx, "error unmarshalling message", "error", err, "payload", string(payload))
			return
		}

		handler, ok := handlers[msg.Type]
		slog.Debug("msg", "payload", string(payload), "ok", ok)
		if !ok {
			slog.Warn("invalid message received", "msgType", msg.Type)
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
			slog.Error("error connecting to websocket", "error", err)
			return err
		}
		r.ws = ws
		return nil
	}, backoff.WithContext(backoff.NewExponentialBackOff(), r.ctx)); err != nil {
		slog.ErrorContext(r.ctx, "error connecting to websocket", "error", err)
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
		slog.ErrorContext(r.ctx, "error unmarshalling connection_ack", "error", err)
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
		slog.ErrorContext(r.ctx, "error marshalling connection_init", "error", err)
		return err
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, init); err != nil {
		slog.ErrorContext(r.ctx, "error writing connection_init", "error", err)
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
		slog.ErrorContext(r.ctx, "error setting read deadline", "error", err)
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
		slog.ErrorContext(r.ctx, "error marshalling start", "error", err)
		return err
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		slog.ErrorContext(r.ctx, "error writing start", "error", err)
	}
	startack, ok := <-r.startackCh
	if !ok {
		return errors.New("subscription registration failed")
	}

	r.subscriptionID = startack.ID
	slog.Debug("subscriptionID", "id", r.subscriptionID)

	return nil
}

func (r *realtimeWebSocketOperation) onStarted(payload []byte) bool {
	startack := new(startAckMessage)
	if err := json.Unmarshal(payload, startack); err != nil {
		slog.ErrorContext(r.ctx, "error unmarshalling start_ack", "error", err)
		return true
	}
	r.startackCh <- *startack
	return false
}

func (r *realtimeWebSocketOperation) onData(payload []byte) bool {
	data := new(processingDataMessage)
	if err := json.Unmarshal(payload, data); err != nil {
		slog.ErrorContext(r.ctx, "error unmarshalling onData payload", "error", err)
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
		slog.ErrorContext(r.ctx, "error marshalling stop", "error", err)
		return
	}
	if err := r.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		slog.ErrorContext(r.ctx, "error writing stop", "error", err)
		return
	}
	if _, ok := <-r.completeCh; !ok {
		slog.Warn("subscription stop failed")
	}
	r.subscriptionID = ""
}

func (r *realtimeWebSocketOperation) onStopped(payload []byte) bool {
	complete := new(completeMessage)
	if err := json.Unmarshal(payload, complete); err != nil {
		slog.ErrorContext(r.ctx, "error unmarshalling onStopped complete msg", "error", err, "payload", string(payload))
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
		slog.ErrorContext(r.ctx, "error closing websocket", "error", err)
	}
	r.connectionTimeout = 0
	r.ws = nil
}

func (r *realtimeWebSocketOperation) onError(payload []byte) bool {
	em := new(errorMessage)
	if err := json.Unmarshal(payload, em); err != nil {
		slog.ErrorContext(r.ctx, "error unmarshalling onError payload", "error", err, "payload", string(payload))
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
