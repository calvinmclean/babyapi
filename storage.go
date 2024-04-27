package babyapi

import (
	"context"
	"errors"
	"net/url"

	"golang.org/x/exp/maps"
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
	// GetAll will return all resources that match the provided query filters
	GetAll(context.Context, url.Values) ([]T, error)
	// Set will save the provided resource
	Set(context.Context, T) error
	// Delete will delete a resource by ID
	Delete(context.Context, string) error
}

// MapStorage is the default implementation of the Storage interface that just uses a map
type MapStorage[T Resource] map[string]T

func (m MapStorage[T]) Get(_ context.Context, id string) (T, error) {
	resource, ok := m[id]
	if !ok {
		return *new(T), ErrNotFound
	}
	return resource, nil
}

// GetAll for MapStorage does not make use of the query parameters, but the API's FilterFunc can filter after
func (m MapStorage[T]) GetAll(_ context.Context, _ url.Values) ([]T, error) {
	return maps.Values[map[string]T](m), nil
}

func (m MapStorage[T]) Set(_ context.Context, resource T) error {
	m[resource.GetID()] = resource
	return nil
}

func (m MapStorage[T]) Delete(_ context.Context, id string) error {
	_, ok := m[id]
	if !ok {
		return ErrNotFound
	}

	delete(m, id)
	return nil
}
