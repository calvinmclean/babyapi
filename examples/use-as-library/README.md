# Use As Library

This example shows how a subset of `babyapi` features can be used as a library rather than a full framework. The more interesting features that can be used this way are:
  - Use `babyapi.Handler` with stdlib HTTP server to simplify response rendering
  - Use `babyapi.ReadRequestBodyAndDo` as an HTTP handler that automatically reads the request body and responds based on the generic parameter
  - Use `babyapi/html` package to simplify responding with HTML templates
  - Use the generic `babyapi.MakeRequest` function to make a request to the server

Run the application:
```shell
go run main.go
```

Test the `POST` handler with `curl`:
```shell
curl localhost:8080/create-data -H "Content-Type: application/json" -d '{"Data": {"a":"b"}}'
```

Visit the other endpoints in the browser:
- http://localhost:8080/hello?name=YourName
- http://localhost:8080/data
