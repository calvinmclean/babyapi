package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/examples/sql/db"
	"github.com/rs/xid"
	"github.com/spf13/cobra"

	_ "embed"

	_ "github.com/tursodatabase/go-libsql"
)

//go:generate sqlc generate

//go:embed schema.sql
var ddl string

type Author struct {
	db.Author
}

type Book struct {
	db.Book
}

func (b Book) GetID() string {
	return fmt.Sprint(b.Book.ID)
}

func (b Book) ParentID() string {
	return b.AuthorID
}

func (b *Book) Bind(r *http.Request) error {
	if r.Method == http.MethodPost {
		b.ID = xid.New().String()
		b.AuthorID = babyapi.GetIDParam(r, "Authors")
	}
	return nil
}

func (b Book) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (a Author) GetID() string {
	return fmt.Sprint(a.Author.ID)
}

func (a Author) ParentID() string {
	return ""
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
	db *sql.DB
	*db.Queries
}

func (s Storage) Get(ctx context.Context, id string) (*Author, error) {
	a, err := s.Queries.GetAuthor(ctx, id)
	if err != nil {
		return nil, err
	}

	return &Author{a}, nil
}

func (s Storage) Search(ctx context.Context, _ string, query url.Values) ([]*Author, error) {
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
	s.Queries = db.New(s.db)
	api.SetStorage(s)
	return nil
}

type BookStorage struct {
	db *sql.DB
	*db.Queries
}

func (s BookStorage) Get(ctx context.Context, id string) (*Book, error) {
	b, err := s.Queries.GetBook(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Book{b}, nil
}

func (s BookStorage) Search(ctx context.Context, authorID string, query url.Values) ([]*Book, error) {
	books, err := s.Queries.ListBooksByAuthor(ctx, authorID)
	if err != nil {
		return nil, err
	}

	var result []*Book
	for _, b := range books {
		result = append(result, &Book{b})
	}
	return result, nil
}

func (s BookStorage) Set(ctx context.Context, b *Book) error {
	return s.Queries.UpsertBook(ctx, db.UpsertBookParams{
		ID:       b.ID,
		Title:    b.Title,
		Isbn:     b.Isbn,
		Year:     b.Year,
		AuthorID: b.AuthorID,
	})
}

func (s BookStorage) Delete(ctx context.Context, id string) error {
	return s.Queries.DeleteBook(ctx, id)
}

func (s *BookStorage) Apply(api *babyapi.API[*Book]) error {
	s.Queries = db.New(s.db)
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

	// create tables
	for _, stmt := range strings.Split(ddl, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		_, err = database.ExecContext(context.Background(), stmt)
		if err != nil {
			return nil, fmt.Errorf("error creating table: %w", err)
		}
	}

	return database, nil
}

func main() {
	authorAPI := babyapi.NewAPI(
		"Authors", "/authors",
		func() *Author { return &Author{} },
	)

	bookAPI := babyapi.NewAPI(
		"Books", "/books",
		func() *Book { return &Book{} },
	)

	authorAPI.AddNestedAPI(bookAPI)

	cmd := authorAPI.Command()

	var dbMode string
	cmd.PersistentFlags().StringVar(&dbMode, "db", "memory", "set database mode: memory, file, turso")

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Only setup DB for the serve command
		if cmd.Name() != "serve" {
			return nil
		}

		db, err := setupDatabaseConnection(dbMode)
		if err != nil {
			return err
		}

		go func() {
			<-authorAPI.Done()
			db.Close()
		}()

		authorStorage := &Storage{db: db}
		if err := authorStorage.Apply(authorAPI); err != nil {
			return err
		}

		bookStorage := &BookStorage{db: db}
		return bookStorage.Apply(bookAPI)
	}

	err := cmd.Execute()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
