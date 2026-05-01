import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Rate, Trend } from 'k6/metrics';

// Read-only scenario: catalog search + restaurant listing.
// Validates: ≥100 RPS at p99 <500ms.

const errorRate = new Rate('error_rate');
const searchLatency = new Trend('search_latency', true);
const listLatency = new Trend('list_latency', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost';
const CATALOG_URL = `${BASE_URL}:8080`;

const profile = __ENV.PROFILE || 'load';
const profiles = {
  smoke: {
    stages: [
      { duration: '30s', target: 10 },
      { duration: '1m', target: 10 },
      { duration: '10s', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      search_latency: ['p(99)<500'],
    },
  },
  load: {
    stages: [
      { duration: '1m', target: 50 },
      { duration: '5m', target: 100 },
      { duration: '2m', target: 150 },
      { duration: '5m', target: 150 },
      { duration: '1m', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.01'],
      search_latency: ['p(99)<500'],
      list_latency: ['p(99)<500'],
    },
  },
  stress: {
    stages: [
      { duration: '1m', target: 100 },
      { duration: '2m', target: 200 },
      { duration: '2m', target: 300 },
      { duration: '2m', target: 400 },
      { duration: '2m', target: 500 },
      { duration: '1m', target: 0 },
    ],
    thresholds: {
      http_req_failed: ['rate<0.05'],
    },
  },
};

export const options = profiles[profile];

const queries = ['pizza', 'burger', 'sushi', 'pasta', 'salad', 'coffee', 'dessert'];

export default function () {
  // Search
  const query = queries[Math.floor(Math.random() * queries.length)];
  const searchRes = http.get(`${CATALOG_URL}/api/v1/search?q=${query}&limit=20`);
  searchLatency.add(searchRes.timings.duration);
  check(searchRes, { 'search: status 200': (r) => r.status === 200 });
  errorRate.add(searchRes.status !== 200);

  sleep(0.05);

  // List restaurants
  const listRes = http.get(`${CATALOG_URL}/api/v1/restaurants?limit=20&offset=0`);
  listLatency.add(listRes.timings.duration);
  check(listRes, { 'list: status 200': (r) => r.status === 200 });
  errorRate.add(listRes.status !== 200);

  sleep(0.05);
}
