# LEARN.md — What every file does and why it matters

A guided tour of `task_crud_api/`. For each file: **what it is**, **why it exists**, and **what breaks without it**. Files are grouped by role, not alphabetically, so the architecture reads top to bottom.

```
task_crud_api/
├── cmd/api/main.go          wiring + the root router that mounts the rest
├── internal/                the app, split into layers
│   ├── model/
│   │   ├── task.go          Task, TaskInput, field rules (Validate)
│   │   ├── user.go          User, UserWithToken, UserInput + Validate
│   │   └── errors.go        the five sentinel errors
│   ├── repository/
│   │   ├── db.go            *sql.DB + lib/pq driver, ping on startup
│   │   ├── task.go          the only file that knows task SQL
│   │   └── user.go          user SQL + token lookup
│   ├── service/
│   │   ├── task.go          business rules; runs validation, calls the repo
│   │   └── user.go          same, plus token generation and the self-only rule
│   └── handler/
│       ├── task.go          TaskHandler + its own /tasks router
│       ├── user.go          UserHandler + its own /users router
│       ├── middleware.go    Auth.RequireUser: token → user → request context
│       ├── errors.go        respondError: one error → status code map
│       └── http.go          Health, decodeJSON, parseID, writeJSON
├── Go module (dependencies + build)
│   ├── go.mod               module name, Go version, direct deps
│   └── go.sum               cryptographic checksums of every dep
├── Infrastructure (the database)
│   ├── docker-compose.yml   defines the Postgres container
│   └── initdb/schema.sql    creates + seeds users and tasks on first boot
├── Config
│   ├── .env                 real PORT + DATABASE_URL (gitignored)
│   └── .env.example         committable template
└── Tooling & docs
    ├── insomnia-collection.json   ready-made API requests to import
    ├── README.md                  how to run it
    └── LEARN.md                   this file
```

**Dependencies point one way:** `handler` → `service` → `repository` → `model`. Nothing points back. `model` imports none of them, which is why both the repository and the handler can share the sentinel errors without importing each other.

**Why `internal/`?** It's a magic directory name in Go: packages under `internal/` can only be imported by code in the same module. The compiler enforces your architecture. If this project were ever imported by someone else, they could reach `cmd/` but never your handlers or SQL.

---

## The entry point

### `cmd/api/main.go` — wiring and mounting
**What:** `func main()`. Loads `.env`, opens the database, constructs repository → service → handler for both resources, builds the root router, mounts each sub-router, starts the server.

**Why it matters:** `func main()` is a hard requirement for any runnable Go program, and this is the only file that has one. It is mostly **composition** — the dependency graph gets assembled here, in one readable place, and every layer receives its dependency instead of reaching for a global. That's constructor injection, same idea as Spring's `@Autowired` constructors or passing collaborators into a Python class `__init__`, just done by hand.

It also owns the **route map**, and only the map:
```go
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)

r.Get("/health", handler.Health)
r.Mount("/api/tasks", taskHandler.Router())
r.Mount("/api/users", userHandler.Router())
```
`Mount` hangs a whole sub-router off a prefix. Each handler declares its routes with **relative** paths (`/`, `/{id}`) and has no idea it lives under `/api/tasks` — so the URL layout is one readable block here rather than a prefix repeated across six registrations. Move the API to `/v2/tasks` and you edit one line.

Middleware nests: `Logger` and `Recoverer` are on the root, so they wrap everything including `/health`; `RequireUser` is inside each sub-router, so it guards only what it should.

**Why `cmd/api/` and not the root?** The convention scales: a project later grows `cmd/worker/`, `cmd/migrate/`, `cmd/cli/`, each a separate `main` package with its own binary name, all sharing the same `internal/` code.

**Without it:** No program. `go run ./cmd/api` has nothing to start.

---

## `internal/model` — the domain

### `model/task.go`
**What:** `Task` (the full record with `json:"..."` tags), `TaskInput` (the DTO for create/update bodies), and `Validate()`.

