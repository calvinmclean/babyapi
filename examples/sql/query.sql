-- name: GetAuthor :one
SELECT * FROM authors
WHERE id = ? LIMIT 1;

-- name: ListAuthors :many
SELECT * FROM authors
ORDER BY name;

-- name: SearchAuthors :many
SELECT * FROM authors WHERE genre = ?;

-- name: UpsertAuthor :exec
INSERT INTO authors (
  id, name, genre, bio
) VALUES (
  ?, ?, ?, ?
) ON CONFLICT (id)
DO UPDATE SET
  name = EXCLUDED.name,
  genre = EXCLUDED.genre,
  bio = EXCLUDED.bio;

-- name: DeleteAuthor :exec
DELETE FROM authors
WHERE id = ?;

-- name: GetBook :one
SELECT id, title, isbn, year, author_id FROM books
WHERE id = ? LIMIT 1;

-- name: ListBooksByAuthor :many
SELECT id, title, isbn, year, author_id FROM books
WHERE author_id = ?
ORDER BY year DESC, title;

-- name: UpsertBook :exec
INSERT INTO books (
  id, title, isbn, year, author_id
) VALUES (
  ?, ?, ?, ?, ?
) ON CONFLICT (id)
DO UPDATE SET
  title = EXCLUDED.title,
  isbn = EXCLUDED.isbn,
  year = EXCLUDED.year,
  author_id = EXCLUDED.author_id;

-- name: DeleteBook :exec
DELETE FROM books
WHERE id = ?;
