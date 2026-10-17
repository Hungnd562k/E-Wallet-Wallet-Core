-- ledger: double-entry accounting (erd §3, §4.4). The only module that changes balances.
CREATE SCHEMA IF NOT EXISTS ledger;

CREATE TABLE ledger.accounts (
  id             UUID PRIMARY KEY,
  code           TEXT NOT NULL UNIQUE, -- SAFEGUARD_BANK, WALLET:<uuid>, ...
  type           TEXT NOT NULL CHECK (type IN ('ASSET', 'LIABILITY', 'REVENUE', 'EXPENSE')),
  normal_side    CHAR(1) NOT NULL CHECK (normal_side IN ('D', 'C')),
  owner_type     TEXT NOT NULL CHECK (owner_type IN ('SYSTEM', 'WALLET', 'MERCHANT')),
  owner_id       UUID NULL, -- ref wallet.wallets / merchant.merchants
  currency       CHAR(3) NOT NULL DEFAULT 'VND',
  balance        BIGINT NOT NULL DEFAULT 0,
  allow_negative BOOLEAN NOT NULL DEFAULT false,
  status         TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED')),
  version        INT NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- Last line of defence for BR-24: a balance can never go negative.
  CONSTRAINT accounts_non_negative CHECK (allow_negative OR balance >= 0)
);

CREATE TRIGGER accounts_updated_at BEFORE UPDATE ON ledger.accounts
  FOR EACH ROW EXECUTE FUNCTION platform.set_updated_at();

CREATE TABLE ledger.transactions (
  id             UUID PRIMARY KEY,
  kind           TEXT NOT NULL CHECK (kind IN ('TOPUP', 'TRANSFER', 'HOLD', 'HOLD_CAPTURE', 'HOLD_RELEASE',
                                               'QR_PAYMENT', 'REFUND', 'FEE', 'ADJUSTMENT', 'REVERSAL')),
  payment_id     UUID NULL, -- ref payment.payments
  adjustment_id  UUID NULL, -- ref recon.adjustments
  reverses_id    UUID NULL UNIQUE REFERENCES ledger.transactions (id),
  description    TEXT NOT NULL DEFAULT '',
  effective_date DATE NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger.entries (
  seq            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  transaction_id UUID NOT NULL REFERENCES ledger.transactions (id),
  account_id     UUID NOT NULL REFERENCES ledger.accounts (id),
  direction      CHAR(1) NOT NULL CHECK (direction IN ('D', 'C')),
  amount         BIGINT NOT NULL CHECK (amount > 0),
  currency       CHAR(3) NOT NULL,
  balance_after  BIGINT NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX entries_account_seq_idx ON ledger.entries (account_id, seq DESC);
CREATE INDEX entries_transaction_idx ON ledger.entries (transaction_id);

CREATE TABLE ledger.holds (
  id            UUID PRIMARY KEY,
  account_id    UUID NOT NULL REFERENCES ledger.accounts (id),
  payment_id    UUID NOT NULL UNIQUE, -- ref payment.payments
  amount        BIGINT NOT NULL CHECK (amount > 0),
  status        TEXT NOT NULL CHECK (status IN ('ACTIVE', 'CAPTURED', 'RELEASED')),
  hold_txn_id   UUID NOT NULL REFERENCES ledger.transactions (id),
  settle_txn_id UUID NULL REFERENCES ledger.transactions (id),
  version       INT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX holds_active_idx ON ledger.holds (account_id) WHERE status = 'ACTIVE';

CREATE TRIGGER holds_updated_at BEFORE UPDATE ON ledger.holds
  FOR EACH ROW EXECUTE FUNCTION platform.set_updated_at();

-- 1. Append-only.
CREATE TRIGGER entries_immutable BEFORE UPDATE OR DELETE ON ledger.entries
  FOR EACH ROW EXECUTE FUNCTION platform.forbid_mutation();
CREATE TRIGGER transactions_immutable BEFORE UPDATE OR DELETE ON ledger.transactions
  FOR EACH ROW EXECUTE FUNCTION platform.forbid_mutation();

-- 2. Σdebit = Σcredit per transaction, checked at COMMIT.
CREATE FUNCTION ledger.check_txn_balanced() RETURNS trigger AS $$
DECLARE d BIGINT; c BIGINT;
BEGIN
  SELECT COALESCE(SUM(amount) FILTER (WHERE direction = 'D'), 0),
         COALESCE(SUM(amount) FILTER (WHERE direction = 'C'), 0)
    INTO d, c
    FROM ledger.entries
   WHERE transaction_id = NEW.transaction_id;
  IF d <> c THEN
    RAISE EXCEPTION 'ledger transaction % unbalanced: D=% C=%', NEW.transaction_id, d, c;
  END IF;
  RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER entries_balanced
  AFTER INSERT ON ledger.entries
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION ledger.check_txn_balanced();

-- 3. Entry currency must match its account.
CREATE FUNCTION ledger.check_entry_currency() RETURNS trigger AS $$
BEGIN
  IF NEW.currency <> (SELECT currency FROM ledger.accounts WHERE id = NEW.account_id) THEN
    RAISE EXCEPTION 'currency mismatch for account %', NEW.account_id;
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER entries_currency BEFORE INSERT ON ledger.entries
  FOR EACH ROW EXECUTE FUNCTION ledger.check_entry_currency();

-- System accounts (erd §3.1). Per-wallet / per-merchant accounts are created by the app.
INSERT INTO ledger.accounts (id, code, type, normal_side, owner_type, allow_negative) VALUES
  ('00000000-0000-7000-8000-000000000001', 'SAFEGUARD_BANK', 'ASSET',     'D', 'SYSTEM', false),
  ('00000000-0000-7000-8000-000000000002', 'WALLET_HOLD',    'LIABILITY', 'C', 'SYSTEM', false),
  ('00000000-0000-7000-8000-000000000003', 'FEE_REVENUE',    'REVENUE',   'C', 'SYSTEM', false),
  ('00000000-0000-7000-8000-000000000004', 'RECON_SUSPENSE', 'LIABILITY', 'C', 'SYSTEM', true);
