# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`wallet-core` of the E-Wallet platform ("Core ví"): a Go modular monolith with two processes, `cmd/api` (REST) and `cmd/worker` (outbox and background jobs), sharing one Postgres database with one schema per module.

The design is the source of truth. It lives in the sibling repo `../E-Wallet/docs/`, in Vietnamese:
- `architecture.md`: modules, dependency rules (§4.3), transactions and outbox (§5), worker (§6), security (§8), data rules (§9).
- `erd.md`: tables, ledger triggers, posting matrix (§3.2), state machines (§6).
- `business-rules.md`: the `BR-xx` business rules cited in code.
- `fsd/`: use cases (`UC-xx`) and the shared behaviours `CF-xx`.

`api/openapi.yaml` is the source of truth for the wallet-core API contract; the design docs link to it instead of keeping their own copy. Lint it with `npx @redocly/cli lint` (config in `redocly.yaml`).

The owner is new to Go and comes from C#. `docs/go-map/` is a Vietnamese learning map that anchors Go concepts to `file:line` in this repo, and `learn/` holds runnable exercises. **If you move code that docs/go-map links to, update the line anchors.** Write docs in Vietnamese and code comments in English.

## Commands

Go ≥ 1.26 is required (golang.org/x/sync and sqlc need it). Docker is needed for Postgres and the integration tests.

```bash
cp .env.example .env           # Makefile auto-loads .env; Postgres is on host port 5433
make up migrate-up seed        # start Postgres, apply migrations, create demo wallets (alice/bob, 1,000,000 VND)
make run-api / make run-worker
make test                      # go test -short ./...: unit + archtest + learn (no Docker)
make test-integration          # testcontainers Postgres, -race; RUN=<regex> to filter
make test-one PKG=./internal/ledger/... RUN='TestPostingValidate/unbalanced'
make lint                      # gofmt check + go vet (+ golangci-lint via Docker)
make sqlc                      # regenerate */store from queries.sql (go tool sqlc; pinned in go.mod)
make migrate-down / migrate-version / reset-db
```

Running one integration test directly: `go test -race -count=1 -run TestConcurrentSameKey ./test/integration`.

Learn exercises skip themselves with `t.Skip("bài tập")` until the user implements them; that is expected, not a failure.

## Layout and conventions

- `internal/<module>/service.go` is the module's **only** public API. Domain types are re-exported there with type aliases, e.g. `type Posting = domain.Posting`.
- `domain/` holds pure rules. `store/` is sqlc-generated, never edited by hand. `http/` is the transport (package `<module>http`, so it does not shadow `net/http`).
- `internal/platform/*`: shared infrastructure. It may not import any module.
- `internal/app` is the composition root, used by `cmd/*` and the integration tests. It is the only place, together with `cmd/`, that may import module sub-packages.
- `internal/archtest` enforces the dependency rules from the real import graph:
  - allowlist in `allowed`;
  - only module roots may be imported by other modules.

  **Adding a cross-module dependency means editing `allowed` and architecture.md §4.3 together.**
- **Adding a module:**
  1. `service.go`, plus `domain/` and `store/queries.sql`.
  2. Migration `migrations/NNNN_<module>_<change>.{up,down}.sql`, numbered globally.
  3. A `sqlc.yaml` entry whose `schema:` list is only `0001_platform_init.up.sql` plus the module's own migrations, so generated models never contain other modules' tables.
  4. `make sqlc`.
  5. Wiring in `internal/app/app.go`.
  6. An entry in archtest `allowed`.
- Migrations are embedded (`migrations.FS`) and applied by `cmd/migrate` or `migrations.Up` (golang-migrate as a library, `pgx5://`). The version table is pinned to `public.schema_migrations`. This matters because the DB user is named `wallet`, so `search_path` (`"$user", public`) resolves to the wallet module's schema once it exists.

## How money moves (read before touching payment/ledger)

