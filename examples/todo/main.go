package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/mark3labs/mcp-go/server"
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

func main() {
	api := babyapi.NewAPI("TODOs", "/todos", func() *TODO { return &TODO{} })
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

	api.EnableMCP("TODOs", "/mcp", babyapi.MCPPermCRUD, babyapi.MCPConfig{
		ServerOpts: []server.ServerOption{
			server.WithInstructions("This is a web server for managing TODO list items"),
		},
	})

	api.RunCLI()
}
