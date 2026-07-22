# task_crud_api

A tiny CRUD REST API for `tasks` and `users`, built with Go + [chi](https://github.com/go-chi/chi),
`database/sql` + [lib/pq](https://github.com/lib/pq), and PostgreSQL in Docker.
Tasks belong to users; everything except `GET /health` and signup needs a bearer token.

## Layout
```
cmd/
  api/                   the server: main.go, config.go, server.go
  echo/                  dev-only webhook receiver (second binary, same module)
internal/
  model/                 Task, User, Webhook, inputs + Validate, sentinel errors
  repository/            db.go (sql.DB + lib/pq), task_, user_, webhook_repository.go
  service/               business rules, validation, outbound calls
  client/                HTTP client for calling OTHER people's APIs
  response/              JSON + ResponseError, one error → status code map
  handler/
    task_handler.go      TaskHandler + its own /tasks router
    user_handler.go      UserHandler + its own /users router
    webhook_handler.go   WebhookHandler + its own /webhooks router
    auth_middleware.go   Auth.RequireUser: token → user → request context
    health_handler.go    public health check
    request.go           decodeJSON (generic), parseID
docs/ERD.md              database diagram + constraints, generated from the live DB
docs/screenshots/        UI walkthrough of the approval workflow
frontend/                React + Vite + TypeScript UI (see below)
initdb/schema.sql        runs once on an empty volume: creates + seeds users and tasks
docker-compose.yml       Postgres 17 on host port 5433
```
`handler` receives HTTP; `client` sends it. Each handler owns its routes with
relative paths; `server.go` decides where they hang with `r.Mount`.
Each handler owns its routes with relative paths; `main.go` decides where they
hang with `r.Mount("/api/tasks", ...)` and `r.Mount("/api/users", ...)`.
Dependencies point one way: `handler` → `service` → `repository` → `model`.
Everything under `internal/` is importable only by this module, enforced by the
Go toolchain.

## Run
```powershell
cd task_crud_api
docker compose up -d       # starts Postgres 17 (host port 5433)
go mod tidy                # downloads chi + lib/pq + godotenv, fills go.sum
go run ./cmd/api           # run from the project root so .env is found
# connected to postgres
# listening on http://localhost:8090
```

> **Note:** the DB is published on host port **5433**, not 5432, because this
> machine already runs a native PostgreSQL 16 service on 5432. The app's default
> `DATABASE_URL` points at 5433 to match. Override with `.env` or the env var.

> **`sslmode=disable` is required.** lib/pq defaults to `sslmode=require`; the
> local container has no TLS, so the URL must end in `?sslmode=disable`.

> **Changed `initdb/schema.sql`?** It only runs on an empty volume. Re-run it
> with `docker compose down -v` (this deletes all data), then `up -d`.

Port comes from `PORT` in `.env` (8090 here), falling back to 3000.

## Auth
Sign up to get a token (this is the only response that ever contains it):
```powershell
curl.exe -X POST http://localhost:8090/api/users -H "Content-Type: application/json" -d "{\"email\":\"carol@example.com\",\"name\":\"Carol\"}"
# {"id":3,"email":"carol@example.com","name":"Carol","created_at":"...","token":"3d96938a1ae6..."}
```
Or use a seeded demo user:

| User  | Email               | Role     | Token             |
|-------|---------------------|----------|-------------------|
| Alice | alice@example.com   | `admin`  | `alice-token-123` |
| Bob   | bob@example.com     | `member` | `bob-token-456`   |

Send it on every protected request:
```
Authorization: Bearer alice-token-123
```
Middleware looks the token up, loads the user, and puts it in the request
context. A missing, malformed, or unknown token is a **401**.

> Tokens are stored in **plain text**. A real API stores a hash of the token,
> compares hashes, and can therefore never print it back. Signup tokens are
> generated with `crypto/rand`; the two demo ones are hand-seeded.

## Endpoints
| Method | Path             | Auth | Body                          | Success | Errors             |
|--------|------------------|------|-------------------------------|---------|--------------------|
| GET    | /health          | no   | –                             | 200     | –                  |
| POST   | /api/users       | no   | `{"email":"...","name":"..."}`| 201     | 400, 409, 415      |
| GET    | /api/users       | yes  | –                             | 200     | 401                |
| GET    | /api/users/me    | yes  | –                             | 200     | 401                |
| GET    | /api/users/{id}  | yes  | –                             | 200     | 400, 401, 404      |
| PUT    | /api/users/{id}  | yes  | `{"email":"...","name":"..."}`| 200     | 400, 401, 403, 404, 409, 415 |
| DELETE | /api/users/{id}  | yes  | –                             | 204     | 400, 401, 403, 404 |
| GET    | /api/tasks       | yes  | –                             | 200     | 401                |
| POST   | /api/tasks       | yes  | `{"title":"..."}`             | 201     | 400, 401, 415      |
| POST   | /api/tasks/import?limit=N | yes | –                    | 201     | 400, 401, 502      |
| GET    | /api/tasks/{id}  | yes  | –                             | 200     | 400, 401, 404      |
| PUT    | /api/tasks/{id}  | yes  | `{"title":"..."}`             | 200     | 400, 401, 404, 415 |
| POST   | /api/tasks/{id}/submit | yes | –                      | 200     | 400, 401, 404, 409 |
| DELETE | /api/tasks/{id}  | yes  | –                             | 204     | 400, 401, 404      |
| GET    | /api/webhooks    | yes  | –                             | 200     | 401                |
| POST   | /api/webhooks    | yes  | `{"url":"https://..."}`       | 201     | 400, 401, 415      |
| DELETE | /api/webhooks/{id} | yes | –                            | 204     | 400, 401, 404      |
| GET    | /api/notifications | yes | –                            | 200     | 401                |
| POST   | /api/notifications/{id}/read | yes | –                  | 204     | 400, 401, 404      |
| GET    | /api/admin/tasks?status=submitted | **admin** | –      | 200     | 400, 401, 403      |
| POST   | /api/admin/tasks/{id}/approve | **admin** | –          | 200     | 400, 401, 403, 404, 409 |
| POST   | /api/admin/tasks/{id}/reject  | **admin** | –          | 200     | 400, 401, 403, 404, 409 |

## Roles and the approval workflow

Two roles, `admin` and `member`, stored on `users.role` with a `CHECK`
constraint so the database refuses anything else.

A task moves through a state machine, not a boolean:
```
  pending  --submit(member)-->  submitted  --approve(admin)-->  approved
                                    |                            (terminal)
                                    +------reject(admin)------>  rejected
                                                                     |
                                    <-------resubmit(member)---------+
```
Illegal moves return **409**, e.g. approving a task that is still `pending`, or
touching one that is already `approved`. The rules live in one place,
`TaskStatus.CanTransitionTo`, so no handler can invent its own version.

Notifications flow both ways: submitting alerts every admin, and a verdict
alerts the task's owner. Read them at `GET /api/notifications`.

**Audit trail.** Approving or rejecting records `reviewed_by` and `reviewed_at`
on the task. Both are null until a decision, and a resubmission clears them, so
they always describe the *current* status. A `CHECK` enforces that pairing, and
`reviewed_by` uses `ON DELETE SET NULL` — deleting an admin must not delete the
work they reviewed. See [docs/ERD.md](docs/ERD.md).

**Authorization is two middlewares, in order** — `RequireUser` (who are you?)
then `RequireAdmin` (may you be here?). They guard the whole `/api/admin`
sub-router, so you cannot add an admin endpoint and forget the check.

```powershell
$ADMIN = "Authorization: Bearer alice-token-123"   # Alice
$MEM   = "Authorization: Bearer bob-token-456"     # Bob
$J     = "Content-Type: application/json"

# Bob does the work
curl.exe -X POST http://localhost:8090/api/tasks -H $MEM -H $J -d "{\"title\":\"my task\"}"
curl.exe -X POST http://localhost:8090/api/tasks/4/submit -H $MEM

# Alice reviews it
curl.exe -H $ADMIN http://localhost:8090/api/notifications
curl.exe -H $ADMIN "http://localhost:8090/api/admin/tasks?status=submitted"
curl.exe -X POST -H $ADMIN http://localhost:8090/api/admin/tasks/4/approve

# Bob hears the verdict
curl.exe -H $MEM http://localhost:8090/api/notifications

# Bob tries to review his own work
curl.exe -i -X POST -H $MEM http://localhost:8090/api/admin/tasks/4/approve   # 403
```

**Two different privacy rules, on purpose:**
- **Tasks are private.** You only see your own; someone else's id is a **404**,
  never a 403, because a 403 would confirm it exists.
- **Users are listable.** So editing someone else is an honest **403** — hiding
  their existence would be theatre when `GET /api/users` just listed them.

Deleting a user deletes their tasks (`ON DELETE CASCADE`, done by Postgres).

## Frontend (React + Vite + TypeScript)

```powershell
cd frontend
npm install
npm run dev          # http://localhost:5173
```
Run the Go API at the same time; the UI is useless without it.

**Why a proxy and not CORS.** A browser on `:5173` calling `:8090` is a
cross-origin request, and the Go API sends no CORS headers, so the browser
would block it. `vite.config.ts` proxies `/api` to `localhost:8090`, so the
browser thinks the API is same-origin and the backend stays untouched. The
alternative — adding CORS middleware to chi — is what you would do when the
frontend is deployed separately.

**What's in it:**
- `src/api/types.ts` mirrors `internal/model` field for field. The Go
  `json:"..."` tags decide these names, which is exactly why stripping a tag
  breaks the UI.
- `src/api/client.ts` is the mirror of `internal/client`: attach the token,
  send, **check the status**, decode. `fetch` only rejects on network failure,
  so a 404 or 500 resolves normally with `ok === false` — the same trap as Go's
  `http.Client.Do` returning `err == nil` for a 500.
- Token goes in `localStorage`; there is no password login because the API has
  no login endpoint. Sign in as Alice (admin) or Bob (member).
- The Review tab is hidden for members, but `RequireAdmin` on the server is the
  real guard. **Hiding UI is convenience, never security** — a member calling
  `/api/admin/tasks` directly still gets a 403.

## Outbound HTTP (webhooks + import)

The app also *calls* other APIs, via `internal/client`.

**Webhooks.** Register a URL and every task you create is POSTed to it as JSON:
```json
{"event":"task.created","task":{"id":1,"user_id":2,"title":"...","status":"pending","created_at":"..."},"sent_at":"..."}
```
Delivery runs in a goroutine on a context detached with `context.WithoutCancel`,
so it is **not** killed when the HTTP request that triggered it ends. Failures
are logged, never returned: a broken subscriber must not fail your request.

See it locally without the internet — run the receiver in a second terminal:
```powershell
go run ./cmd/echo        # listens on http://localhost:9999/hook
curl.exe -X POST http://localhost:8090/api/webhooks -H $A -H $J -d "{\"url\":\"http://localhost:9999/hook\"}"
curl.exe -X POST http://localhost:8090/api/tasks -H $A -H $J -d "{\"title\":\"fires a webhook\"}"
```

**Import.** `POST /api/tasks/import?limit=5` fetches todos from
`jsonplaceholder.typicode.com`, unmarshals them, and saves them as your tasks.
Unlike webhooks it uses the request's context directly, because the caller is
waiting for the result.

> **Timeouts stack, shortest wins.** The client has a 10s timeout, the router
> caps every request at 5s, and webhook delivery gets its own 20s budget. An
> early bug here had webhooks inheriting the 5s request deadline and dying
> mid-flight against a slow endpoint.

> **SSRF caveat.** Webhook URLs are only checked for an `http(s)` scheme. A real
> service must also block private and loopback addresses, or a user can point
> your server at internal infrastructure.

## Validation
- `502` — a third-party API we depend on failed (`ErrUpstream`)
- `401` — missing, malformed, or unknown bearer token
- `403` — editing or deleting a user who isn't you
- `409` — email already taken (Postgres `unique_violation`, code 23505)
- `415` — `Content-Type` is not `application/json`
- `400` — malformed JSON, unknown field, body over 1 MiB, non-positive id,
  or a field rule broken (`invalid input: title is required`)

Strings are trimmed before validation, so `"   "` is empty and rejected.
Emails are checked with `net/mail.ParseAddress`.

## Try it (use `curl.exe`, not PowerShell's `curl` alias)
```powershell
$A = "Authorization: Bearer alice-token-123"
$B = "Authorization: Bearer bob-token-456"
$J = "Content-Type: application/json"

curl.exe -H $A http://localhost:8090/api/users/me
curl.exe -H $A http://localhost:8090/api/users
curl.exe -H $A http://localhost:8090/api/tasks
curl.exe -H $B http://localhost:8090/api/tasks     # different list entirely

curl.exe -X POST http://localhost:8090/api/tasks -H $A -H $J -d "{\"title\":\"buy milk\"}"

curl.exe -X PUT http://localhost:8090/api/tasks/1 -H $A -H $J -d "{\"title\":\"buy oat milk\"}"

curl.exe -i -X DELETE http://localhost:8090/api/tasks/1 -H $A

# failures
curl.exe -i http://localhost:8090/api/tasks                                                     # 401 no token
curl.exe -i -H $B http://localhost:8090/api/tasks/1                                             # 404 Alice's task, as Bob
curl.exe -i -X PUT http://localhost:8090/api/users/2 -H $A -H $J -d "{\"email\":\"x@y.com\",\"name\":\"X\"}"  # 403 not you
curl.exe -i -X POST http://localhost:8090/api/users -H $J -d "{\"email\":\"alice@example.com\",\"name\":\"X\"}"  # 409 taken
curl.exe -i -X POST http://localhost:8090/api/tasks -H $A -H $J -d "{\"title\":\"  \"}"           # 400
```
