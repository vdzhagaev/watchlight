# Roadmap

`watchlight` is evolving in small, shippable phases. Each phase corresponds
to a GitHub milestone; concrete work lives in issues attached to that
milestone. This file is the long-term compass — it rarely changes.

## Guiding principles

- **Feature-package layout.** Domain entity, Service, Repository contract,
  and DTOs all live in `internal/monitor/`. Storage and HTTP are plugins.
- **Service owns business logic, not storage.** Defaults, ID generation,
  validation, and orchestration live in `Service`. Storage just persists.
- **Small, reviewable changes.** Each issue maps to ~one PR. Phases are not
  merged atomically — they close when their issues close.
- **Tests at the boundaries that change.** Service + memory suite first;
  reuse it for SQLite parity.
- **No premature optimization.** Correctness and clarity beat cleverness
  until there's a measurable reason to reach for it.

## Architecture decisions already made

These are settled; changing them requires a rethink.

- IDs are `uuid.UUID` (UUIDv7, time-ordered). Generated in `Service.New()`,
  not by the database.
- Repository interface lives with the feature package; storage packages
  import the feature and satisfy the interface structurally.
- `Create` and `Update` are separate methods (no `Save`/upsert). `Update`
  uses pointer-based partial-input semantics.
- Routing: `chi` v5. Config: `cleanenv` (YAML + env overrides).
- Logging: `slog` with a custom pretty handler in dev, JSON in prod.
- SQLite driver: `modernc.org/sqlite` (pure Go, no cgo).

## Phases

### v0.1 — Monitor CRUD complete [done]

Close the CRUD loop and put the Service layer under tests so subsequent
phases can refactor safely.

- `PATCH /monitors/{id}` handler + `UpdateRequest` DTO
- `DELETE /monitors/{id}` handler
- Router wiring for both
- Test suite against `monitor.Service` + in-memory backend covering Create,
  Get, Update, List, Delete, and known error paths
- Optional: memory backend slice → `map[uuid.UUID]Monitor` for O(1) lookups
- CI: GitHub Actions running `go build`, `go vet`, `go test` on push and PR

**Exit criteria:** all five endpoints respond correctly (including 404/409
for known error cases), tests ain`.

### v0.2 — SQLite storage metho

Bring the SQLite backend up to emory, so it can
later be swapped in. Per-method error semantics match the memory backend
(e.g. `ErrMonitorNotFound` on a

- `GetMonitorList` on `sqlite.Srows.Err()` checked
- `DeleteMonitor` on `sqlite.Storage` — bound `id`, `404` on zero rows affected

**Exit criteria:** SQLite implements the full read/delete surface; behavior
matches memory on the shared er

### v0.3 — Checker (manual trig

Make the service actually *checs.

- `internal/checker` package wild the existing
  `CheckTCP` under it
- HTTP checker: GET with configd keyword checks
- Headless checker stub (decide inside the issue whether to fully implement
  with chromedp/rod or defer to
- `ResultRepository` interface with memory + SQLite implementations
- `POST /monitors/{id}/check[?t the selected
  check, saves the result, and updates `Monitor.Status`
- Structured error surface — nok failures

**Exit criteria:** manual triggt and updates
the monitor's status; latest result is queryable.

### v0.4 — Background scheduler

Run enabled check configs on their declared intervals without human input.

- `NextCheckAt` field on `MonitorCheckConfig` (schema, struct, persistence)
- `Repository.ListDueConfigs(ctan, oldest-first
- `internal/scheduler` package: long-running loop owned by `cmd/server`,
  ticking every N seconds, disper pool
- Config fields: `scheduler.tick_interval`, `scheduler.max_in_flight`
- Graceful shutdown: in-flight
- Structured logs per tick and per check
- Assumption: single-node deploDocument it.

**Exit criteria:** creating a m produces a
steady stream of result rows; stopping the server drains in-flight work
within `shutdown_timeout`.

### v0.5 — SQLite parity + stor

Make the SQLite backend a drop-electable at
startup via config. Reuse the Service test suite to verify behavioral
parity between the two backends

**Scope:**

- Compile-time assertion `var _te.Storage)(nil)`
- Config field `storage.type` (`memory` | `sqlite`) plus `storage.path`;
  selection happens in `cmd/ser
- Refactor the v0.1 Service test suite to be repository-agnostic via a
  factory, then run it against
- README note on the "delete `storage.db` if schema changed" caveat,
  marked as temporary until v0.

**Deliberately out of scope:**

- Migration mechanism (own mile
- Connection pooling / WAL mode tuning
- Storage backends beyond memor

**Exit criteria:** either backeg without code
changes; the full Service test suite passes against both.

### v0.6 — Migrations

Build a hand-rolled migration mechanism (no third-party tool) so the
SQLite schema can evolve withouounds. The
goal is to understand the moving parts of a migration system before
reaching for a library in a fut

**Scope:**

- `migrations/` directory with 01_initial.sql`)
- Schema-tracking table (e.g. `_migrations` with `id`, `name`, `applied_at`)
  created automatically on firs
- Migrator on startup: read applied set, run pending migrations in
  numeric order, each inside itirst failure
- Convert the current SQLite schema into `0001_initial.sql`
- README section explaining how

**Deliberately out of scope:**

- Down-migrations / rollback
- Concurrent-safe locking (single-node assumption holds through v0.6)
- Checksums / drift detection

**Exit criteria:** schema changes instead of
README notes; running the server against an existing `storage.db` applies
any pending migrations cleanly

## Beyond v0.6

### Incidents + notifications

Turn streaks of failures into ton change.
Not yet a milestone — promoted when v0.6 lands.

- `Incident` entity: `id`, `monitor_id`, `started_at`, `ended_at`, `reason`,
  `last_result_id`
- `IncidentRepository` with memory + SQLite implementations
- Rule engine: N consecutive fast success
  closes it. Thresholds in config.
- `internal/notifier` with a `Nplementation via
  incoming webhook
- Wire notifier into the schedu
- `GET /monitors/{id}/incidents` endpoint

### Other possible directions

Not committed to:

- Additional notifier channels (email, PagerDuty, Telegram)
- Multi-node support with leade
- Web UI
- Public authentication (API ke
- Historical metrics / dashboards
- Multi-region probing
