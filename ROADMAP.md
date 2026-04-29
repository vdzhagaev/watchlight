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
for known error cases), tests are green, CI is green on `main`.

### v0.2 — SQLite parity + storage switch

Make the SQLite backend a drop-in replacement for memory, selectable at
startup via config. Reuse the existing Service test suite to verify
behavioral parity between the two backends.

**Scope:**

- `GetMonitorList` and `DeleteMonitor` on `sqlite.Storage`
- Compile-time assertion `var _ monitor.Repository = (*sqlite.Storage)(nil)`
- Config field `storage.type` (`memory` | `sqlite`) plus `storage.path`;
  selection happens in `cmd/server`
- Refactor the v0.1 Service test suite to be repository-agnostic via a
  factory, then run it against both backends
- README note on the "delete `storage.db` if schema changed" caveat,
  marked as temporary until v0.3

**Deliberately out of scope:**

- Migration mechanism (own milestone in v0.3)
- Connection pooling / WAL mode tuning
- Storage backends beyond memory and SQLite

**Exit criteria:** either backend can be selected via config without code
changes; the full Service test suite passes against both.

### v0.3 — Migrations

Build a hand-rolled migration mechanism (no third-party tool) so the
SQLite schema can evolve without "delete storage.db" workarounds. The
goal is to understand the moving parts of a migration system before
reaching for a library in a future project.

**Scope:**

- `migrations/` directory with numbered SQL files (e.g. `0001_initial.sql`)
- Schema-tracking table (e.g. `_migrations` with `id`, `name`, `applied_at`)
  created automatically on first run
- Migrator on startup: read applied set, run pending migrations in
  numeric order, each inside its own transaction, stop on first failure
- Convert the current SQLite schema into `0001_initial.sql`
- README section explaining how to add a new migration

**Deliberately out of scope:**

- Down-migrations / rollback
- Concurrent-safe locking (single-node assumption holds through v0.6)
- Checksums / drift detection

**Exit criteria:** schema changes ship as new migration files instead of
README notes; running the server against an existing `storage.db` applies
any pending migrations cleanly on startup.

### v0.4 — Manual check trigger + runners

Make the service actually *check* a monitor. Persist results.

- `ResultRepository` interface with memory + SQLite implementations
- `internal/checker` package with a `Checker` interface
- HTTP checker: GET with configured timeout, status-code and keyword checks
- Headless checker stub (decide inside the issue whether to fully implement
  with chromedp/rod or defer to a later phase)
- `POST /monitors/{id}/check[?type=http]` handler that runs the selected
  check, saves the result, and updates `Monitor.Status`
- Structured error surface — no bare 500s for expected check failures

**Exit criteria:** manual trigger returns a persisted result and updates
the monitor's status; latest result is queryable.

### v0.5 — Background scheduler

Run enabled check configs on their declared intervals without human input.

- `NextCheckAt` field on `MonitorCheckConfig` (schema, struct, persistence)
- `Repository.ListDueConfigs(ctx, now, limit)` — indexed scan, oldest-first
- `internal/scheduler` package: long-running loop owned by `cmd/server`,
  ticking every N seconds, dispatching due checks to a worker pool
- Config fields: `scheduler.tick_interval`, `scheduler.max_in_flight`
- Graceful shutdown: in-flight checks drain before exit
- Structured logs per tick and per check
- Assumption: single-node deployment (no leader election). Document it.

**Exit criteria:** creating a monitor with a short interval produces a
steady stream of result rows; stopping the server drains in-flight work
within `shutdown_timeout`.

### v0.6 — Incidents + notifications

Turn streaks of failures into tracked incidents and notify on change.

- `Incident` entity: `id`, `monitor_id`, `started_at`, `ended_at`, `reason`,
  `last_result_id`
- `IncidentRepository` with memory + SQLite implementations
- Rule engine: N consecutive failures open an incident; first success
  closes it. Thresholds in config.
- `internal/notifier` with a `Notifier` interface; Slack implementation via
  incoming webhook
- Wire notifier into the scheduler/checker pipeline
- `GET /monitors/{id}/incidents` endpoint

**Exit criteria:** a failing monitor opens an incident and posts to Slack;
recovery closes the incident and posts a second message.

## Beyond v0.6

Possible directions, not committed to:

- Additional notifier channels (email, PagerDuty, Telegram)
- Multi-node support with leader election
- Web UI
- Public authentication (API keys, OIDC)
- Historical metrics / dashboards
- Multi-region probing

These are explicitly out of scope until v0.6 lands.
