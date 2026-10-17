-- name: InsertAccount :exec
INSERT INTO ledger.accounts (id, code, type, normal_side, owner_type, owner_id, currency, allow_negative)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetAccount :one
SELECT * FROM ledger.accounts WHERE id = $1;

-- name: GetAccountByCode :one
SELECT * FROM ledger.accounts WHERE code = $1;

-- name: LockAccounts :many
-- Row-locks the accounts in ascending id order. Every writer locks in the same
-- order, so two concurrent postings on the same accounts cannot deadlock.
SELECT * FROM ledger.accounts
 WHERE id = ANY(sqlc.arg(ids)::uuid[])
 ORDER BY id
 FOR UPDATE;

-- name: UpdateBalance :exec
UPDATE ledger.accounts SET balance = $2, version = version + 1 WHERE id = $1;

-- name: InsertTransaction :exec
INSERT INTO ledger.transactions (id, kind, payment_id, description, effective_date)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertEntry :exec
INSERT INTO ledger.entries (transaction_id, account_id, direction, amount, currency, balance_after)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: SumEntriesByAccount :one
-- Recomputes an account balance from its entries; used by drift checks and tests.
SELECT COALESCE(SUM(CASE WHEN e.direction = a.normal_side THEN e.amount ELSE -e.amount END), 0)::bigint AS balance
  FROM ledger.accounts a
  LEFT JOIN ledger.entries e ON e.account_id = a.id
 WHERE a.id = $1;