**Why it matters:** `Task` is the shape that travels between database, code, and the wire. The struct tags are what make `{"id":1,...}` come out lowercase even though Go fields must be capitalized (capitalized = *exported* = visible to the JSON encoder). `TaskInput` exists so clients **cannot** set server-owned fields like `id` or `created_at` — the database generates those. `Validate()` lives next to the struct it validates, so the rules can't drift away from the fields. It trims the title first, which is why `"   "` is rejected as empty.

**Without it:** No type to decode requests into or encode responses from; nothing else compiles.

### `model/user.go`
**What:** `User` (`ID`, `Email`, `Name`, `CreatedAt`), `UserWithToken`, and `UserInput` + `Validate`.

**Why it matters:** Look at what's **missing** from `User`: there is no `APIToken` field. The token is matched inside `UserRepo.ByToken` as a `WHERE` argument and never selected back out, so it cannot accidentally appear in a JSON response. The safest way to not leak a secret is to never load it into memory in the first place.

`UserWithToken` is the one exception, returned only by signup:
```go
type UserWithToken struct {
	User
	Token string `json:"token"`
}
```
That's **struct embedding**: `User` is declared with no field name, so its fields are promoted — `u.Email` works directly, and the JSON comes out as one flat object with an extra `"token"` key rather than a nested `{"user":{...}}`. It's Go's composition answer to inheritance: you get the fields and methods without a subclass relationship.

`Validate` uses `net/mail.ParseAddress` rather than a hand-rolled regex. Email syntax is a genuinely gnarly spec; the standard library already implements it.

### `model/errors.go`
**What:** Five sentinels: `ErrNotFound`, `ErrInvalid`, `ErrUnauthorized`, `ErrForbidden`, `ErrConflict`.

**Why it matters:** A **sentinel error** is one value, declared once, compared by identity with `errors.Is`. They live in `model` because both the repository/service (which return them) and the handler (which checks them) need them, and neither should import the other.

`Validate` wraps: `fmt.Errorf("%w: title is required", ErrInvalid)`. The `%w` verb is what makes the wrapper still *count as* `ErrInvalid` — `errors.Is` unwraps the chain looking for that exact value. Use `%v` instead and the link is severed: the message reads the same, but `errors.Is` returns false and your 400 becomes a 500.

> **The trap:** `errors.Is` compares *identity*, not text. Returning a fresh `errors.New("not found")` from the repository creates a different value, `errors.Is` returns false, and your 404 silently becomes a 500. Declare the sentinel once, always return (or `%w`-wrap) that variable.

---

## `internal/repository` — the data-access layer

### `repository/db.go` — the connection pool
**What:** `MustConnect()` opens a `*sql.DB` from `DATABASE_URL`, pings it, and dies loudly if Postgres is unreachable.

**Why it matters:** `*sql.DB` is **already a pool**, not a single connection — the name is misleading. It's safe for concurrent use and hands out connections as requests need them, with sensible defaults you only tune once you have a reason (`SetMaxOpenConns` and friends). Two things worth remembering:
- `sql.Open` is **lazy**: it validates the URL but connects to nothing. `Ping` is what actually reaches the database, which is why the app fails fast at startup instead of on the first request.
- The `_ "github.com/lib/pq"` **blank import** looks like dead code but isn't. Importing it for its side effect runs the package's `init()`, which registers the driver named `"postgres"` with `database/sql`. Without that line, `sql.Open("postgres", ...)` fails with `unknown driver`.

**Without it:** No pool to run queries against.

### `repository/user.go` — user SQL + token lookup
**What:** `ByToken`, plus the CRUD five: `List`, `Get`, `Create`, `Update`, `Delete`.

**Why it matters:** `ByToken` is the entire authentication check, in one query. `sql.ErrNoRows` (no user has that token) becomes `ErrUnauthorized` rather than `ErrNotFound`, because from the client's side "your token is bad" is a 401, not a 404. Same translation trick as tasks, different sentinel.

