package babyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tarmac-project/hord"
)

// KVStorage implements the Storage interface for the provided type using hord.Database for the storage backend
//
// It allows soft-deleting if your type implements the kv.EndDateable interface. This means Delete will set the end-date
// to now and update in storage instead of deleting. If something is already end-dated, then it is hard-deleted. Also,
// the GetAll method will automatically read the 'end_dated' query param to determine if end-dated resources should
// be filtered out
type KVStorage[T Resource] struct {
	prefix string
	db     hord.Database
}

// NewKVStorage creates a new storage client for the specified type. It stores resources with keys prefixed by 'prefix'
func NewKVStorage[T Resource](db hord.Database, prefix string) Storage[T] {
	return &KVStorage[T]{prefix, db}
}

func (c *KVStorage[T]) key(id string) string {
	return fmt.Sprintf("%s_%s", c.prefix, id)
}

// Delete will delete a resource by the key. If the resource implements EndDateable, it will first soft-delete by
// setting the EndDate to time.Now()
func (c *KVStorage[T]) Delete(ctx context.Context, id string) error {
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
func (c *KVStorage[T]) Get(_ context.Context, id string) (T, error) {
	return c.get(c.key(id))
}

func (c *KVStorage[T]) get(key string) (T, error) {
	if c.db == nil {
		return *new(T), fmt.Errorf("error missing database connection")
	}

	dataBytes, err := c.db.Get(key)
	if err != nil {
		if errors.Is(err, hord.ErrNil) {
			return *new(T), ErrNotFound
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
func (c *KVStorage[T]) GetAll(_ context.Context, parentID string, query url.Values) ([]T, error) {
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

		itemParentID := result.ParentID()
		hasParent := itemParentID != ""
		if hasParent && itemParentID != parentID {
			continue
		}

		getEndDated := query.Get("end_dated") == "true"
		endDateable, ok := any(result).(EndDateable)
		if ok && !getEndDated && endDateable.EndDated() {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// Set marshals the provided item and writes it to the database
func (c *KVStorage[T]) Set(_ context.Context, item T) error {
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
