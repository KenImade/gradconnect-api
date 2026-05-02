# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make run/api              # Run server (port 4000)
make build/api            # Compile to ./bin/api
make test                 # Run all tests with race detector
make audit                # go mod tidy + vet + staticcheck + tests

make db/migrations/new name=<name>   # Create migration pair
make db/migrations/up                # Apply all pending migrations
make db/migrations/down              # Rollback last migration
make db/seed/all                     # Seed test data
make db/psql                         # Connect to dev DB via Docker

make docs/generate        # Regenerate Swagger/OpenAPI docs from annotations

make test/integration     # Run integration tests (requires test DB on port 5433)
                          # First-time setup: migrate -path ./migrations -database $GRADCONNECT_TEST_DB_DSN up
```

## Architecture

Go modular monolith following the Edwards pattern — one binary, clear layer separation:

- **`cmd/api/`** — HTTP layer: handlers, routes, middleware, server setup. The `application` struct (defined in `main.go`) holds all dependencies and is the receiver on every handler.
- **`internal/data/`** — Data access layer. `Models` struct in `models.go` collects all repositories. Each model receives `*pgxpool.Pool` and `context.Context` at call time (no global state, no ORM).
- **`internal/worker/`** — Background job pool. Jobs are enqueued to the `task_queue` table as JSONB payloads; the pool polls with `FOR UPDATE SKIP LOCKED` for safe concurrency. Retry schedule: 1 min → 5 min → 30 min; after 3 failures, status becomes `dead`.
- **`internal/mailer/`** — SMTP email abstraction (Resend). Templates live in `internal/mailer/templates/`.
- **`internal/storage/`** — Cloudflare R2 (S3-compatible) file storage abstraction.
- **`internal/validator/`** — Accumulates field-level validation errors into `map[string]string`.

### Request flow

```
Sentry → logRequests → enableCORS → rateLimitAll → authenticate → router
                                                                     ↓
                                              [requireAuthenticatedUser]
                                              [requireVerifiedUser]
                                              [requirePermission("code")]
                                                     ↓
                                              handler → model → DB
```

- `authenticate` always runs and sets the user (or `AnonymousUser`) in context via session cookie.
- Auth gates wrap individual handlers, not route groups.

### Handler pattern

Every handler follows: parse input → validate → call model → map domain errors → write JSON envelope.

Responses are always enveloped: `{"data": ..., "meta": {...}}`.

Domain errors (`ErrRecordNotFound`, `ErrEditConflict`, etc.) are defined in `internal/data/` and mapped to HTTP status codes in handlers.

### Auth & permissions

- Session cookie (`session_id`) validated against DB on every request.
- Google OAuth supported.
- RBAC via `user_permission` table. Permission codes: `admin:full`, `review:submit`, `review:edit`.
- Email verification required for write operations (bookmark, track application, submit review).

### Database

PostgreSQL 16 with `golang-migrate`. 15 migrations in `migrations/`. Key patterns:
- **Optimistic concurrency**: `version` column on editable entities; increment on update.
- **JSONB**: used for flexible/nested data (offices, preferences, stage_breakdown, task payloads, import row errors).
- **Full-text search**: GIN indexes on employer and opportunity tables.
- **Enums**: `auth_provider_type`, `review_outcome_type`, `review_status_type`, `task_status_type`.
- **Cron idempotency**: `cron_run` table with composite unique `(job_name, run_date)`.
- `updated_at` auto-maintained by trigger (migration 10).

### Rate limiting

In-memory limiter. Auth endpoints: 3 req/hour per IP. Global: 100 req/min per session or IP.

### Background jobs

Job types: `email:verify`, `email:welcome`, `email:password_reset`, `email:deadline_reminder`, `admin:import`, `employer:recalc_ratings`. Admin endpoints at `/api/v1/admin/jobs/{deadline-reminders,recalculate-ratings,cleanup-sessions}` trigger manual runs.

### API docs

- ReDoc: `http://localhost:4000/api/v1/docs/redoc` (all envs)
- Swagger UI: `http://localhost:4000/api/v1/docs/swagger/` (dev only)
- Run `make docs/generate` after changing handler annotations.

### Local dev

Docker Compose provides two Postgres 16 instances: dev on port 5432 (`gradconnect`), test on port 5433 (`gradconnect_test`). Use `direnv` with `.envrc` for environment variables (DSN, SMTP, OAuth, R2, Sentry, CORS origins).
