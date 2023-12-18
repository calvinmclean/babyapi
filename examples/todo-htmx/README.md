# TODO Example Application

This is an example that uses `babyapi` to create a TODO list application with an API and HTMX frontend.

Run the application:
```shell
go run main.go
```

Then, use the UI at http://localhost:8080/todos

## Acorn

[Acorn](https://www.acorn.io) makes it very easy and convenient to run this application in the cloud for free. 

A pre-requisite to running in the cloud is having some form of persistent storage. The `babyapi/storage` package makes it simple to connect the application to Redis.

Normally, you would use Docker to run Redis for local development. Then, you need to write Kubernetes manifests or manage Helm Charts to run in the cloud. Acorn simplifies this process tremendously with the provided [Redis Acorn Service](https://www.acorn.io/resources/tutorials/exploring-the-redis-acorn-service) which runs Redis with persistent volume and auto-generated password.

[![Run in Acorn](https://acorn.io/v1-ui/run/badge?image=ghcr.io+calvinmclean+babyapi-htmx-acorn&ref=calvinmclean&style=for-the-badge&color=brightgreen)](https://acorn.io/run/ghcr.io/calvinmclean/babyapi-htmx-acorn?ref=calvinmclean)

Use the button above to run my pre-built image in the Acorn Sandbox for free, or run locally by installing `acorn` and using:
```shell
acorn run -i .
```
