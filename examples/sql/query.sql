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
