package babyapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// relatedAPI declares a subset of methods from the API struct that are required to enable
// nested/parent-child API relationships
type relatedAPI interface {
	Router() chi.Router
	Route(chi.Router)
	Name() string
	GetIDParam(*http.Request) string
	Parent() relatedAPI

	setParent(relatedAPI)
}

// Parent returns the API's parent API
func (a *API[T]) Parent() relatedAPI {
	return a.parent
}

// GetParentIDParam reads the URL param from the request to get the ID of the parent resource
func (a *API[T]) GetParentIDParam(r *http.Request) string {
	return a.parent.GetIDParam(r)
}

// AddNestedAPI adds a child API to this API and initializes the parent relationship on the child's side
func (a *API[T]) AddNestedAPI(childAPI relatedAPI) {
	a.subAPIs[childAPI.Name()] = childAPI
	childAPI.setParent(a)
}

func (a *API[T]) setParent(parent relatedAPI) {
	a.parent = parent
}
