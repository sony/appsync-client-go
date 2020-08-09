package appsync

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
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
	if len(extensions.Subscription.MqttConnections) == 0 {
		log.Printf("There is no mqtt connections.\n%+v\n", extensions)
		return nil
	}

	topic := func() string {
		if len(extensions.Subscription.NewSubscriptions) > 1 {
			log.Printf("Multiple subscriptions are currently not supported.\n%+v\n", extensions)
			return ""
		}
		for _, v := range extensions.Subscription.NewSubscriptions {
			return v.Topic
		}
		return ""
	}()
	if len(topic) == 0 {
		log.Printf("There are no new subscriptions.\n%+v\n", extensions)
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
		log.Printf("There are no new subscriptions.\n%+v\n", extensions)
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
				log.Println(err)
				return
			}
			s.callback(r)
		}

		subscribe := func() error {
			if token := c.Subscribe(s.topic, 0, mqttCallback); token.Wait() && token.Error() != nil {
				return token.Error()
			}
			s.started.store(true)
			return nil
		}

		// Here be dragons.
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 60 * time.Second
		ch <- backoff.Retry(subscribe, b)
	}

	mqtt := MQTT.NewClient(opts)
	connect := func() error {
		if token := mqtt.Connect(); token.Wait() && token.Error() != nil {
			return token.Error()
		}
		s.mqtt = mqtt
		return nil
	}

	// Here be dragons.
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute
	if err := backoff.Retry(connect, b); err != nil {
		return err
	}

	return <-ch
}

// Stop ends the subscription.
func (s *Subscriber) Stop() {
	if s.mqtt == nil {
		return
	}
	s.started.store(false)
	defer func() {
		s.mqtt.Disconnect(quiesce)
		s.mqtt = nil
	}()
	if token := s.mqtt.Unsubscribe(s.topic); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
	}
}
