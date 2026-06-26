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
- **Tests at the boundaries that change.** Service suite first, then storage
  parity against a throwaway SQLite file.
- **No premature optimization.** Correctness and clarity beat cleverness
  until there's a measurable reason to reach for it.

## Architecture decisions already made

These are settled; changing them requires a rethink.

- IDs are `uuid.UUID` (UUIDv7, time-ordered). Generated in `Service`,
  not by the database.
- Repository interface lives with the feature package; storage packages
  import the feature and satisfy the interface structurally.
- `Create` and `Update` are separate methods (no `Save`/upsert). `Update`
  uses pointer-based partial-input semantics.
- Routing: `chi` v5. Config: `cleanenv` (YAML + env overrides).
- Logging: `slog` with a custom pretty handler in dev, JSON in prod.
- SQLite driver: `modernc.org/sqlite` (pure Go, no cgo).
- **Storage is SQLite-only.** The in-memory backend was removed in #35, so
  there is a single persistence layer and no `storage.type` switch.
- Raw check facts cross the scheduler→domain seam as a `CheckResultInput`
  DTO; the domain derives status and mints the result ID.

## Phases

### v0.1 — Monitor CRUD [done]

Close the CRUD loop and put the Service layer under tests so subsequent
phases can refactor safely.

- `PATCH /monitors/{id}` handler + partial-update DTO
- `DELETE /monitors/{id}` handler, router wiring
- Service test suite covering Create, Get, Update, List, Delete, and known
  error paths
- CI: GitHub Actions running `go build`, `go vet`, `go test` on push and PR

**Exit criteria:** all five endpoints respond correctly (including 404/409
for known error cases), tests are green, CI is green on `main`.

### v0.2 — SQLite storage [done]

Persist monitors and check results in SQLite as the single storage backend,
wired into `cmd/server`.

- Full read/write/delete surface on `sqlite.Storage` (`CreateMonitor`,
  `GetMonitor`, `GetMonitorList`, `UpdateMonitor`, `DeleteMonitor`,
  `SaveCheckResult`)
- Compile-time assertion `var _ monitor.Repository = (*sqlite.Storage)(nil)`
- Per-method error semantics match the domain contract
  (`ErrMonitorNotFound`, `ErrMonitorExists`, `ErrCheckResultExists`)
- Selected at startup via the `STORAGE_PATH` env var

**Exit criteria:** the server persists monitors and results to disk; a
restart preserves data; the storage suite passes against a throwaway file.

### v0.3 — Checker [done]

Make the service actually *check* a monitor and persist the result.

- `internal/services/checker` package with a `Checker` interface
- HTTP checker: GET/HEAD with configured timeout, status-code and keyword
  checks
- Ping reachability check (TCP-level)
- Check results persisted via `SaveCheckResult`; per-check status
  (`CheckSuccess` / `CheckFailure`) derived in the domain from raw facts
- Headless (browser) checker — deferred to a later phase

Note: the originally planned `POST /monitors/{id}/check` manual-trigger
endpoint was not shipped — checks are driven by the scheduler (v0.4)
instead. Revisit if an on-demand trigger is wanted.

**Exit criteria:** a check runs against a real endpoint and persists a
timestamped result row.

### v0.4 — Background scheduler [done]

Run enabled check configs on their declared intervals without human input.

- `internal/services/scheduler` package: long-running loop owned by
  `cmd/server`, a min-heap ordered by next-due, dispatching due checks to a
  worker pool
- Per-config interval with start-up jitter
- Graceful shutdown: in-flight checks drain before exit (two-context
  cancellation — dispatch stops first, checks force-cancel only on deadline)
- Structured logs per check
- Assumption: single-node deployment (no leader election)

**Exit criteria:** creating a monitor with a short interval produces a
steady stream of result rows; stopping the server drains in-flight work
within `shutdown_timeout`.

**Known limitation:** check configs are loaded once at start-up; monitors
created or changed while the scheduler is running are not picked up until
restart. Addressed in v0.5.

### v0.5 — Live monitoring [next]

Make monitoring stateful and self-updating: the scheduler keeps up with
configuration changes, and each monitor carries a derived status.

**Scheduler reconfiguration**

- The scheduler picks up monitors and check configs created, updated, or
  deleted while it is running — no restart required
- New/changed configs enter the schedule; removed ones are dropped; interval
  changes take effect on the next cycle
- Mechanism decided in the issue (periodic reload-and-diff vs. push from the
  Service on CRUD)

**Monitor status**

- `MonitorStatus` (up / down / unknown) derived in the domain after a check
  result is recorded
- Confirmation policy (N consecutive failures before `down`) living on the
  check config; start with N=1 (stateless) and grow the policy later
- Layered checks roll up by severity: a deeper-layer failure dominates
  (ping < http < browser)
- Status transition surfaced as domain data (a `StatusTransition`), kept in
  the application layer — no DB-level triggers, no event bus until a real
  second consumer (notifications) lands

**Exit criteria:** adding or editing a monitor at runtime changes what the
scheduler checks without a restart; a failing check moves the monitor to
`down` and a recovery moves it back to `up`; current status is queryable.

### v0.6 — Incidents + notifications [planned]

Turn status changes into tracked incidents and notifications.

- `Incident` entity: `id`, `monitor_id`, `started_at`, `ended_at`,
  `reason`, `last_result_id`
- `IncidentRepository` on SQLite
- Rule engine: a `down` transition opens an incident; recovery closes it
- `internal/notifier` with a `Notifier` interface; Slack implementation via
  incoming webhook (config scaffolded as `SLACK_WEBHOOK_URL`)
- Wire notifier into the scheduler pipeline (the second consumer of the
  status transition)
- `GET /monitors/{id}/incidents` endpoint

**Exit criteria:** a failing monitor opens an incident and posts to Slack;
recovery closes the incident and posts a second message.

### v0.7 — Migrations [planned]

Build a hand-rolled migration mechanism (no third-party tool) so the
SQLite schema can evolve without "delete `storage.db`" workarounds. The
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
- Concurrent-safe locking (single-node assumption holds)
- Checksums / drift detection

**Exit criteria:** schema changes ship as new migration files instead of
README notes; running the server against an existing `storage.db` applies
any pending migrations cleanly on startup.

## Beyond v0.6

Possible directions, not committed to:

- Additional notifier channels (email, PagerDuty, Telegram)
- Multi-node support with leader election
- Web UI
- Public authentication (API keys, OIDC)
- Historical metrics / dashboards
- Multi-region probing
