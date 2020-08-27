package appsync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mec07/appsync-client-go/graphql"
)

func TestPostReturnsData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := graphql.Response{
			Data:   "data",
			Errors: nil,
		}
		b, err := json.Marshal(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer server.Close()

	api := NewGraphQLClient(graphql.NewClient(server.URL))
	res, err := api.Post(http.Header{}, graphql.PostRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.FailNow()
	}
	if res.Data != "data" {
		t.FailNow()
	}
	if res.Errors != nil {
		t.Fatal(res.Errors)
	}
	if *res.StatusCode != http.StatusOK {
		t.Fatal(res.StatusCode)
	}
}

func TestPostAsyncReturnsData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := graphql.Response{
			Data:   "data",
			Errors: nil,
		}
		b, err := json.Marshal(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer server.Close()

	ch := make(chan ret, 1)
	api := NewGraphQLClient(graphql.NewClient(server.URL))
	cancel, err := api.PostAsync(http.Header{}, graphql.PostRequest{}, func(r *graphql.Response, err error) { ch <- ret{r, err} })
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil {
		t.FailNow()
	}
	ret := <-ch
	if ret.response.Data != "data" {
		t.FailNow()
	}
	if ret.response.Errors != nil {
		t.Fatal(ret.response.Errors)
	}
	if *ret.response.StatusCode != http.StatusOK {
		t.Fatal(ret.response.StatusCode)
	}
}
