# Baby API
[![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/calvinmclean/babyapi?filename=go.mod)](https://github.com/calvinmclean/babyapi/blob/main/go.mod)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/calvinmclean/babyapi/main.yml?branch=main)
[![License](https://img.shields.io/github/license/calvinmclean/babyapi)](https://github.com/calvinmclean/babyapi/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/calvinmclean/babyapi.svg)](https://pkg.go.dev/github.com/calvinmclean/babyapi)
[![codecov](https://codecov.io/gh/calvinmclean/babyapi/graph/badge.svg?token=BCVPF745D8)](https://codecov.io/gh/calvinmclean/babyapi)

A Go CRUD API framework so simple a baby could use it.

`babyapi` is a super simple framework that automatically creates an HTTP API for create, read, update, and delete operations on a struct. Simply extend the `babyapi.DefaultResource` type to get started.

Implement custom request/response handling by implemented `Renderer` and `Binder` from [`go-chi/render`](https://github.com/go-chi/render). Use provided extension functions to add additional API functionality:
  - `OnCreateOrUpdate`: additional handling for create/update requests
  - `SetStorage`: set a different storage backend implementing the `babyapi.Storage` interface
  - `AddCustomRoute`: add more routes on the base API 
  - `Patch`: add custom logic for handling `PATCH` requests
  - And many more! (see [examples](https://github.com/calvinmclean/babyapi/tree/main/examples) and [docs](https://pkg.go.dev/github.com/calvinmclean/babyapi))


## Getting Started

1. Create a new Go module:
    ```shell
    mkdir babyapi-example
    cd babyapi-example
    go mod init babyapi-example
    ```
2. Write `main.go` to create a `TODO` struct and initialize `babyapi.API`:
    ```go
    package main

    import (
        "github.com/calvinmclean/babyapi"
    )

    type TODO struct {
        babyapi.DefaultResource

        Title       string
        Description string
        Completed   bool
    }

    func main() {
        api := babyapi.NewAPI[*TODO](
            "TODOs", "/todos",
            func() *TODO { return &TODO{} },
        )
        api.Start(":8080")
    }
    ```
3. Run!
    ```shell
    go mod tidy
    go run main.go
    ```
4. Use curl to explore the API (automatic CLI coming soon!)
    ```shell
    # Create a new TODO
    curl localhost:8080/todos -d '{"title": "Use babyapi for everything!"}'

    # Get all TODOs
    curl localhost:8080/todos
    ```


## Client

In addition to providing the HTTP API backend, `babyapi` is also able to create a client that provides access to the base endpoints:

```go
// Create a client from an existing API struct (mostly useful for unit testing):
client := api.Client(serverURL)

// Create a client from the Resource type:
client := babyapi.NewClient[*TODO](addr, "/todos")
```

```go
// Create a new TODO item
todo, err := client.Post(context.Background(), &TODO{Title: "use babyapi!"})

// Get an existing TODO item by ID
todo, err := client.Get(context.Background(), todo.GetID())

// Get all incomplete TODO items
incompleteTODOs, err := client.GetAll(context.Background(), url.Values{
    "completed": []string{"false"},
})

// Delete a TODO item
err := client.Delete(context.Background(), todo.GetID())
```
 
The client provides methods for interacting with the base API and `MakeRequest` and `MakeRequestWithResponse` to interact with custom routes. You can replace the underlying `http.Client` and set a request editor function that can be used to set authorization headers for a client.


## Testing

`babyapi` also makes it easy to unit test your APIs with functions that start an HTTP server with routes, execute the provided request, and return the `httptest.ResponseRecorder`.


## Storage

You can bring any storage backend to `babyapi` by implementing the `Storage` interface. By default, the API will use the built-in `MapStorage` which just uses an in-memory map.

The `babyapi/storage` package provides another generic `Storage` implementation using [`madflojo/hord`](https://github.com/madflojo/hord) to support a variety of key-value store backends. `babyapi/storage` provides helper functions for initializing the `hord` client for Redis or file-based storage.

```go
db, err := storage.NewFileDB(hashmap.Config{
    Filename: "storage.json",
})
db, err := storage.NewRedisDB(redis.Config{
    Server: "localhost:6379",
})

api.SetStorage(storage.NewClient[*TODO](db, "TODO"))
```


## Examples

|                                        | Description                                                                                                                                                               | Features                                                                                                                                                                              |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [TODO list](./examples/todo/)          | This example expands upon the base example to create a realistic TODO list application                                                                                    | <ul><li>Custom `PATCH` logic</li><li>Additional request validation</li><li>Automatically set `CreatedAt` field</li><li>Query parameter parsing to only show completed items</li></ul> |
| [Nested resources](./examples/nested/) | Demonstrates how to build APIs with nested/related resources. The root resource is an `Artist` which can have `Albums` and `MusicVideos`. Then, `Albums` can have `Songs` | <ul><li>Nested API resources</li><li>Custom `ResponseWrapper` to add fields from related resources</li></ul>                                                                          |
| [Storage](./examples/storage/)         | The example shows how to use the `babyapi/storage` package to implement persistent storage                                                                                | <ul><li>Use `SetStorage` to use a custom storage implementation</li><li>Create a `hord` storage client using `babyapi/storage`</li></ul>                                              |

Also see a full example of an application implementing a REST API using `babyapi` in my [`automated-garden` project](https://github.com/calvinmclean/automated-garden/tree/main/garden-app).


## Contributing

Please open issues for bugs or feature requests and feel free to create a PR.
