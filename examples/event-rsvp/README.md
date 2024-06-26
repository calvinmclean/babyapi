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
