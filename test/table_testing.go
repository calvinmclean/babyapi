package babytest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/stretchr/testify/require"
)

// PreviousResponseGetter is used to get the output of previous tests in a TableTest
type PreviousResponseGetter func(testName string) *babyapi.Response[*babyapi.AnyResource]

// Request contains the necessary details to make a test request to the API. The Func fields allow dynamically
// creating parts of the request. When used in a TableTest, a PreviousResponseGetter is provided so you can get
// IDs from previous responses or use other details. When not used in a table test, this will always be nil
type Request struct {
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

// ExpectedResponse sets up the expectations when running a test
type ExpectedResponse struct {
	// NoBody sets the expectation that the response will have an empty body. This is used because leaving Body
	// empty will just skip the test, not assert the response is empty
	NoBody bool
	// Body is the expected response body string
	Body string
	// BodyRegexp allows comparing a request body by regex
	BodyRegexp string
	// Status is the expected HTTP response code
	Status int
	// Error is an expected error string to be returned by the client
	Error string
	// Assert allows setting a function for custom assertions after making a request
	Assert func(*babyapi.Response[*babyapi.AnyResource])
}

// Test is a single test step that executes the provided ClientRequest or RequestFunc and compares to the
// ExpectedResponse
type Test struct {
	Name string

	// RequestFunc is used to create an *http.Request from the provided URL. Mutually exclusive with ClientRequest
	RequestFunc func(getResponse PreviousResponseGetter, url string) *http.Request

	// ClientRequest uses the provided Request to execute the relevant Client method. This will simultaneously test
	// the client and server. Mutually exclusive with RequestFunc
	ClientRequest *Request

	// ClientName is the name of the API, or child API, which should be used to execute this test. Leave empty
	// to use the default provided API Client. When set, CreateClientMap is used to create a map of child clients
	// This is only available for TableTest because it has the ClientMap
	ClientName string

	// Expected response to compare
	ExpectedResponse
}

// Run will execute a Test using t.Run to run with the test name. The API is expected to already be running. If your
// test uses any PreviousResponseGetter, it will have a nil panic since that is used only for TableTest
func (tt Test) Run(t *testing.T, client *babyapi.Client[*babyapi.AnyResource]) {
	t.Run(tt.Name, func(t *testing.T) {
		if tt.ClientName != "" {
			t.Errorf("cannot use ClientName field when executing without TableTest")
			return
		}

		_ = tt.run(t, client, nil)
	})
}

func (tt Test) run(t *testing.T, client *babyapi.Client[*babyapi.AnyResource], getResponse PreviousResponseGetter) *babyapi.Response[*babyapi.AnyResource] {
	if tt.RequestFunc != nil && tt.ClientRequest != nil {
		t.Error("invalid test: defines RequestFunc and ClientRequest")
		t.FailNow()
	}

	var r *babyapi.Response[*babyapi.AnyResource]
	var err error
	switch {
	case tt.RequestFunc != nil:
		r, err = tt.requestTest(t, client, getResponse)
	case tt.ClientRequest != nil:
		r, err = tt.clientTest(t, client, getResponse)
	}

	tt.assertError(t, err)
	tt.assertBody(t, r)

	if tt.Assert != nil {
		tt.Assert(r)
	}

	return r
}

// requestTest executes the RequestFunc of a Test
func (tt Test) requestTest(t *testing.T, client *babyapi.Client[*babyapi.AnyResource], getResponse PreviousResponseGetter) (*babyapi.Response[*babyapi.AnyResource], error) {
	url, err := client.URL("")
	require.NoError(t, err)

	return client.MakeRequest(tt.RequestFunc(getResponse, url), 0)
}

// clientTest executes the ClientRequest of a Test
func (tt Test) clientTest(t *testing.T, client *babyapi.Client[*babyapi.AnyResource], getResponse PreviousResponseGetter) (*babyapi.Response[*babyapi.AnyResource], error) {
	id := tt.ClientRequest.ID
	if tt.ClientRequest.IDFunc != nil {
		id = tt.ClientRequest.IDFunc(getResponse)
	}

	body := tt.ClientRequest.Body
	if tt.ClientRequest.BodyFunc != nil {
		body = tt.ClientRequest.BodyFunc(getResponse)
	}

	rawQuery := tt.ClientRequest.RawQuery
	if tt.ClientRequest.RawQueryFunc != nil {
		rawQuery = tt.ClientRequest.RawQueryFunc(getResponse)
	}
	if rawQuery != "" {
		client.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = rawQuery
			return nil
		})
		defer client.SetRequestEditor(babyapi.DefaultRequestEditor)
	}

	parentIDs := tt.ClientRequest.ParentIDs
	if tt.ClientRequest.ParentIDsFunc != nil {
		parentIDs = tt.ClientRequest.ParentIDsFunc(getResponse)
	}

	// TODO: Can't use GetAll because it doesn't return *babyapi.Response[T]
	switch tt.ClientRequest.Method {
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

func (tt Test) assertError(t *testing.T, err error) {
	if tt.ExpectedResponse.Error == "" {
		require.NoError(t, err)
		return
	}

	require.Error(t, err)
	require.Equal(t, tt.ExpectedResponse.Error, err.Error())

	var errResp *babyapi.ErrResponse
	if errors.As(err, &errResp) {
		require.Equal(t, tt.ExpectedResponse.Status, errResp.HTTPStatusCode)
	}
}

func (tt Test) assertBody(t *testing.T, r *babyapi.Response[*babyapi.AnyResource]) {
	switch {
	case tt.NoBody:
		require.Equal(t, http.NoBody, r.Response.Body)
		require.Equal(t, "", r.Body)
	case tt.BodyRegexp != "":
		require.Regexp(t, tt.ExpectedResponse.BodyRegexp, strings.TrimSpace(r.Body))
	case tt.Body != "":
		require.Equal(t, tt.ExpectedResponse.Body, strings.TrimSpace(r.Body))
	}
}

// tableTest allows
type tableTest[T babyapi.Resource] struct {
	api     *babyapi.API[T]
	tests   []Test
	results map[string]*babyapi.Response[*babyapi.AnyResource]
}

// RunTableTest will start the provided API and execute all provided tests in-order. This allows the usage of a
// PreviousResponseGetter in each test to access data from previous tests. The API's ClientMap is used to execute
// tests with child clients if the test uses ClientName field
func RunTableTest[T babyapi.Resource](t *testing.T, api *babyapi.API[T], tests []Test) {
	tt := tableTest[T]{
		api,
		tests,
		map[string]*babyapi.Response[*babyapi.AnyResource]{},
	}

	tt.run(t)
}

func (ts tableTest[T]) run(t *testing.T) {
	client, stop := NewTestAnyClient[T](t, ts.api)
	defer stop()

	for _, tt := range ts.tests {
		t.Run(tt.Name, func(t *testing.T) {
			testClient := client

			if tt.ClientName != "" {
				clientMap := ts.api.CreateClientMap(client)
				var ok bool
				testClient, ok = clientMap[tt.ClientName]
				if !ok {
					t.Errorf("missing subclient for key %q. available clients are %v", tt.ClientName, clientMap)
				}
			}

			ts.results[tt.Name] = tt.run(t, testClient, ts.getResponse)
		})
	}
}

func (ts tableTest[T]) getResponse(testName string) *babyapi.Response[*babyapi.AnyResource] {
	return ts.results[testName]
}
