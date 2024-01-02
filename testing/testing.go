package babyapi_testing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/go-chi/chi/v5"
)

// Test is meant to be used in external tests to automatically handle setting up routes and using httptest
func TestServe[T babyapi.Resource](t *testing.T, api *babyapi.API[T]) (string, func()) {
	server := httptest.NewServer(api.Router())
	return server.URL, server.Close
}

// Test is meant to be used in external tests to automatically handle setting up routes and using httptest
func Test[T babyapi.Resource](t *testing.T, api *babyapi.API[T], r *http.Request) *httptest.ResponseRecorder {
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

func (p *mockParent) Parent() babyapi.RelatedAPI {
	return p
}

func (p *mockParent) Base() string {
	return ""
}

func (p *mockParent) Name() string {
	return p.name
}

func (p *mockParent) GetIDParam(r *http.Request) string {
	return babyapi.GetIDParam(r, p.name)
}

func (p *mockParent) setParent(relatedAPI) {}
func (p *mockParent) isRoot() bool         { return false }

func (p *mockParent) buildClientMap(*babyapi.Client[*babyapi.AnyResource], map[string]*babyapi.Client[*babyapi.AnyResource], func(*http.Request) error) {
}

type relatedAPI interface {
	setParent(babyapi.RelatedAPI)
	buildClientMap(*babyapi.Client[*babyapi.AnyResource], map[string]*babyapi.Client[*babyapi.AnyResource], func(*http.Request) error)
}

// Test is meant to be used in external tests of nested APIs
func TestWithParentRoute[T, P babyapi.Resource](t *testing.T, api *babyapi.API[T], parent P, parentName, parentBasePath string, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	relAPI, ok := any(api).(relatedAPI)
	if !ok {
		panic(fmt.Sprintf("incompatible type for child API: %T", api))
	}

	relAPI.setParent(&mockParent{parentName})

	router := chi.NewRouter()
	api.DefaultMiddleware(router)
	router.Route(fmt.Sprintf("%s/{%s}", parentBasePath, babyapi.IDParamKey(parentName)), func(r chi.Router) {
		r.Mount("/", api.Router())
	})

	router.ServeHTTP(w, r.WithContext(context.WithValue(context.Background(), babyapi.ContextKey(parentName), parent)))

	return w
}
