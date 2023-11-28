# Baby API
[![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/calvinmclean/babyapi?filename=go.mod)](https://github.com/calvinmclean/babyapi/blob/main/go.mod)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/calvinmclean/babyapi/main.yml?branch=main)
[![License](https://img.shields.io/github/license/calvinmclean/babyapi)](https://github.com/calvinmclean/babyapi/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/calvinmclean/babyapi.svg)](https://pkg.go.dev/github.com/calvinmclean/babyapi)
[![codecov](https://codecov.io/gh/calvinmclean/babyapi/branch/main/graph/badge.svg?token=20LWQYHKE8)](https://codecov.io/gh/calvinmclean/babyapi)

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

// TODO: Add GIFs?


## Examples
// TODO: make markdown table describing each example
// TODO: link to garden-app


## Contributing
Please open issues for bugs or feature requests and feel free to create a PR.
