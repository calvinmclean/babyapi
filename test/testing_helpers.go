package babytest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

// Test is meant to be used in external tests to automatically handle setting up routes and using httptest
func TestServe[T babyapi.Resource](t *testing.T, api *babyapi.API[T]) (string, func()) {
	router, err := api.Router()
	require.NoError(t, err)
	server := httptest.NewServer(router)
	return server.URL, server.Close
}

// TestRequest is meant to be used in external tests to automatically handle setting up routes and using httptest
func TestRequest[T babyapi.Resource](t *testing.T, api *babyapi.API[T], r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	apiRouter, err := api.Router()
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Mount("/", apiRouter)
	router.ServeHTTP(w, r)

	return w
}

// TestWithParentRoute allows testing a child API independently with a pre-configured parent resource in the context to
// mock a middleware
func TestWithParentRoute[T, P babyapi.Resource](t *testing.T, api *babyapi.API[T], parent P, parentName, parentBasePath string, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	parentAPI := babyapi.NewAPI[P](parentName, parentBasePath, func() P { return parent })
	parentAPI.AddNestedAPI(api)

	apiRouter, err := api.Router()
	require.NoError(t, err)

	router := chi.NewRouter()
	api.ApplyDefaultMiddleware(router)
	router.Route(fmt.Sprintf("%s/{%s}", parentBasePath, babyapi.IDParamKey(parentName)), func(r chi.Router) {
		r.Mount("/", apiRouter)
	})

	router.ServeHTTP(w, r.WithContext(context.WithValue(context.Background(), babyapi.ContextKey(parentName), parent)))

	return w
}

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
