package babyapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// IDParamKey gets the chi URL param key used for the ID of a resource
func IDParamKey(name string) string {
	return fmt.Sprintf("%sID", name)
}

// GetIDParam gets resource ID from the request URL for a resource by name
func GetIDParam(r *http.Request, name string) string {
	return chi.URLParam(r, IDParamKey(name))
}

// IDParamKey gets the chi URL param key used for this API
func (a *API[T]) IDParamKey() string {
	return IDParamKey(a.name)
}

// GetIDParam gets resource ID from the request URL for this API's resource
func (a *API[T]) GetIDParam(r *http.Request) string {
	return GetIDParam(r, a.name)
}

// GetRequestedResourceAndDo is a wrapper that handles getting a resource from storage based on the ID in the request URL
// and rendering the response. This is useful for imlementing a CustomIDRoute
func (a *API[T]) GetRequestedResourceAndDo(do func(*http.Request, T) (render.Renderer, *ErrResponse)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := GetLoggerFromContext(r.Context())

		resource, httpErr := a.GetRequestedResource(r)
		if httpErr != nil {
			logger.Error("error getting requested resource", "error", httpErr.Error())
			_ = render.Render(w, r, httpErr)
			return
		}

		resp, httpErr := do(r, resource)
		if httpErr != nil {
			_ = render.Render(w, r, httpErr)
			return
		}

		if resp == nil {
			render.NoContent(w, r)
			return
		}

		err := render.Render(w, r, resp)
		if err != nil {
			logger.Error("unable to render response", "error", err)
			_ = render.Render(w, r, ErrRender(err))
		}
	}
}

// ReadRequestBodyAndDo is a wrapper that handles decoding the request body into the resource type and rendering a response
func (a *API[T]) ReadRequestBodyAndDo(do func(*http.Request, T) (T, *ErrResponse)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := GetLoggerFromContext(r.Context())

		resource, httpErr := a.GetFromRequest(r)
		if httpErr != nil {
			logger.Error("invalid request to create resource", "error", httpErr.Error())
			_ = render.Render(w, r, httpErr)
			return
		}

		resp, httpErr := do(r, resource)
		if httpErr != nil {
			_ = render.Render(w, r, httpErr)
			return
		}

		if resp == *new(T) {
			render.NoContent(w, r)
			return
		}

		err := render.Render(w, r, a.responseWrapper(resp))
		if err != nil {
			logger.Error("unable to render response", "error", err)
			_ = render.Render(w, r, ErrRender(err))
		}
	}
}

// GetFromRequest will read the API's resource type from the request body or request context
func (a *API[T]) GetFromRequest(r *http.Request) (T, *ErrResponse) {
	resource := a.GetRequestBodyFromContext(r.Context())
	if resource != *new(T) {
		return resource, nil
	}

	resource = a.instance()
	err := render.Bind(r, resource)
	if err != nil {
		return *new(T), ErrInvalidRequest(err)
	}

	return resource, nil
}

// GetRequestedResource reads the API's resource from storage based on the ID in the request URL
func (a *API[T]) GetRequestedResource(r *http.Request) (T, *ErrResponse) {
	id := a.GetIDParam(r)

	resource, err := a.storage.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return *new(T), ErrNotFoundResponse
		}

		return *new(T), InternalServerError(err)
	}

	return resource, nil
}

// ServerSentEvent is a simple struct that represents an event used in HTTP event stream
type ServerSentEvent struct {
	Event string
	Data  string
}

// Write will write the ServerSentEvent to the HTTP response stream and flush. It removes all newlines
// in the event data
func (sse *ServerSentEvent) Write(w http.ResponseWriter) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sse.Event, strings.ReplaceAll(sse.Data, "\n", ""))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// AddServerSentEventHandler is a shortcut for HandleServerSentEvents that automatically creates and returns
// the events channel and adds a custom handler for GET requests matching the provided pattern
func (a *API[T]) AddServerSentEventHandler(pattern string) chan *ServerSentEvent {
	events := make(chan *ServerSentEvent)

	a.AddCustomRoute(chi.Route{
		Pattern: pattern,
		Handlers: map[string]http.Handler{
			http.MethodGet: a.HandleServerSentEvents(events),
		},
	})

	return events
}

// HandleServerSentEvents is a handler function that will listen on the provided channel and write events
// to the HTTP response
func (a *API[T]) HandleServerSentEvents(events <-chan *ServerSentEvent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "text/event-stream")

		for {
			select {
			case e := <-events:
				e.Write(w)
			case <-r.Context().Done():
				return
			case <-a.Done():
				return
			}
		}
	}
}
