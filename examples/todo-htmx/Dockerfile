FROM golang:1.21-alpine AS build
RUN mkdir /build
ADD . /build
WORKDIR /build
RUN go mod init todo-htmx && \
    go mod tidy && \
    go build -o todo-htmx .

FROM alpine:latest AS production
RUN mkdir /app
WORKDIR /app
COPY --from=build /build/todo-htmx .
ENTRYPOINT ["/app/todo-htmx", "serve"]
