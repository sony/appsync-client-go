package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
)

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
		opt.Apply(c)
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

	req.Header.Set("Content-Type", "application/json")
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
		var errToCallback error
		_ = backoff.Retry(func() error {
			r, e := http.DefaultClient.Do(req)
			if e != nil {
				log.Println(e)
				errToCallback = e
				if e.(*url.Error).Timeout() {
					if t, ok := http.DefaultTransport.(*http.Transport); ok {
						t.CloseIdleConnections()
					}
				}
				return nil
			}
			defer func() {
				if e := r.Body.Close(); e != nil {
					log.Println(e)
				}
			}()

			body, e := ioutil.ReadAll(r.Body)
			if e != nil {
				log.Println(e)
				errToCallback = e
				return nil
			}

			if r.StatusCode != http.StatusOK {
				response.StatusCode = &r.StatusCode
				errors := []interface{}{http.StatusText(r.StatusCode)}
				response.Errors = &errors
				// should retry
				if r.StatusCode == http.StatusInternalServerError || r.StatusCode == http.StatusServiceUnavailable {
					e := fmt.Errorf(http.StatusText(r.StatusCode))
					log.Println(e)
					return e
				}
				return nil
			}

			if e := json.Unmarshal(body, &response); e != nil {
				log.Println(e)
				errToCallback = e
				return nil
			}
			response.StatusCode = &r.StatusCode

			return nil
		}, b)

		if errToCallback != nil {
			callback(nil, errToCallback)
			return
		}

		callback(&response, nil)
	}()

	return cancel, nil
}
