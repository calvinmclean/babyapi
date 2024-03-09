package main

import (
	"html/template"
	"net/http"
	"os"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/extensions"

	"github.com/go-chi/render"
)

const (
	allTODOsTemplate = `<!doctype html>
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>TODOs</title>
		<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/uikit@3.17.11/dist/css/uikit.min.css" />
		<script src="https://unpkg.com/htmx.org@1.9.8"></script>
		<script src="https://unpkg.com/htmx.org/dist/ext/sse.js"></script>
	</head>

	<style>
	tr.htmx-swapping td {
		opacity: 0;
		transition: opacity 1s ease-out;
	}
	</style>

	<body>
		<table class="uk-table uk-table-divider uk-margin-left uk-margin-right">
			<colgroup>
				<col>
				<col>
				<col style="width: 300px;">
			</colgroup>

			<thead>
				<tr>
					<th>Title</th>
					<th>Description</th>
					<th></th>
				</tr>
			</thead>

			<tbody hx-ext="sse" sse-connect="/todos/listen" sse-swap="newTODO" hx-swap="beforeend">
				<form hx-post="/todos" hx-swap="none" hx-on::after-request="this.reset()">
					<td>
						<input class="uk-input" name="Title" type="text">
					</td>
					<td>
						<input class="uk-input" name="Description" type="text">
					</td>
					<td>
						<button type="submit" class="uk-button uk-button-primary">Add TODO</button>
					</td>
				</form>

				{{ range . }}
				{{ template "todoRow" . }}
				{{ end }}
			</tbody>
		</table>
	</body>
</html>`

	todoRowTemplate = `<tr hx-target="this" hx-swap="outerHTML">
	<td>{{ .Title }}</td>
	<td>{{ .Description }}</td>
	<td>
		{{- $color := "primary" }}
		{{- $disabled := "" }}
		{{- if .Completed }}
			{{- $color = "secondary" }}
			{{- $disabled = "disabled" }}
		{{- end -}}

		<button class="uk-button uk-button-{{ $color }}"
			hx-put="/todos/{{ .ID }}"
			hx-headers='{"Accept": "text/html"}'
			hx-include="this"
			{{ $disabled }}>

			<input type="hidden" name="Completed" value="true">
			<input type="hidden" name="Title" value="{{ .Title }}">
			<input type="hidden" name="Description" value="{{ .Description }}">
			<input type="hidden" name="ID" value="{{ .ID }}">
			Complete
		</button>

		<button class="uk-button uk-button-danger" hx-delete="/todos/{{ .ID }}" hx-swap="swap:1s">
			Delete
		</button>
	</td>
</tr>`
)

type TODO struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   *bool
}

func (t *TODO) HTML(*http.Request) string {
	tmpl := template.Must(template.New("todoRow").Parse(todoRowTemplate))
	return babyapi.MustRenderHTML(tmpl, t)
}

type AllTODOs []*TODO

func (at AllTODOs) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (at AllTODOs) HTML(*http.Request) string {
	tmpl := template.Must(template.New("todoRow").Parse(todoRowTemplate))
	tmpl = template.Must(tmpl.New("allTODOs").Parse(allTODOsTemplate))
	return babyapi.MustRenderHTML(tmpl, at)
}

func createAPI() *babyapi.API[*TODO] {
	api := babyapi.NewAPI("TODOs", "/todos", func() *TODO { return &TODO{} })

	// Use AllTODOs in the GetAll response since it implements HTMLer
	api.SetGetAllResponseWrapper(func(todos []*TODO) render.Renderer {
		return AllTODOs(todos)
	})

	api.ApplyExtension(extensions.HTMX[*TODO]{})

	// Add SSE handler endpoint which will receive events on the returned channel and write them to the front-end
	todoChan := api.AddServerSentEventHandler("/listen")

	// Push events onto the SSE channel when new TODOs are created
	api.SetOnCreateOrUpdate(func(r *http.Request, t *TODO) *babyapi.ErrResponse {
		if r.Method != http.MethodPost {
			return nil
		}

		select {
		case todoChan <- &babyapi.ServerSentEvent{Event: "newTODO", Data: t.HTML(r)}:
		default:
			logger := babyapi.GetLoggerFromContext(r.Context())
			logger.Info("no listeners for server-sent event")
		}
		return nil
	})

	// Optionally setup redis storage if environment variables are defined
	api.ApplyExtension(extensions.KeyValueStorage[*TODO]{
		KVConnectionConfig: extensions.KVConnectionConfig{
			RedisHost:     os.Getenv("REDIS_HOST"),
			RedisPassword: os.Getenv("REDIS_PASS"),
			Filename:      os.Getenv("STORAGE_FILE"),
			Optional:      true,
		},
	})

	return api
}

func main() {
	api := createAPI()
	api.RunCLI()
}
