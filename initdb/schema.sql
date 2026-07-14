-- Runs automatically the first time the Postgres volume initializes.
CREATE TABLE IF NOT EXISTS tasks (
    id         SERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    done       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tasks (title) VALUES
    ('learn Go structs'),
    ('wire up chi');
