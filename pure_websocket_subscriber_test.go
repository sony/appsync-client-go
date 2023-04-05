package appsync

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sony/appsync-client-go/graphql"
)

func TestPureWebSocketSubscriber_StartStop(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(NewPureWebSocketHandlerFunc(0, 0, 0)))
	defer func() {
		s.Close()
	}()

	realtimeEndpoint := strings.Replace(s.URL, "http", "ws", 1)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Success",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPureWebSocketSubscriber(realtimeEndpoint, graphql.PostRequest{},
				func(response *graphql.Response) {}, func(err error) {})

			if err := p.Start(); (err != nil) != tt.wantErr {
				t.Errorf("PureWebSocketSubscriber.Start() error = %v, wantErr %v", err, tt.wantErr)
			}
			p.Stop()
		})
	}
}

func TestPureWebSocketSubscriber_AbortStartOnBackOff(t *testing.T) {

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Success",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(NewPureWebSocketHandlerFunc(0, 0, 0)))
			p := NewPureWebSocketSubscriber(strings.Replace(s.URL, "http", "ws", 1),
				graphql.PostRequest{}, func(response *graphql.Response) {}, func(err error) {})
			s.Close()
			go func() {
				time.Sleep(time.Second * 1)
				p.Abort()
			}()
			if err := p.Start(); (err != nil) != tt.wantErr {
				t.Errorf("PureWebSocketSubscriber.Start() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func TestPureWebSocketSubscriber_AbortStartOnReadMessage(t *testing.T) {
	tests := []struct {
		name            string
		conn_ack_delay  time.Duration
		start_ack_delay time.Duration
		wantErr         bool
		errMsg          string
	}{
		{
			name:            "Abort waiting conn_ack Success",
			conn_ack_delay:  time.Second * 5,
			start_ack_delay: 0,
			wantErr:         true,
			errMsg:          "connection failed",
		},
		{
			name:            "Abort waiting start_ack Success",
			conn_ack_delay:  0,
			start_ack_delay: time.Second * 5,
			wantErr:         true,
			errMsg:          "subscription registration failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(NewPureWebSocketHandlerFunc(tt.conn_ack_delay, tt.start_ack_delay, 0)))
			p := NewPureWebSocketSubscriber(strings.Replace(s.URL, "http", "ws", 1),
				graphql.PostRequest{}, func(response *graphql.Response) {}, func(err error) {})
			go func() {
				time.Sleep(time.Second * 2)
				p.Abort()
			}()
			if err := p.Start(); (err != nil) != tt.wantErr {
				t.Errorf("PureWebSocketSubscriber.Start() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("PureWebSocketSubscriber.Start() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
			p.Stop()
			s.Close()
		})
	}
}

func TestPureWebSocketSubscriber_AbortStopOnReadMessage(t *testing.T) {
	tests := []struct {
		name           string
		complete_delay time.Duration
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "Abort waiting complete Success",
			complete_delay: time.Second * 5,
			wantErr:        true,
			errMsg:         "connection failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(NewPureWebSocketHandlerFunc(0, 0, tt.complete_delay)))
			p := NewPureWebSocketSubscriber(strings.Replace(s.URL, "http", "ws", 1),
				graphql.PostRequest{}, func(response *graphql.Response) {}, func(err error) {})
			go func() {
				time.Sleep(time.Second * 3)
				p.Abort()
			}()
			if err := p.Start(); err != nil {
				t.Errorf("PureWebSocketSubscriber.Start() error = %v, wantErr %v", err, tt.wantErr)
			}
			p.Stop()
			s.Close()
		})
	}
}

func pureWebSocketSession(ws *websocket.Conn, conn_ack_delay, start_ack_delay, complete_delay time.Duration) {
	defer func() {
		if err := ws.Close(); err != nil {
			log.Println(err)
		}
	}()

	handlers := map[string]func([]byte) ([]byte, bool){
		"connection_init": func(in []byte) ([]byte, bool) {
			m := connectionAckMessage{
				message: message{
					Type: "connection_ack",
				},
				Payload: struct {
					ConnectionTimeoutMs int64 `json:"connectionTimeoutMs"`
				}{ConnectionTimeoutMs: 1000},
			}
			out, err := json.Marshal(&m)
			if err != nil {
				return nil, true
			}
			time.Sleep(conn_ack_delay)
			return out, false
		},
		"start": func(in []byte) ([]byte, bool) {
			m := startAckMessage{
				message: message{Type: "start_ack"},
				ID:      uuid.New().String(),
			}
			out, err := json.Marshal(&m)
			if err != nil {
				return nil, true
			}
			time.Sleep(start_ack_delay)
			return out, false
		},
		"stop": func(in []byte) ([]byte, bool) {
			m := completeMessage{
				message: message{Type: "complete"},
			}
			out, err := json.Marshal(&m)
			if err != nil {
				return nil, true
			}
			time.Sleep(complete_delay)
			return out, true
		},
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		msg := new(message)
		if err := json.Unmarshal(payload, msg); err != nil {
			log.Println(err)
			return
		}

		handler, ok := handlers[msg.Type]
		if !ok {
			log.Println("invalid message received: " + msg.Type)
			continue
		}

		out, finish := handler(payload)
		if err := ws.WriteMessage(websocket.TextMessage, out); err != nil {
			log.Println(err)
			return
		}
		if finish {
			return
		}
	}
}

func NewPureWebSocketHandlerFunc(conn_ack_delay, start_ack_delay, complete_delay time.Duration) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		go pureWebSocketSession(ws, conn_ack_delay, start_ack_delay, complete_delay)
	}
}
