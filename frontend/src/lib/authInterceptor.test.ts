import test from 'node:test'
import assert from 'node:assert/strict'
import axios, {
  AxiosError,
  type AxiosAdapter,
  type AxiosInstance,
  type InternalAxiosRequestConfig,
} from 'axios'

import {
  createSessionRefresher,
  installAuthInterceptors,
  SessionExpiredError,
  type AuthInterceptorDeps,
  type RefreshedTokens,
} from './authInterceptor.ts'

interface SessionState {
  access: string | null
  refresh: string | null
  cleared: number
  expired: number
  refreshed: number
  saved: RefreshedTokens[]
}

interface Harness {
  state: SessionState
  deps: AuthInterceptorDeps
  setRefresh(fn: (token: string) => Promise<RefreshedTokens>): void
}

function harness(options: {
  access?: string | null
  refresh?: string | null
  requestRefresh?: (token: string) => Promise<RefreshedTokens>
} = {}): Harness {
  const state: SessionState = {
    access: options.access === undefined ? 'access-1' : options.access,
    refresh: options.refresh === undefined ? 'refresh-1' : options.refresh,
    cleared: 0,
    expired: 0,
    refreshed: 0,
    saved: [],
  }
  let refreshImpl: (token: string) => Promise<RefreshedTokens> =
    options.requestRefresh ?? (async () => ({ accessToken: 'access-2', refreshToken: 'refresh-2' }))

  const deps: AuthInterceptorDeps = {
    getToken: () => state.access,
    getRefreshToken: () => state.refresh,
    saveTokens: (tokens) => {
      state.access = tokens.accessToken
      if (tokens.refreshToken) state.refresh = tokens.refreshToken
      state.saved.push(tokens)
    },
    clearSession: () => {
      state.access = null
      state.refresh = null
      state.cleared += 1
    },
    requestRefresh: (token) => {
      state.refreshed += 1
      return refreshImpl(token)
    },
    onSessionExpired: () => {
      state.expired += 1
    },
    // Mirrors api.ts: credential endpoints never trigger a refresh.
    isAuthEndpoint: (url) =>
      url.includes('/auth/login') || url.includes('/auth/register') || url.includes('/auth/refresh'),
  }

  return { state, deps, setRefresh: (fn) => { refreshImpl = fn } }
}

function adapter(handler: (config: InternalAxiosRequestConfig) => { status: number; data?: unknown }): AxiosAdapter {
  return async (config) => {
    const result = handler(config)
    const response = {
      data: result.data ?? {},
      status: result.status,
      statusText: '',
      headers: {},
      config,
    }
    if (result.status >= 200 && result.status < 300) return response
    // A custom adapter has to settle the promise itself, like axios' built-in
    // adapters do.
    throw new AxiosError(
      `Request failed with status code ${result.status}`,
      String(result.status),
      config,
      undefined,
      response as never,
    )
  }
}

function instanceWith(a: AxiosAdapter, h: Harness): AxiosInstance {
  const instance = axios.create({ baseURL: 'http://test.local/api/v1', adapter: a })
  installAuthInterceptors(instance, h.deps)
  return instance
}

const authorization = (config: InternalAxiosRequestConfig) => String(config.headers?.Authorization ?? '')

test('a continuously active player is never logged out', async () => {
  const h = harness()
  const calls: InternalAxiosRequestConfig[] = []
  const instance = instanceWith(
    adapter((config) => {
      calls.push(config)
      return { status: 200, data: { code: 'success', data: { ok: true } } }
    }),
    h,
  )

  for (let i = 0; i < 30; i += 1) {
    const res = await instance.get('/p2p/matches/m1/state')
    assert.deepEqual(res.data, { code: 'success', data: { ok: true } })
  }

  assert.equal(calls.length, 30)
  for (const call of calls) assert.equal(authorization(call), 'Bearer access-1')
  assert.equal(h.state.refreshed, 0, 'a valid token must not trigger refreshes')
  assert.equal(h.state.cleared, 0)
  assert.equal(h.state.expired, 0)
})

