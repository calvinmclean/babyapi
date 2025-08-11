CREATE TABLE IF NOT EXISTS authors (
  id    text PRIMARY KEY,
  name  text NOT NULL,
  genre text NOT NULL,
  bio   text NOT NULL
);

CREATE TABLE IF NOT EXISTS books (
  id         text PRIMARY KEY,
  title      text NOT NULL,
  isbn       text NOT NULL,
  year       integer NOT NULL,
  author_id  text NOT NULL,
  FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE CASCADE
);
