package appsync

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sony/appsync-client-go/graphql"
)

func TestPureWebSocketSubscriber_StartStop(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(pureWebSocketHandlerFunc))
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

func pureWebSocketSession(ws *websocket.Conn) {
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

func pureWebSocketHandlerFunc(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	go pureWebSocketSession(ws)
}
