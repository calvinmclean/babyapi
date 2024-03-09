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
- `Storage`: set a different storage backend implementing the `babyapi.Storage` interface
- `AddCustomRoute`: add more routes on the base API
- `Patch`: add custom logic for handling `PATCH` requests
- And many more! (see [examples](https://github.com/calvinmclean/babyapi/tree/main/examples) and [docs](https://pkg.go.dev/github.com/calvinmclean/babyapi))
- Override any of the default handlers and use `babyapi.Handler` shortcut to easily render errors and responses

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

    import "github.com/calvinmclean/babyapi"

    type TODO struct {
        babyapi.DefaultResource

        Title       string
        Description string
        Completed   bool
    }

    func main() {
        api := babyapi.NewAPI(
            "TODOs", "/todos",
            func() *TODO { return &TODO{} },
        )
        api.RunCLI()
    }
    ```

3. Run!

    ```shell
    go mod tidy
    go run main.go serve
    ```

4. Use the built-in CLI to interact with the API:

    ```shell
    # Create a new TODO
    go run main.go post TODOs '{"title": "use babyapi!"}'

    # Get all TODOs
    go run main.go list TODOs

    # Get TODO by ID (use ID from previous responses)
    go run main.go get TODOs cljvfslo4020kglbctog
    ```

<img alt="Simple Example" src="examples/simple/simple.gif" width="600" />

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

The `babytest` package provides some shortcuts and utilities for easily building table tests or simple individual tests. This allows seamlessly creating tests for an API using the convenient `babytest.RequestTest` struct, a function returning an `*http.Request`, or a slice of command-line arguments.

Check out some of the [examples](./examples) for examples of using the `babytest` package.

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

## Extensions

`babyapi` provides an `Extension` interface that can be applied to any API with `api.ApplyExtension()`. Implementations of this interface create custom configurations and modifications that can be applied to multiple APIs. A few extensions are provided by the `babyapi/extensions` package:

- `HATEOAS`: "Hypertext as the engine of application state" is the [3rd and final level of REST API maturity](https://en.wikipedia.org/wiki/Richardson_Maturity_Model#Level_3:_Hypermedia_controls), making your API fully RESTful
- `KVStorage`: provide a few simple configurations to use the `babyapi/storage` package's KV storage client with a local file or Redis
- `HTMX`: HTMX expects 200 responses from DELETE requests, so this changes the response code

## Examples

|                                                 | Description                                                                                                                                                                                                                     | Features                                                                                                                                                                                                                                                                                                                                                                          |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [TODO list](./examples/todo/)                   | This example expands upon the base example to create a realistic TODO list application                                                                                                                                          | <ul><li>Custom `PATCH` logic</li><li>Additional request validation</li><li>Automatically set `CreatedAt` field</li><li>Query parameter parsing to only show completed items</li></ul>                                                                                                                                                                                             |
| [Nested resources](./examples/nested/)          | Demonstrates how to build APIs with nested/related resources. The root resource is an `Artist` which can have `Albums` and `MusicVideos`. Then, `Albums` can have `Songs`                                                       | <ul><li>Nested API resources</li><li>Custom `ResponseWrapper` to add fields from related resources</li></ul>                                                                                                                                                                                                                                                                      |
| [Storage](./examples/storage/)                  | The example shows how to use the `babyapi/storage` package to implement persistent storage                                                                                                                                      | <ul><li>Use `SetStorage` to use a custom storage implementation</li><li>Create a `hord` storage client using `babyapi/storage`</li></ul>                                                                                                                                                                                                                                          |
| [TODO list with HTMX UI](./examples/todo-htmx/) | This is a more complex example that demonstrates an application with HTMX frontend. It uses server-sent events to automatically update with newly-created items                                                                 | <ul><li>Implement `babyapi.HTMLer` for HTML responses</li><li>Set custom HTTP response codes per HTTP method</li><li>Use built-in helpers for handling server-sent events on a custom route</li><li>Use `SetOnCreateOrUpdate` to do additional actions on create</li><li>Handle HTML forms as input instead of JSON (which works automatically and required no changes)</li></ul> |
| [Event RSVP](./examples/event-rsvp/)            | This is a more complex nested example that implements basic authentication, middlewares, and relationships between nested types. The app can be used to create `Events` and provide guests with a link to view details and RSVP | <ul><li>Demonstrates middlewares and nested resource relationships</li><li>Authentication</li><li>Custom non-CRUD endpoints</li><li>More complex HTML templating</li></ul>                                                                                                                                                                                                        |
| [Multiple APIs](./examples/multiple-apis/)      | This example shows how multiple top-level (or any level) sibling APIs can be served, and have CLI functionality, under one root API                                                                                             | <ul><li>Use `NewRootAPI` to create a root API</li><li>Add multiple children to create siblings</li>                                                                                                                                                                                                                                                                               |

Also see a full example of an application implementing a REST API using `babyapi` in my [`automated-garden` project](https://github.com/calvinmclean/automated-garden/tree/main/garden-app).

## Contributing

Please open issues for bugs or feature requests and feel free to create a PR.
