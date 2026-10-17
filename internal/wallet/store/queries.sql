-- name: InsertWallet :exec
INSERT INTO wallet.wallets (id, customer_id, ledger_account_id, currency, status)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertStatusHistory :exec
INSERT INTO wallet.status_history (id, wallet_id, from_status, to_status, reason, actor_type, actor_id, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetWallet :one
SELECT * FROM wallet.wallets WHERE id = $1;

-- name: GetWalletByCustomer :one
SELECT * FROM wallet.wallets WHERE customer_id = $1;

-- name: LockWallets :many
-- Same ordering rule as ledger.LockAccounts: ascending id, to avoid deadlocks.
SELECT * FROM wallet.wallets
 WHERE id = ANY(sqlc.arg(ids)::uuid[])
 ORDER BY id
 FOR UPDATE;

-- name: TouchActivity :exec
UPDATE wallet.wallets SET last_activity_at = sqlc.arg(at)::timestamptz WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpdateStatus :execrows
-- Conditional update (erd §6): succeeds only if nobody changed the row since we read it.
UPDATE wallet.wallets
   SET status = sqlc.arg(to_status), status_reason = sqlc.narg(reason), version = version + 1
 WHERE id = sqlc.arg(id) AND status = sqlc.arg(from_status) AND version = sqlc.arg(version);
