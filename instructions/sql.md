# BabyAPI SQL Storage Integration Guide

This guide demonstrates how to integrate SQLC-generated database code with the BabyAPI library to create a complete storage layer for your web applications.

## Overview

The BabyAPI library provides a `Storage` interface that allows you to plug in any persistence mechanism. This example shows how to use SQLC (a SQL code generator) with PostgreSQL to create type-safe database operations that integrate seamlessly with BabyAPI.

## Architecture

The key principle is **clean separation** between storage and domain layers to prevent circular imports:

```
storage/                    # Pure storage layer (no domain types)
├── config.go           # Database configuration
├── db.go              # Database client and connection management
├── schema.sql         # Database schema definition
├── sqlc.yaml          # SQLC configuration
├── queries/           # SQL query definitions
│   ├── users_queries.sql
│   ├── sales_queries.sql
│   └── ...
└── db/                # SQLC-generated code (independent storage models)
    ├── models.go      # Pure database models (NO business logic)
    └── *_queries.sql.go # Generated query functions

api/                        # Domain layer (business logic)
├── users/
│   ├── users.go            # Domain models + fromDB() conversion
│   ├── users_storage.go    # Storage adapter (bridges storage↔domain)
│   └── api.go             # API endpoints
└── sales/
    ├── sale.go             # Domain models + fromDB() conversion
    ├── sales_storage.go    # Storage adapter
    └── api.go             # API endpoints
```

**Key Separation Principles:**
- **Storage layer** (`storage/`): Contains only database models and queries - no domain types
- **Domain layer** (`api/*/`): Contains business logic and conversion functions
- **Adapters** (`*_storage.go`): Bridge between storage models and domain models

## Step 1: Database Schema Design

Create your database schema in `storage/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(20) PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  is_admin BOOL NOT NULL DEFAULT FALSE,
  is_verified BOOL NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS sales (
  id VARCHAR(20) PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  start_at TIMESTAMP NOT NULL,
  end_at TIMESTAMP NOT NULL
);
```

## Step 2: SQLC Configuration

Create `storage/sqlc.yaml` to configure SQLC:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "schema.sql"
    gen:
      go:
        package: "db"
        out: "db"
```

## Step 3: Define SQL Queries

Create query files in `storage/queries/`. For example, `users_queries.sql`:

```sql
-- name: GetUser :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users;

-- name: UpsertUser :exec
INSERT INTO users (id, name, email, is_admin, is_verified)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  email = EXCLUDED.email,
  is_verified = EXCLUDED.is_verified;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
```

**Key SQLC conventions:**
- `-- name: FunctionName :operation` defines the function signature
- `:one` returns a single record
- `:many` returns multiple records  
- `:exec` executes without returning data
- `$1, $2, ...` are positional parameters

## Step 4: Database Client

Create the database client that manages the SQLC-generated code and database connection:

```go
// storage/db.go
package storage

import (
    "database/sql"
    "embed"
    "fmt"
    
    _ "github.com/lib/pq"
    "<your_module>/storage/db"
)

//go:generate sqlc generate

//go:embed schema.sql
var ddl string
```

**Important Design Notes:**
- **Storage package imports NO domain types** - only imports generated `storage/db` package
- **Client exposes only `*db.Queries` and `*sql.DB`** - pure storage primitives
- **No business logic** - just connection management and schema initialization
- **Prevents circular imports** because storage layer knows nothing about domain models

**Note:** At this point, the `storage/db/` directory doesn't exist yet - it will be created when you run `go generate`.

## Step 5: Generate Code

Run SQLC to generate the type-safe database code:

```bash
cd storage
go generate
```

This creates:
- `db/models.go` - Struct definitions matching your tables
- `db/users_queries.sql.go` - Type-safe query functions (and similar files for each query file)

Complete the database client implementation:

```go
// storage/db.go (continued)
type Client struct {
    Queries *db.Queries
    DB      *sql.DB
}