`Create` and `Update` translate a second kind of database failure:
```go
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
```
The `UNIQUE` constraint on `email` is enforced by Postgres, not by a "does this email exist?" check in Go — a check like that has a race between the lookup and the insert, and two simultaneous signups would slip through. Let the database be the referee, then translate its complaint: `23505` is `unique_violation`, which becomes `ErrConflict`, which becomes a **409**. Without this the client would get a 500 for a mistake they could fix.

Note `errors.As` here, not `errors.Is`: we need the `*pq.Error` **value** to read its `Code`. Matching on the message text instead would break across Postgres versions and locales; the code is stable and documented.

This is also the first named import of lib/pq. `db.go` still blank-imports it to register the driver; that stays, because it's the line that documents *why* the driver exists.

### `repository/task.go` — the only file that knows SQL
**What:** `TaskRepo` with `List`, `Get`, `Create`, `Update`, `Delete`. Each runs parameterized SQL and scans rows into `model.Task` via the shared `scanTask` helper.

**Ownership lives here.** Every method takes a `userID` and every query filters on it:
```sql
SELECT ... FROM tasks WHERE id = $1 AND user_id = $2
```
The filter is in the `WHERE` clause, not an `if` in the handler. A task you don't own produces zero rows, which becomes `ErrNotFound`, which becomes a **404**. That is deliberate: a 403 ("forbidden") would confirm the task exists. A 404 tells an attacker nothing. Enforcing this in SQL rather than in Go also means you cannot forget the check on a new method and quietly leak everyone's data — the query simply won't return rows that aren't yours.

**Why it matters:** Swap Postgres for MySQL and you rewrite **this file only**. It shows the three `database/sql` calls:
- `QueryRowContext` → exactly one row (get by id, `INSERT ... RETURNING`).
- `QueryContext` → many rows (list). Loop `rows.Next()`, then check `rows.Err()`.
- `ExecContext` → no rows (delete). Read `RowsAffected()` — note it returns `(int64, error)`, unlike pgx's single value.

It also owns error translation: `sql.ErrNoRows` becomes `model.ErrNotFound`, which lets the handler return a clean 404 instead of a 500. Every query uses `$1`, `$2` placeholders — **never** string concatenation — which is what prevents SQL injection.

The little `scanner` interface (`interface{ Scan(dest ...any) error }`) is satisfied by both `*sql.Row` and `*sql.Rows`, so one helper serves both the single-row and many-row queries. Nobody declares they implement it; they just have the method.

**Without it:** No persistence.

---

## `internal/service` — the business rules

### `service/user.go`
**What:** The user rules: validation, token generation, and "you may only edit yourself".

**Why it matters:** It stopped being a pass-through the moment users got CRUD. Two rules live here and nowhere else:

**Token generation** — signup mints the token, and this is the only moment it's ever visible:
```go
func newToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {   // crypto/rand
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```
`crypto/rand`, never `math/rand`. `math/rand` is a deterministic sequence from a seed: fast, repeatable, and completely predictable to anyone who can guess the seed. A predictable token is not a token. The two packages have nearly the same API, which is exactly why this mistake is common.

**Self-only edits** — `callerID` comes from the token, `id` from the URL:
```go
func (s *UserService) Update(ctx context.Context, callerID, id int, in model.UserInput) (model.User, error) {
	if callerID != id {
		return model.User{}, model.ErrForbidden
	}
	// ...
}
```
This one *can't* live in the `WHERE` clause the way task ownership does, because the rule isn't "which rows can you see" but "which row may you write". So it's an explicit check — and it sits in the service, where a future CLI or admin job gets it for free, rather than in the handler.

### `service/task.go`
**What:** `TaskService` wraps the repository. Every method takes the caller's `userID` and passes it down. `Create`/`Update` call `Validate()` before touching SQL; the rest pass through.

**Why it matters:** This is the layer that answers "what are the rules?", separate from "how do we speak HTTP?" (handler) and "how do we store it?" (repository). Validation lives here rather than in the handler so **no caller can skip it** — add a CLI or a background job tomorrow and the rules still run.

