package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type TODO struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   *bool
	CreatedAt   time.Time
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

func (t *TODO) Bind(r *http.Request) error {
	err := t.DefaultResource.Bind(r)
	if err != nil {
		return err
	}

	switch r.Method {
	case http.MethodPost:
		t.CreatedAt = time.Now()
		fallthrough
	case http.MethodPut:
		if t.Title == "" {
			return errors.New("missing required title field")
		}
	}

	return nil
}

func (t *TODO) HTML() string {
	rowTemplate := `<tr>
<td>{{ .Title }}</td>
<td>{{ .Description }}</td>
<td>
	<button
		class="btn btn-{{ if .Completed }}secondary{{ else }}primary{{ end }}"
		hx-put="/todos/{{ .ID }}/complete"
		hx-headers='{"Accept": "text/html"}'
		hx-swap="outerHTML"
		hx-target="closest tr"
		{{ if .Completed }}disabled{{ end }}>
	Complete
	</button>
	<button
		class="btn btn-danger"
		hx-delete="/todos/{{ .ID }}"
		hx-swap="outerHTML swap:1s"
		hx-target="closest tr">
	Delete
	</button>
</td>
</tr>
`

	tmpl := template.Must(template.New("TODO").Parse(rowTemplate))

	var renderedOutput bytes.Buffer
	err := tmpl.Execute(&renderedOutput, t)
	if err != nil {
		panic(err)
	}

	return renderedOutput.String()
}

type AllTODOs struct {
	Items []*TODO
}

func (at *AllTODOs) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (at *AllTODOs) HTML() string {
	htmlTemplate := `<!doctype html>
	<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>TODOs</title>
		<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css">
		<script src="https://unpkg.com/htmx.org@1.9.8"></script>
		<script src="https://unpkg.com/htmx.org/dist/ext/sse.js"></script>
		<script src="https://unpkg.com/htmx.org/dist/ext/json-enc.js"></script>
	</head>

	<style>
	tr.htmx-swapping td {
		opacity: 0;
		transition: opacity 1s ease-out;
	}
	</style>
	<body>
		<table class="table">
			<thead>
				<tr>
					<th>Title</th>
					<th>Description</th>
					<th></th>
				</tr>
			</thead>
			<tbody id="todos-table" hx-ext="sse"sse-connect="/todos/listen" sse-swap="data" hx-swap="beforeend">
				{{ range .Items }}
				{{ renderHTML . }}
				{{ end }}
			</tbody>
		</table>
	</body>
</html>`

	tmpl := template.Must(template.New("outerHTML").Funcs(map[string]any{
		"renderHTML": func(htmler babyapi.HTMLer) template.HTML {
			return template.HTML(htmler.HTML())
		},
	}).Parse(htmlTemplate))

	var renderedOutput bytes.Buffer
	err := tmpl.Execute(&renderedOutput, at)
	if err != nil {
		panic(err)
	}

	return renderedOutput.String()
}

func main() {
	api := babyapi.NewAPI[*TODO]("TODOs", "/todos", func() *TODO { return &TODO{} })
	api.SetGetAllFilter(func(r *http.Request) babyapi.FilterFunc[*TODO] {
		return func(t *TODO) bool {
			getCompletedParam := r.URL.Query().Get("completed")
			// No filtering if param is not provided
			if getCompletedParam == "" {
				return true
			}

			if getCompletedParam == "true" {
				return t.Completed != nil && *t.Completed
			}

			return t.Completed == nil || !*t.Completed
		}
	})

	api.SetGetAllResponseWrapper(func(todos []*TODO) render.Renderer {
		return &AllTODOs{todos}
	})

	// HTMX requires a 200 response code to do a swap after delete
	api.SetCustomResponseCode(http.MethodDelete, http.StatusOK)

	// Although this can already be achieved with a PATCH request, this is simpler to handle with HTMX
	// since it doesn't require any JSON or parameters
	// Other options are to create a `Render` method for TODO that will allow reading "completed" from a query parameter
	// or form data, or use HTMX json-enc and a custom unmarshaller that allows boolean strings
	api.AddCustomIDRoute(chi.Route{
		Pattern: "/complete",
		Handlers: map[string]http.Handler{
			http.MethodPut: api.GetRequestedResourceAndDo(func(r *http.Request, t *TODO) (render.Renderer, *babyapi.ErrResponse) {
				trueBool := true
				t.Completed = &trueBool

				err := api.Storage().Set(t)
				if err != nil {
					return nil, babyapi.InternalServerError(fmt.Errorf("error storing: %w", err))
				}

				return t, nil
			}),
		},
	})

	todoChan := api.AddServerSentEventHandler("/listen")

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
