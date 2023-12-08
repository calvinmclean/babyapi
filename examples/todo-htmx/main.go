package main

import (
	"html/template"
	"net/http"

	"github.com/calvinmclean/babyapi"
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

			<tbody hx-ext="sse" sse-connect="/todos/listen" sse-swap="data" hx-swap="beforeend">
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

				{{ range .Items }}
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
			hx-patch="/todos/{{ .ID }}"
			hx-headers='{"Accept": "text/html"}'
			hx-include="this"
			{{ $disabled }}>

			<input type="hidden" name="Completed" value="true">
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

func (t *TODO) Patch(newTODO *TODO) *babyapi.ErrResponse {
	if newTODO.Title != "" {
		t.Title = newTODO.Title
	}
	if newTODO.Description != "" {
		t.Description = newTODO.Description
	}
	if newTODO.Completed != nil {
		t.Completed = newTODO.Completed
	}

	return nil
}

func (t *TODO) HTML() string {
	tmpl := template.Must(template.New("todoRow").Parse(todoRowTemplate))
	return babyapi.MustRenderHTML(tmpl, t)
}

type AllTODOs struct {
	Items []*TODO
}

func (at *AllTODOs) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (at *AllTODOs) HTML() string {
	tmpl := template.Must(template.New("todoRow").Parse(todoRowTemplate))
	tmpl = template.Must(tmpl.New("allTODOs").Parse(allTODOsTemplate))
	return babyapi.MustRenderHTML(tmpl, at)
}

func main() {
	api := babyapi.NewAPI[*TODO]("TODOs", "/todos", func() *TODO { return &TODO{} })

	// Use AllTODOs in the GetAll response since it implements HTMLer
	api.SetGetAllResponseWrapper(func(todos []*TODO) render.Renderer {
		return &AllTODOs{todos}
	})

	// HTMX requires a 200 response code to do a swap after delete
	api.SetCustomResponseCode(http.MethodDelete, http.StatusOK)

	// Add SSE handler endpoint which will receive events on the returned channel and write them to the front-end
	todoChan := api.AddServerSentEventHandler("/listen")

	// Push events onto the SSE channel when new TODOs are created
	api.SetOnCreateOrUpdate(func(r *http.Request, t *TODO) *babyapi.ErrResponse {
		if r.Method == http.MethodPost {
			select {
			case todoChan <- &babyapi.ServerSentEvent{Event: "data", Data: t.HTML()}:
			default:
			}
		}
		return nil
	})

	api.RunCLI()
}