Right now `List`/`Get`/`Delete` are one-line pass-throughs, which looks like pure ceremony. That's honest: in a project this small the service earns its keep only in `Create`/`Update`. It's the seam where the next rule lands (ownership checks, quotas, publishing an event) without handlers or SQL noticing.

**Without it:** Validation would live in handlers, and every new entry point would have to remember to repeat it.

---

## `internal/handler` — the HTTP layer

### `handler/task.go` — router + handlers
**What:** `Handler` holds the service. `Router()` builds the chi router with `middleware.Logger` and `middleware.Recoverer` and registers six routes. Each handler method decodes, calls the service, and maps the result to a status code.

**Why it matters:** This is the **HTTP boundary** and the only place that knows what a status code is. Every handler has the signature `func(w http.ResponseWriter, r *http.Request)` — the one contract for all Go HTTP handlers.

Each handler owns a `Router()` returning its own sub-router with **relative** paths, and every route needs a token:
```go
func (h *TaskHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(h.auth.RequireUser)   // all task routes
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	// ...
}
```
Each handler is three steps: pull the user from the context, call the service, hand any error to `respondError`.

### `handler/user.go` — the users router
**What:** `UserHandler` and its `/users` sub-router.

**Why it matters:** It has a wrinkle tasks don't: **signup can't require a token**, because you have no token until you've signed up. So the middleware covers a subset:
```go
r.Post("/", h.create)          // public signup

r.Group(func(r chi.Router) {
	r.Use(h.auth.RequireUser)
	r.Get("/", h.list)
	r.Get("/me", h.me)
	r.Get("/{id}", h.get)
	// ...
})
```
`Group` is `Route`'s sibling: same middleware scoping, no URL prefix. It exists exactly for "these routes, but not those."

`/me` and `/{id}` coexist because chi ranks **static segments above parameters** — `/me` matches the literal route, not `id="me"` (which would 400 anyway).

### `handler/middleware.go` — the token gate
**What:** `Auth` owns `RequireUser`, which reads the `Authorization: Bearer <token>` header, loads the user, and stores it in the request context. `userFrom` reads it back.

`Auth` is its own type rather than a method on a handler because both routers need it. One `Auth` gets constructed in `main.go` and handed to each handler — the alternative was every handler carrying a `*service.UserService` just to authenticate.

**Why it matters:** Middleware is just a function that takes a handler and returns a handler, so it can run code before and after the next one in the chain:
```go
func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ... before
		next.ServeHTTP(w, r.WithContext(ctx))
		// ... after (we don't need any)
	})
}
```
That's the entire concept. No framework, no annotations, no filter registry — a function wrapping a function.

Three details that matter:
- **`r.WithContext` returns a copy.** A `*http.Request` is not meant to be mutated in place, so you attach the value to a new context, build a new request from it, and pass *that* one down.
- **The context key is an unexported type**, `type ctxKey struct{}`. If the key were the string `"user"`, any library in your process storing `"user"` would collide with you. An unexported type from your package cannot be constructed elsewhere, so collision is impossible by construction.
- **`userFrom` panics** if the user is missing. That looks reckless, but a missing user can only mean a route was mounted without `RequireUser` — a programmer error, not a bad request. Panicking makes the bug loud and immediate. `middleware.Recoverer` catches it and returns a 500 instead of killing the server.

### `handler/errors.go` — the "exception handler"
**What:** `respondError`: one `switch` mapping domain errors to status codes.

**Why it matters:** Go has no exceptions, so this is **not** a catch block. Errors arrive here as ordinary return values the handlers passed along by hand. But it plays the role you'd want a catch block for: one place that decides what the outside world sees.
- `ErrInvalid` → 400, real message (the client can fix it)
- `ErrUnauthorized` → 401 (who are you?)
- `ErrForbidden` → 403 (I know who you are; no)
- `ErrNotFound` → 404
- `ErrConflict` → 409 (email taken)
- anything else → **log the detail, return a generic 500**

Adding users cost this file five lines and touched nothing else. That's the payoff for centralizing it: eleven handlers, one status-code policy.

That last line is the important one. An unrecognized error is *our* bug, and its text may contain SQL, table names, or connection strings. The client gets `internal error`; the log gets everything.

