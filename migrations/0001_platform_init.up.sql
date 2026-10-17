-- platform: shared infrastructure tables (erd §4.14).
CREATE SCHEMA IF NOT EXISTS platform;

-- Generic helpers reused by every module's migrations.
CREATE FUNCTION platform.set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE FUNCTION platform.forbid_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION '%.% is append-only', TG_TABLE_SCHEMA, TG_TABLE_NAME;
END $$ LANGUAGE plpgsql;

CREATE TABLE platform.settings (
  key         TEXT PRIMARY KEY,
  value       JSONB NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  updated_by  UUID NULL, -- ref backoffice.staff_users
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE platform.outbox_events (
  id             UUID PRIMARY KEY,
  aggregate_type TEXT NOT NULL,
  aggregate_id   UUID NOT NULL,
  event_type     TEXT NOT NULL,
  payload        JSONB NOT NULL,
  headers        JSONB NOT NULL DEFAULT '{}',
  status         TEXT NOT NULL DEFAULT 'PENDING'
                 CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'DEAD')),
  attempts       INT NOT NULL DEFAULT 0,
  available_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_until   TIMESTAMPTZ NULL,
  locked_by      TEXT NULL,
  last_error     TEXT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at   TIMESTAMPTZ NULL
);

CREATE INDEX outbox_events_claim_idx ON platform.outbox_events (available_at)
  WHERE status IN ('PENDING', 'PROCESSING');

CREATE TABLE platform.processed_events (
  consumer     TEXT NOT NULL,
  event_id     UUID NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer, event_id)
);

CREATE TABLE platform.idempotency_keys (
  scope_type      TEXT NOT NULL CHECK (scope_type IN ('CUSTOMER', 'MERCHANT', 'STAFF')),
  scope_id        UUID NOT NULL,
  key             TEXT NOT NULL CHECK (length(key) BETWEEN 1 AND 128),
  request_hash    BYTEA NOT NULL,
  endpoint        TEXT NOT NULL,
  status          TEXT NOT NULL CHECK (status IN ('IN_PROGRESS', 'COMPLETED')),
  response_status INT NULL,
  response_body   JSONB NULL,
  resource_id     UUID NULL,
  locked_until    TIMESTAMPTZ NULL,
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (scope_type, scope_id, key)
);

CREATE TABLE platform.job_runs (
  id            UUID PRIMARY KEY,
  job_name      TEXT NOT NULL,
  scheduled_for TIMESTAMPTZ NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED')),
  instance      TEXT NOT NULL,
  started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at   TIMESTAMPTZ NULL,
  summary       JSONB NULL,
  error         TEXT NULL,
  UNIQUE (job_name, scheduled_for)
);
