-- Runs automatically the first time the Postgres volume initializes.
-- Changed this file? Wipe the volume so it re-runs: docker compose down -v

-- pgcrypto gives us crypt()/gen_salt('bf') to bcrypt the seed passwords below.
-- gen_salt('bf') produces standard $2a$ bcrypt hashes, which Go's
-- golang.org/x/crypto/bcrypt verifies without any special handling.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    -- Never the password: only its bcrypt hash. bcrypt salts internally and is
    -- deliberately slow, so a database leak does not hand over the passwords.
    password_hash TEXT NOT NULL,
    -- CHECK is the database refusing to store a role the app doesn't know.
    -- Validation in Go can be bypassed by any other client; this cannot.
    role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tasks (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    -- The task lifecycle. Replaced the old `done` boolean, which could not
    -- express "finished, waiting on an admin".
    --   pending   -> submitted            (member completes it)
    --   submitted -> approved | rejected  (admin reviews it)
    --   rejected  -> submitted            (member tries again)
    status     TEXT NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'submitted', 'approved', 'rejected')),
    -- Audit trail: who decided, and when. Nullable because most tasks have
    -- never been reviewed.
    --
    -- ON DELETE SET NULL, not CASCADE: deleting an admin must not delete the
    -- work they reviewed. Compare user_id above, where CASCADE is right
    -- because a task genuinely belongs to its owner.
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- The two columns move together, and only into a reviewed state. This is
    -- the state machine written as a constraint, so no buggy UPDATE can leave
    -- a "pending" task carrying a reviewer.
    CONSTRAINT tasks_review_consistent CHECK (
        (status IN ('approved', 'rejected') AND reviewed_at IS NOT NULL)
        OR (status IN ('pending', 'submitted') AND reviewed_by IS NULL AND reviewed_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS tasks_user_id_idx ON tasks (user_id);
-- Admins list by status across all users, so that column is worth indexing too.
CREATE INDEX IF NOT EXISTS tasks_status_idx ON tasks (status);

-- A URL the app POSTs a JSON event to whenever the owner creates a task.
CREATE TABLE IF NOT EXISTS webhooks (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS webhooks_user_id_idx ON webhooks (user_id);

-- In-app notifications: admins hear about submissions, members hear the verdict.
CREATE TABLE IF NOT EXISTS notifications (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    message    TEXT NOT NULL,
    read       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications (user_id, read);

-- Refresh tokens, stored as SHA-256 hashes, never the token itself. A row is
-- deleted on logout or when rotated by a refresh. An access token cannot be
-- revoked (it is stateless and short-lived); a refresh token can, by deleting
-- its row here.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx ON refresh_tokens (user_id);

-- Seed users. Both have the dev password "password123", bcrypt-hashed by
-- pgcrypto so no plaintext lives here. Log in via POST /api/auth/login.
-- Alice is the admin (reviews work); Bob is a member (submits work).
INSERT INTO users (email, name, password_hash, role) VALUES
    ('alice@example.com', 'Alice', crypt('password123', gen_salt('bf')), 'admin'),
    ('bob@example.com',   'Bob',   crypt('password123', gen_salt('bf')), 'member');

INSERT INTO tasks (user_id, title) VALUES
    (1, 'learn Go structs'),
    (1, 'wire up chi'),
    (2, 'read the pq docs');
