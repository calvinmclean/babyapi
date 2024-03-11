# Background Worker Example

This example shows a common scenario where an application has a running API and some kind of background worker doing its own tasks. It is useful to be able to shut down the API and the background worker gracefully. This demo shows two ways that this can be achieved with `babyapi`:

1. Use `WithContext()` to pass a `context.Context` to the API. This allows the API to shutdown when the context is cancelled. Use this context in the background worker as well to easily shut it down also
    - This example will automatically shutdown using a context cancellation after 5 seconds. Simply run `go run main.go serve` and let it run to see this in action

2. Use `api.Done()` similarly to `ctx.Done()` so the background worker can automatically shut down when the API is stopped
    - When using `api.RunCLI()`, the API can be stopped using Ctrl-C, so run `go run main.go serve` followed by the Ctrl-C keystroke to see the background worker gracefully shutdown as well

Both of these methods will also work if you are using `api.Serve(addr)` instead of `api.RunCLI()`. Just replace Ctrl-C in the second example with a call to `api.Stop()` from another point in your code.
