import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter } from 'k6/metrics'

const rateLimited = new Counter('rate_limited')

export const options = {
  scenarios: {
    read_mix: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 5),
      duration: __ENV.DURATION || '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.1'],
    http_req_duration: ['p(95)<750'],
  },
}

const API = __ENV.API_BASE || 'http://127.0.0.1:8080'
const TOKEN = __ENV.ACCESS_TOKEN || ''
const PID = __ENV.PROJECT_ID || ''

export default function () {
  const auth = TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {}

  let res = http.get(`${API}/healthz`)
  track(res, 'healthz')
  if (!TOKEN) {
    sleep(0.5)
    return
  }

  res = http.get(`${API}/v1/auth/me`, { headers: auth })
  track(res, 'auth me')

  res = http.get(`${API}/v1/projects`, { headers: auth })
  track(res, 'list projects')

  if (PID) {
    res = http.get(`${API}/v1/projects/${PID}`, { headers: auth })
    track(res, 'get project')
  }

  res = http.get(`${API}/readyz`, { headers: auth })
  track(res, 'readyz')

  // ~2 req/s per VU keeps 5 VUs under default RATE_LIMIT_RPS=20 when mixed.
  sleep(0.5)
}

function track(res, name) {
  if (res.status === 429) {
    rateLimited.add(1)
  }
  check(res, {
    [`${name} ok`]: (r) => r.status >= 200 && r.status < 300,
  })
}

