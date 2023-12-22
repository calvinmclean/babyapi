package babyapi

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// API encapsulates all handlers and other pieces of code required to run the CRUID API based on
// the provided Resource type
type API[T Resource] struct {
	name string
	base string

	subAPIs       map[string]RelatedAPI
	middlewares   []func(http.Handler) http.Handler
	idMiddlewares []func(http.Handler) http.Handler
	storage       Storage[T]

	server *http.Server
	quit   chan os.Signal

	// instance is currently required for PUT because render.Bind() requires a non-nil input for T. Since
	// I need to have pointer receivers on Bind and Render implementations, `new(T)` creates a nil instance
	instance func() T

	customRoutes   []chi.Route
	customIDRoutes []chi.Route

	responseWrapper       func(T) render.Renderer
	getAllResponseWrapper func([]T) render.Renderer

	getAllFilter func(*http.Request) FilterFunc[T]

	beforeDelete beforeAfterFunc
	afterDelete  beforeAfterFunc

	onCreateOrUpdate func(*http.Request, T) *ErrResponse

	parent RelatedAPI

	customResponseCodes map[string]int
	serverCtx           context.Context
}

// NewAPI initializes an API using the provided name, base URL path, and function to create a new instance of
// the resource with defaults
func NewAPI[T Resource](name, base string, instance func() T) *API[T] {
	return &API[T]{
		name,
		base,
		map[string]RelatedAPI{},
		nil,
		nil,
		MapStorage[T]{},
		nil,
		make(chan os.Signal, 1),
		instance,
		nil,
		nil,
		func(r T) render.Renderer { return r },
		nil,
		func(*http.Request) FilterFunc[T] { return func(T) bool { return true } },
		defaultBeforeAfter,
		defaultBeforeAfter,
		func(*http.Request, T) *ErrResponse { return nil },
		nil,
		map[string]int{},
		nil,
	}
}

// Base returns the API's base path
func (a *API[T]) Base() string {
	return a.base
}

// Name returns the name of the API
func (a *API[T]) Name() string {
	return a.name
}

// SetCustomResponseCode will override the default response codes for the specified HTTP verb
func (a *API[T]) SetCustomResponseCode(verb string, code int) *API[T] {
	a.customResponseCodes[verb] = code
	return a
}

// SetGetAllResponseWrapper sets a function that can create a custom response for GetAll. This function will receive
// a slice of Resources from storage and must return a render.Renderer
func (a *API[T]) SetGetAllResponseWrapper(getAllResponder func([]T) render.Renderer) *API[T] {
	a.getAllResponseWrapper = getAllResponder
	return a
}

// SetOnCreateOrUpdate runs on POST, PATCH, and PUT requests before saving the created/updated resource.
// This is useful for adding more validations or performing tasks related to resources such as initializing
// schedules or sending events
func (a *API[T]) SetOnCreateOrUpdate(onCreateOrUpdate func(*http.Request, T) *ErrResponse) *API[T] {
	a.onCreateOrUpdate = onCreateOrUpdate
	return a
}

// SetBeforeDelete sets a function that is executing before deleting a resource. It is useful for additional
// validation before completing the delete
func (a *API[T]) SetBeforeDelete(before func(*http.Request) *ErrResponse) *API[T] {
	if before == nil {
		before = defaultBeforeAfter
	}
	a.beforeDelete = before

	return a
}

// SetAfterDelete sets a function that is executed after deleting a resource. It is useful for additional
// cleanup or other actions that should be done after deleting
func (a *API[T]) SetAfterDelete(after func(*http.Request) *ErrResponse) *API[T] {
	if after == nil {
		after = defaultBeforeAfter
	}
	a.afterDelete = after

	return a
}

// SetGetAllFilter sets a function that can use the request context to create a filter for GetAll. Use this
// to introduce query parameters for filtering resources
func (a *API[T]) SetGetAllFilter(f func(*http.Request) FilterFunc[T]) *API[T] {
	a.getAllFilter = f
	return a
}

// SetResponseWrapper sets a function that returns a new Renderer before responding with T. This is used to add
// more data to responses that isn't directly from storage
func (a *API[T]) SetResponseWrapper(responseWrapper func(T) render.Renderer) *API[T] {
	a.responseWrapper = responseWrapper
	return a
}

// Client returns a new Client based on the API's configuration. It is a shortcut for NewClient
func (a *API[T]) Client(addr string) *Client[T] {
	return NewClient[T](addr, a.base)
}

// AnyClient returns a new Client based on the API's configuration. It is a shortcut for NewClient
func (a *API[T]) AnyClient(addr string) *Client[*AnyResource] {
	return NewClient[*AnyResource](addr, a.base)
}

// AddCustomRoute appends a custom API route to the base path: /base/custom-route
func (a *API[T]) AddCustomRoute(route chi.Route) *API[T] {
	a.customRoutes = append(a.customRoutes, route)
	return a
}

// AddCustomIDRoute appends a custom API route to the base path after the ID URL parameter: /base/{ID}/custom-route.
// The handler for this route can access the requested resource using GetResourceFromContext
func (a *API[T]) AddCustomIDRoute(route chi.Route) *API[T] {
	a.customIDRoutes = append(a.customIDRoutes, route)
	return a
}

// SetStorage sets a custom storage interface for the API
func (a *API[T]) SetStorage(s Storage[T]) *API[T] {
	a.storage = s
	return a
}

// Storage returns the storage interface for the API so it can be used in custom routes or other use cases
func (a *API[T]) Storage() Storage[T] {
	return a.storage
}

// AddMiddleware adds a middleware which is active only on the paths without resource ID
func (a *API[T]) AddMiddleware(m func(http.Handler) http.Handler) *API[T] {
	a.middlewares = append(a.middlewares, m)
	return a
}

// AddIDMiddleware adds a middleware which is active only on the paths including a resource ID
func (a *API[T]) AddIDMiddleware(m func(http.Handler) http.Handler) *API[T] {
	a.idMiddlewares = append(a.idMiddlewares, m)
	return a
}

// Serve will serve the API on the given port
func (a *API[T]) Serve(port string) {
	a.server = &http.Server{Addr: port, Handler: a.Router()}

	var serverStopCtx context.CancelFunc
	a.serverCtx, serverStopCtx = context.WithCancel(context.Background())

	signal.Notify(a.quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-a.quit

		shutdownCtx, cancel := context.WithTimeout(a.serverCtx, 10*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		err := a.server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()

	slog.Info("starting server", "port", port, "api", a.name)
	err := a.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("server shutdown error", "error", err)
	}

	<-a.serverCtx.Done()
}

// Stop will stop the API
func (a *API[T]) Stop() {
	a.quit <- os.Interrupt
	<-a.serverCtx.Done()
}

// Done returns a channel that's closed when the API stops, similar to context.Done()
func (a *API[T]) Done() <-chan os.Signal {
	return a.quit
}

type beforeAfterFunc func(*http.Request) *ErrResponse

func defaultBeforeAfter(*http.Request) *ErrResponse {
	return nil
}
