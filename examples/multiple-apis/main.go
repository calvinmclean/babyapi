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

type GOAL struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   bool
}

func main() {
	babyapi.NewRootAPI("root", "/").
		AddNestedAPI(babyapi.NewAPI[*TODO]("TODOs", "/todos", func() *TODO { return &TODO{} })).
		AddNestedAPI(babyapi.NewAPI[*GOAL]("GOALs", "/goals", func() *GOAL { return &GOAL{} })).
		RunCLI()
}
