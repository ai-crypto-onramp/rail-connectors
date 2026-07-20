CREATE TABLE IF NOT EXISTS rail_requests (
    payment_id      TEXT        PRIMARY KEY,
    rail            TEXT        NOT NULL,
    operation       TEXT        NOT NULL DEFAULT '',
    amount          DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency        TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL,
    idempotency_key TEXT        NOT NULL DEFAULT '',
    rail_ref        TEXT        NOT NULL DEFAULT '',
    error_code      TEXT        NOT NULL DEFAULT '',
    error_message   TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rail_requests_status_idx ON rail_requests (status);
CREATE INDEX IF NOT EXISTS rail_requests_rail_idx   ON rail_requests (rail);

CREATE TABLE IF NOT EXISTS rail_settlements (
    settle_id   TEXT        PRIMARY KEY,
    rail        TEXT        NOT NULL,
    payment_id  TEXT        NOT NULL,
    amount      DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency    TEXT        NOT NULL DEFAULT '',
    settled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_ref  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rail_settlements_payment_id_idx ON rail_settlements (payment_id);

CREATE TABLE IF NOT EXISTS rail_chargebacks (
    chargeback_id TEXT        PRIMARY KEY,
    rail          TEXT        NOT NULL,
    payment_id    TEXT        NOT NULL,
    amount        DOUBLE PRECISION NOT NULL DEFAULT 0,
    reason_code   TEXT        NOT NULL DEFAULT '',
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    status        TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rail_chargebacks_payment_id_idx ON rail_chargebacks (payment_id);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key        TEXT        PRIMARY KEY,
    response   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);