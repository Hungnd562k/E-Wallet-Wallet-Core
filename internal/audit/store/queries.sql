-- name: InsertAuditLogs :exec
INSERT INTO audit.audit_logs (id, occurred_at, actor_type, actor_id, action, resource_type, resource_id, outcome, reason, request_id, trace_id, ip, user_agent, metadata) 
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);