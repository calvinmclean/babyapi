package babytest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/stretchr/testify/require"
)

// MethodGetAll is the same as http.MethodGet, but can be used in a RequestTest to get all resources
const MethodGetAll = "GetAll"

// TestCase is a single test step that executes the provided ClientRequest or RequestFunc and compares to the
// ExpectedResponse
type TestCase[T babyapi.Resource] struct {
	Name string

	// Test is the runnable test to execute before assertions
	Test Test[T]

	// ClientName is the name of the API, or child API, which should be used to execute this test. Leave empty
	// to use the default provided API Client. When set, CreateClientMap is used to create a map of child clients
	// This is only available for TableTest because it has the ClientMap
	ClientName string

	// Assert allows setting a function for custom assertions after making a request. It is part of the Test instead
	// of the ExpectedResponse because it needs the type parameter
	Assert func(*Response[T])

	// Expected response to compare
	ExpectedResponse
}

// Test is an interface that allows executing different types of tests before running assertions
type Test[T babyapi.Resource] interface {
	Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*Response[T], error)
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
}

// Response wraps a *babyapi.Response and a *ResourceList response to enable GetAll/List endpoints
// GetAll/List requests can be made using babytest.MethodGetAll and the response will be in this GetAllResponse field
type Response[T babyapi.Resource] struct {
	*babyapi.Response[T]
	GetAllResponse *babyapi.Response[*babyapi.ResourceList[T]]
}

// Run will execute a Test using t.Run to run with the test name. The API is expected to already be running. If your
// test uses any PreviousResponseGetter, it will have a nil panic since that is used only for TableTest
func (tt TestCase[T]) Run(t *testing.T, client *babyapi.Client[T]) {
	t.Run(tt.Name, func(t *testing.T) {
		if tt.ClientName != "" {
			t.Errorf("cannot use ClientName field when executing without TableTest")
			return
		}

		_ = tt.run(t, client, nil)
	})
}

// RunWithResponse is the same as Run but returns the Response
func (tt TestCase[T]) RunWithResponse(t *testing.T, client *babyapi.Client[T]) *Response[T] {
	var resp *Response[T]
	t.Run(tt.Name, func(t *testing.T) {
		if tt.ClientName != "" {
			t.Errorf("cannot use ClientName field when executing without TableTest")
			return
		}

		resp = tt.run(t, client, nil)
	})

	return resp
}

func (tt TestCase[T]) run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) *Response[T] {
	r, err := tt.Test.Run(t, client, getResponse)

	skipBody := tt.assertError(t, err)
	if !skipBody {
		tt.assertResponse(t, r)
	}

	if tt.Assert != nil {
		tt.Assert(r)
	}

	return r
}

// assertError returns true when the err is a *babyapi.ErrResponse since we compare the body here and want to skip assertBody
func (tt TestCase[T]) assertError(t *testing.T, err error) bool {
	if tt.ExpectedResponse.Error == "" {
		require.NoError(t, err)
		return false
	}

	require.Error(t, err)
	require.Equal(t, tt.ExpectedResponse.Error, err.Error())

	var errResp *babyapi.ErrResponse
	if errors.As(err, &errResp) {
		require.Equal(t, tt.ExpectedResponse.Status, errResp.HTTPStatusCode)

		// Compare JSON response to expected Body
		data, jsonErr := json.Marshal(errResp)
		require.NoError(t, jsonErr)
		require.Equal(t, tt.ExpectedResponse.Body, string(data))

		return true
	}

	return false
}

func (tt TestCase[T]) assertResponse(t *testing.T, r *Response[T]) {
	var body string
	var resp *http.Response
	switch {
	case r.GetAllResponse != nil:
		body = r.GetAllResponse.Body
		resp = r.GetAllResponse.Response
	case r.Response != nil:
		body = r.Response.Body
		resp = r.Response.Response
	}

	require.NotNil(t, r)

	require.Equal(t, tt.ExpectedResponse.Status, resp.StatusCode)

	switch {
	case tt.NoBody:
		require.Equal(t, http.NoBody, resp.Body)
		require.Equal(t, "", body)
	case tt.BodyRegexp != "":
		require.Regexp(t, tt.ExpectedResponse.BodyRegexp, strings.TrimSpace(body))
	case tt.Body != "":
		if r == nil {
			t.Error("response is nil")
			return
		}
		require.Equal(t, tt.ExpectedResponse.Body, strings.TrimSpace(body))
	}
}