test('an active match survives access-token expiry through a transparent refresh', async () => {
  const h = harness()
  const calls: InternalAxiosRequestConfig[] = []
  const instance = instanceWith(
    adapter((config) => {
      calls.push(config)
      if (authorization(config) !== 'Bearer access-2') return { status: 401 }
      return { status: 200, data: { board: { pieces: 12 }, turn: 'user-1' } }
    }),
    h,
  )

  const res = await instance.get('/p2p/matches/match-42/state')

  assert.deepEqual(res.data, { board: { pieces: 12 }, turn: 'user-1' })
  assert.equal(calls.length, 2)
  assert.equal(authorization(calls[1]), 'Bearer access-2')
  assert.equal(h.state.refreshed, 1)
  assert.equal(h.state.access, 'access-2')
  assert.equal(h.state.refresh, 'refresh-2')
  assert.equal(h.state.cleared, 0)
})

test('a retried move is not duplicated when a response arrives after refresh', async () => {
  // The original request never reached the server (it failed auth), so exactly
  // one POST is delivered with the new token.
  const h = harness()
  const bodies: string[] = []
  const instance = instanceWith(
    adapter((config) => {
      if (config.url === '/p2p/matches/m1/move') {
        if (authorization(config) !== 'Bearer access-2') return { status: 401 }
        bodies.push(String(config.data))
        return { status: 200, data: { code: 'success', data: { applied: true } } }
      }
      return { status: 200 }
    }),
    h,
  )

  await instance.post('/p2p/matches/m1/move', { from: 35, to: 49 })
  assert.equal(bodies.length, 1)
  assert.deepEqual(JSON.parse(bodies[0]), { from: 35, to: 49 })
})

test('concurrent 401s share exactly one refresh and all retry', async () => {
  const h = harness()
  const calls: InternalAxiosRequestConfig[] = []
  let refreshCalls = 0
  h.setRefresh(async () => {
    refreshCalls += 1
    await new Promise((resolve) => setTimeout(resolve, 15))
    return { accessToken: 'access-2', refreshToken: 'refresh-2' }
  })

  const instance = instanceWith(
    adapter((config) => {
      calls.push(config)
      if (authorization(config) !== 'Bearer access-2') return { status: 401 }
      return { status: 200, data: { ok: true } }
    }),
    h,
  )

  const results = await Promise.all(
    Array.from({ length: 5 }, (_, i) => instance.get(`/p2p/matches/m${i}/state`)),
  )

  assert.equal(refreshCalls, 1, 'five simultaneous 401s must produce one refresh')
  assert.equal(calls.length, 10)
  assert.equal(results.length, 5)
  assert.ok(results.every((r) => r.status === 200))
  assert.equal(h.state.expired, 0)
})

test('temporary network and server failures never clear the session', async () => {
  const h = harness({ requestRefresh: async () => { throw new Error('Network Error') } })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.get('/wallet'), (err: unknown) => err instanceof AxiosError)
  assert.equal(h.state.cleared, 0, 'a network failure is not proof the session is invalid')
  assert.equal(h.state.expired, 0)
  assert.equal(h.state.access, 'access-1')

  // Later, when the network recovers, the same session refreshes and works.
  h.setRefresh(async () => ({ accessToken: 'access-2' }))
  const recovered = instanceWith(
    adapter((config) => (authorization(config) === 'Bearer access-2' ? { status: 200, data: { ok: true } } : { status: 401 })),
    h,
  )
  const res = await recovered.get('/wallet')
  assert.deepEqual(res.data, { ok: true })
  assert.equal(h.state.cleared, 0)
})

test('a 5xx from the refresh endpoint keeps the session', async () => {
  const failure = Object.assign(new Error('upstream error'), { status: 503 })
  const h = harness({ requestRefresh: async () => { throw failure } })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.get('/wallet'))
  assert.equal(h.state.cleared, 0)
  assert.equal(h.state.expired, 0)
})

