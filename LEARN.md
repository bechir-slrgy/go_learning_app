# LEARN.md — What every file does and why it matters

A guided tour of `task_crud_api/`. For each file: **what it is**, **why it
exists**, and **what breaks without it**. Grouped by role, not alphabetically,
so the architecture reads top to bottom.

The source has no comments in it, by choice. That makes this file the place
where the reasoning lives, so it is worth keeping honest.

```
task_crud_api/
├── cmd/                          one directory per binary
│   ├── api/                      the server
│   │   ├── main.go               signals, then Load -> NewServer -> Run
│   │   ├── config.go             every env var, read once
│   │   └── server.go             wiring, routes, graceful shutdown
│   └── echo/main.go              dev-only webhook receiver
├── internal/                     importable only by this module
│   ├── model/                    the domain: types, rules, sentinel errors
│   ├── auth/                     JWT issue/verify, refresh-token hashing
│   ├── repository/               the only code that writes SQL
│   ├── service/                  business rules (incl. login/refresh/logout)
│   ├── handler/                  HTTP in (incl. auth middleware + /auth routes)
│   ├── client/                   HTTP out
│   └── response/                 JSON out, error -> status code
├── frontend/                     React + Vite + TypeScript UI
├── docs/
│   ├── ERD.md                    database diagram, from the live DB
│   └── screenshots/              the approval workflow, end to end
├── initdb/schema.sql             runs once on an empty Postgres volume
├── docker-compose.yml            Postgres 17 on host port 5433
├── go.mod / go.sum               module + checksums
├── .env / .env.example           config (.env is gitignored)
└── insomnia-collection.json      importable API requests
```

**Dependencies point one way:** `handler` → `service` → `repository` → `model`.
`response` and `client` are leaves. Nothing points back, and `model` imports
none of them, which is why every layer can share the sentinel errors without
importing each other.

**Why `internal/`?** It is a compiler rule, not a convention: packages under
`internal/` can only be imported from inside the same module. Publish this repo
and nobody can reach your handlers or your SQL. It is the one access-control
keyword Go has, spelled as a directory name.

---

## The entry point

### `cmd/api/config.go`
**What:** A `Config` struct and `LoadConfig`, reading `PORT` and `DATABASE_URL`
with fallbacks.

**Why it matters:** Nothing else in the program calls `os.Getenv`, so there is
one place to see everything that is configurable. Passing the URL into
`MustConnect` rather than letting the repository read the environment is what
lets you point the same code at a different database (a test one, for instance).

### `cmd/api/server.go`
**What:** The `Server` struct (config, db, router), `NewServer` which wires
every layer, and `Run` which owns the lifecycle.

**Why it matters:** Two jobs, both worth reading.

**Wiring.** The whole dependency graph is assembled in one readable place, and
each layer is handed what it needs instead of reaching for a global. That is
constructor injection, the same idea as Spring's `@Autowired` constructors,
done by hand. Go has no DI container and does not want one.

**The route map**, and only the map:
```go
r.Get("/health", handler.Health)
r.Mount("/api/tasks", taskHandler.Router())
r.Mount("/api/users", userHandler.Router())
r.Mount("/api/webhooks", webhookHandler.Router())
r.Mount("/api/notifications", noteHandler.Router())
r.Mount("/api/admin", adminHandler.Router())
```
`Mount` hangs a whole sub-router off a prefix. Each handler declares its routes
with **relative** paths and has no idea it lives under `/api/tasks`, so the URL
layout is one block here instead of a prefix repeated across six registrations.

**Graceful shutdown.** `ListenAndServe` blocks, so it runs in a goroutine and
reports a real failure (port already in use) back on a channel. `Run` then waits
on either that channel or `ctx.Done()`. On a signal it stops accepting new
connections, gives in-flight requests 10s to finish, and closes the database.

