package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/madflojo/hord"
	"github.com/madflojo/hord/drivers/hashmap"
	"github.com/madflojo/hord/drivers/redis"
)

const invitesCtxKey babyapi.ContextKey = "invites"

const (
	// HTML parent page
	rootTemplate = `<!doctype html>
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Event RSVP</title>
		<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/uikit@3.17.11/dist/css/uikit.min.css" />
		<script src="https://unpkg.com/htmx.org@1.9.8"></script>
	</head>

	<body>
	{{ template "body" . }}
	</body>
</html>`

	// /events/{EventID}: display event details and admin view if admin
	eventPage = `
{{ template "eventDetails" . }}
{{ if .Password }}
{{ template "eventAdmin" . }}
{{ end }}
`

	// Show basic event details in a table
	eventDetailsTable = `
<div class="uk-card uk-card-body uk-card-default uk-margin-left uk-margin-right uk-margin-top">
    <h2 class="uk-card-title uk-heading-medium uk-text-center">{{ .Name }}</h2>
	<table class="uk-table uk-table-divider">
		<thead>
		</thead>

		<tbody>
			<tr>
				<td><b>Name</b></td>
				<td>{{ .Name }}</td>
			</tr>
			<tr>
				<td><b>Date</b></td>
				<td>{{ .Date }}</td>
			</tr>
			<tr>
				<td><b>Address</b></td>
				<td>{{ .Address }}</td>
			</tr>
			<tr>
				<td><b>Details</b></td>
				<td>{{ .Details }}</td>
			</tr>
		</tbody>
	</table>
</div>`

	// Show list of invites and RSVP status
	// Form to create new invite
	// Form for bulk invite from a list of names
	eventAdminPage = `
<div class="uk-card uk-card-body uk-card-default uk-margin-left uk-margin-right uk-margin-top">
    <h2 class="uk-card-title uk-heading-medium uk-text-center">Invites</h2>
	<table class="uk-table uk-table-divider uk-margin-left uk-margin-right">
		<thead>
			<tr>
				<th>Name</th>
				<th>RSVP</th>
			</tr>
		</thead>
	
		<tbody>
			{{ range .Invites }}
			{{ template "inviteRow" . }}
			{{ end }}
		</tbody>
	</table>
</div>`

	// Used in eventAdminTemplate to show individual invite, delete button, and button to copy invite link
	inviteRow = `
<tr>
	<td>{{ .Name }}</td>
	{{ if .RSVP }}
	<td>{{ .RSVP }}</td>
	{{ else }}
	<td>N/A</td>
	{{ end }}
</tr>`

	// /invites/{InviteID}: display event details and include buttons to set RSVP
	invitePage = `
{{ template "eventDetails" .Event }}

<div class="uk-card uk-card-body uk-card-default uk-margin-left uk-margin-right uk-margin-top">
	<h2 class="uk-card-title uk-heading-medium uk-text-center">
	Hello {{ .Name }}!
	</h2>

	{{ template "rsvpButtons" . }}
</div>
`
	rsvpButtons = `
<div class="uk-text-center" id="rsvp-buttons">
	<p>Your current status is: {{ .Attending }}</p>

	{{ $attendDisabled := "" }}
	{{ $unattendDisabled := "" }}
	{{ if eq .Attending "attending" }}
		{{ $attendDisabled = "disabled" }}
	{{ end }}
	{{ if eq .Attending "not attending" }}
		{{ $unattendDisabled = "disabled" }}
	{{ end }}

	<div
		hx-headers='{"Accept": "text/html"}'
		hx-target="#rsvp-buttons"
		hx-swap="outerHTML">

		<button class="uk-button uk-button-primary uk-button-large"
			hx-put="/events/{{ .EventID }}/invites/{{ .ID }}/rsvp"
			hx-include="this"
			{{ $attendDisabled }}>
			
			<input type="hidden" name="RSVP" value="true">
			Attend
		</button>

		<button class="uk-button uk-button-danger uk-button-large"
			hx-put="/events/{{ .EventID }}/invites/{{ .ID }}/rsvp"
			hx-include="this"
			{{ $unattendDisabled }}>

			<input type="hidden" name="RSVP" value="false">
			Do Not Attend
		</button>
	</div>
</div>`
)

