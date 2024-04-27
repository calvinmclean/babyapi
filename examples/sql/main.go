package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/examples/sql/db"
	"github.com/rs/xid"
	"github.com/spf13/cobra"

	_ "embed"

	_ "github.com/tursodatabase/go-libsql"
)

//go:embed schema.sql
var ddl string

type Author struct {
	db.Author
}

func (a Author) GetID() string {
	return fmt.Sprint(a.Author.ID)
}

func (a *Author) Bind(r *http.Request) error {
	if r.Method == http.MethodPost {
		a.ID = xid.New().String()
	}
	return nil
}

func (a Author) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// Storage implements the babyapi.Storage interface with the sqlc-generated queries
type Storage struct {
	dbMode string
	*db.Queries
}

func (s Storage) Get(ctx context.Context, id string) (*Author, error) {
	a, err := s.Queries.GetAuthor(ctx, id)
	if err != nil {
		return nil, err
	}

	return &Author{a}, nil
}

func (s Storage) GetAll(ctx context.Context, query url.Values) ([]*Author, error) {
	var authors []db.Author
	var err error

	genre := query.Get("genre")
	if genre != "" {
		authors, err = s.Queries.SearchAuthors(ctx, genre)
	} else {
		authors, err = s.Queries.ListAuthors(ctx)
	}
	if err != nil {
		return nil, err
	}

	var result []*Author
	for _, a := range authors {
		result = append(result, &Author{a})
	}

	return result, nil
}

func (s Storage) Set(ctx context.Context, a *Author) error {
	return s.Queries.UpsertAuthor(ctx, db.UpsertAuthorParams{
		ID:    a.ID,
		Name:  a.Name,
		Bio:   a.Bio,
		Genre: a.Genre,
	})
}

func (s Storage) Delete(ctx context.Context, id string) error {
	return s.Queries.DeleteAuthor(ctx, id)
}

func (s *Storage) Apply(api *babyapi.API[*Author]) error {
	database, err := setupDatabaseConnection(s.dbMode)
	if err != nil {
		return err
	}

	go func() {
		<-api.Done()
		database.Close()
	}()

	// create tables
	_, err = database.ExecContext(context.Background(), ddl)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	s.Queries = db.New(database)
	api.SetStorage(s)

	return nil
}

func setupDatabaseConnection(dbMode string) (*sql.DB, error) {
	var dbName string
	switch dbMode {
	case "memory":
		dbName = ":memory:"
	case "file":
		dbName = os.Getenv("SQLITE_FILE")
		if dbName == "" {
			dbName = "local.db"
		}
		dbName = fmt.Sprintf("file:%s", dbName)
	case "turso":
		dbURL, ok := os.LookupEnv("TURSO_DATABASE_URL")
		if !ok {
			return nil, errors.New("missing TURSO_DATABASE_URL env var")
		}

		dbAuth, ok := os.LookupEnv("TURSO_AUTH_TOKEN")
		if !ok {
			return nil, errors.New("missing TURSO_DATABASE_URL env var")
		}

		dbName = fmt.Sprintf("%s?authToken=%s", dbURL, dbAuth)
	default:
		return nil, fmt.Errorf("invalid database option: %q", dbMode)
	}

	database, err := sql.Open("libsql", dbName)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	return database, nil
}

func main() {
	api := babyapi.NewAPI(
		"Authors", "/authors",
		func() *Author { return &Author{} },
	)

	cmd := api.Command()

	var dbMode string
	cmd.PersistentFlags().StringVar(&dbMode, "db", "memory", "set database mode: memory, file, turos")

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Only setup DB for the serve command
		if cmd.Name() != "serve" {
			return nil
		}

		storage := &Storage{dbMode: dbMode}
		return storage.Apply(api)
	}

	err := cmd.Execute()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
