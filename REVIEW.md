# REVIEW.md

Review rules specific to wallet-core. The AI reviewer applies these **on top of** the shared policy in the docs repo, `docs/review-policy.md` (scope, sources of truth, P0–P3 severity, evidence, output format). Process: `docs/review-process.md`. Findings are written in Vietnamese.

## Always check

- **Money movement** (`internal/ledger`, `internal/payment`, `internal/wallet`, `internal/limit`):
  - Balances change only through `ledger.Post`; postings balance and follow the posting matrix (erd §3.2).
  - Ledger, wallet, payment, idempotency and outbox writes share one `db.WithTx`; no network call inside the tx.
  - Wallets and ledger accounts are locked with `SELECT … FOR UPDATE … ORDER BY id`.
  - Money is `int64` VND plus currency, never float.
  - A business-error transfer stores no payment row (erd §4.7).
- **Idempotency**: the request hash uses semantic command fields; business errors are stored via `SaveFailure`, `KindInternal` errors are not.
- **Status changes** are conditional updates (`WHERE status = $from AND version = $v`) and write `*_history`.
- **Outbox**: events are enqueued in the business tx with versioned names (`*.v1`); consumers are idempotent through `processed_events`.
- **Errors**: business rules return `*apperr.Error` with a 4xx kind; never a 5xx.
- **Time** comes from `clock.Clock`; no `time.Now()` in modules.
- **Logs** use `observability.NewLogger` and `*Context` methods; no request bodies or sensitive keys.
- **Module boundaries**: other modules import only `internal/<module>` roots; a new cross-module import needs both archtest `allowed` and architecture §4.3 updated.
- **Generated code**: `*/store` is sqlc output; changes must come from `queries.sql` plus `make sqlc`.
- **Migrations** are numbered globally, have a matching `.down.sql`, and never weaken the ledger triggers or `CHECK` constraints from 0002.

## Tests

- Money-moving changes need an integration test in `test/integration` (testcontainers, `-race`), including a concurrent case when locking changes.
- Pure rules in `domain/` need unit tests.
- Never accept a skipped or deleted test to get CI green. `learn/` exercises that call `t.Skip("bài tập")` are expected.

## Docs that must move with the code (report as P2)

- API change without `api/openapi.yaml` change.
- Moved code that `docs/go-map/` anchors to by `file:line` without updating the anchors.
- New module without the steps listed in `CLAUDE.md` ("Adding a module").

## Known gaps (do not report as defects)

The "Deliberate gaps vs. the design" list in `CLAUDE.md` (dev header auth, `recipient.phone` → 422, authz/limit TODOs, partial payment columns, no OpenTelemetry) is intentional for the current slice. Report only if a PR claims to close a gap and does it wrong.
