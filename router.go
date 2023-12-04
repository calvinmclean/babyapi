package babyapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// HTMLer allows for easily represending reponses as HTML strings when accepted content
// type is text/html
type HTMLer interface {
	HTML() string
}

// Create API routes on the given router
func (a *API[T]) Route(r chi.Router) {
	render.Respond = func(w http.ResponseWriter, r *http.Request, v interface{}) {
		if render.GetAcceptedContentType(r) == render.ContentTypeHTML {
			htmler, ok := v.(HTMLer)
			if ok {
				render.HTML(w, r, htmler.HTML())
				return
			}
		}

		render.DefaultResponder(w, r, v)
	}

	r.Route(a.base, func(r chi.Router) {
		// Only set these middleware for root-level API
		if a.parent == nil {
			a.defaultMiddleware(r)
		}

		r.With(a.requestBodyMiddleware).Post("/", a.Post)
		r.Get("/", a.GetAll)

		r.With(a.resourceExistsMiddleware).Route(fmt.Sprintf("/{%s}", a.IDParamKey()), func(r chi.Router) {
			r.Get("/", a.Get)
			r.Delete("/", a.Delete)
			r.With(a.requestBodyMiddleware).Put("/", a.Put)
			r.With(a.requestBodyMiddleware).Patch("/", a.Patch)

			for _, subAPI := range a.subAPIs {
				subAPI.Route(r)
			}

			a.doCustomRoutes(r, a.customIDRoutes)
		})

		a.doCustomRoutes(r, a.customRoutes)
	})
}

// Create a new router with API routes
func (a *API[T]) Router() chi.Router {
	r := chi.NewRouter()
	a.Route(r)

	return r
}

func (a *API[T]) doCustomRoutes(r chi.Router, routes []chi.Route) {
	for _, cr := range routes {
		for method, handler := range cr.Handlers {
			r.MethodFunc(method, cr.Pattern, handler.ServeHTTP)
		}
	}
}

// Get is the handler for /base/{ID} and returns a requested resource by ID
func (a *API[T]) Get(w http.ResponseWriter, r *http.Request) {
	logger := GetLoggerFromContext(r.Context())

	resource, httpErr := a.GetRequestedResource(r)
	if httpErr != nil {
		logger.Error("error getting requested resource", "error", httpErr.Error())
		_ = render.Render(w, r, httpErr)
		return
	}

	codeOverride, ok := a.customResponseCodes[http.MethodGet]
	if ok {
		render.Status(r, codeOverride)
	}

	err := render.Render(w, r, a.responseWrapper(resource))
	if err != nil {
		logger.Error("unable to render response", "error", err)
		_ = render.Render(w, r, ErrRender(err))
	}
}

// GetAll is the handler for /base and returns an array of resources
func (a *API[T]) GetAll(w http.ResponseWriter, r *http.Request) {
	logger := GetLoggerFromContext(r.Context())

	resources, err := a.storage.GetAll(a.getAllFilter(r))
	if err != nil {
		logger.Error("error getting resources", "error", err)
		_ = render.Render(w, r, InternalServerError(err))
		return
	}
	logger.Debug("responding with resources", "count", len(resources))

	var resp render.Renderer
	if a.getAllResponseWrapper != nil {
		resp = a.getAllResponseWrapper(resources)
	} else {
		items := []render.Renderer{}
		for _, item := range resources {
			items = append(items, a.responseWrapper(item))
		}
		resp = &ResourceList[render.Renderer]{Items: items}
	}

	codeOverride, ok := a.customResponseCodes[http.MethodGet]
	if ok {
		render.Status(r, codeOverride)
	}

	err = render.Render(w, r, resp)
	if err != nil {
		logger.Error("unable to render response", "error", err)
		_ = render.Render(w, r, ErrRender(err))
	}
}