test('403 and 5xx responses never log the player out', async () => {
  for (const status of [403, 500, 502]) {
    const h = harness()
    const instance = instanceWith(adapter(() => ({ status })), h)

    await assert.rejects(instance.get('/admin/users'))
    assert.equal(h.state.refreshed, 0, `${status} is not an authentication failure`)
    assert.equal(h.state.cleared, 0)
    assert.equal(h.state.expired, 0)
    assert.equal(h.state.access, 'access-1')
  }
})

test('login failures do not trigger refresh or logout', async () => {
  const h = harness({ access: null, refresh: null })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.post('/auth/login', { identifier: 'x', password: 'y' }))
  assert.equal(h.state.refreshed, 0)
  assert.equal(h.state.cleared, 0)
  assert.equal(h.state.expired, 0)
})

test('a rejected refresh token ends the session once with a clear error', async () => {
  const rejection = Object.assign(new Error('refresh rejected'), { status: 401 })
  const h = harness({ requestRefresh: async () => { throw rejection } })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.get('/p2p/matches/m1/state'), (err: unknown) => {
    assert.ok(err instanceof SessionExpiredError)
    assert.match((err as Error).message, /session has expired/i)
    return true
  })

  assert.equal(h.state.cleared, 1)
  assert.equal(h.state.expired, 1)
  assert.equal(h.state.access, null)
  assert.equal(h.state.refresh, null)
})

test('concurrent requests with a dead refresh token expire the session once', async () => {
  const rejection = Object.assign(new Error('refresh rejected'), { status: 403 })
  const h = harness()
  h.setRefresh(async () => {
    await new Promise((resolve) => setTimeout(resolve, 10))
    throw rejection
  })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  const results = await Promise.allSettled(
    Array.from({ length: 4 }, (_, i) => instance.get(`/p2p/matches/m${i}/state`)),
  )

  assert.ok(results.every((r) => r.status === 'rejected'))
  assert.ok(results.every((r) => r.status === 'rejected' && r.reason instanceof SessionExpiredError))
  assert.equal(h.state.refreshed, 1)
  assert.equal(h.state.cleared, 1)
  assert.equal(h.state.expired, 1, 'only one "session expired" notice for a burst of failures')
})

test('a 401 after a successful refresh does not loop forever', async () => {
  const h = harness()
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.get('/wallet'), (err: unknown) => err instanceof SessionExpiredError)
  assert.equal(h.state.refreshed, 1)
  assert.equal(h.state.cleared, 1)
})

test('a session with no refresh token expires without hammering the server', async () => {
  const h = harness({ access: 'access-1', refresh: null })
  const instance = instanceWith(adapter(() => ({ status: 401 })), h)

  await assert.rejects(instance.get('/wallet'), (err: unknown) => err instanceof SessionExpiredError)
  assert.equal(h.state.refreshed, 0)
  assert.equal(h.state.cleared, 1)
  assert.equal(h.state.expired, 1)
})

test('the refresher itself is single-flight', async () => {
  const h = harness()
  let calls = 0
  h.setRefresh(async () => {
    calls += 1
    await new Promise((resolve) => setTimeout(resolve, 10))
    return { accessToken: 'access-2' }
  })

  const refresher = createSessionRefresher(h.deps)
  const results = await Promise.all([1, 2, 3, 4].map(() => refresher.refresh()))

  assert.deepEqual(results, [true, true, true, true])
  assert.equal(calls, 1)
  assert.equal(h.state.saved.length, 1)
})

test('a refreshed session recovers on a later request even after a transient failure', async () => {
  const h = harness()
  let attempt = 0
  h.setRefresh(async () => {
    attempt += 1
    if (attempt === 1) throw Object.assign(new Error('socket hang up'), { code: 'ECONNRESET' })
    return { accessToken: 'access-2' }
  })
  const instance = instanceWith(
    adapter((config) => (authorization(config) === 'Bearer access-2' ? { status: 200, data: { ok: true } } : { status: 401 })),
    h,
  )

  await assert.rejects(instance.get('/wallet'))
  assert.equal(h.state.cleared, 0)

  const res = await instance.get('/wallet')
  assert.deepEqual(res.data, { ok: true })
  assert.equal(attempt, 2)
})