- **Synchronous money changes run in one transaction.** `db.WithTx(ctx, pool, fn)` stores the `pgx.Tx` in ctx, and every repository uses `store.New(db.Conn(ctx, pool))`, so ledger, wallet, payment, idempotency and outbox writes commit or roll back together.
  - `ledger.Post`, `wallet.LockActive`, `outbox.Enqueue` and `idempotency.Begin` refuse to run outside a tx.
  - Keep these transactions short, with no network calls inside.
- **Locking:** wallets and ledger accounts are locked with `SELECT … FOR UPDATE … ORDER BY id`. Keep that ordering everywhere; it is what prevents deadlocks under `TestConcurrentCrossTransfers`.
- **Idempotency** (`internal/platform/idempotency`):
  - `Begin` either claims the key with `INSERT … ON CONFLICT DO NOTHING`, which blocks on a concurrent holder, or returns the stored response.
  - Business errors roll the tx back, then `SaveFailure` stores the error so retries replay it.
  - Infrastructure (`KindInternal`) errors are not stored.
  - The request hash is computed from the semantic command fields, not the raw body.
- **Outbox:** events are written in the business transaction, with versioned names like `payment.succeeded.v1`.
  - `outbox.Dispatcher` claims with `FOR UPDATE SKIP LOCKED` plus a lease and runs consumers via errgroup.
  - Each consumer runs in its own tx with a `processed_events(consumer, event_id)` marker, so delivery is at-least-once but effectively once.
  - Retries use exponential backoff with jitter; after `MaxAttempts` the event becomes `DEAD`.
  - Consumers declare the payload struct they need instead of importing the producer module.
- **Errors:** modules return `*apperr.Error{Kind, Code, Message}` sentinels, wrapped with `%w` as needed.
  - `httpx.WriteError` maps `Kind` to an HTTP status and the body `{error, message, details, request_id}`.
  - Anything that is not an `apperr` becomes an opaque 500.
  - Never return a 5xx for a business rule.
- **Time:** depend on `clock.Clock`, never `time.Now()`. `clock.Real` is UTC and truncated to microseconds so values round-trip through `timestamptz`. Tests use `clock.NewFake`.

## Invariants (do not weaken)

- `ledger` is the only module that changes balances. Postings are double-entry and validated in the app.
- The DB enforces the ledger rules too (migration 0002):
  - `ledger.entries` and `transactions` are append-only (triggers);
  - a deferred trigger checks Σdebit = Σcredit at COMMIT;
  - `CHECK (allow_negative OR balance >= 0)`.
- Money is `int64` / `BIGINT` VND plus currency. Never use floats.
- IDs are UUIDv7 (`id.New`). Public IDs are prefixed (`txn_`, `wal_`) via `id.Format` / `id.Parse`.
- Status changes are conditional updates (`WHERE status = $from AND version = $v`) and also write to `*_history`.
- A business-error transfer stores no payment row (erd §4.7).
- Logs go through `observability.NewLogger`, which redacts keys containing the words `pin`, `otp`, `token`, `cccd`, `authorization`, `password`, `secret` and `image`. Never log request bodies. Use the `*Context` slog methods so `request_id` is attached.

## Deliberate gaps vs. the design (current slice)

Only the wallet-to-wallet transfer is implemented end to end. The other modules are doc-comment stubs.
- **Auth:** `auth.DevHeaderAuth` trusts `X-Dev-Customer-Id` (local/test only). It will be replaced by identity's JWT authenticator.
- **Recipient:** `recipient.wallet_id` only; `recipient.phone` returns `422 NOT_SUPPORTED_YET`. The `pin` field is accepted but ignored.
- **authz / limit:** not called yet. There are TODOs at the exact insertion point in `payment.transferInTx`.
- **Payment columns:** `payment.payments` only has the columns the transfer needs; later migrations add the bank and merchant columns.
- **Observability:** no OpenTelemetry or metrics yet. No corebank, MinIO or merchant-sim.
