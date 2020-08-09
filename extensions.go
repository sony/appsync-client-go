package appsync

import (
	"encoding/json"
	"fmt"

	"github.com/sony/appsync-client-go/graphql"
)

// Extensions represents AWS AppSync subscription response extensions
type Extensions struct {
	Subscription struct {
		Version         string `json:"version"`
		MqttConnections []struct {
			URL    string   `json:"url"`
			Topics []string `json:"topics"`
			Client string   `json:"client"`
		} `json:"mqttConnections"`
		NewSubscriptions map[string]Subscription `json:"newSubscriptions"`
	} `json:"subscription"`
}

// Subscription represents AWS AppSync subscription mqtt topic
type Subscription struct {
	Topic      string      `json:"topic"`
	ExpireTime interface{} `json:"expireTime"`
}

// NewExtensions returns Extensions instance
func NewExtensions(response *graphql.Response) (*Extensions, error) {
	j, ok := (*response.Extensions).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Extensions is invalid")
	}

	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}

	ext := new(Extensions)
	if err := json.Unmarshal(b, ext); err != nil {
		return nil, err
	}
	return ext, nil
}
