CREATE TABLE orders (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id          UUID NOT NULL,
    restaurant_id    UUID NOT NULL,
    status           VARCHAR(32) NOT NULL DEFAULT 'created',
    items_json       JSONB NOT NULL DEFAULT '[]',
    total_amount     INTEGER NOT NULL DEFAULT 0,
    delivery_fee     INTEGER NOT NULL DEFAULT 0,
    delivery_address JSONB NOT NULL DEFAULT '{}',
    comment          TEXT NOT NULL DEFAULT '',
    idempotency_key  UUID NOT NULL UNIQUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_restaurant_id ON orders (restaurant_id);
CREATE INDEX idx_orders_status ON orders (status) WHERE status NOT IN ('delivered', 'cancelled');

CREATE TABLE payments (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    order_id          UUID NOT NULL REFERENCES orders(id),
    status            VARCHAR(32) NOT NULL DEFAULT 'processing',
    amount            INTEGER NOT NULL DEFAULT 0,
    payment_method    VARCHAR(50) NOT NULL DEFAULT 'card',
    payment_intent_id VARCHAR(255) UNIQUE,
    idempotency_key   UUID NOT NULL UNIQUE,
    failure_reason    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_order_id ON payments (order_id);

CREATE TABLE outbox_events (
    id             BIGSERIAL PRIMARY KEY,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id   UUID NOT NULL,
    event_type     VARCHAR(100) NOT NULL,
    payload        JSONB NOT NULL DEFAULT '{}',
    published      BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_unpublished ON outbox_events (published, created_at) WHERE published = false;
