package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/calvinmclean/babyapi"
	"github.com/spf13/cobra"
)

type Pokemon struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Abilities []any  `json:"abilities"`
	Moves     []any  `json:"moves"`
	Types     []struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
}

func (p *Pokemon) String() string {
	var sb strings.Builder

	writef := func(format string, a ...any) {
		sb.WriteString(fmt.Sprintf(format, a...))
	}

	var types []string
	for _, t := range p.Types {
		types = append(types, t.Type.Name)
	}

	writef("Name: %s\n", p.Name)
	writef("Types: %s\n", types)
	writef("%d Abilities\n", len(p.Abilities))
	writef("%d Moves\n", len(p.Moves))

	return sb.String()
}

func (p *Pokemon) GetID() string {
	return fmt.Sprint(p.ID)
}

func (p *Pokemon) ParentID() string {
	return ""
}

func (*Pokemon) Bind(*http.Request) error {
	return nil
}

func (*Pokemon) Render(http.ResponseWriter, *http.Request) error {
	return nil
}

func main() {
	api := babyapi.NewAPI("pokemon", "/api/v2/pokemon/", func() *Pokemon { return &Pokemon{} })

	cmd := api.Command()

	// Add a custom get command because the regular get command will just print raw JSON
	cmd.AddCommand(&cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				address = "https://pokeapi.co"
			}

			resp, err := api.Client(address).Get(context.Background(), args[0])
			if err != nil {
				return err
			}

			if resp.Data != nil {
				fmt.Println(resp.Data.String())
			}

			return nil
		},
	})

	err := cmd.Execute()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
