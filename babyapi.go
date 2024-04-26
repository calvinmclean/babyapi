package babyapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// MethodGetAll is the same as http.MethodGet, but can be used when setting custom response codes
const MethodGetAll = "GetAll"

// API encapsulates all handlers and other pieces of code required to run the CRUID API based on
// the provided Resource type
type API[T Resource] struct {
	name string
	base string

	subAPIs       map[string]relatedAPI
	middlewares   []func(http.Handler) http.Handler
	idMiddlewares []func(http.Handler) http.Handler

	// Storage is the interface used by the API server to read/write resources
	Storage[T]

	// context is set by WithContext to allow external goroutines to control API shutdown
	context context.Context

	// quit is used for the Stop() method to send a shutdown signal to the server
	quit chan struct{}

	// shutdown is used so the Stop() method can block until the API is fully shutdown
	shutdown chan struct{}

	// instance is currently required for PUT because render.Bind() requires a non-nil input for T. Since
	// I need to have pointer receivers on Bind and Render implementations, `new(T)` creates a nil instance
	instance func() T

	// rootRoutes only applies if there are no parent APIs because otherwise it would conflict
	rootRoutes []chi.Route

	customRoutes   []chi.Route
	customIDRoutes []chi.Route

	responseWrapper       func(T) render.Renderer
	getAllResponseWrapper func([]T) render.Renderer

	getAllFilter func(*http.Request) FilterFunc[T]

	beforeDelete beforeAfterFunc
	afterDelete  beforeAfterFunc

	onCreateOrUpdate    func(*http.Request, T) *ErrResponse
	afterCreateOrUpdate func(*http.Request, T) *ErrResponse

	parent relatedAPI

	responseCodes map[string]int

	// GetAll is the handler for /base and returns an array of resources
	GetAll http.HandlerFunc

	// Get is the handler for /base/{ID} and returns a requested resource by ID
	Get http.HandlerFunc

	// Post is used to create new resources at /base
	Post http.HandlerFunc

	// Put is used to idempotently create or modify resources at /base/{ID}
	Put http.HandlerFunc

	// Patch is used to modify resources at /base/{ID}
	Patch http.HandlerFunc

	// Delete is used to delete the resource at /base/{ID}
	Delete http.HandlerFunc

	rootAPI bool

	readOnly sync.Mutex

	errors []error

	cliArgs cliArgs
}

