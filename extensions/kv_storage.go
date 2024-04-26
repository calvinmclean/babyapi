package extensions

import (
	"fmt"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/storage/kv"
	"github.com/madflojo/hord"
	"github.com/madflojo/hord/drivers/hashmap"
	"github.com/madflojo/hord/drivers/redis"
)

// KeyValueStorage sets up a connection to Redis or local file storage and applies to the API's Storage Client
// If you pass environment variables for configurations, this can dynamically determine filesystem or Redis
// storage based on the available configs
type KeyValueStorage[T babyapi.Resource] struct {
	// Optional key to use as a prefix when storing in key-value store. If empty, api Name is used
	StorageKeyPrefix string

	// KVConnectionConfig has connection data for KV store. Optional if DB is provided
	KVConnectionConfig

	// DB is the database connection. It is created if not provided. This is useful if multiple APIs share
	// a storage backend
	DB hord.Database
}

type KVConnectionConfig struct {
	// Filename to write JSON data to
	Filename string

	// Host of Redis instance
	RedisHost string
	// Password for Redis instance
	RedisPassword string

	// If other configurations are empty, this will not return an error and skips setting api Storage.
	// This is useful if using env vars as the values for configs
	Optional bool
}

func (h KeyValueStorage[T]) Apply(api *babyapi.API[T]) error {
	db := h.DB
	if db == nil {
		var err error
		db, err = h.CreateDB()
		if err != nil {
			return fmt.Errorf("error creating database connection: %w", err)
		}
	}
	if db == nil && h.Optional {
		return nil
	}

	storageKeyPrefix := h.StorageKeyPrefix
	if storageKeyPrefix == "" {
		storageKeyPrefix = api.Name()
	}

	api.SetStorage(kv.NewClient[T](db, storageKeyPrefix))

	return nil
}

func (h KVConnectionConfig) CreateDB() (hord.Database, error) {
	switch {
	case h.RedisHost != "" && h.RedisPassword != "":
		return kv.NewRedisDB(redis.Config{
			Server:   h.RedisHost + ":6379",
			Password: h.RedisPassword,
		})
	case h.Filename != "":
		return kv.NewFileDB(hashmap.Config{
			Filename: h.Filename,
		})
	case h.Optional:
		return nil, nil
	default:
		return nil, fmt.Errorf("filename or redis configuration is required")
	}
}