// Post is used to create new resources at /base
func (a *API[T]) Post(w http.ResponseWriter, r *http.Request) {
	a.ReadRequestBodyAndDo(func(r *http.Request, resource T) (T, *ErrResponse) {
		logger := GetLoggerFromContext(r.Context())

		httpErr := a.onCreateOrUpdate(r, resource)
		if httpErr != nil {
			return *new(T), httpErr
		}

		logger.Info("storing resource", "resource", resource)
		err := a.storage.Set(resource)
		if err != nil {
			logger.Error("error storing resource", "error", err)
			return *new(T), InternalServerError(err)
		}

		codeOverride, ok := a.customResponseCodes[http.MethodPost]
		if ok {
			render.Status(r, codeOverride)
		} else {
			render.Status(r, http.StatusCreated)
		}

		return resource, nil
	})(w, r)
}

// Put is used to idempotently create or modify resources at /base/{ID}
func (a *API[T]) Put(w http.ResponseWriter, r *http.Request) {
	a.ReadRequestBodyAndDo(func(r *http.Request, resource T) (T, *ErrResponse) {
		logger := GetLoggerFromContext(r.Context())

		if resource.GetID() != a.GetIDParam(r) {
			return *new(T), ErrInvalidRequest(fmt.Errorf("id must match URL path"))
		}

		httpErr := a.onCreateOrUpdate(r, resource)
		if httpErr != nil {
			return *new(T), httpErr
		}

		logger.Info("storing resource", "resource", resource)
		err := a.storage.Set(resource)
		if err != nil {
			logger.Error("error storing resource", "error", err)
			return *new(T), InternalServerError(err)
		}

		codeOverride, ok := a.customResponseCodes[http.MethodPut]
		if ok {
			render.Status(r, codeOverride)
			return *new(T), nil
		}

		render.NoContent(w, r)
		return *new(T), nil
	})(w, r)
}

// Put is used to modify resources at /base/{ID}
func (a *API[T]) Patch(w http.ResponseWriter, r *http.Request) {
	a.ReadRequestBodyAndDo(func(r *http.Request, patchRequest T) (T, *ErrResponse) {
		logger := GetLoggerFromContext(r.Context())

		resource, httpErr := a.GetRequestedResource(r)
		if httpErr != nil {
			logger.Error("error getting requested resource", "error", httpErr.Error())
			return *new(T), httpErr
		}

		patcher, ok := any(resource).(Patcher[T])
		if !ok {
			return *new(T), ErrMethodNotAllowedResponse
		}

		httpErr = patcher.Patch(patchRequest)
		if httpErr != nil {
			logger.Error("error patching resource", "error", httpErr.Error())
			return *new(T), httpErr
		}

		httpErr = a.onCreateOrUpdate(r, resource)
		if httpErr != nil {
			return *new(T), httpErr
		}

		logger.Info("storing updated resource", "resource", resource)

		err := a.storage.Set(resource)
		if err != nil {
			logger.Error("error storing updated resource", "error", err)
			return *new(T), InternalServerError(err)
		}

		codeOverride, ok := a.customResponseCodes[http.MethodPatch]
		if ok {
			render.Status(r, codeOverride)
		}

		return resource, nil
	})(w, r)
}

// Delete is used to delete the resource at /base/{ID}
func (a *API[T]) Delete(w http.ResponseWriter, r *http.Request) {
	logger := GetLoggerFromContext(r.Context())
	httpErr := a.beforeDelete(r)
	if httpErr != nil {
		logger.Error("error executing before func", "error", httpErr)
		_ = render.Render(w, r, httpErr)
		return
	}

	id := a.GetIDParam(r)

	logger.Info("deleting resource", "id", id)

	err := a.storage.Delete(id)
	if err != nil {
		logger.Error("error deleting resource", "error", err)

		if errors.Is(err, ErrNotFound) {
			_ = render.Render(w, r, ErrNotFoundResponse)
			return
		}

		_ = render.Render(w, r, InternalServerError(err))
		return
	}

	httpErr = a.afterDelete(r)
	if httpErr != nil {
		logger.Error("error executing after func", "error", httpErr)
		_ = render.Render(w, r, httpErr)
		return
	}

	codeOverride, ok := a.customResponseCodes[http.MethodDelete]
	if ok {
		render.Status(r, codeOverride)
		return
	}

	render.NoContent(w, r)
}