`middleware.Timeout(5 * time.Second)` is the line that makes the `ctx` threaded
through every query actually do something: when it fires, the request's context
is cancelled, which reaches `QueryContext`, and lib/pq aborts the running query
instead of leaving it to pin a connection.

### `cmd/api/main.go`
**What:** Barely twenty lines. `godotenv.Load()`, `signal.NotifyContext`, then
`NewServer(LoadConfig())` and `Run`.

**Why it matters:** `signal.NotifyContext` returns a context cancelled on
SIGINT (Ctrl+C) or SIGTERM (what `docker stop` sends). That cancellation is the
shutdown trigger `Run` waits on, so the whole lifecycle is expressed as a
context rather than a signal handler squirrelled away somewhere.

### `cmd/echo/main.go`
**What:** A throwaway HTTP server on `:9999/hook` that prints whatever JSON it
receives.

**Why it matters:** It is how you watch webhook delivery without depending on
httpbin or postman-echo (both were slow or unreachable during development). It
is also why `cmd/` has subdirectories at all: one module, two binaries, because
a package can only have one `func main()`.

Dev-only. Nothing imports it and deleting the folder breaks nothing.

---

## `internal/model` — the domain

### `model/task.go`
**What:** `Task`, `TaskInput`, and the `TaskStatus` state machine.

**Why it matters:** `TaskStatus` is a **named string type**, not a bare string,
so the compiler rejects a typo where a status is expected. The lifecycle lives
in one method:

```go
func (s TaskStatus) CanTransitionTo(next TaskStatus) bool
```

```
pending  --submit(member)-->  submitted  --approve(admin)-->  approved (terminal)
                                  |
                                  +-------reject(admin)---->  rejected
                                                                  |
                                  <------resubmit(member)---------+
```

Handlers and services ask this one question instead of each inventing their own
`if`, which is how state machines quietly grow contradictions. `TaskInput`
deliberately carries **no status**: changes go through the submit/approve/reject
endpoints, which enforce the rules. Letting `PUT` set it directly would route
around the machine entirely.

The `json:"..."` tags **are the API contract**. Go only marshals exported
(capitalized) fields, and without a tag it uses the Go field name verbatim, so
dropping them turns `{"id":1}` into `{"ID":1}` and breaks every client.

`ReviewedBy` and `ReviewedAt` are **pointers** because those columns are NULL
until someone reviews the task. A `*int` marshals to JSON `null` where a plain
`int` would silently claim the reviewer was user 0.

### `model/user.go`
**What:** `Role`, `User`, `UserInput`, `LoginInput`, `RefreshInput`,
`TokenPair`, and validation.

**Why it matters:** Look at what is **missing** from `User`: there is no
`PasswordHash` field. The hash is only ever a `WHERE` argument or a second
return value from the repository, never a column on the struct, so it cannot
leak into a response. The safest way to not leak a secret is to never load it
onto a serializable type.

`UserInput.Validate` splits into `ValidateProfile` (email + name, shared with
update) and the full `Validate` (adds the password rule), so an update reuses
the profile checks without demanding a password it does not carry. The password
is **not trimmed** — leading and trailing spaces are legitimate password
characters — and is capped at 72 bytes, which is bcrypt's own limit.

`LoginInput` is deliberately **not** length-validated: telling an attacker "that
is too short to be our password" leaks the rule. A wrong login is always a plain
401. `Validate` uses `net/mail.ParseAddress` rather than a regex, because email
syntax is a gnarly spec the standard library already implements.

### `model/errors.go`
**What:** Six sentinels: `ErrNotFound`, `ErrInvalid`, `ErrUnauthorized`,
`ErrForbidden`, `ErrConflict`, `ErrUpstream`.

**Why it matters:** A sentinel is one value, declared once, compared by identity
with `errors.Is`. They live in `model` because every layer needs them and none
should import another.

