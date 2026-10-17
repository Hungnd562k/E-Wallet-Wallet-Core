-- name: InsertEvent :exec
INSERT INTO platform.outbox_events
  (id, aggregate_type, aggregate_id, event_type, payload, headers, available_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ClaimEvents :many
-- Leases up to `batch` due events to one worker. SKIP LOCKED lets several
-- workers claim concurrently without blocking on each other's rows.
UPDATE platform.outbox_events e
   SET status = 'PROCESSING', locked_by = sqlc.arg(worker_id)::text, locked_until = sqlc.arg(lease_until)::timestamptz
 WHERE e.id IN (
   SELECT o.id FROM platform.outbox_events o
    WHERE o.status IN ('PENDING', 'PROCESSING')
      AND o.available_at <= sqlc.arg(now)
      AND (o.locked_until IS NULL OR o.locked_until < sqlc.arg(now))
    ORDER BY o.available_at
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED)
RETURNING e.*;

-- name: MarkDone :exec
UPDATE platform.outbox_events
   SET status = 'DONE', processed_at = sqlc.arg(processed_at)::timestamptz, locked_until = NULL, locked_by = NULL, attempts = attempts + 1
 WHERE id = sqlc.arg(id);

-- name: MarkRetry :exec
UPDATE platform.outbox_events
   SET status = 'PENDING', attempts = attempts + 1, available_at = sqlc.arg(available_at),
       last_error = sqlc.arg(last_error)::text, locked_until = NULL, locked_by = NULL
 WHERE id = sqlc.arg(id);

-- name: MarkDead :exec
UPDATE platform.outbox_events
   SET status = 'DEAD', attempts = attempts + 1, last_error = sqlc.arg(last_error)::text, locked_until = NULL, locked_by = NULL
 WHERE id = sqlc.arg(id);

-- name: InsertProcessed :execrows
INSERT INTO platform.processed_events (consumer, event_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;
