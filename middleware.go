package babyapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

func (a *API[T]) DefaultMiddleware(r chi.Router) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(a.logMiddleware)
}

func (a *API[T]) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := slog.Default()
		logger = logger.With(
			"method", r.Method,
			"path", r.RequestURI,
			"host", r.Host,
			"from", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		t1 := time.Now()
		defer func() {
			if r.URL.Path == "/metrics" {
				return
			}
			logger.With(
				"status", ww.Status(),
				"bytes_written", ww.BytesWritten(),
				"time_elapsed", time.Since(t1),
			).Info("response completed")
		}()

		next.ServeHTTP(ww, r.WithContext(NewContextWithLogger(r.Context(), logger)))
	})
}

func (a *API[T]) requestBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, httpErr := a.GetFromRequest(r)
		if httpErr != nil {
			_ = render.Render(w, r, httpErr)
			return
		}

		logger := GetLoggerFromContext(r.Context())
		logger.Info("received request body", "body", body)

		next.ServeHTTP(w, r.WithContext(a.NewContextWithRequestBody(r.Context(), body)))
	})
}

func (a *API[T]) resourceExistsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource, httpErr := a.GetRequestedResource(r)
		if httpErr != nil {
			// Skip for PUT because it can be used to create new resources
			if r.Method == http.MethodPut {
				next.ServeHTTP(w, r)
				return
			}
			_ = render.Render(w, r, httpErr)
			return
		}

		logger := GetLoggerFromContext(r.Context())
		logger = logger.With(a.IDParamKey(), resource.GetID())
		logger.Info("got resource")

		ctx := a.newContextWithResource(r.Context(), resource)
		ctx = NewContextWithLogger(ctx, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