// NewAPI initializes an API using the provided name, base URL path, and function to create a new instance of
// the resource with defaults
func NewAPI[T Resource](name, base string, instance func() T) *API[T] {
	api := &API[T]{
		name,
		base,
		map[string]relatedAPI{},
		nil,
		nil,
		MapStorage[T]{},
		context.Background(),
		make(chan struct{}, 1),
		make(chan struct{}, 1),
		instance,
		nil,
		nil,
		nil,
		func(r T) render.Renderer { return r },
		nil,
		func(*http.Request) FilterFunc[T] { return func(T) bool { return true } },
		defaultBeforeAfter,
		defaultBeforeAfter,
		func(*http.Request, T) *ErrResponse { return nil },
		func(*http.Request, T) *ErrResponse { return nil },
		nil,
		defaultResponseCodes(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		false,
		sync.Mutex{},
		nil,
		cliArgs{},
	}

	api.GetAll = api.defaultGetAll()
	api.Get = api.defaultGet()
	api.Post = api.defaultPost()
	api.Put = api.defaultPut()
	api.Patch = api.defaultPatch()
	api.Delete = api.defaultDelete()

	return api
}

// NewRootAPI initializes an API which can serve as a top-level parent of other APIs, so multiple unrelated resources
// can exist without any parent/child relationship. This API does not have any default handlers, but custom handlers can
// still be added. Since there are no IDs in the path, Get and GetAll routes cannot be differentiated so only Get is used
func NewRootAPI(name, base string) *API[*NilResource] {
	api := NewAPI[*NilResource](name, base, nil)
	api.rootAPI = true

	api.GetAll = nil
	api.Get = nil
	api.Post = nil
	api.Put = nil
	api.Patch = nil
	api.Delete = nil

	return api
}

// Base returns the API's base path
func (a *API[T]) Base() string {
	return a.base
}

// Name returns the name of the API
func (a *API[T]) Name() string {
	return a.name
}

// SetCustomResponseCode will override the default response codes for the specified HTTP verb. Use MethodGetAll to set the
// response code for listing all resources
func (a *API[T]) SetCustomResponseCode(verb string, code int) *API[T] {
	a.panicIfReadOnly()

	a.responseCodes[verb] = code
	return a
}

// SetGetAllResponseWrapper sets a function that can create a custom response for GetAll. This function will receive
// a slice of Resources from storage and must return a render.Renderer
func (a *API[T]) SetGetAllResponseWrapper(getAllResponder func([]T) render.Renderer) *API[T] {
	a.panicIfReadOnly()

	a.getAllResponseWrapper = getAllResponder
	return a
}

// SetOnCreateOrUpdate runs on POST, PATCH, and PUT requests before saving the created/updated resource.
// This is useful for adding more validations or performing tasks related to resources such as initializing
// schedules or sending events
func (a *API[T]) SetOnCreateOrUpdate(onCreateOrUpdate func(*http.Request, T) *ErrResponse) *API[T] {
	a.panicIfReadOnly()

	a.onCreateOrUpdate = onCreateOrUpdate
	return a
}

func (a *API[T]) SetAfterCreateOrUpdate(afterCreateOrUpdate func(*http.Request, T) *ErrResponse) *API[T] {
	a.panicIfReadOnly()

	a.afterCreateOrUpdate = afterCreateOrUpdate
	return a
}

// SetBeforeDelete sets a function that is executing before deleting a resource. It is useful for additional
// validation before completing the delete
func (a *API[T]) SetBeforeDelete(before func(*http.Request) *ErrResponse) *API[T] {
	a.panicIfReadOnly()

	if before == nil {
		before = defaultBeforeAfter
	}
	a.beforeDelete = before

	return a
}

// SetAfterDelete sets a function that is executed after deleting a resource. It is useful for additional
// cleanup or other actions that should be done after deleting
func (a *API[T]) SetAfterDelete(after func(*http.Request) *ErrResponse) *API[T] {
	a.panicIfReadOnly()

	if after == nil {
		after = defaultBeforeAfter
	}
	a.afterDelete = after

	return a
}

// SetGetAllFilter sets a function that can use the request context to create a filter for GetAll. Use this
// to introduce query parameters for filtering resources
func (a *API[T]) SetGetAllFilter(f func(*http.Request) FilterFunc[T]) *API[T] {
	a.panicIfReadOnly()

	a.getAllFilter = f
	return a
}

// SetResponseWrapper sets a function that returns a new Renderer before responding with T. This is used to add
// more data to responses that isn't directly from storage
func (a *API[T]) SetResponseWrapper(responseWrapper func(T) render.Renderer) *API[T] {
	a.panicIfReadOnly()

	a.responseWrapper = responseWrapper
	return a
}

// Client returns a new Client based on the API's configuration. It is a shortcut for NewClient
func (a *API[T]) Client(addr string) *Client[T] {
	return NewClient[T](addr, makePathWithRoot(a.base, a.parent)).
		SetCustomResponseCodeMap(a.responseCodes)
}

// AnyClient returns a new Client based on the API's configuration. It is a shortcut for NewClient
func (a *API[T]) AnyClient(addr string) *Client[*AnyResource] {
	client := NewClient[*AnyResource](addr, makePathWithRoot(a.base, a.parent)).
		SetCustomResponseCodeMap(a.responseCodes)
	client.name = a.name
	return client
}

// AddCustomRootRoute appends a custom API route to the absolute root path ("/"). It does not work for APIs with
// parents because it would conflict with the parent's route. Panics if the API is already a child when this is called
func (a *API[T]) AddCustomRootRoute(method, pattern string, handler http.Handler) *API[T] {
	a.panicIfReadOnly()

	if a.parent != nil {
		a.errors = append(a.errors, fmt.Errorf("AddCustomRootRoute: cannot be applied to child APIs"))
		return a
	}
	a.rootRoutes = append(a.rootRoutes, chi.Route{
		Pattern: pattern,
		Handlers: map[string]http.Handler{
			method: handler,
		},
	})
	return a
}

// AddCustomRoute appends a custom API route to the base path: /base/custom-route
func (a *API[T]) AddCustomRoute(method, pattern string, handler http.Handler) *API[T] {
	a.panicIfReadOnly()

	a.customRoutes = append(a.customRoutes, chi.Route{
		Pattern: pattern,
		Handlers: map[string]http.Handler{
			method: handler,
		},
	})
	return a
}

// AddCustomIDRoute appends a custom API route to the base path after the ID URL parameter: /base/{ID}/custom-route.
// The handler for this route can access the requested resource using GetResourceFromContext
func (a *API[T]) AddCustomIDRoute(method, pattern string, handler http.Handler) *API[T] {
	a.panicIfReadOnly()

	if a.rootAPI {
		a.errors = append(a.errors, fmt.Errorf("AddCustomIDRoute: ID routes cannot be used with a root API"))
		return a
	}
	a.customIDRoutes = append(a.customIDRoutes, chi.Route{
		Pattern: pattern,
		Handlers: map[string]http.Handler{
			method: handler,
		},
	})
	return a
}

// AddMiddleware adds a middleware which is active only on the paths without resource ID
func (a *API[T]) AddMiddleware(m func(http.Handler) http.Handler) *API[T] {
	a.panicIfReadOnly()

	a.middlewares = append(a.middlewares, m)
	return a
}

// AddIDMiddleware adds a middleware which is active only on the paths including a resource ID
func (a *API[T]) AddIDMiddleware(m func(http.Handler) http.Handler) *API[T] {
	a.panicIfReadOnly()

	if a.rootAPI {
		a.errors = append(a.errors, fmt.Errorf("AddIDMiddleware: ID middleware cannot be used with a root API"))
		return a
	}
	a.idMiddlewares = append(a.idMiddlewares, m)
	return a
}

// Serve will serve the API on the given port
func (a *API[T]) Serve(address string) error {
	if address == "" {
		address = ":8080"
	}

	router, err := a.Router()
	if err != nil {
		return fmt.Errorf("error creating router: %w", err)
	}
	server := &http.Server{Addr: address, Handler: router}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for shutdown signal from internal or from externally-controlled context
		select {
		case <-a.Done():
		case <-a.context.Done():
			// if shutdown by context, need to close a.quit for a.Done()
			close(a.quit)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer func() {
			cancel()
			close(a.shutdown)
		}()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
	}()

	slog.Info("starting server", "address", address, "api", a.name)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error starting the server: %w", err)
	}

	wg.Wait()

	return nil
}

// Stop will stop the API
func (a *API[T]) Stop() {
	close(a.quit)
	<-a.shutdown
}

// Done returns a channel that's closed when the API stops, similar to context.Done()
func (a *API[T]) Done() <-chan struct{} {
	return a.quit
}

type beforeAfterFunc func(*http.Request) *ErrResponse

func defaultBeforeAfter(*http.Request) *ErrResponse {
	return nil
}

// ChildAPIs returns the nested children APIs
func (a *API[T]) ChildAPIs() map[string]RelatedAPI {
	children := map[string]RelatedAPI{}
	for _, child := range a.subAPIs {
		children[child.Name()] = child
	}
	return children
}

func (a *API[T]) SetStorage(s Storage[T]) *API[T] {
	a.panicIfReadOnly()

	a.Storage = s
	return a
}

// WithContext adds a context to the API so that it will automatically shutdown when the context is closed
func (a *API[T]) WithContext(ctx context.Context) *API[T] {
	a.panicIfReadOnly()

	a.context = ctx
	return a
}

func (a *API[T]) panicIfReadOnly() {
	if !a.readOnly.TryLock() {
		panic(errors.New("API cannot be modified after starting"))
	}
	a.readOnly.Unlock()
}
