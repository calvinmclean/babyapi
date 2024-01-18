package main

import (
	"github.com/calvinmclean/babyapi"
	"github.com/go-chi/chi/v5"
	"log/slog"
	"net/http"
)

type TODO struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   bool
}

type GOAL struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   bool
}

// RoutableAPI defines a minimum interface to register APIs to the router, along with displaying usable logs.
type RoutableAPI interface {
	Route(chi.Router)
	Name() string
}

// Takes an addr string same as http.ListenAndServe and one or more APIs and will serve all of them.
// The APIs must not have conflicting base routes.
// This does not allow CLI functionality.
func serveAll(addr string, apis ...RoutableAPI) {
	router := chi.NewRouter()
	for _, api := range apis {
		slog.Info("Setting up API", api.Name())
		api.Route(router)
	}
	slog.Info("starting server", "address", addr)
	err := http.ListenAndServe(addr, router)
	if err != nil && err != http.ErrServerClosed {
		slog.Error("server shutdown error", "error", err)
	}
}

func main() {
	TodoApi := babyapi.NewAPI[*TODO]("TODOs", "/todos", func() *TODO { return &TODO{} })
	GoalApi := babyapi.NewAPI[*GOAL]("GOALs", "/goals", func() *GOAL { return &GOAL{} })

	serveAll(":3000", TodoApi, GoalApi)
}