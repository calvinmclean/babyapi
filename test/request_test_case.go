package babytest

import (
	"context"
	"net/http"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/stretchr/testify/require"
)

// RequestTest contains the necessary details to make a test request to the API. The Func fields allow dynamically
// creating parts of the request. When used in a TableTest, a PreviousResponseGetter is provided so you can get
// IDs from previous responses or use other details. When not used in a table test, this will always be nil
type RequestTest[T babyapi.Resource] struct {
	// HTTP request method/verb
	Method string

	// RawQuery is the query params
	RawQuery string
	// RawQueryFunc returns query params from a function which can access previous test responses
	RawQueryFunc func(getResponse PreviousResponseGetter) string

	// ID is the resource ID used in the request path
	ID string
	// IDFunc returns the resource ID from a function which can access previous test responses
	IDFunc func(getResponse PreviousResponseGetter) string

	// Body is the request body as a string
	Body string
	// BodyFunc returns request body from a function which can access previous test responses
	BodyFunc func(getResponse PreviousResponseGetter) string

	// ParentIDs is a list of parent resource IDs, in order
	ParentIDs []string
	// ParentIDsFunc returns parent resource IDs from a function which can access previous test responses
	ParentIDsFunc func(getResponse PreviousResponseGetter) []string
}

var _ Test[*babyapi.AnyResource] = RequestTest[*babyapi.AnyResource]{}

func (tt RequestTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*babyapi.Response[T], error) {
	id := tt.ID
	if tt.IDFunc != nil {
		id = tt.IDFunc(getResponse)
	}

	body := tt.Body
	if tt.BodyFunc != nil {
		body = tt.BodyFunc(getResponse)
	}

	rawQuery := tt.RawQuery
	if tt.RawQueryFunc != nil {
		rawQuery = tt.RawQueryFunc(getResponse)
	}
	if rawQuery != "" {
		client.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = rawQuery
			return nil
		})
		defer client.SetRequestEditor(babyapi.DefaultRequestEditor)
	}

	parentIDs := tt.ParentIDs
	if tt.ParentIDsFunc != nil {
		parentIDs = tt.ParentIDsFunc(getResponse)
	}

	// TODO: Can't use GetAll because it doesn't return *babyapi.Response[T]
	switch tt.Method {
	case http.MethodPost:
		return client.PostRaw(context.Background(), body, parentIDs...)
	case http.MethodGet:
		return client.Get(context.Background(), id, parentIDs...)
	case http.MethodPut:
		return client.PutRaw(context.Background(), id, body, parentIDs...)
	case http.MethodPatch:
		return client.PatchRaw(context.Background(), id, body, parentIDs...)
	case http.MethodDelete:
		return client.Delete(context.Background(), id, parentIDs...)
	}

	return nil, nil
}

// RequestFuncTest is used to create an *http.Request from the provided URL and create a response for assertions
type RequestFuncTest[T babyapi.Resource] func(getResponse PreviousResponseGetter, url string) *http.Request

var _ Test[*babyapi.AnyResource] = RequestFuncTest[*babyapi.AnyResource](func(getResponse PreviousResponseGetter, url string) *http.Request {
	return nil
})

func (tt RequestFuncTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*babyapi.Response[T], error) {
	url, err := client.URL("")
	require.NoError(t, err)

	return client.MakeRequest(tt(getResponse, url), 0)
}
