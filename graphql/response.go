package graphql

import (
	"encoding/json"
	"fmt"
)

// Response represents a generic GraphQL response body.
type Response struct {
	StatusCode *int           `json:"statusCode"`
	Data       interface{}    `json:"data"`
	Errors     *[]interface{} `json:"errors"`
	Extensions *interface{}   `json:"extensions"`
}

// DataAs converts Response.Data to the specified struct
func (r *Response) DataAs(v interface{}) error {
	m, ok := r.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("data is invalid")
	}
	if len(m) != 1 {
		return fmt.Errorf("the data is not exist")
	}
	for _, value := range m {
		if b, err := json.Marshal(value); err == nil {
			return json.Unmarshal(b, v)
		}
	}
	return fmt.Errorf("data is invalid")
}
