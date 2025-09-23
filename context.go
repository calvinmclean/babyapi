package babyapi

import (
	"context"
	"fmt"
	"log/slog"
)

// ContextKey is used to store API resources in the request context
type ContextKey string

type ctxKey int

const (
	loggerCtxKey ctxKey = iota
	requestBodyCtxKey
)

// GetLoggerFromContext returns the structured logger from the context. It expects to use an HTTP
// request context to get a logger with details from middleware
func GetLoggerFromContext(ctx context.Context) (*slog.Logger, bool) {
	logger, ok := ctx.Value(loggerCtxKey).(*slog.Logger)
	if !ok {
		return slog.Default(), false
	}

	return logger, true
}

// NewContextWithLogger stores a structured logger in the context
func NewContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

// GetRequestBodyFromContext gets an API resource from the request context. It can only be used in
// URL paths that include the resource ID
func GetRequestBodyFromContext[T any](ctx context.Context) (T, bool) {
	value, ok := ctx.Value(requestBodyCtxKey).(T)
	if !ok {
		return *new(T), false
	}
	return value, true
}

// NewContextWithRequestBody stores the API resource in the context
func (a *API[T]) NewContextWithRequestBody(ctx context.Context, item T) context.Context {
	return context.WithValue(ctx, requestBodyCtxKey, item)
}

// ParentContextKey returns the context key for the direct parent's resource
func (a *API[T]) ParentContextKey() ContextKey {
	return ContextKey(a.parent.Name())
}

// GetResourceFromContext gets the API resource from request context
func (a *API[T]) GetResourceFromContext(ctx context.Context) (T, error) {
	return GetResourceFromContext[T](ctx, a.contextKey())
}

// GetResourceFromContext gets the API resource from request context
func GetResourceFromContext[T Resource](ctx context.Context, key ContextKey) (T, error) {
	v := ctx.Value(key)
	if v == nil {
		return *new(T), ErrNotFound
	}

	val, ok := v.(T)
	if !ok {
		return *new(T), fmt.Errorf("unexpected type %T in context", v)
	}

	return val, nil
}

func (a *API[T]) newContextWithResource(ctx context.Context, value T) context.Context {
	return context.WithValue(ctx, a.contextKey(), value)
}

func (a *API[T]) contextKey() ContextKey {
	return ContextKey(a.name)
}
