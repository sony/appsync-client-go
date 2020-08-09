package graphql

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	statusOK  = http.StatusOK
	errors    = []interface{}{"error1", "error2"}
	posttests = []struct {
		want Response
	}{
		{want: Response{&statusOK, "data", nil, nil}},
		{want: Response{&statusOK, "", &errors, nil}},
	}
)

func checkResponse(t *testing.T, want, got *Response) {
	t.Helper()
	if got == nil {
		t.Fatal("got is nil")
	}
	if got.StatusCode == nil {
		t.Fatal("StatusCode is nil")
	}
	if *want.StatusCode != *got.StatusCode {
		t.Fatalf("StatusCode is %v", *got.StatusCode)
	}
	if want.Data != got.Data {
		t.Fatalf("want:%+v, got: %+v", want.Data, got.Data)
	}
	if want.Errors != got.Errors {
		if !reflect.DeepEqual(*want.Errors, *got.Errors) {
			t.Fatalf("want:%+v, got: %+v", want.Errors, got.Errors)
		}
	}
}

func checkTimeoutError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("err is nil")
	}
	if !strings.HasPrefix(err.Error(), "Post") {
		t.Fatal(err)
	}
	errStr := err.Error()
	if !strings.HasSuffix(errStr, context.DeadlineExceeded.Error()) &&
		!strings.HasSuffix(errStr, "i/o timeout") {
		t.Fatal(err)
	}
}

func checkCanceledError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("err is nil")
	}
	if !strings.HasPrefix(err.Error(), "Post") {
		t.Fatal(err)
	}
	if !strings.HasSuffix(err.Error(), "context canceled") {
		t.Fatal(err)
	}
}

func TestPost(t *testing.T) {
	for _, tt := range posttests {
		server := newEchoServer(tt.want)
		defer server.Close()

		client := NewClient(server.URL)
		got, err := client.Post(http.Header{}, PostRequest{})
		if err != nil {
			t.Fatal(err)
		}
		checkResponse(t, &tt.want, got)

		resCh := make(chan *Response, 1)
		errCh := make(chan error, 1)
		cancel, err := client.PostAsync(http.Header{}, PostRequest{},
			func(r *Response, err error) {
				resCh <- r
				errCh <- err
			})
		if err != nil {
			t.Fatal(err)
		}
		if cancel == nil {
			t.Fatal("cancel is nil")
		}
		checkResponse(t, &tt.want, <-resCh)
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

func TestInternalServerError(t *testing.T) {
	server := newInternalServerErrorServer()
	defer server.Close()

	internalServerError := http.StatusInternalServerError
	errors = []interface{}{http.StatusText(internalServerError)}
	want := Response{
		StatusCode: &internalServerError,
		Data:       nil,
		Errors:     &errors,
		Extensions: nil,
	}

	client := NewClient(server.URL, WithMaxElapsedTime(1*time.Microsecond))
	got, err := client.Post(http.Header{}, PostRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("got is nil")
	}
	checkResponse(t, &want, got)

	resCh := make(chan *Response, 1)
	errCh := make(chan error, 1)
	cancel, err := client.PostAsync(http.Header{}, PostRequest{},
		func(r *Response, err error) {
			resCh <- r
			errCh <- err
		})
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil {
		t.Fatal("cancel is nil")
	}
	checkResponse(t, &want, <-resCh)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestUnauthorizedError(t *testing.T) {
	server := newUnauthorizedErrorServer()
	defer server.Close()

	unauthorizedError := http.StatusUnauthorized
	errors = []interface{}{http.StatusText(unauthorizedError)}
	want := Response{
		StatusCode: &unauthorizedError,
		Data:       nil,
		Errors:     &errors,
		Extensions: nil,
	}

	client := NewClient(server.URL, WithMaxElapsedTime(1*time.Microsecond))
	got, err := client.Post(http.Header{}, PostRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("got is nil")
	}
	checkResponse(t, &want, got)

	resCh := make(chan *Response, 1)
	errCh := make(chan error, 1)
	cancel, err := client.PostAsync(http.Header{}, PostRequest{},
		func(r *Response, err error) {
			resCh <- r
			errCh <- err
		})
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil {
		t.Fatal("cancel is nil")
	}
	checkResponse(t, &want, <-resCh)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestTimeout(t *testing.T) {
	server := newDelayServer(3 * time.Microsecond)
	defer server.Close()

	client := NewClient(server.URL, WithTimeout(1*time.Microsecond))
	res, got := client.Post(http.Header{}, PostRequest{})
	if res != nil {
		t.Fatal(res)
	}
	checkTimeoutError(t, got)

	resCh := make(chan *Response, 1)
	errCh := make(chan error, 1)
	cancel, err := client.PostAsync(http.Header{}, PostRequest{},
		func(r *Response, err error) {
			resCh <- r
			errCh <- err
		})
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil {
		t.Fatal("cancel is nil")
	}
	checkTimeoutError(t, <-errCh)
	if res = <-resCh; res != nil {
		t.Fatal(res)
	}
}

func TestCancel(t *testing.T) {
	server := newDelayServer(3 * time.Microsecond)
	defer server.Close()

	resCh := make(chan *Response, 1)
	errCh := make(chan error, 1)
	client := NewClient(server.URL)
	cancel, err := client.PostAsync(http.Header{}, PostRequest{},
		func(r *Response, err error) {
			resCh <- r
			errCh <- err
		})
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil {
		t.Fatal("cancel is nil")
	}
	cancel()
	if res := <-resCh; res != nil {
		t.Fatal(res)
	}
	checkCanceledError(t, <-errCh)
}
