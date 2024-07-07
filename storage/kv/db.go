package kv

import (
	"fmt"

	"github.com/tarmac-project/hord"
	"github.com/tarmac-project/hord/drivers/hashmap"
	"github.com/tarmac-project/hord/drivers/redis"
)

// NewDefaultDB creates a default in-memory KV-storage. Theoretically it should not error, but if it does, it panics
func NewDefaultDB() hord.Database {
	db, err := NewFileDB(hashmap.Config{})
	if err != nil {
		panic(err)
	}

	return db
}

func NewFileDB(cfg hashmap.Config) (hord.Database, error) {
	db, err := hashmap.Dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating database connection: %w", err)
	}

	err = db.Setup()
	if err != nil {
		return nil, fmt.Errorf("error setting up database: %w", err)
	}

	return db, nil
}

func NewRedisDB(cfg redis.Config) (hord.Database, error) {
	db, err := redis.Dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating database connection: %w", err)
	}

	err = db.Setup()
	if err != nil {
		return nil, fmt.Errorf("error setting up database: %w", err)
	}

	return db, nil
}
