package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/extensions"
	"github.com/calvinmclean/babyapi/html"
	"github.com/go-chi/render"
)

//go:embed template.html
var templates embed.FS

const (
	invitesCtxKey  babyapi.ContextKey = "invites"
	passwordCtxKey babyapi.ContextKey = "password"

	eventPage       html.Template = "eventPage"
	invitePage      html.Template = "invitePage"
	rsvpButtons     html.Template = "rsvpButtons"
	bulkInvites     html.Template = "bulkInvites"
	createEventPage html.Template = "createEventPage"
)

type API struct {
	Events  *babyapi.API[*Event]
	Invites *babyapi.API[*Invite]
}

// Export invites to CSV format for use with external tools
func (api *API) export(w http.ResponseWriter, r *http.Request) render.Renderer {
	event, httpErr := api.Events.GetRequestedResource(r)
	if httpErr != nil {
		return httpErr
	}

	invites, err := api.Invites.Storage.GetAll(r.Context(), func(i *Invite) bool {
		return i.EventID == event.GetID()
	})
	if err != nil {
		return babyapi.InternalServerError(err)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=event_%s_invites.csv", event.GetID()))

	csvWriter := csv.NewWriter(w)
	err = csvWriter.Write([]string{"ID", "Name", "Contact", "RSVP", "Link"})
	if err != nil {
		return babyapi.InternalServerError(err)
	}

	for _, invite := range invites {
		rsvp := ""
		if invite.RSVP != nil {
			rsvp = fmt.Sprintf("%t", *invite.RSVP)
		}
		err = csvWriter.Write([]string{
			invite.GetID(),
			invite.Name,
			invite.Contact,
			rsvp,
			invite.link(r),
		})
		if err != nil {
			return babyapi.InternalServerError(err)
		}
	}

	csvWriter.Flush()

	err = csvWriter.Error()
	if err != nil {
		return babyapi.InternalServerError(err)
	}

	return nil
}

// Use a custom route to set RSVP so rsvpResponse can be used to return HTML buttons
func (api *API) rsvp(r *http.Request, invite *Invite) (render.Renderer, *babyapi.ErrResponse) {
	if err := r.ParseForm(); err != nil {
		return nil, babyapi.ErrInvalidRequest(fmt.Errorf("error parsing form data: %w", err))
	}

	rsvp := r.Form.Get("RSVP") == "true"
	invite.RSVP = &rsvp

	err := api.Invites.Storage.Set(r.Context(), invite)
	if err != nil {
		return nil, babyapi.InternalServerError(err)
	}

	return rsvpButtons.Renderer(struct {
		*Invite
		Attending string
	}{invite, invite.attending()}), nil
}

// Allow adding bulk invites with a single request
func (api *API) addBulkInvites(r *http.Request, event *Event) (render.Renderer, *babyapi.ErrResponse) {
	if err := r.ParseForm(); err != nil {
		return nil, babyapi.ErrInvalidRequest(fmt.Errorf("error parsing form data: %w", err))
	}

	inputs := strings.Split(r.Form.Get("invites"), ";")

	invites := []*Invite{}
	for _, invite := range inputs {
		split := strings.Split(invite, ",")

		name := split[0]

		var contact string
		if len(split) > 1 {
			contact = split[1]
		}

		inv := &Invite{
			DefaultResource: babyapi.NewDefaultResource(),
			Name:            strings.TrimSpace(name),
			Contact:         strings.TrimSpace(contact),
			EventID:         event.GetID(),
		}
		invites = append(invites, inv)

		err := api.Invites.Storage.Set(r.Context(), inv)
		if err != nil {
			return nil, babyapi.InternalServerError(err)
		}
	}

	return bulkInvites.Renderer(invites), nil
}

// authenticationMiddleware enforces access to Events and Invites. Admin access to an Event requires a password query parameter.
// Access to Invites is allowed by the invite ID and requires no extra auth. The invite ID in the path or query parameter allows
// read-only access to the Event
func (api *API) authenticationMiddleware(r *http.Request, event *Event) (*http.Request, *babyapi.ErrResponse) {
	password := r.URL.Query().Get("password")
	inviteID := r.URL.Query().Get("invite")
	if inviteID == "" {
		inviteID = api.Invites.GetIDParam(r)
	}

	switch {
	case password != "":
		err := event.Authenticate(password)
		if err == nil {
			return r, nil
		}
	case inviteID != "":
		invite, err := api.Invites.Storage.Get(r.Context(), inviteID)
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
}

// getAllInvitesMiddleware will get all invites when rendering HTML so it is accessible to the endpoint
func (api *API) getAllInvitesMiddleware(r *http.Request, event *Event) (*http.Request, *babyapi.ErrResponse) {
	if render.GetAcceptedContentType(r) != render.ContentTypeHTML {
		return r, nil
	}
	// If password auth is used and this middleware is reached, we know it's admin
	// Otherwise, don't fetch invites
	if r.URL.Query().Get("password") == "" {
		return r, nil
	}

	invites, err := api.Invites.Storage.GetAll(r.Context(), func(i *Invite) bool {
		return i.EventID == event.GetID()
	})
	if err != nil {
		return r, babyapi.InternalServerError(err)
	}

	ctx := context.WithValue(r.Context(), invitesCtxKey, invites)
	r = r.WithContext(ctx)
	return r, nil
}

type Event struct {
	babyapi.DefaultResource

	Name     string
	Contact  string
	Date     string
	Location string
	Details  string

	// Password should only be used in POST requests to create new Events and then is removed
	Password string `json:",omitempty"`
	// this unexported password allows using it internally without exporting to storage or responses
	password string

	// These fields are excluded from responses
	Salt string `json:",omitempty"`
	Key  string `json:",omitempty"`
}

func (e *Event) Render(w http.ResponseWriter, r *http.Request) error {
	// Keep Salt and Key private when creating responses
	e.Salt = ""
	e.Key = ""

	if r.Method == http.MethodPost {
		path := fmt.Sprintf("/events/%s?password=%s", e.GetID(), e.password)
		headers := `{"Accept": "text/html"}`
		w.Header().Add("HX-Location", fmt.Sprintf(`{"path": "%s", "headers": %s}`, path, headers))
	}

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
		if e.Password == "" {
			return errors.New("missing required 'password' field")
		}

		var err error
		e.Salt, err = randomSalt()
		if err != nil {
			return fmt.Errorf("error generating random salt: %w", err)
		}

		e.Key = hash(e.Salt, e.Password)

		e.password = e.Password
		e.Password = ""
	}

	return e.DefaultResource.Bind(r)
}

func (e *Event) HTML(r *http.Request) string {
	return eventPage.Render(r, struct {
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
	Contact string

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

	return invitePage.Render(r, struct {
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

func (i *Invite) link(r *http.Request) string {
	return fmt.Sprintf("%s/events/%s/invites/%s", r.Host, i.EventID, i.GetID())
}

func main() {
	api := createAPI()
	api.Events.RunCLI()
}

func createAPI() *API {
	api := &API{
		Events: babyapi.NewAPI(
			"Event", "/events",
			func() *Event { return &Event{} },
		),
		Invites: babyapi.NewAPI(
			"Invite", "/invites",
			func() *Invite { return &Invite{} },
		),
	}

	api.Events.AddCustomRootRoute(http.MethodGet, "/", http.RedirectHandler("/events", http.StatusFound))

	api.Invites.ApplyExtension(extensions.HTMX[*Invite]{})

	api.Invites.AddCustomRoute(http.MethodPost, "/bulk", api.Events.GetRequestedResourceAndDo(api.addBulkInvites))

	api.Invites.AddCustomRoute(http.MethodGet, "/export", babyapi.Handler(api.export))

	api.Invites.AddCustomIDRoute(http.MethodPut, "/rsvp", api.Invites.GetRequestedResourceAndDo(api.rsvp))

	api.Events.AddCustomRootRoute(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, api.Events.Base(), http.StatusSeeOther)
	}))

	api.Events.AddNestedAPI(api.Invites)

	api.Events.GetAll = babyapi.Handler(func(_ http.ResponseWriter, r *http.Request) render.Renderer {
		if render.GetAcceptedContentType(r) != render.ContentTypeHTML {
			return babyapi.ErrForbidden
		}

		return createEventPage.Renderer(map[string]any{})
	})

	api.Events.AddIDMiddleware(api.Events.GetRequestedResourceAndDoMiddleware(api.authenticationMiddleware))

	api.Events.AddIDMiddleware(api.Events.GetRequestedResourceAndDoMiddleware(api.getAllInvitesMiddleware))

	filename := os.Getenv("STORAGE_FILE")
	if filename == "" {
		filename = "storage.json"
	}

	dbConfig := extensions.KVConnectionConfig{
		Filename:      filename,
		RedisHost:     os.Getenv("REDIS_HOST"),
		RedisPassword: os.Getenv("REDIS_PASS"),
	}

	db, err := dbConfig.CreateDB()
	if err != nil {
		panic(err)
	}

	api.Events.ApplyExtension(extensions.KeyValueStorage[*Event]{DB: db})
	api.Invites.ApplyExtension(extensions.KeyValueStorage[*Invite]{DB: db})

	html.SetFS(templates, "template.html")
	html.SetFuncs(func(r *http.Request) map[string]any {
		return map[string]any{
			"serverURL": func() string {
				return r.Host
			},
			"attending": func(i *Invite) string {
				return i.attending()
			},
		}
	})

	return api
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
