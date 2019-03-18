package graphql

import (
	"encoding/json"
	"strings"
)

// PostRequest is a generic GraphQL POST request body.
type PostRequest struct {
	Query         string           `json:"query"`
	OperationName *string          `json:"operationName"`
	Variables     *json.RawMessage `json:"variables"`
}

// IsQuery checks if the Request is "Query" or not.
func (p *PostRequest) IsQuery() bool {
	return strings.HasPrefix(strings.TrimSpace(p.Query), "query")
}

// IsMutation checks if the Request is "Mutation" or not.
func (p *PostRequest) IsMutation() bool {
	return strings.HasPrefix(strings.TrimSpace(p.Query), "mutation")
}

// IsSubscription checks if the Request is "Subscription" or not.
func (p *PostRequest) IsSubscription() bool {
	return strings.HasPrefix(strings.TrimSpace(p.Query), "subscription")
}
