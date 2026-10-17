-- wallet: wallets and their lifecycle (erd §4.3).
CREATE SCHEMA IF NOT EXISTS wallet;

CREATE TABLE wallet.wallets (
  id                  UUID PRIMARY KEY,
  customer_id         UUID NOT NULL UNIQUE, -- ref identity.customers (BR-02: 1 wallet per customer)
  ledger_account_id   UUID NOT NULL UNIQUE, -- ref ledger.accounts (WALLET:<id>)
  currency            CHAR(3) NOT NULL DEFAULT 'VND',
  status              TEXT NOT NULL
                      CHECK (status IN ('INACTIVE', 'ACTIVE', 'SUSPENDED', 'FROZEN', 'CLOSED')),
  status_reason       TEXT NULL,
  last_activity_at    TIMESTAMPTZ NULL,
  dormant_notified_at TIMESTAMPTZ NULL,
  closed_at           TIMESTAMPTZ NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  version             INT NOT NULL DEFAULT 0
);

CREATE INDEX wallets_status_activity_idx ON wallet.wallets (status, last_activity_at);

CREATE TRIGGER wallets_updated_at BEFORE UPDATE ON wallet.wallets
  FOR EACH ROW EXECUTE FUNCTION platform.set_updated_at();

CREATE TABLE wallet.status_history (
  id          UUID PRIMARY KEY,
  wallet_id   UUID NOT NULL REFERENCES wallet.wallets (id),
  from_status TEXT NULL,
  to_status   TEXT NOT NULL,
  reason      TEXT NULL,
  actor_type  TEXT NOT NULL CHECK (actor_type IN ('SYSTEM', 'CUSTOMER', 'STAFF')),
  actor_id    UUID NULL,
  request_id  TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER status_history_immutable BEFORE UPDATE OR DELETE ON wallet.status_history
  FOR EACH ROW EXECUTE FUNCTION platform.forbid_mutation();
