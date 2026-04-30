import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Rate, Trend } from 'k6/metrics';

// Write-only scenario: order creation + payment.
// Validates: ≥30 RPS at p99 <1s.

const errorRate = new Rate('error_rate');
const orderLatency = new Trend('order_create_latency', true);
const paymentLatency = new Trend('payment_create_latency', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost';
const ORDER_URL = `${BASE_URL}:8082`;
const PAYMENT_URL = `${BASE_URL}:8083`;

const RESTAURANT_ID = __ENV.RESTAURANT_ID || '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901';
const MENU_ITEM_ID = __ENV.MENU_ITEM_ID || '0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001';

const profile = __ENV.PROFILE || 'load';
const profiles = {
  smoke: {
    stages: [
      { duration: '30s', target: 5 },
      { duration: '1m', target: 5 },
      { duration: '10s', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      order_create_latency: ['p(99)<1000'],
    },
  },
  load: {
    stages: [
      { duration: '1m', target: 15 },
      { duration: '5m', target: 30 },
      { duration: '5m', target: 30 },
      { duration: '1m', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      order_create_latency: ['p(99)<1000'],
      payment_create_latency: ['p(99)<2500'],
    },
  },
  stress: {
    stages: [
      { duration: '1m', target: 20 },
      { duration: '2m', target: 50 },
      { duration: '2m', target: 80 },
      { duration: '2m', target: 120 },
      { duration: '2m', target: 160 },
      { duration: '1m', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.05'],
    },
  },
};

export const options = profiles[profile];

export default function () {
  const idempotencyKey = uuidv4();

  // 1. Create order.
  const orderPayload = JSON.stringify({
    restaurant_id: RESTAURANT_ID,
    items: [{ menu_item_id: MENU_ITEM_ID, quantity: 1 }],
    delivery_address: {
      address_text: 'Тестовая ' + Math.floor(Math.random() * 1000),
      lat: 55.75,
      lon: 37.62,
    },
    comment: '',
  });

  const orderRes = http.post(`${ORDER_URL}/api/v1/orders`, orderPayload, {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
    },
  });
  orderLatency.add(orderRes.timings.duration);
  const orderOk = check(orderRes, {
    'create order: status 201': (r) => r.status === 201,
  });
  errorRate.add(!orderOk);

  if (!orderOk) return;

  const orderId = JSON.parse(orderRes.body).id;
  sleep(0.1);

  // 2. Create payment.
  const paymentRes = http.post(
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
  paymentLatency.add(paymentRes.timings.duration);
  const payOk = check(paymentRes, {
    'create payment: status 201': (r) => r.status === 201,
  });
  errorRate.add(!payOk);

  sleep(0.2);
}
