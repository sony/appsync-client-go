package appsync

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/sony/appsync-client-go/graphql"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Subscriber has MQTT connections and subscription information.
type Subscriber struct {
	clientID         string
	topic            string
	url              string
	callback         func(response *graphql.Response)
	onConnectionLost func(err error)
	started          rwBool

	mqtt MQTT.Client
}

const (
	quiesce = 100
)

// NewSubscriber returns a new Subscriber instance.
func NewSubscriber(extensions Extensions, callback func(response *graphql.Response), onConnectionLost func(err error)) *Subscriber {
	slog.Debug("creating new subscriber", "extensions", extensions)
	if len(extensions.Subscription.MqttConnections) == 0 {
		slog.Warn("There is no mqtt connections.", "extensions", extensions)
		return nil
	}

	topic := func() string {
		if len(extensions.Subscription.NewSubscriptions) > 1 {
			slog.Warn("Multiple subscriptions are currently not supported.", "extensions", extensions)
			return ""
		}
		for _, v := range extensions.Subscription.NewSubscriptions {
			slog.Debug("topics", "topic", v.Topic)
			return v.Topic
		}
		return ""
	}()
	if len(topic) == 0 {
		slog.Warn("There are no new subscriptions.", "extensions", extensions)
		return nil
	}

	index := func() int {
		for i, c := range extensions.Subscription.MqttConnections {
			for _, t := range c.Topics {
				if t == topic {
					return i
				}
			}
		}
		return -1
	}()
	if index < 0 {
		slog.Warn("There are no new subscriptions.", "extensions", extensions)
		return nil
	}

	mqttConnection := extensions.Subscription.MqttConnections[index]
	return &Subscriber{
		clientID:         mqttConnection.Client,
		url:              mqttConnection.URL,
		topic:            topic,
		callback:         callback,
		onConnectionLost: onConnectionLost,
		started:          rwBool{b: false},
	}
}

type rwBool struct {
	sync.RWMutex
	b bool
}

func (r *rwBool) load() bool {
	r.RLock()
	defer r.RUnlock()
	return r.b
}

func (r *rwBool) store(b bool) {
	r.Lock()
	defer r.Unlock()
	r.b = b
}

// Start starts a new subscription.
func (s *Subscriber) Start() error {
	slog.Debug("starting new subscriber", "clientID", s.clientID, "url", s.url, "topic", s.topic)
	opts := MQTT.NewClientOptions().
		AddBroker(s.url).
		SetClientID(s.clientID).
		SetAutoReconnect(false).
		SetConnectionLostHandler(func(c MQTT.Client, err error) {
			if !s.started.load() {
				return
			}
			s.onConnectionLost(err)
		})

	ch := make(chan error, 1)
	defer close(ch)

	opts.OnConnect = func(c MQTT.Client) {
		mqttCallback := func(client MQTT.Client, msg MQTT.Message) {
			if !s.started.load() {
				return
			}
			r := new(graphql.Response)
			if err := json.Unmarshal(msg.Payload(), r); err != nil {
				slog.Error("unable to unmarshal mqtt message", "error", err, "message", string(msg.Payload()))
				return
			}
			s.callback(r)
		}

		subscribe := func() (string, error) {
			if token := c.Subscribe(s.topic, 0, mqttCallback); token.Wait() && token.Error() != nil {
				slog.Warn("unable to subscribe to topic", "topic", s.topic, "error", token.Error())
				return "", token.Error()
			}
			s.started.store(true)
			return "", nil
		}

		subscribeTimeout := 60 * time.Second
		_, err := backoff.Retry(context.TODO(), subscribe,
			backoff.WithBackOff(backoff.NewExponentialBackOff()),
			backoff.WithMaxElapsedTime(subscribeTimeout))
		ch <- err
	}

	mqtt := MQTT.NewClient(opts)
	connect := func() (string, error) {
		if token := mqtt.Connect(); token.Wait() && token.Error() != nil {
			return "", token.Error()
		}
		s.mqtt = mqtt
		return "", nil
	}

	connectionTimeout := 5 * time.Minute
	_, err := backoff.Retry(context.TODO(), connect,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(connectionTimeout))
	if err != nil {
		slog.Warn("unable to connect to mqtt on retry", "error", err)
		return err
	}

	return <-ch
}

// Stop ends the subscription.
func (s *Subscriber) Stop() {
	slog.Debug("stopping subscriber", "topic", s.topic)
	if s.mqtt == nil {
		return
	}
	s.started.store(false)
	defer func() {
		s.mqtt.Disconnect(quiesce)
		s.mqtt = nil
	}()
	if token := s.mqtt.Unsubscribe(s.topic); token.Wait() && token.Error() != nil {
		slog.Warn("error in token", "topic", s.topic, "error", token.Error())
	}
}
