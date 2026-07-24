-- Runs automatically the first time the Postgres volume initializes.
-- Changed this file? Wipe the volume so it re-runs: docker compose down -v

-- pgcrypto gives us crypt()/gen_salt('bf') to bcrypt the seed passwords below.
-- gen_random_uuid() (used for the UUID primary keys) is built into Postgres 13+
-- core, but pgcrypto also provides it, so requiring the extension is harmless.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- IDs are UUIDs, not serial integers. The database generates a random v4 UUID
-- on insert via the DEFAULT, exactly as SERIAL used to hand out the next int,
-- and RETURNING gives it straight back. UUIDs mean an id leaks no row count and
-- no ordering, and two databases can mint ids without coordinating.
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
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
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS webhooks_user_id_idx ON webhooks (user_id);

-- In-app notifications: admins hear about submissions, members hear the verdict.
CREATE TABLE IF NOT EXISTS notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
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
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx ON refresh_tokens (user_id);

-- Seed users. Both have the dev password "password123", bcrypt-hashed by
-- pgcrypto so no plaintext lives here. Log in via POST /api/auth/login.
-- Alice is the admin (reviews work); Bob is a member (submits work).
-- Their ids are random UUIDs, so unlike before we cannot hardcode 1 and 2.
INSERT INTO users (email, name, password_hash, role) VALUES
    ('alice@example.com', 'Alice', crypt('password123', gen_salt('bf')), 'admin'),
    ('bob@example.com',   'Bob',   crypt('password123', gen_salt('bf')), 'member');

INSERT INTO tasks (user_id, title)
SELECT id, 'learn Go structs' FROM users WHERE email = 'alice@example.com';
INSERT INTO tasks (user_id, title)
SELECT id, 'wire up chi' FROM users WHERE email = 'alice@example.com';
INSERT INTO tasks (user_id, title)
SELECT id, 'read the pq docs' FROM users WHERE email = 'bob@example.com';
