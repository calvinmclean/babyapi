// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: query.sql

package db

import (
	"context"
)

const deleteAuthor = `-- name: DeleteAuthor :exec
DELETE FROM authors
WHERE id = ?
`

func (q *Queries) DeleteAuthor(ctx context.Context, id string) error {
	_, err := q.db.ExecContext(ctx, deleteAuthor, id)
	return err
}

const getAuthor = `-- name: GetAuthor :one
SELECT id, name, genre, bio FROM authors
WHERE id = ? LIMIT 1
`

func (q *Queries) GetAuthor(ctx context.Context, id string) (Author, error) {
	row := q.db.QueryRowContext(ctx, getAuthor, id)
	var i Author
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Genre,
		&i.Bio,
	)
	return i, err
}

const listAuthors = `-- name: ListAuthors :many
SELECT id, name, genre, bio FROM authors
ORDER BY name
`

func (q *Queries) ListAuthors(ctx context.Context) ([]Author, error) {
	rows, err := q.db.QueryContext(ctx, listAuthors)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Author
	for rows.Next() {
		var i Author
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Genre,
			&i.Bio,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const searchAuthors = `-- name: SearchAuthors :many
SELECT id, name, genre, bio FROM authors WHERE genre = ?
`

func (q *Queries) SearchAuthors(ctx context.Context, genre string) ([]Author, error) {
	rows, err := q.db.QueryContext(ctx, searchAuthors, genre)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Author
	for rows.Next() {
		var i Author
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Genre,
			&i.Bio,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const upsertAuthor = `-- name: UpsertAuthor :exec
INSERT INTO authors (
  id, name, genre, bio
) VALUES (
  ?, ?, ?, ?
) ON CONFLICT (id)
DO UPDATE SET
  name = EXCLUDED.name,
  genre = EXCLUDED.genre,
  bio = EXCLUDED.bio
`

type UpsertAuthorParams struct {
	ID    string
	Name  string
	Genre string
	Bio   string
}

func (q *Queries) UpsertAuthor(ctx context.Context, arg UpsertAuthorParams) error {
	_, err := q.db.ExecContext(ctx, upsertAuthor,
		arg.ID,
		arg.Name,
		arg.Genre,
		arg.Bio,
	)
	return err
}
