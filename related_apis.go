package babyapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RelatedAPI declares a subset of methods from the API struct that are required to enable
// nested/parent-child API relationships
type RelatedAPI interface {
	Router() chi.Router
	Route(chi.Router)
	Base() string
	Name() string
	GetIDParam(*http.Request) string
	Parent() RelatedAPI

	setParent(RelatedAPI)
	buildClientMap(*Client[*AnyResource], map[string]*Client[*AnyResource], func(*http.Request) error)
	isRoot() bool
}

// Parent returns the API's parent API
func (a *API[T]) Parent() RelatedAPI {
	return a.parent
}

// GetParentIDParam reads the URL param from the request to get the ID of the parent resource
func (a *API[T]) GetParentIDParam(r *http.Request) string {
	return a.parent.GetIDParam(r)
}

// AddNestedAPI adds a child API to this API and initializes the parent relationship on the child's side
func (a *API[T]) AddNestedAPI(childAPI RelatedAPI) *API[T] {
	a.subAPIs[childAPI.Name()] = childAPI
	childAPI.setParent(a)

	return a
}

func (a *API[T]) setParent(parent RelatedAPI) {
	a.parent = parent
}

func (a *API[T]) isRoot() bool {
	return a.rootAPI
}
