-- name: InsertPayment :exec
INSERT INTO payment.payments
  (id, type, status, customer_id, source_wallet_id, dest_wallet_id, amount, currency,
   description, ledger_txn_id, idempotency_key, request_id, completed_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: InsertStatusHistory :exec
INSERT INTO payment.status_history (id, payment_id, from_status, to_status, reason_code, actor_type, actor_id, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertActivity :exec
INSERT INTO payment.wallet_activities
  (id, wallet_id, payment_id, direction, type, amount, status, counterparty_display, description, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetPayment :one
SELECT * FROM payment.payments WHERE id = $1;