Validation wraps rather than returns them bare:
```go
return fmt.Errorf("%w: title is required", ErrInvalid)
```
The `%w` verb is the whole trick: you get a new error carrying your message, and
`errors.Is` still finds `ErrInvalid` inside it. Swap `%w` for `%v` and the text
looks identical while `errors.Is` quietly returns false, so the 400 becomes a
500. One letter.

> **The trap:** `errors.Is` compares *identity*, not text. A fresh
> `errors.New("not found")` is a different value that merely reads the same, so
> the check fails and your 404 becomes a 500. Declare each sentinel once, then
> always return that variable or `%w`-wrap it.

### `model/webhook.go`, `model/notification.go`
`Webhook` + `WebhookInput` (whose `Validate` accepts only `http`/`https`, so a
caller cannot point the client at `file://`), `TaskEvent` (the outgoing webhook
body), and `Notification`.

---

## `internal/repository` — the only code that knows SQL

### `repository/db.go`
**What:** `MustConnect(url)`: opens the pool, sets limits, pings, or kills the
process.

**Why it matters:** `*sql.DB` is **already a pool**, despite the name. Never
open one per request. Two things worth remembering:

- `sql.Open` is **lazy**: it validates the URL and connects to nothing.
  `PingContext` is the first real network call, which is why it belongs at
  startup with a timeout — a dead database fails in 5s instead of hanging.
- The `_ "github.com/lib/pq"` **blank import** looks like dead code. It is not.
  Importing for the side effect runs the package's `init()`, which registers the
  driver named `"postgres"`. Delete the line and `sql.Open("postgres", ...)`
  fails with `unknown driver`. **This is the single most deletable-looking
  load-bearing line in the codebase.**

`SetMaxOpenConns(25)` matters because without a cap the pool opens a connection
per concurrent request, and Postgres refuses past `max_connections` (100 by
default) — an unbounded pool turns a traffic spike into errors.

### `repository/task_repository.go`
**What:** The task queries, split into two groups.

**Member-scoped** (`List`, `Get`, `Create`, `Update`, `Delete`) take a `userID`
and every query filters on it:
```sql
SELECT ... FROM tasks WHERE id = $1 AND user_id = $2
```
The filter is in the `WHERE` clause, not an `if` in the handler. A task you do
not own produces zero rows, which becomes `ErrNotFound`, which becomes **404**.
That is deliberate: a **403 would confirm the task exists**. It also fails
closed — forget the filter on a new method and the query returns nothing, rather
than leaking everyone's data.

**Privileged** (`GetAny`, `ListByStatus`, `SetStatus`, `SetReviewed`) are *not*
user-scoped, and the names say so, because the caller is responsible for having
checked the role first.

`SetStatus` clears `reviewed_by`/`reviewed_at`; `SetReviewed` sets them with
`now()` from Postgres. That split is what keeps the audit columns describing the
*current* status: resubmitting after a rejection must not keep the stale
reviewer, and the database's CHECK would reject the row if it did.

Note `RowsAffected()` returns `(int64, error)` in `database/sql`, unlike pgx's
single value.

### `repository/user_repository.go`
`ByEmailWithHash` is the login lookup. It returns the bcrypt hash as a **second
value**, never a field on `model.User`, so the hash cannot leak into a JSON
response by accident. Not-found returns `ErrUnauthorized`, not `ErrNotFound`,
because "no such email" and "wrong password" must look identical to the caller
or you leak which emails are registered.

`Create`/`Update` translate a second failure:
```go
var pqErr *pq.Error
return errors.As(err, &pqErr) && pqErr.Code == "23505"
```
Uniqueness is enforced by Postgres, **not** by a "does this email exist?" check
in Go — that has a race between the lookup and the insert, and two simultaneous
signups slip through. Let the database referee, then translate: `23505` is
`unique_violation`, which becomes `ErrConflict`, which becomes **409**.

`errors.As` here, not `errors.Is`, because we need the `*pq.Error` **value** to
read its `.Code`. Matching on message text would break across versions and
locales.

