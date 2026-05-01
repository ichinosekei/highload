import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Rate, Trend, Counter } from 'k6/metrics';

// --- Custom Metrics ---
const errorRate = new Rate('error_rate');
const searchLatency = new Trend('search_latency', true);
const orderCreateLatency = new Trend('order_create_latency', true);
const paymentLatency = new Trend('payment_latency', true);
const trackLatency = new Trend('track_latency', true);
const ordersCreated = new Counter('orders_created');

// --- Configuration ---
const BASE_URL = __ENV.BASE_URL || 'http://localhost';
const PROFILE = __ENV.PROFILE || 'load';
const RESTAURANT_ID = __ENV.RESTAURANT_ID || '0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901';
const MENU_ITEM_ID = __ENV.MENU_ITEM_ID || '0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001';

// --- Profiles & Scenarios ---
const profiles = {
    smoke: {
        scenarios: {
            search_traffic: {
                executor: 'ramping-arrival-rate',
                startRate: 5,
                timeUnit: '1s',
                preAllocatedVUs: 10,
                maxVUs: 50,
                stages: [
                    { duration: '30s', target: 20 },
                    { duration: '30s', target: 20 },
                    { duration: '10s', target: 0 },
                ],
                exec: 'searchScenario',
            },
            order_traffic: {
                executor: 'ramping-arrival-rate',
                startRate: 2,
                timeUnit: '1s',
                preAllocatedVUs: 5,
                maxVUs: 20,
                stages: [
                    { duration: '30s', target: 5 },
                    { duration: '30s', target: 5 },
                    { duration: '10s', target: 0 },
                ],
                exec: 'orderScenario',
            },
        },
        thresholds: {
            'error_rate': ['rate<0.01'],
            'http_req_failed': ['rate<0.01'],
        },
    },
    load: {
        scenarios: {
            search_traffic: {
                executor: 'ramping-arrival-rate',
                startRate: 10,
                timeUnit: '1s',
                preAllocatedVUs: 50,
                maxVUs: 200,
                stages: [
                    { duration: '1m', target: 50 },  // Warmup
                    { duration: '5m', target: 100 }, // Sustained target (NFR: 100 RPS)
                    { duration: '1m', target: 0 },   // Cool down
                ],
                exec: 'searchScenario',
            },
            order_traffic: {
                executor: 'ramping-arrival-rate',
                startRate: 5,
                timeUnit: '1s',
                preAllocatedVUs: 20,
                maxVUs: 100,
                stages: [
                    { duration: '1m', target: 15 }, // Warmup
                    { duration: '5m', target: 30 }, // Sustained target (NFR: 30 RPS)
                    { duration: '1m', target: 0 },  // Cool down
                ],
                exec: 'orderScenario',
            },
        },
        thresholds: {
            'error_rate': ['rate<0.01'], // NFR: < 1% error rate
            'search_latency': ['p(99)<500'], // NFR: p99 < 500ms
            'order_create_latency': ['p(99)<1000'], // NFR: p99 < 1s
            'http_req_failed': ['rate<0.01'],
        },
    }
};

export const options = profiles[PROFILE] || profiles.load;

// --- Helper: Robust Assertion ---
function checkRes(res, name) {
    const success = check(res, {
        [`${name} status is 2xx`]: (r) => r.status >= 200 && r.status < 300,
    });
    if (!success) {
        errorRate.add(1);
        console.warn(`${name} failed! Status: ${res.status}, Body: ${res.body}`);
    } else {
        errorRate.add(0);
    }
    return success;
}

// --- Scenario 1: Search & Catalog (READ) ---
export function searchScenario() {
    group('Search Flow', () => {
        // 1. Search with randomized queries to bypass simple caches
        const queries = ['pizza', 'burger', 'sushi', 'pasta', 'taco'];
        const query = queries[Math.floor(Math.random() * queries.length)];
        
        const res = http.get(`${BASE_URL}/api/v1/search?q=${query}&limit=10`);
        searchLatency.add(res.timings.duration);
        checkRes(res, 'search');

        sleep(Math.random() * 0.5);

        // 2. View restaurant menu
        const menuRes = http.get(`${BASE_URL}/api/v1/catalog/restaurants/${RESTAURANT_ID}/menu`);
        checkRes(menuRes, 'view_menu');
    });
}

// --- Scenario 2: Full Order Flow (WRITE) ---
export function orderScenario() {
    const idempotencyKey = uuidv4();
    let orderId = '';

    group('Checkout Flow', () => {
        // 1. Create Order
        const orderPayload = JSON.stringify({
            restaurant_id: RESTAURANT_ID,
            items: [
                { menu_item_id: MENU_ITEM_ID, quantity: Math.floor(Math.random() * 2) + 1 }
            ],
            delivery_address: {
                address_text: 'ул. Тестовая, д. ' + Math.floor(Math.random() * 100),
                lat: 55.75 + Math.random() * 0.01,
                lon: 37.62 + Math.random() * 0.01,
            },
            comment: 'Load test order - ' + idempotencyKey.substring(0, 8)
        });

        const res = http.post(`${BASE_URL}/api/v1/orders`, orderPayload, {
            headers: { 
                'Content-Type': 'application/json',
                'Idempotency-Key': idempotencyKey
            },
        });
        
        orderCreateLatency.add(res.timings.duration);
        if (checkRes(res, 'create_order')) {
            const body = JSON.parse(res.body);
            orderId = body.id || body.order_id;
            ordersCreated.add(1);
        } else {
            return; // Stop flow if order failed
        }

        sleep(1);

        // 2. Track Order
        const trackRes = http.get(`${BASE_URL}/api/v1/orders/${orderId}/track`);
        trackLatency.add(trackRes.timings.duration);
        checkRes(trackRes, 'track_order');

        sleep(0.5);

        // 3. Initiate Payment
        const paymentPayload = JSON.stringify({
            order_id: orderId,
            payment_method: 'card',
            return_url: 'https://app.example.com/callback'
        });

        const payRes = http.post(`${BASE_URL}/api/v1/payments`, paymentPayload, {
            headers: { 
                'Content-Type': 'application/json',
                'Idempotency-Key': uuidv4()
            },
        });
        paymentLatency.add(payRes.timings.duration);
        checkRes(payRes, 'init_payment');
    });
}
