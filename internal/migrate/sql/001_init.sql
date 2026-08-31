CREATE TABLE IF NOT EXISTS products (
    sku        VARCHAR(64) PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    type       VARCHAR(32) NOT NULL,
    price      INTEGER NOT NULL CHECK (price >= 0),
    currency   VARCHAR(8) NOT NULL DEFAULT 'RUB',
    image      VARCHAR(255) NOT NULL DEFAULT '',
    stock      INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_storefront
    ON products (type, sku)
    INCLUDE (name, price, currency, image, stock)
    WHERE stock > 0;

CREATE TABLE IF NOT EXISTS inventory_keys (
    id                      BIGSERIAL PRIMARY KEY,
    code                    VARCHAR(64) NOT NULL UNIQUE,
    sku                     VARCHAR(64) NOT NULL REFERENCES products (sku),
    reserved_by_request_id  VARCHAR(128) UNIQUE,
    issued_to_order_id      VARCHAR(64),
    provider                VARCHAR(16),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inventory_available
    ON inventory_keys (sku, id)
    WHERE reserved_by_request_id IS NULL;

CREATE TABLE IF NOT EXISTS orders (
    id                   VARCHAR(64) PRIMARY KEY,
    sku                  VARCHAR(64) NOT NULL REFERENCES products (sku),
    amount               INTEGER NOT NULL,
    currency             VARCHAR(8) NOT NULL,
    status               VARCHAR(32) NOT NULL,
    delivery_code        VARCHAR(64),
    delivery_request_id  VARCHAR(128),
    delivery_provider    VARCHAR(16),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_status_updated
    ON orders (status, updated_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_delivery_code
    ON orders (delivery_code)
    WHERE delivery_code IS NOT NULL;

CREATE TABLE IF NOT EXISTS payment_events (
    event_id     VARCHAR(128) PRIMARY KEY,
    order_id     VARCHAR(64) NOT NULL,
    status       VARCHAR(32) NOT NULL,
    amount       INTEGER NOT NULL,
    currency     VARCHAR(8) NOT NULL,
    event_time   TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_events_order
    ON payment_events (order_id, created_at);

CREATE INDEX IF NOT EXISTS idx_payment_events_pending
    ON payment_events (order_id)
    WHERE processed_at IS NULL;

CREATE TABLE IF NOT EXISTS delivery_attempts (
    id         BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL,
    order_id   VARCHAR(64) NOT NULL,
    provider   VARCHAR(16) NOT NULL,
    sku        VARCHAR(64) NOT NULL,
    status     VARCHAR(32) NOT NULL,
    code       VARCHAR(64),
    reason     VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_order
    ON delivery_attempts (order_id, created_at);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_request
    ON delivery_attempts (request_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id         BIGSERIAL PRIMARY KEY,
    order_id   VARCHAR(64) NOT NULL,
    event_id   VARCHAR(128),
    debit      INTEGER NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit     INTEGER NOT NULL DEFAULT 0 CHECK (credit >= 0),
    account    VARCHAR(32) NOT NULL,
    entry_type VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ledger_payment
    ON ledger_entries (event_id, account)
    WHERE event_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ledger_settlement
    ON ledger_entries (order_id, account, entry_type)
    WHERE entry_type = 'settlement';

CREATE INDEX IF NOT EXISTS idx_ledger_order ON ledger_entries (order_id);