### `repository/webhook_repository.go`, `notification_repository.go`
Same shape. Notifications list unread-first, which is what the composite index
`(user_id, read)` serves.

---

## `internal/service` — the business rules

Each service **declares the interfaces it consumes**, so the concrete
repositories satisfy them implicitly and neither package imports the other.

### `service/task_service.go`
**What:** `TaskService` plus the approval workflow and the importer.

- `Submit` — ownership comes free from the user-scoped `Get` (someone else's
  task is 404), then the state machine guards the move, then admins are alerted.
- `Review` — approve and reject share one path so the guard cannot be forgotten
  on one of them. It uses `GetAny` because an admin reviews other people's work
  by definition, and `SetReviewed` to record the audit trail.
- `Import` — fetches todos from a public API, unmarshals into an **unexported**
  `externalTodo` (somebody else's shape is not our domain model), and saves them.

Two context decisions live here, and they are opposites on purpose:

| Operation | Context | Why |
|---|---|---|
| `Import` | the request's `ctx` | the caller is waiting for the result, so abandoning work when they hang up is right |
| webhook delivery | detached (see below) | the caller is not waiting; delivery must outlive the response |

An imported todo the source calls "completed" enters the **review queue**, not
`approved`. Importing must not be a way to approve your own work.

### `service/user_service.go`
`Create` (signup) hashes the password with **bcrypt** and stores the hash, never
the password:
```go
hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
```
bcrypt salts internally and is deliberately slow, so a leaked hash resists brute
force. It returns the plain `User`, with no token — the client logs in
afterwards.

`Update`/`Delete` enforce "you may only edit yourself" by comparing the caller's
id (from the token) with the id in the URL. That rule *cannot* live in a `WHERE`
clause the way task ownership does, because the question is not "which rows can
you see" but "which row may you write".

### `service/auth_service.go`
**What:** `Login`, `Refresh`, `Logout` — the credential-to-token flow.

- **`Login`** loads the user by email, compares the password with
  `bcrypt.CompareHashAndPassword`, and issues a token pair. A wrong email and a
  wrong password return the **same** `ErrUnauthorized`, so you cannot probe which
  emails are registered.
- **`Refresh`** rotates: it hashes the incoming refresh token, looks it up,
  **spends it (deletes the row) first**, then issues a new pair. Rotation means a
  stolen refresh token works at most once before the legitimate client's next
  refresh invalidates it.
- **`Logout`** deletes the refresh-token row. It is deliberately quiet — an
  unknown token is not an error, because logging out something already gone is a
  success.

An access token cannot be revoked (it is stateless and short-lived); a refresh
token can, by deleting its row. That is the whole reason refresh tokens are
stored and access tokens are not.

### `service/webhook_service.go`
Delivery, and the most interesting context lesson in the codebase:

```go
go s.deliver(context.WithoutCancel(ctx), userID, event)
```

`context.WithoutCancel` keeps the values on `ctx` but drops its cancellation.
Passing the request context straight through looks natural and is **wrong**: the
router cancels it after 5s, and instantly if the caller hangs up, which cancelled
real deliveries to a slow endpoint. **Whichever deadline is shortest wins**, and
the request's 5s beat the client's 10s. The goroutine then means creating a task
returns immediately instead of waiting on somebody else's server.

Errors are logged, not returned: a broken subscriber must not fail the request
that triggered it.

### `service/notification_service.go`
Writes a row per admin on submission, and one to the owner on a verdict. Errors
are logged rather than returned for the same reason.

> **Known gap:** the status change and the notification rows are separate
> statements, so a crash between them loses the alert. A real system writes both
> in one transaction.

---

## `internal/auth` — the JWT machinery

### `auth/token.go`
**What:** `TokenService` — issues and verifies JWT access tokens, and mints and
hashes refresh tokens. Holds the signing secret and the two lifetimes.

**Why it matters:** This is the crypto seam, kept out of the handler and service
layers so they depend on behaviour, not on `golang-jwt`.

- **`IssueAccess`** signs an `HS256` token whose claims carry `sub` (user id),
  `role`, and `name`, plus `exp`/`iat`/`iss`. Nothing secret goes in: a JWT is
  **signed, not encrypted**, so anyone can base64-decode and read it. The
  signature only proves it was not tampered with.
- **`ParseAccess`** verifies signature + expiry and rebuilds the user from the
  claims alone — **no database call**. The keyfunc **pins the algorithm** to
  HS256:
  ```go
  if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok { ... reject }
  ```
  Without this an attacker sends `alg=none` (no signature) or passes an RSA
  public key as the HMAC secret, and forges any claims. This one check is the
  single most important line for JWT safety.
- **Refresh tokens** are 32 random bytes, hex-encoded. Only their **SHA-256
  hash** is stored, so a database leak hands over nothing usable. SHA-256, not
  bcrypt: the token already has 256 bits of entropy, so it needs no slow hash to
  resist guessing, and lookups must be fast. bcrypt is for **low-entropy**
  secrets — passwords — where slowness is the defense.

## `internal/handler` — HTTP in

### `handler/auth_middleware.go`
**What:** `Auth`, `RequireUser`, `RequireAdmin`, `userFrom`.

**Why it matters:** Middleware in Go is one signature — **a function taking an
`http.Handler` and returning an `http.Handler`**. No framework, no annotations.
chi's own `Logger` and `Recoverer` are the same shape.

`RequireUser` reads the bearer JWT and calls `tokens.ParseAccess`. Because the
user's id, role, and name live in the verified token, **authentication is one
signature check and authorization is one field read — zero queries per
request.** That statelessness is the whole point of JWT; the cost is that a role
change or ban only takes effect when the 15-minute access token expires.

Three details:

- **`r.WithContext` returns a copy.** A `*http.Request` is not meant to be
  mutated, so you attach the value to a new context, build a new request, and
  pass *that* down.
- **The context key is an unexported `type ctxKey struct{}`.** If the key were
  the string `"user"`, any library storing `"user"` would collide with you. An
  unexported type cannot be constructed elsewhere, so collision is impossible by
  construction rather than by convention.
- **`userFrom` panics** when the user is missing. That can only happen if a
  route was mounted without `RequireUser` — a wiring bug, not a bad request. A
  loud panic beats a silent zero-value `User{ID: 0}` querying nobody's tasks.
  `middleware.Recoverer` turns it into a 500.

**Authentication and authorization are separate middlewares** because most
routes need the first and only `/api/admin` needs both.

### `handler/task_handler.go`, `user_handler.go`, `admin_handler.go`, `webhook_handler.go`, `notification_handler.go`
Each owns a router with **relative** paths. Three notable shapes:

- **`admin_handler`** stacks the guards on the whole sub-router:
  ```go
  r.Use(h.auth.RequireUser)   // who are you?
  r.Use(h.auth.RequireAdmin)  // may you be here?
  ```
  so you cannot add an admin endpoint and forget the check.
- **`user_handler`** uses `r.Group` to protect everything *except* signup —
  you have no token until you have an account. `Group` is `Route`'s sibling:
  same middleware scoping, no URL prefix.
- `/users/me` and `/users/{id}` coexist because chi ranks **static segments
  above parameters**.

Handlers hold the **concrete** `*service.TaskService`, not an interface. There
is one implementation and nothing fakes it, so an interface there would be
ceremony. Contrast `Auth`, which takes a one-method `TokenVerifier` because the
middleware needs only `ParseAccess` — that narrowing is a real abstraction.
**The rule: interface at the edge you don't own (the database, the token
signer), concrete for your own single-implementation code.**