> **Why this exists now but didn't before:** with one error type and one resource, an inline `if errors.Is(...)` per handler was clearer than a helper. At three sentinels across six handlers it was the same eight lines copy-pasted, and any new error type meant editing all six. That's when a central mapper starts paying for itself. Reach for the abstraction when the repetition shows up, not before.

**Panics are the other half.** `middleware.Recoverer` catches a panic anywhere below it, logs the stack, and returns 500 — so one bad request can't take the whole server down. Note that `panic`/`recover` is **not** Go's exception system: it's for programmer errors (nil map write, impossible state), not for control flow. Expected failures are values you return.

**Without it:** No routes; the service would have no way to be reached over HTTP.

### `handler/http.go` — request plumbing
**What:** `Health`, `decodeJSON`, `parseID`, `writeJSON`, and the 1 MiB body cap.

`decodeJSON` is **generic** — one function serving every input type:
```go
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	// ... same checks for every T
}
```
Called as `decodeJSON[model.TaskInput](w, r)` or `decodeJSON[model.UserInput](w, r)`. Before generics (Go 1.18) this would have been two near-identical copies, or a `func(dst any)` that traded away compile-time type safety. `[T any]` says "works for any type, and the caller still gets that exact type back" — the returned value is a real `model.UserInput`, not an `any` needing a cast.

**Why it matters:** This is **HTTP-shape validation**, which is a different job from field rules:

| Check | Failure | Why |
|---|---|---|
| `Content-Type: application/json` | **415** | Wrong media type is a protocol error, not a field error |
| `http.MaxBytesReader` (1 MiB) | **400** | An unbounded body is a memory-exhaustion vector: without the cap, `Decode` will read a 10 GB upload straight into memory |
| `dec.DisallowUnknownFields()` | **400** | `{"titel":"x"}` is a client bug; silently ignoring it means a task saves with an empty title and nobody learns why |
| JSON parses at all | **400** | Malformed syntax |
| id is a positive integer | **400** | `/api/tasks/abc` and `/api/tasks/0` can never match a row |

Field rules (`title` required, length ≤ 200) deliberately live in `model`, called by `service`. The rule of thumb: **shape here, meaning there.**

**Without it:** Handlers would repeat the same decode-and-check boilerplate six times, and the API would accept junk it should reject.

---

## Go module files

### `go.mod` — the module manifest
**What:** Names the module (`task_crud_api`), pins the Go version, lists direct dependencies (`chi/v5`, `lib/pq`, `godotenv`).

**Why it matters:** Go's `package.json` / `pom.xml`. It makes the folder a *module* and resolves import paths — including your own (`task_crud_api/internal/model`). You rarely edit it by hand; `go get` and `go mod tidy` maintain it.

**Without it:** `go build` fails — no module, so imports can't be resolved.

### `go.sum` — the lockfile / checksums
**What:** Cryptographic hashes for every dependency version.

**Why it matters:** Security and reproducibility. Every build verifies downloads against these hashes, so a tampered dependency is rejected.

**Without it:** Builds work but lose integrity verification; `go mod tidy` regenerates it. Never edit by hand.

---

## Config

### `.env` / `.env.example`
**What:** `PORT` and `DATABASE_URL`. `.env` is real and **gitignored**; `.env.example` is the committable template.

**Why it matters:** `godotenv.Load()` reads `.env` into the process environment, and it **never overwrites** a variable that's already set. That gives you a clean precedence chain: real OS env > `.env` > hardcoded default. Production sets real env vars and ships no `.env` at all.

`.env` is loaded from the **current working directory**, not from next to the binary — which is why you run `go run ./cmd/api` from the project root.

> **`sslmode=disable` is mandatory here.** lib/pq defaults to `sslmode=require`; the local container speaks no TLS. Omit it and you get `pq: SSL is not enabled on the server`.

**Without `.env`:** The app falls back to `PORT=3000` and the default URL in `db.go`.

---

## Infrastructure

