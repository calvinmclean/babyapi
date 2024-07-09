package main

import (
	"log"
	"net/http"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/html"
	"github.com/go-chi/render"
)

type DataResponse struct {
	babyapi.DefaultResource

	Data map[string]any
}

func main() {
	// Initialize HTML response handling
	babyapi.EnableHTMLRender()

	hello := html.Template("Hello")
	html.SetMap(map[string]string{
		string(hello): `Hello {{ .Name }}!`,
	})

	// Create simple Hello World endpoint to render HTML
	http.HandleFunc("/hello", babyapi.Handler(func(w http.ResponseWriter, r *http.Request) render.Renderer {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "User"
		}

		return hello.Renderer(map[string]any{"Name": name})
	}))

	// Create a simple GET handler using babyapi.Handler helper
	http.HandleFunc("/data", babyapi.Handler(func(w http.ResponseWriter, r *http.Request) render.Renderer {
		return DataResponse{Data: map[string]any{
			"hello": "world",
		}}
	}))

	// Create a simple POST handler using babyapi.ReadRequestBodyAndDo helper
	http.HandleFunc("/create-data", babyapi.ReadRequestBodyAndDo(func(w http.ResponseWriter, r *http.Request, data *DataResponse) (render.Renderer, *babyapi.ErrResponse) {
		log.Printf("received request body: %+v", data)
		return nil, nil
	}, func() *DataResponse { return &DataResponse{} }))

	// After the server starts up, use babyapi.MakeRequest to get the response
	go func() {
		time.Sleep(1 * time.Second)

		req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/data", http.NoBody)
		dataResponse, _ := babyapi.MakeRequest[DataResponse](req, http.DefaultClient, http.StatusOK, nil)

		log.Printf("received data: %+v\n", dataResponse.Data.Data)
	}()

	http.ListenAndServe(":8080", nil)
}