func New(config Config) (*Client, error) {
    database, err := sql.Open("postgres", config.String())
    if err != nil {
        return nil, err
    }
    
    client := &Client{
        Queries: db.New(database),
        DB:      database,
    }
    
    // Initialize schema
    _, err = client.DB.Exec(ddl)
    if err != nil {
        return nil, fmt.Errorf("error creating tables: %w", err)
    }
    
    return client, nil
}
```

## Step 6: Domain Models

### The BabyAPI Resource Interface

Your domain models must implement the `Resource` interface to work with BabyAPI:

```go
type Resource interface {
    comparable
    RendererBinder
    
    // GetID returns the resource's ID
    GetID() string
    // ParentID returns the resource's ParentID (if it has one). Return empty string if the Resource does not have a parent
    ParentID() string
}
```

**Key requirements:**
- `comparable` - the type must support equality comparisons
- `RendererBinder` - combines `render.Renderer` and `render.Binder` for HTTP request/response handling
- `GetID()` - returns the unique identifier for the resource
- `ParentID()` - returns the parent resource ID for nested resources (or empty string)

### Domain Model Implementation

Create domain models in each resource package. The easiest way is by extending `babyapi.DefaultResource`:

```go
// api/users/users.go
package users

import (
    "github.com/calvinmclean/babyapi"
    "github.com/rs/xid"
    "<your_module>/storage/db"
)

type User struct {
    babyapi.DefaultResource  // Provides ID, Renderer, and Binder implementations
    
    Name       string
    Email      string
    IsAdmin    bool
    IsVerified bool
}
```

### Critical: Conversion Functions

Each domain package must provide conversion functions between storage models and domain models:

```go
// api/users/users.go (continued)

// Convert from database model to domain model
func fromDB(item db.User) (*User, error) {
    id, err := xid.FromString(item.ID)
    if err != nil {
        return nil, err
    }
    
    return &User{
        DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
        Name:            item.Name,
        Email:           item.Email,
        IsAdmin:         item.IsAdmin,
        IsVerified:      item.IsVerified,
    }, nil
}
```

**Why Conversion Functions Are Essential:**
- **Prevents circular imports** - Storage package doesn't import domain packages
- **Encapsulates business logic** - Handle ID parsing, validation, defaults
- **Type safety** - Ensures clean conversion between layers
- **Testability** - Each conversion can be unit tested independently
    "<your_module>/storage/db"
)

## Step 7: The BabyAPI Storage Interface

BabyAPI provides a generic `Storage[T]` interface that you must implement to integrate with any persistence backend:

```go
// Storage defines how the API will interact with a storage backend
type Storage[T Resource] interface {
	// Get a single resource by ID
	Get(context.Context, string) (T, error)
	// Search will return all resources that match the provided query filters. It can also receive a
	// parentID string if it is a nested resource (empty string if not)
	Search(ctx context.Context, parentID string, query url.Values) ([]T, error)
	// Set will save the provided resource
	Set(context.Context, T) error
	// Delete will delete a resource by ID
	Delete(context.Context, string) error
}
```

**Key points about the interface:**
- `T Resource` is a generic type constraint - your domain models must implement the `Resource` interface
- `Get()` retrieves a single resource by its ID
- `Search()` returns multiple resources with optional filtering via query parameters
- `Set()` saves/updates a resource (create or update)
- `Delete()` removes a resource by ID
- All methods accept `context.Context` for proper cancellation and timeout handling

## Step 8: Storage Adapter

Create an adapter that bridges between storage models and domain models by implementing BabyAPI's `Storage` interface.

**Adapter Pattern Benefits:**
- **Clean separation** - Storage layer stays pure, domain layer handles business logic
- **Bidirectional conversion** - Domain <-> Storage model conversion in both directions
- **Type safety** - Leverages SQLC's generated types
- **Testability** - Adapter can be mocked for domain-layer testing

```go
// api/users/users_storage.go
package users

import (
    "context"
    "net/url"
    
    "<your_module>/storage/db"
)

// Storage adapter bridges SQLC queries to BabyAPI Storage interface
type userStorageAdapter struct {
    *db.Queries  // Embed generated queries - no domain types here
}

