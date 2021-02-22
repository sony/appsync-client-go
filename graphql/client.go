package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
)

type httpStatusError struct {
	StatusCode int
}

func (e httpStatusError) Error() string {
	return http.StatusText(e.StatusCode)
}

func (e httpStatusError) shouldRetry() bool {
	return e.StatusCode == http.StatusInternalServerError ||
		e.StatusCode == http.StatusServiceUnavailable
}

// Client represents a generic GraphQL API client.
type Client struct {
	endpoint       string
	timeout        time.Duration
	maxElapsedTime time.Duration
	header         http.Header
}

// NewClient returns a Client instance.
func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		endpoint:       endpoint,
		timeout:        time.Duration(30 * time.Second),
		maxElapsedTime: time.Duration(20 * time.Second),
		header:         map[string][]string{},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

//Post is a synchronous GraphQL POST request.
func (c *Client) Post(header http.Header, request PostRequest) (*Response, error) {
	type ret struct {
		response *Response
		err      error
	}
	ch := make(chan ret, 1)
	if _, err := c.PostAsync(header, request, func(response *Response, err error) { ch <- ret{response, err} }); err != nil {
		return nil, err
	}
	r := <-ch
	return r.response, r.err
}

//PostAsync is an asynchronous GraphQL POST request.
func (c *Client) PostAsync(header http.Header, request PostRequest, callback func(*Response, error)) (context.CancelFunc, error) {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	for k, v := range c.header {
		req.Header[k] = v
	}
	for k, v := range header {
		req.Header[k] = v
	}

	ctx, cancel := context.WithTimeout(req.Context(), c.timeout)
	req = req.WithContext(ctx)

	go func() {
		defer cancel()

		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = c.maxElapsedTime

		var response Response
		op := func() error {
			r, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Println(err)
				if err.(*url.Error).Timeout() {
					http.DefaultClient.CloseIdleConnections()
				}
				return backoff.Permanent(err)
			}
			defer func() {
				if err := r.Body.Close(); err != nil {
					log.Println(err)
				}
			}()

			if r.StatusCode != http.StatusOK {
				httpErr := httpStatusError{StatusCode: r.StatusCode}
				if httpErr.shouldRetry() {
					log.Println(httpErr)
					return httpErr
				}
				return backoff.Permanent(httpErr)
			}

			if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
				log.Println(err)
				return backoff.Permanent(err)
			}
			response.StatusCode = &r.StatusCode

			return nil
		}

		switch err := backoff.Retry(op, b).(type) {
		case nil:
			callback(&response, nil)
		case httpStatusError:
			callback(&Response{&(err.StatusCode), nil, &[]interface{}{err.Error()}, nil}, nil)
		default:
			callback(nil, err)
		}
	}()

	return cancel, nil
}
