// k6 压测脚本 · Ripple 后端基线
//
// 运行：
//   k6 run -e BASE=http://localhost:8000 -e TOKEN=xxx scripts/loadtest/k6-baseline.js
//
// 场景：登录后混合调用 GET /healthz / GET /api/v1/lakes / GET /metrics
// 目标：50 并发持续 1 分钟，p95 < 200ms，错误率 < 1%。

import http from 'k6/http'
import { check, sleep } from 'k6'

export const options = {
  scenarios: {
    mixed: {
      executor: 'ramping-vus',
      stages: [
        { duration: '15s', target: 10 },
        { duration: '30s', target: 50 },
        { duration: '15s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],
  },
}

const BASE = __ENV.BASE || 'http://localhost:8000'
const TOKEN = __ENV.TOKEN || ''

export default function () {
  // 1. health
  const h = http.get(`${BASE}/healthz`)
  check(h, { 'health 200': (r) => r.status === 200 })

  // 2. metrics（无鉴权但需开启）
  const m = http.get(`${BASE}/metrics`)
  check(m, { 'metrics ok': (r) => r.status === 200 || r.status === 404 })

  // 3. 登录后端点（如果 TOKEN 提供）
  if (TOKEN) {
    const headers = { Authorization: `Bearer ${TOKEN}` }
    const lakes = http.get(`${BASE}/api/v1/lakes`, { headers })
    check(lakes, { 'lakes 200': (r) => r.status === 200 })
  }

  sleep(0.5)
}
