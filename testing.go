package babyapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Test is meant to be used in external tests to automatically handle setting up routes and using httptest
func TestServe[T Resource](t *testing.T, api *API[T]) (string, func()) {
	server := httptest.NewServer(api.Router())
	return server.URL, server.Close
}

// Test is meant to be used in external tests to automatically handle setting up routes and using httptest
func Test[T Resource](t *testing.T, api *API[T], r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Mount("/", api.Router())
	router.ServeHTTP(w, r)

	return w
}

type mockParent struct {
	name string
}

func (p *mockParent) Router() chi.Router {
	return chi.NewRouter()
}

func (p *mockParent) Route(chi.Router) {
}

func (p *mockParent) Parent() relatedAPI {
	return p
}

func (p *mockParent) Base() string {
	return ""
}

func (p *mockParent) Name() string {
	return p.name
}

func (p *mockParent) GetIDParam(r *http.Request) string {
	return GetIDParam(r, p.name)
}

func (p *mockParent) setParent(relatedAPI) {}

func (p *mockParent) buildClientMap(*Client[*AnyResource], map[string]*Client[*AnyResource], func(*http.Request) error) {
}

// Test is meant to be used in external tests of nested APIs
func TestWithParentRoute[T, P Resource](t *testing.T, api *API[T], parent P, parentName, parentBasePath string, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	api.setParent(&mockParent{parentName})

	router := chi.NewRouter()
	api.defaultMiddleware(router)
	router.Route(fmt.Sprintf("%s/{%s}", parentBasePath, IDParamKey(parentName)), func(r chi.Router) {
		r.Mount("/", api.Router())
	})

	router.ServeHTTP(w, r.WithContext(context.WithValue(context.Background(), ContextKey(parentName), parent)))

	return w
}