type Event struct {
	babyapi.DefaultResource

	Name    string
	Date    time.Time
	Address string
	Details string

	// These fields are excluded from responses
	Salt string `json:",omitempty"`
	Key  string `json:",omitempty"`
}

func (e *Event) Render(w http.ResponseWriter, r *http.Request) error {
	// Keep Salt and Key private when creating responses
	e.Salt = ""
	e.Key = ""

	return nil
}

// Disable PUT requests for Events because it complicates things with passwords
// When creating a new resource with POST, salt and hash the password for storing
func (e *Event) Bind(r *http.Request) error {
	switch r.Method {
	case http.MethodPut:
		render.Status(r, http.StatusMethodNotAllowed)
		return fmt.Errorf("PUT not allowed")
	case http.MethodPost:
		password := r.URL.Query().Get("password")
		if password == "" {
			return errors.New("missing required 'password' query parameter")
		}

		var err error
		e.Salt, err = randomSalt()
		if err != nil {
			return fmt.Errorf("error generating random salt: %w", err)
		}

		e.Key = hash(e.Salt, password)
	}

	return e.DefaultResource.Bind(r)
}

func (e *Event) HTML(r *http.Request) string {
	templates := map[string]string{
		"body":         eventPage,
		"eventDetails": eventDetailsTable,
		"eventAdmin":   eventAdminPage,
		"inviteRow":    inviteRow,
		"page":         rootTemplate,
	}

	return babyapi.MustRenderHTMLMap(templates, "page", struct {
		Password string
		*Event
		Invites []*Invite
	}{r.URL.Query().Get("password"), e, getInvitesFromContext(r.Context())})
}

func (e *Event) Authenticate(password string) error {
	if hash(e.Salt, password) != e.Key {
		return errors.New("invalid password")
	}
	return nil
}

type Invite struct {
	babyapi.DefaultResource

	Name    string
	EventID string
	RSVP    *bool // nil = no response, otherwise true/false
}

// Set EventID to event from URL path when creating a new Invite
func (i *Invite) Bind(r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		i.EventID = babyapi.GetIDParam(r, "Event")
	}

	return i.DefaultResource.Bind(r)
}

func (i *Invite) HTML(r *http.Request) string {
	event, _ := babyapi.GetResourceFromContext[*Event](r.Context(), babyapi.ContextKey("Event"))

	templates := map[string]string{
		"body":         invitePage,
		"eventDetails": eventDetailsTable,
		"rsvpButtons":  rsvpButtons,
		"page":         rootTemplate,
	}

	return babyapi.MustRenderHTMLMap(templates, "page", struct {
		*Invite
		Attending string
		Event     *Event
	}{i, i.attending(), event})
}

// get RSVP status as a string for easier template processing
func (i *Invite) attending() string {
	attending := "unknown"
	if i.RSVP != nil && *i.RSVP {
		attending = "attending"
	}
	if i.RSVP != nil && !*i.RSVP {
		attending = "not attending"
	}

	return attending
}

// rsvpResponse is a custom response struct that allows implementing a different HTML method for HTMLer
// This will just render the HTML buttons for an HTMX partial swap
type rsvpResponse struct {
	*Invite
}

func (rsvp *rsvpResponse) HTML(r *http.Request) string {
	return babyapi.MustRenderHTML(
		template.Must(template.New("rsvpButtons").Parse(rsvpButtons)),
		struct {
			*Invite
			Attending string
		}{rsvp.Invite, rsvp.attending()},
	)
}

func main() {
	api := createAPI()
	api.RunCLI()
}

