-- Create separate logical databases.
CREATE DATABASE catalog_db;
CREATE DATABASE orders_db;

-- Connect to catalog_db and create schema.
\connect catalog_db;

CREATE TABLE restaurants (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    name             VARCHAR(255) NOT NULL,
    cuisine          TEXT[] NOT NULL DEFAULT '{}',
    rating           NUMERIC(2,1) NOT NULL DEFAULT 0.0,
    delivery_time_min INTEGER NOT NULL DEFAULT 30,
    delivery_fee     INTEGER NOT NULL DEFAULT 0,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    address          JSONB NOT NULL DEFAULT '{}',
    image_url        TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes per architecture doc section 5.2.
CREATE INDEX idx_restaurants_cuisine ON restaurants USING GIN (cuisine);
CREATE INDEX idx_restaurants_is_active ON restaurants (is_active) WHERE is_active = true;
CREATE INDEX idx_restaurants_rating ON restaurants (rating DESC);

CREATE TABLE menu_items (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    restaurant_id   UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    price           INTEGER NOT NULL,
    category        VARCHAR(100) NOT NULL DEFAULT '',
    is_available    BOOLEAN NOT NULL DEFAULT true,
    image_urls      TEXT[] NOT NULL DEFAULT '{}',
    options         JSONB NOT NULL DEFAULT '[]'
);

-- Primary access pattern: get menu for a restaurant.
CREATE INDEX idx_menu_items_restaurant_id ON menu_items (restaurant_id);
CREATE INDEX idx_menu_items_available ON menu_items (restaurant_id, is_available) WHERE is_available = true;

-- Seed data for development and testing.
INSERT INTO restaurants (id, name, cuisine, rating, delivery_time_min, delivery_fee, is_active, address, image_url)
VALUES
    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901', 'Pizza House', ARRAY['italian', 'fast_food'], 4.7, 30, 149, true,
     '{"address_text": "ул. Пушкина, д. 1", "lat": 55.751, "lon": 37.618}',
     'https://cdn.example.com/restaurants/pizza-house/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902', 'Sushi Master', ARRAY['japanese'], 4.5, 45, 199, true,
     '{"address_text": "ул. Лермонтова, д. 5", "lat": 55.753, "lon": 37.621}',
     'https://cdn.example.com/restaurants/sushi-master/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d903', 'Burger King', ARRAY['fast_food', 'american'], 4.2, 20, 99, true,
     '{"address_text": "ул. Тверская, д. 10", "lat": 55.761, "lon": 37.610}',
     'https://cdn.example.com/restaurants/burger-king/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d904', 'Closed Restaurant', ARRAY['russian'], 3.0, 60, 299, false,
     '{"address_text": "ул. Старая, д. 100", "lat": 55.700, "lon": 37.500}',
     ''),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d905', 'Wok & Roll', ARRAY['chinese', 'asian'], 4.8, 35, 150, true,
     '{"address_text": "пр. Мира, д. 25", "lat": 55.782, "lon": 37.632}',
     'https://cdn.example.com/restaurants/wok-roll/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d906', 'Taco Loco', ARRAY['mexican'], 4.4, 25, 120, true,
     '{"address_text": "ул. Арбат, д. 12", "lat": 55.751, "lon": 37.599}',
     'https://cdn.example.com/restaurants/taco-loco/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d907', 'Le Petit Cafe', ARRAY['french', 'bakery'], 4.9, 15, 250, true,
     '{"address_text": "ул. Петровка, д. 3", "lat": 55.760, "lon": 37.617}',
     'https://cdn.example.com/restaurants/le-petit-cafe/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d908', 'Steak House', ARRAY['american', 'grill'], 4.6, 50, 300, true,
     '{"address_text": "Кутузовский пр-т, д. 15", "lat": 55.747, "lon": 37.552}',
     'https://cdn.example.com/restaurants/steak-house/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d909', 'Healthy Greens', ARRAY['vegan', 'healthy'], 4.3, 40, 0, true,
     '{"address_text": "ул. Остоженка, д. 20", "lat": 55.739, "lon": 37.595}',
     'https://cdn.example.com/restaurants/healthy-greens/cover.jpg'),

    ('0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d910', 'Mama Italia', ARRAY['italian'], 4.7, 45, 180, true,
     '{"address_text": "ул. Мясницкая, д. 7", "lat": 55.761, "lon": 37.629}',
     'https://cdn.example.com/restaurants/mama-italia/cover.jpg');

INSERT INTO menu_items (id, restaurant_id, name, description, price, category, is_available, image_urls, options)
VALUES
    -- Pizza House menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Маргарита', 'Классическая пицца с томатным соусом и моцареллой', 49900, 'pizza', true,
     ARRAY['https://cdn.example.com/items/margherita.jpg'],
     '[{"name": "extra_cheese", "price": 5000}, {"name": "double_size", "price": 15000}]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d002', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Пепперони', 'Пицца с острой пепперони и моцареллой', 59900, 'pizza', true,
     ARRAY['https://cdn.example.com/items/pepperoni.jpg'],
     '[{"name": "extra_cheese", "price": 5000}]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d003', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Кока-кола 0.5л', 'Прохладительный напиток', 9900, 'drinks', true,
     ARRAY['https://cdn.example.com/items/coca-cola.jpg'],
     '[]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d004', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Тирамису', 'Итальянский десерт', 34900, 'desserts', false,
     ARRAY['https://cdn.example.com/items/tiramisu.jpg'],
     '[]'),

    -- Sushi Master menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d005', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902',
     'Филадельфия', 'Ролл с лососем, сливочным сыром и авокадо', 69900, 'rolls', true,
     ARRAY['https://cdn.example.com/items/philadelphia.jpg'],
     '[]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d006', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902',
     'Калифорния', 'Ролл с крабом и огурцом', 54900, 'rolls', true,
     ARRAY['https://cdn.example.com/items/california.jpg'],
     '[]'),

    -- Burger King menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d007', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d903',
     'Воппер', 'Классический бургер с говядиной', 29900, 'burgers', true,
     ARRAY['https://cdn.example.com/items/whopper.jpg'],
     '[{"name": "extra_patty", "price": 10000}]'),

    -- Wok & Roll menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d009', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d905',
     'Лапша яичная с курицей', 'Wok с овощами и соусом терияки', 45000, 'noodles', true,
     ARRAY['https://cdn.example.com/items/wok-chicken.jpg'],
     '[]'),

    -- Taco Loco menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d010', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d906',
     'Тако с говядиной', '3 штуки в порции', 38000, 'tacos', true,
     ARRAY['https://cdn.example.com/items/taco-beef.jpg'],
     '[{"name": "extra_salsa", "price": 3000}]'),

    -- Le Petit Cafe menu
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d011', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d907',
     'Круассан классический', 'На сливочном масле', 15000, 'bakery', true,
     ARRAY['https://cdn.example.com/items/croissant.jpg'],
     '[]');

-- ============================================================
-- orders_db: Orders, Payments, Outbox
-- ============================================================
\connect orders_db;

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

