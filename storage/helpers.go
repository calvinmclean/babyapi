package storage

import (
	"fmt"

	"github.com/madflojo/hord"
	"github.com/madflojo/hord/drivers/hashmap"
	"github.com/madflojo/hord/drivers/redis"
)

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