func (s userStorageAdapter) Get(ctx context.Context, id string) (*User, error) {
    // Call storage layer
    user, err := s.Queries.GetUser(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Convert storage model → domain model
    return fromDB(user)
}

func (s userStorageAdapter) Search(ctx context.Context, parentID string, query url.Values) ([]*User, error) {
    // Call storage layer
    results, err := s.Queries.ListUsers(ctx)
    if err != nil {
        return nil, err
    }
    
    // Convert each storage model → domain model
    var users []*User
    for _, result := range results {
        user, err := fromDB(result)
        if err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    return users, nil
}

func (s userStorageAdapter) Set(ctx context.Context, user *User) error {
    return s.Queries.UpsertUser(ctx, db.UpsertUserParams{
        ID:         user.GetID(),
        Name:       user.Name,
        Email:      user.Email,
        IsAdmin:    user.IsAdmin,
        IsVerified: user.IsVerified,
    })
}

func (s userStorageAdapter) Delete(ctx context.Context, id string) error {
    return s.Queries.DeleteUser(ctx, id)
}
```

## Step 9: API Integration

Wire up the storage adapter with your BabyAPI:

```go
// api/users/api.go
package users

import (
    "<your_module>/storage"
    "github.com/calvinmclean/babyapi"
)

func NewAPI(dbClient *storage.Client) *API {
    api := &API{}
    
    api.API = babyapi.
        NewAPI("users", "/users", func() *User { return &User{} }).
        SetStorage(userStorageAdapter{dbClient.Queries}).
        // Add other configurations...
    
    return api
}
```

## Advanced Patterns

### Transactions

For operations requiring multiple database updates, use transactions:

```go
func (c API) SetCartQuantity(ctx context.Context, userID, itemID string, quantity int32) error {
    tx, err := c.dbClient.DB.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    qtx := c.dbClient.Queries.WithTx(tx)
    
    // Perform multiple operations
    err = qtx.UpdateInventory(ctx, ...)
    if err != nil {
        return err
    }
    
    err = qtx.UpdateCart(ctx, ...)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### Complex Queries

For queries returning joined data, create custom SQLC queries:

```sql
-- name: GetCart :many
SELECT 
    ci.inventory_item_id,
    ci.quantity,
    i.name,
    i.price,
    ci.quantity * i.price as total
FROM cart_items ci
JOIN inventory_items i ON ci.inventory_item_id = i.id
WHERE ci.user_id = $1 AND ci.expires_at > NOW();
```

### Custom Search/Filtering

Implement search functionality in the `Search` method:

```go
func (s userStorageAdapter) Search(ctx context.Context, parentID string, query url.Values) ([]*User, error) {
    // Extract search parameters
    name := query.Get("name")
    isAdmin := query.Get("is_admin")
    
    if name != "" {
        results, err := s.Queries.SearchUsersByName(ctx, name)
        // Convert and return...
    } else {
        results, err := s.Queries.ListUsers(ctx)
        // Convert and return...
    }
}
```

## Benefits of This Approach

1. **Type Safety**: SQLC generates type-safe Go code from SQL
2. **Performance**: Direct SQL execution without ORM overhead
3. **Maintainability**: SQL queries are clearly visible and testable
4. **Flexibility**: Complex queries and joins are easily expressed
5. **Integration**: Seamless integration with BabyAPI's storage interface

## Best Practices

1. **Keep SQL queries in separate files** - Don't inline SQL in Go code
2. **Use descriptive query names** - Follow SQLC naming conventions
3. **Handle NULL values properly** - SQLC generates `sql.Null*` types
4. **Use transactions for multi-step operations** - Ensure data consistency
5. **Test queries independently** - SQLC makes it easy to test database logic
6. **Version control your schema** - Keep `schema.sql` in git

## Dependencies

Install SQLC: https://docs.sqlc.dev/en/latest/getting-started/install.html

This pattern provides a robust, type-safe foundation for building web applications with BabyAPI and SQL databases.
