package babyapi

import (
	"context"
	"errors"
	"iter"
	"net/url"
)

var ErrNotFound = errors.New("resource not found")

// FilterFunc is used for Search to filter resources that are read from storage
type FilterFunc[T any] func(T) bool

// CollectIterator iterates through the provided iterator and collects all results into a slice.
// It returns the slice and the first error encountered during iteration.
// This is a convenience helper for custom response wrappers that need to collect all results.
func CollectIterator[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var results []T
	for item, err := range seq {
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

// Filter returns a new iterator that filters items from the source.
// Iteration stops on the first error.
func (f FilterFunc[T]) Filter(seq iter.Seq2[T, error]) iter.Seq2[T, error] {
	if f == nil {
		return seq
	}
	return func(yield func(T, error) bool) {
		for item, err := range seq {
			if err != nil {
				yield(item, err)
				return
			}
			if f(item) {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// Storage defines how the API will interact with a storage backend
type Storage[T Resource] interface {
	// Get a single resource by ID
	Get(context.Context, string) (T, error)
	// Search returns an iterator yielding resources matching the query.
	// Iteration stops on first error.
	Search(ctx context.Context, parentID string, query url.Values) iter.Seq2[T, error]
	// Set will save the provided resource
	Set(context.Context, T) error
	// Delete will delete a resource by ID
	Delete(context.Context, string) error
}
