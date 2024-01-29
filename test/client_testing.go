package babytest

import (
	"testing"

	"github.com/calvinmclean/babyapi"
)

// NewTestAnyClient runs the API using TestServe and returns a Client with the correct base URL. It uses AnyClient for an
// AnyResource so it is compatible with table-driven tests
func NewTestAnyClient[T babyapi.Resource](t *testing.T, api *babyapi.API[T]) (*babyapi.Client[*babyapi.AnyResource], func()) {
	serverURL, stop := TestServe[T](t, api)
	return api.AnyClient(serverURL), stop
}

// NewTestClient runs the API using TestServe and returns a Client with the correct base URL
func NewTestClient[T babyapi.Resource](t *testing.T, api *babyapi.API[T]) (*babyapi.Client[T], func()) {
	serverURL, stop := TestServe[T](t, api)
	return api.Client(serverURL), stop
}
