package main

import (
	"github.com/calvinmclean/babyapi"
)

type TODO struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   bool
}

func main() {
	api := babyapi.NewAPI[*TODO](
		"TODOs", "/todos",
		func() *TODO { return &TODO{} },
	)
	api.RunCLI()
}
