package extensions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/calvinmclean/babyapi"
	"github.com/go-chi/render"
)

// HATEOAS is an babyapi Extension that wraps any babyapi Resource with automatic HATEOAS style links. By default,
// it can automatically add links for "self" and child resources when getting a resource by ID.
// This will add the "links" field to the JSON response, so be wary of conflicting with that key. Set the LinkKey
// to an alternative option if "links" would conflict.
type HATEOAS[T babyapi.Resource] struct {
	// LinkKey will override the default "links" key that the link map is represented by
	LinkKey string
	// CustomLinks runs as part of the Render process for responses and allows adding or overwriting links
	CustomLinks func(*http.Request) map[string]string
}

// Apply the custom ResponseWrapper
func (h HATEOAS[T]) Apply(api *babyapi.API[T]) error {
	api.SetResponseWrapper(h.ResponseWrapper(api))

	return nil
}

func (h HATEOAS[T]) ResponseWrapper(api *babyapi.API[T]) func(resource T) render.Renderer {
	return func(resource T) render.Renderer {
		childPaths := map[string]string{}
		for _, child := range api.ChildAPIs() {
			childPaths[child.Name()] = child.Base()
		}
		return &HATEOASResponse[T]{
			resource,
			map[string]string{},
			childPaths,
			h.LinkKey,
			h.CustomLinks,
		}
	}
}

// HATEOASResponse wraps a babyapi Resource with additional links. The custom MarshalJSON function will handle flattening the
// Resource so the added "links" field is at the same level as other fields of the Resource
type HATEOASResponse[T babyapi.Resource] struct {
	Resource T
	Links    map[string]string `json:"links"`

	childPaths  map[string]string
	linkKey     string
	customLinks func(*http.Request) map[string]string
}

// Render will populate the HATEOAS response with the appropriate links
func (h *HATEOASResponse[T]) Render(w http.ResponseWriter, r *http.Request) error {
	if h.linkKey == "" {
		h.linkKey = "links"
	}

	self := r.URL.Path

	isGetAll := r.Method == http.MethodGet && !strings.HasSuffix(self, h.Resource.GetID())
	isPost := r.Method == http.MethodPost
	if isPost || isGetAll {
		self += "/" + h.Resource.GetID()
	}

	h.Links = map[string]string{
		"self": self,
	}

	for name, path := range h.childPaths {
		h.Links[name] = self + path
	}

	if h.customLinks != nil {
		for name, path := range h.customLinks(r) {
			h.Links[name] = path
		}
	}

	return nil
}

// MarshalJSON will handle flattening the Links and Resource by marshalling separately and then combining. If the
// Resource has its own links field, this will not work unless the LinkKey is overridden
func (h *HATEOASResponse[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(h.Resource)
	if err != nil {
		return data, err
	}

	linksJSON, err := json.Marshal(h.Links)
	if err != nil {
		return data, err
	}

	data = data[0:bytes.LastIndex(data, []byte{'}'})]
	data = append(data, []byte(fmt.Sprintf(`,"%s":`, h.linkKey))...)
	data = append(data, linksJSON...)
	data = append(data, []byte("}\n")...)

	return data, nil
}
