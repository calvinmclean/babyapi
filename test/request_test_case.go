package babytest

import (
	"context"
	"net/http"
	"testing"

	"github.com/calvinmclean/babyapi"
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

func (tt RequestTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*Response[T], error) {
	id := tt.ID
	if tt.IDFunc != nil {
		id = tt.IDFunc(getResponse)
	}

	body := tt.Body
	if tt.BodyFunc != nil {
		body = tt.BodyFunc(getResponse)
	}

	requestEditor := babyapi.DefaultRequestEditor

	rawQuery := tt.RawQuery
	if tt.RawQueryFunc != nil {
		rawQuery = tt.RawQueryFunc(getResponse)
	}
	if rawQuery != "" {
		requestEditor = func(r *http.Request) error {
			r.URL.RawQuery = rawQuery
			return nil
		}
	}

	parentIDs := tt.ParentIDs
	if tt.ParentIDsFunc != nil {
		parentIDs = tt.ParentIDsFunc(getResponse)
	}

	var r any
	var err error
	switch tt.Method {
	case babyapi.MethodGetAll:
		r, err = client.GetAllWithEditor(context.Background(), rawQuery, requestEditor, parentIDs...)
	case http.MethodPost:
		r, err = client.PostRawWithEditor(context.Background(), body, requestEditor, parentIDs...)
	case http.MethodGet:
		r, err = client.GetWithEditor(context.Background(), id, requestEditor, parentIDs...)
	case http.MethodPut:
		r, err = client.PutRawWithEditor(context.Background(), id, body, requestEditor, parentIDs...)
	case http.MethodPatch:
		r, err = client.PatchRawWithEditor(context.Background(), id, body, requestEditor, parentIDs...)
	case http.MethodDelete:
		r, err = client.DeleteWithEditor(context.Background(), id, requestEditor, parentIDs...)
	}

	switch v := r.(type) {
	case *babyapi.Response[T]:
		return &Response[T]{Response: v}, err
	case *babyapi.Response[*babyapi.ResourceList[T]]:
		return &Response[T]{GetAllResponse: v}, err
	}

	return nil, err
}

// RequestFuncTest is used to create an *http.Request from the provided address and create a response for assertions
type RequestFuncTest[T babyapi.Resource] func(getResponse PreviousResponseGetter, address string) *http.Request

var _ Test[*babyapi.AnyResource] = RequestFuncTest[*babyapi.AnyResource](func(getResponse PreviousResponseGetter, url string) *http.Request {
	return nil
})

func (tt RequestFuncTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*Response[T], error) {
	r := tt(getResponse, client.Address)

	if r.Method == babyapi.MethodGetAll {
		r.Method = http.MethodGet
		resp, err := babyapi.MakeRequest[*babyapi.ResourceList[T]](r, http.DefaultClient, 0, func(r *http.Request) error {
			return nil
		})
		return &Response[T]{GetAllResponse: resp}, err
	}

	resp, err := client.MakeRequest(r, 0)

	return &Response[T]{Response: resp}, err
}
