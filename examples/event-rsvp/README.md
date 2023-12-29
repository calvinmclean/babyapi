# Event RSVP Example

This example implements a simple application for managing event invites and RSVPs. An `Event` is created with a password so only the owner can modify it. Then, an `Invite` each includes a unique identifier to ID the RSVPer and grant them read-only access to the `Event`.

> [!CAUTION]
> This example application deals with passwords, salts, and hashes but is not intended to be 100% cryptographically secure. Passwords are included in visible query params and sent without encryption. The salt and hash are stored in plain text. Invite IDs are used to grant read-only access to the `Event` and [`rs/xid`](https://github.com/rs/xid) is not cryptographically secure

 You can use the CLI the create Events and Invites:

```shell
# Create a new Event
go run examples/event-rsvp/main.go \
  post Event '{"Name": "Party", "Password": "password", "Address": "My House", "Details": "Party on!", "Date": "2024-01-01T20:00:00-07:00"}'

# Add Invite to the Event
go run examples/event-rsvp/main.go \
  -q 'password=password' \
  post Invite '{"Name": "Firstname Lastname"}' cm2vm15o4026969kq690
```

Or use the UI!


## How To

Run the application:
```shell
go run main.go
```

Then, use the UI at http://localhost:8080/events


## Acorn

[Acorn](https://www.acorn.io) makes it very easy and convenient to run this application in the cloud for free. 

A pre-requisite to running in the cloud is having some form of persistent storage. The `babyapi/storage` package makes it simple to connect the application to Redis.

Normally, you would use Docker to run Redis for local development. Then, you need to write Kubernetes manifests or manage Helm Charts to run in the cloud. Acorn simplifies this process tremendously with the provided [Redis Acorn Service](https://www.acorn.io/resources/tutorials/exploring-the-redis-acorn-service) which runs Redis with persistent volume and auto-generated password.

[![Run in Acorn](https://acorn.io/v1-ui/run/badge?image=ghcr.io+calvinmclean+babyapi-event-rsvp-acorn&ref=calvinmclean&style=for-the-badge&color=brightgreen)](https://acorn.io/run/ghcr.io/calvinmclean/babyapi-event-rsvp-acorn?ref=calvinmclean)

Use the button above to run my pre-built image in the Acorn Sandbox for free, or run locally by installing `acorn` and using:
```shell
acorn run -i .
```