### `docker-compose.yml` — the database, declared
**What:** Postgres 17: image, `POSTGRES_USER/PASSWORD/DB`, host→container port mapping (**5433**→5432), a named volume `taskdata`, and a healthcheck.

**Why it matters:** Turns "install and configure Postgres" into `docker compose up -d`. The **named volume** is what makes rows survive `docker compose stop/start`. The port is 5433 because this machine already runs a native Postgres on 5432 — `localhost:5432` would silently hit the *wrong server* and fail auth.

**Without it:** Install and run Postgres by hand, and keep its config in sync yourself.

### `initdb/schema.sql` — first-boot schema + seed
**What:** Creates `users` and `tasks`, links them, indexes the link, and seeds two users with two demo tokens plus three tasks.

**Why it matters:** Any `.sql`/`.sh` mounted into `/docker-entrypoint-initdb.d` runs **once**, automatically, the first time the volume initializes. A fresh `docker compose up` gives you ready tables with no manual `psql`. `SERIAL` is why the database owns id generation; `now()` is why it owns the timestamp.

The `tasks.user_id` line does a lot of work in one clause:
```sql
user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE
```
- `REFERENCES` is the **foreign key**: Postgres now refuses to store a task whose `user_id` doesn't exist. Your data cannot drift into an orphaned state, no matter what the Go code does.
- `NOT NULL` means every task has an owner. No "somebody's" tasks.
- `ON DELETE CASCADE` means deleting a user deletes their tasks automatically. Without it, the delete would fail with a foreign-key violation.

`tasks_user_id_idx` exists because *every* task query now filters on `user_id`. An index on a column you filter by on every request is not premature; it's the whole point of indexes.

**Without it:** The container starts empty and the app's first query fails: `relation "tasks" does not exist`.

> **Gotcha:** initdb scripts run *only* on an empty volume. Change this file and you must `docker compose down -v` (wipe the volume) for it to re-run.

---

## Tooling & docs

### `insomnia-collection.json`
**What:** An Insomnia v4 export with every endpoint plus the 404/400/415 cases.

**Why it matters:** Faster, repeatable testing than typing `curl` each time. Not required to run the app.

### `README.md` — how to run it
### `LEARN.md` — this file
README says *how to run*; LEARN says *why each piece exists*.

---

## The request lifecycle (how the layers cooperate)

A single `POST /api/tasks` with `{"title":"  buy milk  "}` and Alice's token:

```
HTTP request
  → cmd/api/main.go        root router: Logger, Recoverer, then Mount picks /api/tasks
  → handler/middleware.go  RequireUser reads "Bearer alice-token-123"
  → service/user.go        → repository/user.go: SELECT ... WHERE api_token = $1
  ←                        Alice loaded, stored in the request context
  → handler/http.go        Content-Type ok? under 1 MiB? JSON parses? no stray fields?
  → handler/task.go        route "/" matched, decoded, userFrom(ctx) gives Alice
  → service/task.go        Validate() trims to "buy milk", rules pass
  → repository/task.go     INSERT INTO tasks (user_id, title) VALUES ($1, $2) RETURNING ...
  → repository/db.go       a pooled connection carries it to Postgres
  → docker-compose         the container assigns id + created_at
  ← repository/task.go     scans the returned row into a model.Task
  ← service/task.go        passes it back untouched
  ← handler/task.go        writeJSON encodes it, 201 Created
HTTP response
```

Five ways a request can stop early, each at the cheapest possible point:

| Request | Stops at | Result |
|---|---|---|
| No token | `RequireUser` | 401, never reaches a handler |
| `{"titel":"x"}` | `handler/http.go` | 400, never reaches the service |
| `{"title":""}` | `service` | 400 `invalid input: title is required`, never reaches SQL |
| Bob asks for Alice's task 1 | `repository` (WHERE) | zero rows → `ErrNotFound` → 404, existence not confirmed |
| Alice edits Bob's user | `service` (callerID != id) | 403, never reaches SQL |

Every layer has exactly one job. That separation is the whole point: change how data is stored (`repository`) without touching how it's served (`handler`), and vice versa.
