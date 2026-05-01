-- Seed an order for payment tests
INSERT INTO orders (id, user_id, restaurant_id, status, items_json, total_amount, delivery_fee, delivery_address, comment, idempotency_key)
VALUES
    ('0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d001',
     '0196ca5b-8fd3-7c09-b2f4-d4f3b6c8d001',
     '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901',
     'created',
     '[{"menu_item_id": "0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001", "quantity": 2}]',
     100000, 149,
     '{"address_text": "Test", "lat": 55.75, "lon": 37.62}',
     '',
     '0196ca5b-8fd3-7c09-b2f4-e4f3b6c8d001'),

    ('0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d002',
     '0196ca5b-8fd3-7c09-b2f4-d4f3b6c8d002',
     '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902',
     'created',
     '[{"menu_item_id": "0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d005", "quantity": 1}]',
     50000, 199,
     '{"address_text": "Test 2", "lat": 55.76, "lon": 37.63}',
     '',
     '0196ca5b-8fd3-7c09-b2f4-e4f3b6c8d002');

-- Seed a payment for read tests
INSERT INTO payments (id, order_id, status, amount, payment_method, payment_intent_id, idempotency_key)
VALUES
    ('0196ca5b-8fd3-7c09-b2f4-f4f3b6c8d001',
     '0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d001',
     'processing',
     100000,
     'card',
     'pi_existing_intent',
     '0196ca5b-8fd3-7c09-b2f4-f4f3b6c8e001');