func createAPI() *babyapi.API[*Event] {
	eventAPI := babyapi.NewAPI[*Event](
		"Event", "/events",
		func() *Event { return &Event{} },
	)

	inviteAPI := babyapi.NewAPI[*Invite](
		"Invite", "/invites",
		func() *Invite { return &Invite{} },
	)

	// Use a custom route to set RSVP so rsvpResponse can be used to return HTML buttons
	inviteAPI.AddCustomIDRoute(chi.Route{
		Pattern: "/rsvp",
		Handlers: map[string]http.Handler{
			http.MethodPut: inviteAPI.GetRequestedResourceAndDo(func(r *http.Request, invite *Invite) (render.Renderer, *babyapi.ErrResponse) {
				if err := r.ParseForm(); err != nil {
					return nil, babyapi.ErrInvalidRequest(fmt.Errorf("error parsing form data: %w", err))
				}

				rsvp := r.Form.Get("RSVP") == "true"
				invite.RSVP = &rsvp

				err := inviteAPI.Storage().Set(invite)
				if err != nil {
					return nil, babyapi.InternalServerError(err)
				}
				return &rsvpResponse{invite}, nil
			}),
		},
	})

	eventAPI.AddNestedAPI(inviteAPI)

	// This middleware is responsible for authentication. It uses a password for admin access or the invite ID from URL or
	// query parameter for read-only access to the Event
	eventAPI.AddIDMiddleware(eventAPI.GetRequestedResourceAndDoMiddleware(func(r *http.Request, event *Event) (*http.Request, *babyapi.ErrResponse) {
		password := r.URL.Query().Get("password")
		inviteID := r.URL.Query().Get("invite")
		if inviteID == "" {
			// TODO: this should be more easily accessible through go-chi or babyapi if I can't get go-chi to work
			inviteID = strings.TrimPrefix(r.URL.String(), fmt.Sprintf("/events/%s/invites/", event.ID))
			inviteID = strings.TrimSuffix(inviteID, "/rsvp")
		}

		switch {
		case password != "":
			err := event.Authenticate(password)
			if err == nil {
				return r, nil
			}
		case inviteID != "":
			invite, err := inviteAPI.Storage().Get(inviteID)
			if err != nil {
				if errors.Is(err, babyapi.ErrNotFound) {
					return r, babyapi.ErrForbidden
				}
				return r, babyapi.InternalServerError(err)
			}
			if invite.EventID == event.GetID() {
				return r, nil
			}
		}

		return r, babyapi.ErrForbidden
	}))

	// Get all invites when rendering HTML so it is accessible to the endpoint
	eventAPI.AddIDMiddleware(eventAPI.GetRequestedResourceAndDoMiddleware(
		func(r *http.Request, event *Event) (*http.Request, *babyapi.ErrResponse) {
			if render.GetAcceptedContentType(r) != render.ContentTypeHTML {
				return r, nil
			}
			// If password auth is used and this middleware is reached, we know it's admin
			// Otherwise, don't fetch invites
			if r.URL.Query().Get("password") == "" {
				return r, nil
			}

			invites, err := inviteAPI.Storage().GetAll(func(i *Invite) bool {
				return i.EventID == event.GetID()
			})
			if err != nil {
				return r, babyapi.InternalServerError(err)
			}

			ctx := context.WithValue(r.Context(), invitesCtxKey, invites)
			r = r.WithContext(ctx)
			return r, nil
		},
	))

	db, err := createDB()
	if err != nil {
		panic(err)
	}

	eventAPI.SetStorage(storage.NewClient[*Event](db, "Event"))
	inviteAPI.SetStorage(storage.NewClient[*Invite](db, "Invite"))

	return eventAPI
}

func hash(salt, password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(salt + password))

	return hex.EncodeToString(hasher.Sum(nil))
}

func randomSalt() (string, error) {
	randomBytes := make([]byte, 24)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(randomBytes), nil
}

func getInvitesFromContext(ctx context.Context) []*Invite {
	ctxData := ctx.Value(invitesCtxKey)
	if ctxData == nil {
		return nil
	}

	invites, ok := ctxData.([]*Invite)
	if !ok {
		return nil
	}

	return invites
}

// Optionally setup redis storage if environment variables are defined
func createDB() (hord.Database, error) {
	host := os.Getenv("REDIS_HOST")
	password := os.Getenv("REDIS_PASS")

	if password == "" && host == "" {
		filename := os.Getenv("STORAGE_FILE")
		if filename == "" {
			filename = "storage.json"
		}
		return storage.NewFileDB(hashmap.Config{
			Filename: filename,
		})
	}

	return storage.NewRedisDB(redis.Config{
		Server:   host + ":6379",
		Password: password,
	})
}
