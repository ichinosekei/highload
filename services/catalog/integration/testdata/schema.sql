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

CREATE INDEX idx_menu_items_restaurant_id ON menu_items (restaurant_id);
CREATE INDEX idx_menu_items_available ON menu_items (restaurant_id, is_available) WHERE is_available = true;
