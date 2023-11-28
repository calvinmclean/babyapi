package babyapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/rs/xid"
)

// Resource is an interface/constraint used for API resources. In order to use API, you must have types that implement this.
// It enables HTTP request/response handling and getting resources by ID
type Resource interface {
	comparable
	// Renderer is used to control the output behavior when creating a response.
	// Use this for any after-request logic or response modifications
	render.Renderer

	// Binder is used to control the input behavior, after decoding the request.
	// Use it for input validation or additional modification of the resource using request headers or other params
	render.Binder

	GetID() string
}

// Patcher is used to optionally-enable PATCH endpoint. Since the library cannot generically modify resources without using
// reflection, implement Patch function to use the input to modify the receiver
type Patcher[T Resource] interface {
	Patch(T) *ErrResponse
}

// DefaultResource implements Resource and uses the provided ID type. Extending this type is the easiest way to implement a
// Resource based around the provided ID type
type DefaultResource struct {
	ID ID `json:"id"`
}

// NewDefaultResource creates a DefaultResource with a new random ID
func NewDefaultResource() DefaultResource {
	return DefaultResource{NewID()}
}

var _ render.Renderer = &DefaultResource{}
var _ render.Binder = &DefaultResource{}

func (dr *DefaultResource) GetID() string {
	return dr.ID.String()
}

func (*DefaultResource) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (dr *DefaultResource) Bind(r *http.Request) error {
	err := dr.ID.Bind(r)
	if err != nil {
		return err
	}
	return nil
}

// ID is a type that can be optionally used to improve Resources and their APIs. It uses xid to create unique
// identifiers and implements a custom Bind method to:
//   - Disallow POST requests with IDs
//   - Automatically set new ID on POSTed resources
//   - Enforce that ID is set
//   - Do not allow changing ID with PATCH
type ID struct {
	xid.ID
}

func NewID() ID {
	return ID{xid.New()}
}

func (id *ID) Bind(r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		if !id.ID.IsNil() {
			return errors.New("unable to manually set ID")
		}

		id.ID = xid.New()
		fallthrough
	case http.MethodPut:
		if id.ID.IsNil() {
			return errors.New("missing required id field")
		}
	case http.MethodPatch:
		if !id.ID.IsNil() {
			return errors.New("updating ID is not allowed")
		}
	}

	return nil
}

// ResourceList is used to automatically enable the GetAll endpoint that returns an array of Resources
type ResourceList[T render.Renderer] struct {
	Items []T `json:"items"`
}

func (rl *ResourceList[T]) Render(w http.ResponseWriter, r *http.Request) error {
	for _, item := range rl.Items {
		err := item.Render(w, r)
		if err != nil {
			return fmt.Errorf("error rendering item: %w", err)
		}
	}
	return nil
}
