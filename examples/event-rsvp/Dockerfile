FROM golang:1.21-alpine AS build
RUN mkdir /build
ADD . /build
WORKDIR /build
RUN go mod init event-rsvp && \
    go mod tidy && \
    go build -o event-rsvp .

FROM alpine:latest AS production
RUN mkdir /app
WORKDIR /app
COPY --from=build /build/event-rsvp .
ENTRYPOINT ["/app/event-rsvp", "serve"]
