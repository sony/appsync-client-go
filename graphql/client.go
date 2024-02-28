package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	http           *http.Client
}

// NewClient returns a Client instance.
func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		endpoint:       endpoint,
		timeout:        time.Duration(30 * time.Second),
		maxElapsedTime: time.Duration(20 * time.Second),
		header:         map[string][]string{},
		http:           &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Post is a synchronous GraphQL POST request.
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

// PostAsync is an asynchronous GraphQL POST request.
func (c *Client) PostAsync(header http.Header, request PostRequest, callback func(*Response, error)) (context.CancelFunc, error) {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		slog.Error("unable to marshal request", "error", err, "request", request)
		return nil, err
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		slog.Error("unable to create request", "error", err)
		return nil, err
	}
	req.Header = merge(req.Header, merge(c.header, header))

	ctx, cancel := context.WithTimeout(req.Context(), c.timeout)
	req = req.WithContext(ctx)

	go func() {
		defer cancel()

		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = c.maxElapsedTime

		var response Response
		op := func() error {
			r, err := c.http.Do(req)
			if err != nil {
				slog.Warn("unable to send request", "error", err, "request", request)
				if err.(*url.Error).Timeout() {
					c.http.CloseIdleConnections()
				}
				return backoff.Permanent(err)
			}
			defer func() {
				if err := r.Body.Close(); err != nil {
					slog.Error("unable to close response body", "error", err, "request", request)
				}
			}()

			if r.StatusCode != http.StatusOK {
				httpErr := httpStatusError{StatusCode: r.StatusCode}
				if httpErr.shouldRetry() {
					slog.Error("unable to send request", "error", httpErr, "request", request)
					return httpErr
				}
				return backoff.Permanent(httpErr)
			}

			if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
				slog.Error("unable to decode response", "error", err, "request", request)
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

func merge(h1, h2 http.Header) http.Header {
	h := h1.Clone()
	for k, vv := range h2 {
		for _, v := range vv {
			h.Add(k, v)
		}
	}
	return h
}
