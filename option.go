package appsync

// ClientOption represents options for an AppSync client.
type ClientOption interface {
	Apply(*Client)
}

// WithSubscriberID returns a ClientOption configured with the given AppSync subscriber ID
func WithSubscriberID(subscriberID string) ClientOption {
	return withSubscriberID{subscriberID}
}

type withSubscriberID struct {
	subscriberID string
}

func (w withSubscriberID) Apply(c *Client) {
	c.subscriberID = w.subscriberID
}
