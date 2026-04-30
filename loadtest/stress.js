import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('error_rate');
const orderLatency = new Trend('order_create_latency', true);
const paymentLatency = new Trend('payment_create_latency', true);
const trackLatency = new Trend('track_order_latency', true);
const statusLatency = new Trend('status_update_latency', true);
const searchLatency = new Trend('search_latency', true);
const ordersCreated = new Counter('orders_created');

// Configuration — override with env vars or CLI flags.
const BASE_URL = __ENV.BASE_URL || 'http://localhost';
const CATALOG_URL = `${BASE_URL}:8080`;
const ORDER_URL = `${BASE_URL}:8082`;
const PAYMENT_URL = `${BASE_URL}:8083`;

// Restaurant and menu item IDs — set after seeding.
const RESTAURANT_ID = __ENV.RESTAURANT_ID || '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901';
const MENU_ITEM_ID = __ENV.MENU_ITEM_ID || '0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001';

// Exported options — different test profiles.
// Use: k6 run --env PROFILE=smoke loadtest/stress.js
const profiles = {
  // Smoke test: quick verification.
  smoke: {
    stages: [
      { duration: '30s', target: 5 },
      { duration: '1m', target: 5 },
      { duration: '10s', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      http_req_duration: ['p(99)<2000'],
    },
  },
  // Load test: sustained target RPS from TASK.md.
  load: {
    stages: [
      { duration: '1m', target: 30 },    // Ramp up
      { duration: '5m', target: 30 },    // Sustained (write ≥30 RPS)
      { duration: '1m', target: 100 },   // Read-heavy
      { duration: '5m', target: 100 },   // Sustained (read ≥100 RPS)
      { duration: '1m', target: 0 },     // Ramp down
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      order_create_latency: ['p(99)<1000'],
      track_order_latency: ['p(99)<500'],
      search_latency: ['p(99)<500'],
    },
  },
  // Stress test: push to breaking point.
  stress: {
    stages: [
      { duration: '1m', target: 20 },
      { duration: '2m', target: 50 },
      { duration: '2m', target: 100 },
      { duration: '2m', target: 150 },
      { duration: '2m', target: 200 },
      { duration: '2m', target: 250 },
      { duration: '1m', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.05'],
    },
  },
  // Spike test: sudden 2x burst.
  spike: {
    stages: [
      { duration: '1m', target: 30 },    // Warm up
      { duration: '3m', target: 30 },    // Baseline
      { duration: '10s', target: 60 },   // 2x spike
      { duration: '2m', target: 60 },    // Sustained spike
      { duration: '10s', target: 30 },   // Return to baseline
      { duration: '2m', target: 30 },    // Recovery period
      { duration: '30s', target: 0 },    // Ramp down
    ],
    thresholds: {
      http_req_failed: ['rate<0.05'],
    },
  },
};

const profile = __ENV.PROFILE || 'smoke';
export const options = profiles[profile];

// --- Scenario: Full Order Flow ---
export default function () {
  const idempotencyKey = uuidv4();

  // 1. Search/catalog (read-heavy operation).
  group('search', () => {
    const searchRes = http.get(`${CATALOG_URL}/api/v1/search?q=pizza&limit=10`);
    searchLatency.add(searchRes.timings.duration);
    check(searchRes, {
      'search: status 200': (r) => r.status === 200,
    });
    errorRate.add(searchRes.status !== 200);
  });

  sleep(0.1);

  // 2. Create order (write operation).
  let orderId = '';
  group('create_order', () => {
    const payload = JSON.stringify({
      restaurant_id: RESTAURANT_ID,
      items: [
        { menu_item_id: MENU_ITEM_ID, quantity: Math.floor(Math.random() * 3) + 1 },
      ],
      delivery_address: {
        address_text: 'ул. Тестовая, д. ' + Math.floor(Math.random() * 100),
        lat: 55.75 + Math.random() * 0.1,
        lon: 37.62 + Math.random() * 0.1,
      },
      comment: 'Load test order',
    });

    const res = http.post(`${ORDER_URL}/api/v1/orders`, payload, {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': idempotencyKey,
      },
    });

    orderLatency.add(res.timings.duration);
    const ok = check(res, {
      'create order: status 201': (r) => r.status === 201,
    });
    errorRate.add(!ok);

    if (ok) {
      const body = JSON.parse(res.body);
      orderId = body.id;
      ordersCreated.add(1);
    }
  });

  if (!orderId) return;
  sleep(0.1);

  // 3. Track order (read operation).
  group('track_order', () => {
    const res = http.get(`${ORDER_URL}/api/v1/orders/${orderId}/track`);
    trackLatency.add(res.timings.duration);
    const ok = check(res, {
      'track order: status 200': (r) => r.status === 200,
    });
    errorRate.add(!ok);
  });

  sleep(0.1);

  // 4. Update order status (write operation).
  group('update_status', () => {
    const res = http.post(
      `${ORDER_URL}/api/v1/orders/${orderId}/status`,
      JSON.stringify({ status: 'cooking' }),
      { headers: { 'Content-Type': 'application/json' } },
    );
    statusLatency.add(res.timings.duration);
    const ok = check(res, {
      'update status: status 200': (r) => r.status === 200,
    });
    errorRate.add(!ok);
  });

  sleep(0.1);

  // 5. Create payment (write operation).
  group('create_payment', () => {
    const res = http.post(
      `${PAYMENT_URL}/api/v1/payments`,
      JSON.stringify({
        order_id: orderId,
        payment_method: 'card',
        return_url: 'https://app.example.com/callback',
      }),
      {
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': uuidv4(),
        },
      },
    );
    paymentLatency.add(res.timings.duration);
    const ok = check(res, {
      'create payment: status 201': (r) => r.status === 201,
    });
    errorRate.add(!ok);
  });

  sleep(0.3);
}
