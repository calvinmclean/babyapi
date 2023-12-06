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

	subAPIs     map[string]relatedAPI
	middlewares chi.Middlewares
	storage     Storage[T]

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

	parent relatedAPI

	customResponseCodes map[string]int
	serverCtx           context.Context
}

// NewAPI initializes an API using the provided name, base URL path, and function to create a new instance of
// the resource with defaults
func NewAPI[T Resource](name, base string, instance func() T) *API[T] {
	return &API[T]{
		name,
		base,
		map[string]relatedAPI{},
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
func (a *API[T]) SetCustomResponseCode(verb string, code int) {
	a.customResponseCodes[verb] = code
}

// SetGetAllResponseWrapper sets a function that can create a custom response for GetAll. This function will receive
// a slice of Resources from storage and must return a render.Renderer
func (a *API[T]) SetGetAllResponseWrapper(getAllResponder func([]T) render.Renderer) {
	a.getAllResponseWrapper = getAllResponder
}

// SetOnCreateOrUpdate runs on POST, PATCH, and PUT requests before saving the created/updated resource.
// This is useful for adding more validations or performing tasks related to resources such as initializing
// schedules or sending events
func (a *API[T]) SetOnCreateOrUpdate(onCreateOrUpdate func(*http.Request, T) *ErrResponse) {
	a.onCreateOrUpdate = onCreateOrUpdate
}

// SetBeforeDelete sets a function that is executing before deleting a resource. It is useful for additional
// validation before completing the delete
func (a *API[T]) SetBeforeDelete(before func(*http.Request) *ErrResponse) {
	if before == nil {
		before = defaultBeforeAfter
	}
	a.beforeDelete = before
}

// SetAfterDelete sets a function that is executed after deleting a resource. It is useful for additional
// cleanup or other actions that should be done after deleting
func (a *API[T]) SetAfterDelete(after func(*http.Request) *ErrResponse) {
	if after == nil {
		after = defaultBeforeAfter
	}
	a.afterDelete = after
}

// SetGetAllFilter sets a function that can use the request context to create a filter for GetAll. Use this
// to introduce query parameters for filtering resources
func (a *API[T]) SetGetAllFilter(f func(*http.Request) FilterFunc[T]) {
	a.getAllFilter = f
}

// ResponseWrapper sets a function that returns a new Renderer before responding with T. This is used to add
// more data to responses that isn't directly from storage
func (a *API[T]) ResponseWrapper(responseWrapper func(T) render.Renderer) {
	a.responseWrapper = responseWrapper
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
func (a *API[T]) AddCustomRoute(route chi.Route) {
	a.customRoutes = append(a.customRoutes, route)
}

// AddCustomIDRoute appends a custom API route to the base path after the ID URL parameter: /base/{ID}/custom-route.
// The handler for this route can access the requested resource using GetResourceFromContext
func (a *API[T]) AddCustomIDRoute(route chi.Route) {
	a.customIDRoutes = append(a.customIDRoutes, route)
}

// SetStorage sets a custom storage interface for the API
func (a *API[T]) SetStorage(s Storage[T]) {
	a.storage = s
}

// Storage returns the storage interface for the API so it can be used in custom routes or other use cases
func (a *API[T]) Storage() Storage[T] {
	return a.storage
}

// AddMiddlewares appends chi.Middlewares to existing middlewares
func (a *API[T]) AddMiddlewares(m chi.Middlewares) {
	a.middlewares = append(a.middlewares, m...)
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
