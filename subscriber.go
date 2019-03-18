package appsync

import (
	"encoding/json"
	"log"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/sony/appsync-client-go/graphql"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Subscriber has MQTT connections and subscription information.
type Subscriber struct {
	clientID         string
	topics           []string
	url              string
	callback         func(response *graphql.Response)
	onConnectionLost func(err error)

	mqtt MQTT.Client
}

// NewSubscriber returns a new Subscriber instance.
func NewSubscriber(extensions Extensions, callback func(response *graphql.Response), onConnectionLost func(err error)) *Subscriber {
	connections := len(extensions.Subscription.MqttConnections)
	switch {
	case connections == 0:
		log.Println("There is no mqtt connections")
		return nil
	case connections > 1:
		log.Println("Multiple mqtt connections are currently not supported.")
		return nil
	}

	mqttConnection := extensions.Subscription.MqttConnections[0]
	return &Subscriber{
		topics:           mqttConnection.Topics,
		url:              mqttConnection.URL,
		clientID:         mqttConnection.Client,
		callback:         callback,
		onConnectionLost: onConnectionLost,
	}
}

// Start starts a new subscription.
func (s *Subscriber) Start() error {
	opts := MQTT.NewClientOptions().AddBroker(s.url).SetClientID(s.clientID).SetAutoReconnect(false)

	ch := make(chan error, 1)
	defer close(ch)

	opts.OnConnect = func(c MQTT.Client) {
		filters := map[string]byte{}
		for _, topic := range s.topics {
			filters[topic] = 0
		}

		mqttCallback := func(client MQTT.Client, msg MQTT.Message) {
			r := new(graphql.Response)
			if err := json.Unmarshal(msg.Payload(), r); err != nil {
				log.Println(err)
				return
			}
			s.callback(r)
		}

		subscribe := func() error {
			if token := c.SubscribeMultiple(filters, mqttCallback); token.Wait() && token.Error() != nil {
				return token.Error()
			}
			return nil
		}

		// Here be dragons.
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 60 * time.Second
		ch <- backoff.Retry(subscribe, b)
	}

	opts.OnConnectionLost = func(c MQTT.Client, err error) {
		s.onConnectionLost(err)
	}

	// Here be dragons.
	time.Sleep(2 * time.Second)

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
	for _, topic := range s.topics {
		if token := s.mqtt.Unsubscribe(topic); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
		}
	}
	s.mqtt.Disconnect(100)
}
