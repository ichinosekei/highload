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
     '');

INSERT INTO menu_items (id, restaurant_id, name, description, price, category, is_available, image_urls, options)
VALUES
    -- Pizza House menu (3 items: 2 available, 1 unavailable)
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Маргарита', 'Классическая пицца с томатным соусом и моцареллой', 49900, 'pizza', true,
     ARRAY['https://cdn.example.com/items/margherita.jpg'],
     '[{"name": "extra_cheese", "price": 5000}, {"name": "double_size", "price": 15000}]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d002', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Пепперони', 'Пицца с острой пепперони и моцареллой', 59900, 'pizza', true,
     ARRAY['https://cdn.example.com/items/pepperoni.jpg'],
     '[]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d003', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'Тирамису', 'Итальянский десерт', 34900, 'desserts', false,
     ARRAY['https://cdn.example.com/items/tiramisu.jpg'],
     '[]'),

    -- Sushi Master menu (2 items, both available)
    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d005', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902',
     'Филадельфия', 'Ролл с лососем, сливочным сыром и авокадо', 69900, 'rolls', true,
     ARRAY['https://cdn.example.com/items/philadelphia.jpg'],
     '[]'),

    ('0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d006', '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902',
     'Калифорния', 'Ролл с крабом и огурцом', 54900, 'rolls', true,
     ARRAY['https://cdn.example.com/items/california.jpg'],
     '[]');
