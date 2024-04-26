package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/madflojo/hord"
)

// Client implements the babyapi.Storage interface for the provided type using hord.Database for the storage backend
type Client[T babyapi.Resource] struct {
	prefix string
	db     hord.Database
}

// NewClient creates a new storage client for the specified type. It stores resources with keys prefixed by 'prefix'
func NewClient[T babyapi.Resource](db hord.Database, prefix string) babyapi.Storage[T] {
	return &Client[T]{prefix, db}
}

func (c *Client[T]) key(id string) string {
	return fmt.Sprintf("%s_%s", c.prefix, id)
}

// Delete will delete a resource by the key. If the resource implements EndDateable, it will first soft-delete by
// setting the EndDate to time.Now()
func (c *Client[T]) Delete(ctx context.Context, id string) error {
	key := c.key(id)

	result, err := c.get(key)
	if err != nil {
		return fmt.Errorf("error getting resource before deleting: %w", err)
	}

	endDateable, ok := any(result).(EndDateable)
	if !ok {
		return c.db.Delete(key)
	}

	if endDateable.EndDated() {
		return c.db.Delete(key)
	}

	endDateable.SetEndDate(time.Now())

	return c.Set(ctx, result)
}

// Get will use the provided key to read data from the data source. Then, it will Unmarshal
// into the generic type
func (c *Client[T]) Get(_ context.Context, id string) (T, error) {
	return c.get(c.key(id))
}

func (c *Client[T]) get(key string) (T, error) {
	if c.db == nil {
		return *new(T), fmt.Errorf("error missing database connection")
	}

	dataBytes, err := c.db.Get(key)
	if err != nil {
		if errors.Is(hord.ErrNil, err) {
			return *new(T), babyapi.ErrNotFound
		}
		return *new(T), fmt.Errorf("error getting data: %w", err)
	}

	var result T
	err = json.Unmarshal(dataBytes, &result)
	if err != nil {
		return *new(T), fmt.Errorf("error parsing data: %w", err)
	}

	return result, nil
}

// GetAll will use the provided prefix to read data from the data source. Then, it will use Get
// to read each element into the correct type
func (c *Client[T]) GetAll(_ context.Context, filter babyapi.FilterFunc[T]) ([]T, error) {
	keys, err := c.db.Keys()
	if err != nil {
		return nil, fmt.Errorf("error getting keys: %w", err)
	}

	results := []T{}
	for _, key := range keys {
		if !strings.HasPrefix(key, c.prefix) {
			continue
		}

		result, err := c.get(key)
		if err != nil {
			return nil, fmt.Errorf("error getting data: %w", err)
		}

		if filter == nil || filter(result) {
			results = append(results, result)
		}
	}

	return results, nil
}

// Set marshals the provided item and writes it to the database
func (c *Client[T]) Set(_ context.Context, item T) error {
	asBytes, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}

	err = c.db.Set(c.key(item.GetID()), asBytes)
	if err != nil {
		return fmt.Errorf("error writing data to database: %w", err)
	}

	return nil
}
