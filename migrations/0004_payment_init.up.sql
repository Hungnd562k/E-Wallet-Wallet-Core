-- payment: payment orchestration and the transaction-history read-model (erd §4.7).
-- Only the columns needed by the wallet-to-wallet transfer slice are created here;
-- later migrations add bank/merchant columns as those flows are implemented.
CREATE SCHEMA IF NOT EXISTS payment;

CREATE TABLE payment.payments (
  id               UUID PRIMARY KEY,
  type             TEXT NOT NULL
                   CHECK (type IN ('TOPUP', 'TRANSFER', 'WITHDRAW', 'INTERBANK', 'QR_PAYMENT', 'REFUND', 'ADJUSTMENT')),
  status           TEXT NOT NULL
                   CHECK (status IN ('CREATED', 'PENDING', 'SUCCEEDED', 'FAILED', 'NEEDS_RECONCILIATION', 'REVERSED')),
  customer_id      UUID NOT NULL, -- ref identity.customers
  source_wallet_id UUID NULL,     -- ref wallet.wallets
  dest_wallet_id   UUID NULL,     -- ref wallet.wallets
  amount           BIGINT NOT NULL CHECK (amount > 0),
  fee              BIGINT NOT NULL DEFAULT 0 CHECK (fee >= 0),
  currency         CHAR(3) NOT NULL DEFAULT 'VND',
  description      TEXT NULL,
  ledger_txn_id    UUID NULL,     -- ref ledger.transactions
  failure_code     TEXT NULL,
  idempotency_key  TEXT NOT NULL,
  request_id       TEXT NOT NULL DEFAULT '',
  pending_since    TIMESTAMPTZ NULL,
  completed_at     TIMESTAMPTZ NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  version          INT NOT NULL DEFAULT 0,
  UNIQUE (customer_id, idempotency_key)
);

CREATE INDEX payments_pending_idx ON payment.payments (status, pending_since)
  WHERE status IN ('PENDING', 'NEEDS_RECONCILIATION');

CREATE TRIGGER payments_updated_at BEFORE UPDATE ON payment.payments
  FOR EACH ROW EXECUTE FUNCTION platform.set_updated_at();

CREATE TABLE payment.status_history (
  id          UUID PRIMARY KEY,
  payment_id  UUID NOT NULL REFERENCES payment.payments (id),
  from_status TEXT NULL,
  to_status   TEXT NOT NULL,
  reason_code TEXT NULL,
  actor_type  TEXT NOT NULL CHECK (actor_type IN ('SYSTEM', 'CUSTOMER', 'STAFF')),
  actor_id    UUID NULL,
  request_id  TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER status_history_immutable BEFORE UPDATE OR DELETE ON payment.status_history
  FOR EACH ROW EXECUTE FUNCTION platform.forbid_mutation();

CREATE TABLE payment.wallet_activities (
  id                   UUID PRIMARY KEY,
  wallet_id            UUID NOT NULL, -- ref wallet.wallets
  payment_id           UUID NOT NULL REFERENCES payment.payments (id),
  direction            TEXT NOT NULL CHECK (direction IN ('IN', 'OUT')),
  type                 TEXT NOT NULL,
  amount               BIGINT NOT NULL,
  status               TEXT NOT NULL,
  counterparty_display TEXT NOT NULL DEFAULT '',
  description          TEXT NULL,
  occurred_at          TIMESTAMPTZ NOT NULL,
  UNIQUE (payment_id, wallet_id, direction)
);

CREATE INDEX wallet_activities_page_idx ON payment.wallet_activities (wallet_id, occurred_at DESC, id DESC);
