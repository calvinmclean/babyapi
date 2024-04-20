package html

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"os"

	"github.com/go-chi/render"
)

var templateFS embed.FS
var templateGlob string
var templateFuncs func(r *http.Request) map[string]any
var templateStrings map[string]string

func SetMap(templates map[string]string) {
	templateStrings = templates
}

func SetFS(fs embed.FS, glob string) {
	templateFS = fs
	templateGlob = glob
}

func SetFuncs(funcs func(r *http.Request) map[string]any) {
	templateFuncs = funcs
}

type Template string

func (t Template) Render(r *http.Request, data any) string {
	templates := template.New("base")
	if templateFuncs != nil {
		templates = templates.Funcs(templateFuncs(r))
	}

	for name, text := range templateStrings {
		templates = template.Must(templates.New(name).Parse(text))
	}

	if dir := os.Getenv("DEV_TEMPLATE"); dir != "" {
		templates = template.Must(templates.ParseGlob(dir))
	} else if templateGlob != "" {
		templates = template.Must(templates.ParseFS(templateFS, templateGlob))
	}

	var renderedOutput bytes.Buffer
	err := templates.ExecuteTemplate(&renderedOutput, string(t), data)
	if err != nil {
		panic(err)
	}

	return renderedOutput.String()
}

func (t Template) Renderer(data any) render.Renderer {
	return htmlRenderer{t: t, data: data}
}

type htmlRenderer struct {
	t    Template
	data any
}

func (h htmlRenderer) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

func (h htmlRenderer) HTML(r *http.Request) string {
	return h.t.Render(r, h.data)
}
