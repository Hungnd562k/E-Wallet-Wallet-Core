-- name: InsertKey :execrows
-- Claims the key for this request. Zero rows means the key already exists.
INSERT INTO platform.idempotency_keys
  (scope_type, scope_id, key, request_hash, endpoint, status, locked_until, expires_at)
VALUES ($1, $2, $3, $4, $5, 'IN_PROGRESS', $6, $7)
ON CONFLICT DO NOTHING;

-- name: GetKeyForUpdate :one
SELECT * FROM platform.idempotency_keys
 WHERE scope_type = $1 AND scope_id = $2 AND key = $3
 FOR UPDATE;

-- name: CompleteKey :exec
UPDATE platform.idempotency_keys
   SET status = 'COMPLETED', response_status = $4, response_body = $5,
       resource_id = $6, locked_until = NULL
 WHERE scope_type = $1 AND scope_id = $2 AND key = $3;

-- name: UpsertCompletedKey :exec
-- Stores the outcome of a request whose main transaction rolled back (business
-- error), so a retry with the same key gets the same answer.
INSERT INTO platform.idempotency_keys
  (scope_type, scope_id, key, request_hash, endpoint, status, response_status, response_body, expires_at)
VALUES ($1, $2, $3, $4, $5, 'COMPLETED', $6, $7, $8)
ON CONFLICT (scope_type, scope_id, key) DO NOTHING;