### `handler/auth_handler.go`
**What:** The public `/api/auth` router: `login`, `refresh`, `logout`.

**Why it matters:** All three are public — **you cannot require a token to
obtain a token.** Each decodes a body, calls the auth service, and returns the
token pair (or 204 for logout). No middleware, no `userFrom`.

### `handler/request.go`
**What:** `decodeJSON[T]` and `parseID`.

`decodeJSON` is **generic**, so one function serves every input type:
```go
decodeJSON[model.TaskInput](w, r)
decodeJSON[model.UserInput](w, r)
```
Before generics this would have been near-identical copies, or a `func(dst any)`
that traded away compile-time type safety.

It does **HTTP-shape** validation, a different job from field rules:

| Check | Failure | Why |
|---|---|---|
| `Content-Type: application/json` | 415 | wrong media type is a protocol error |
| `http.MaxBytesReader` (1 MiB) | 400 | without it `Decode` reads a 10 GB upload into memory |
| `DisallowUnknownFields()` | 400 | `{"titel":"x"}` is a client bug; ignoring it saves an empty title and nobody learns why |
| JSON parses | 400 | malformed syntax |
| id is a valid UUID | 400 | `/tasks/abc` and `/tasks/1` are not UUIDs, so they can never match a row |

Field rules live in `model`, run by `service`. **Shape here, meaning there.**

