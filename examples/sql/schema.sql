CREATE TABLE IF NOT EXISTS authors (
  id    text PRIMARY KEY,
  name  text NOT NULL,
  genre text NOT NULL,
  bio   text NOT NULL
);