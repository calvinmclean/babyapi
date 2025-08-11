package babyapi

import (
	"context"
	"errors"
	"net/url"
)

var ErrNotFound = errors.New("resource not found")

// FilterFunc is used for GetAll to filter resources that are read from storage
type FilterFunc[T any] func(T) bool

func (f FilterFunc[T]) Filter(in []T) []T {
	if f == nil {
		return in
	}

	out := []T{}

	for _, item := range in {
		if f(item) {
			out = append(out, item)
		}
	}

	return out
}

// Storage defines how the API will interact with a storage backend
type Storage[T Resource] interface {
	// Get a single resource by ID
	Get(context.Context, string) (T, error)
	// GetAll will return all resources that match the provided query filters. It can also receive a
	// parentID string if it is a nested resource (empty string if not)
	GetAll(ctx context.Context, parentID string, query url.Values) ([]T, error)
	// Set will save the provided resource
	Set(context.Context, T) error
	// Delete will delete a resource by ID
	Delete(context.Context, string) error
}