---

## `internal/response` — JSON out

**What:** `JSON`, `Error`, `ErrorFrom`, and the `ResponseError` body.

**Why it matters:** `ErrorFrom` is the one place that turns a domain error into
a status code. Go has no exceptions, so this is **not** a catch block — the
errors arrived as ordinary return values the handlers passed along by hand. It
is a translation table.

The `default` branch is a security control: an unrecognised error is *our* bug,
and its text may carry SQL, table names, or a connection string. The client gets
`internal error`; the log gets everything. Never `http.Error(w, err.Error(), 500)`.

> **When to extract this:** with one sentinel and one resource, an inline
> `errors.Is` per handler was clearer. At six sentinels across five handlers it
> became the same block pasted everywhere. Extract when the repetition shows you
> its shape, not when you first imagine it.

---

## `internal/client` — HTTP out

**What:** `GetJSON`, `PostJSON`, and the shared `do`.

**Why it matters:** The mirror image of `handler`. Three things every caller
must do and most forget:

- **Close the body**, or the connection leaks and the pool starves.
- **Cap what you read** from a server you do not control (`io.LimitReader`).
- **Check the status yourself.** A 4xx/5xx is a *successful* HTTP exchange as
  far as `Do` is concerned: `err` is nil. Skip the check and an outage looks
  like success.

`New(timeout)` rather than `http.DefaultClient`, which has **no timeout at all**,
so a server that accepts your connection and goes silent hangs you forever.

---

## Infrastructure

### `initdb/schema.sql`
Runs **once**, automatically, the first time the Postgres volume initializes.

The constraints are the interesting part. `CHECK (role IN ('admin','member'))`
and the status CHECK mean the database refuses values the app does not know:
Go validation protects the API, a CHECK protects the data from *any* client,
including a stray `psql` session.

`tasks_review_consistent` goes further and encodes the state machine as a
constraint, so a `pending` task cannot carry a reviewer even if the code has a
bug.

Two foreign keys to `users`, two **different** delete rules, and the difference
matters:

| Column | On user delete | Why |
|---|---|---|
| `tasks.user_id` | CASCADE | the task belongs to its owner; without them it is meaningless |
| `tasks.reviewed_by` | SET NULL | the task belongs to the *member*, not the reviewer — deleting an admin must not destroy work they approved |

> **Gotcha:** initdb scripts run only on an **empty** volume. Change this file
> and you must `docker compose down -v` (which deletes all data) for it to
> re-run. That pain is exactly what migration tools exist for.

See [docs/ERD.md](docs/ERD.md) for the full diagram, generated from the live
database rather than from this file.

### `docker-compose.yml`
Postgres 17, a named volume so rows survive `stop`/`start`, and a healthcheck.
Published on **5433**, not 5432, because this machine already runs a native
Postgres — `localhost:5432` would silently hit the wrong server and fail auth.

### `.env` / `.env.example`
`godotenv.Load()` reads `.env` into the environment and **never overwrites** a
variable that is already set, giving a clean precedence chain: real env >
`.env` > hardcoded default. `.env` is gitignored; `.env.example` is the template.

`.env` is read from the **current working directory**, which is why you run
`go run ./cmd/api` from the project root.

> **`sslmode=disable` is mandatory.** lib/pq defaults to `sslmode=require` where
> pgx defaulted to `prefer`. Omit it and you get `pq: SSL is not enabled on the
> server`.

---

## `frontend/` — the UI

React + Vite + TypeScript. `npm install && npm run dev`, with the API running.

- **`src/api/types.ts`** mirrors `internal/model` field for field. The Go json
  tags decide these names, which is why stripping a tag breaks the UI.
  `TaskStatus` is a union of the same four string literals as the Go named type,
  so both compilers reject anything else.
- **`src/api/client.ts`** mirrors `internal/client`, with the same trap
  inverted: **`fetch` only rejects on a network failure**. A 404 or 500 resolves
  normally with `ok === false`, so you must check it yourself — exactly like
  `http.Client.Do` returning `err == nil` for a 500. It also does **silent
  refresh**: on a 401 it spends the stored refresh token for a new access token
  and replays the request once, which is why a 15-minute access token is
  invisible to the user.
- **`vite.config.ts`** proxies `/api` to `:8090`. A browser on `:5173` calling
  `:8090` is cross-origin and the Go API sends no CORS headers, so the browser
  would block it. The proxy makes the API look same-origin and leaves the
  backend untouched. Deploy the frontend separately and you *will* need CORS.

The Review tab is hidden for members, but that is **convenience, not security**.
`RequireAdmin` still answers 403 to a member calling `/api/admin/tasks`
directly. "Hide the button" is the most common way people think they have
implemented authorization.

---

## The request lifecycle

`POST /api/tasks` with `{"title":"  buy milk  "}` and Bob's JWT:

```
HTTP request
  → server.go            Logger, Recoverer, Timeout(5s), Mount picks /api/tasks
  → auth_middleware.go   RequireUser verifies the JWT signature + expiry
  → auth/token.go        ParseAccess: claims -> User{id, role, name}, NO db query
  ←                      Bob (from the token) stored on the request context
  → handler/request.go   Content-Type? under 1 MiB? parses? no stray fields?
  → task_handler.go      route "/" matched, decoded, userFrom(ctx) gives Bob
  → service/task         Validate() trims to "buy milk", rules pass
  → repository/task      INSERT ... RETURNING  ($1, $2 placeholders)
  → repository/db        a pooled connection carries it to Postgres
  ← task_handler.go      response.JSON encodes it, 201 Created
  ⋮ (separately, in a goroutine on a detached context)
  → webhook_service      POSTs the task.created event to Bob's webhooks
```

## Where each rule is enforced

| Request | Stops at | Result |
|---|---|---|
| Missing/expired/forged JWT | `RequireUser` | 401 + `WWW-Authenticate`, never reaches a handler |
| Wrong login credentials | `service/auth` | 401, same for bad email or bad password |
| Member hits `/api/admin/*` | `RequireAdmin` | 403 |
| `{"titel":"x"}` | `handler/request.go` | 400, never reaches the service |
| `{"title":""}` | `service` | 400, never reaches SQL |
| Bob reads Alice's task | `repository` (WHERE) | 404, existence not confirmed |
| Alice edits Bob's user | `service` (callerID != id) | 403, never reaches SQL |
| Approving a `pending` task | `service` (state machine) | 409 |
| Duplicate email | **Postgres** (UNIQUE) | 409, race-free |
| `pending` task with a reviewer | **Postgres** (CHECK) | rejected even from raw SQL |

The bottom two are the point: the further down a rule is enforced, the fewer
ways there are to get around it.
