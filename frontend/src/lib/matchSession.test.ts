import test from 'node:test'
import assert from 'node:assert/strict'
import axios, {
  AxiosError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'

import { createAuthStorage } from './authStorage.ts'
import { installAuthInterceptors, type AuthInterceptorDeps, type RefreshedTokens } from './authInterceptor.ts'

/**
 * A match is played over many authenticated requests that outlive the access
 * token. These tests follow one whole game through the real interceptor and the
 * real session storage: when the short-lived access token expires mid-match the
 * player's session is renewed in place and never dropped.
 */

// Minimal in-memory Storage stand-in (node --test has no DOM).
function fakeStore(): Storage {
  const map = new Map<string, string>()
  return {
    get length() {
      return map.size
    },
    clear: () => map.clear(),
    getItem: (k: string) => map.get(k) ?? null,
    key: (i: number) => Array.from(map.keys())[i] ?? null,
    removeItem: (k: string) => void map.delete(k),
    setItem: (k: string, v: string) => void map.set(k, v),
  }
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
    // A custom adapter settles the promise itself, like axios' built-in ones.
    throw new AxiosError(
      `Request failed with status code ${result.status}`,
      String(result.status),
      config,
      undefined,
      response as never,
    )
  }
}

const authorization = (config: InternalAxiosRequestConfig) =>
  String(config.headers?.Authorization ?? '')

/** A signed-in player, with the session persisted exactly as a reload would find it. */
function signedInPlayer() {
  const auth = createAuthStorage(fakeStore())
  auth.save('access-1', { id: 'user-1', full_name: 'Player One' }, 'refresh-1')
  return auth
}

function interceptorDeps(
  auth: ReturnType<typeof signedInPlayer>,
  counters: { clears: number; expiredNotices: number },
  requestRefresh: (token: string) => Promise<RefreshedTokens>,
): AuthInterceptorDeps {
  return {
    getToken: () => auth.getToken(),
    getRefreshToken: () => auth.getRefreshToken(),
    saveTokens: ({ accessToken, refreshToken }) => auth.saveTokens(accessToken, refreshToken),
    clearSession: () => {
      auth.clear()
      counters.clears += 1
    },
    requestRefresh,
    onSessionExpired: () => {
      counters.expiredNotices += 1
    },
    // Mirrors api.ts: credential endpoints never trigger a refresh.
    isAuthEndpoint: (url) =>
      url.includes('/auth/login') ||
      url.includes('/auth/register') ||
      url.includes('/auth/refresh') ||
      url.includes('/auth/logout'),
  }
}

/**
 * A fake backend for one match: the match endpoints only accept the newest
 * access token, and every refresh retires the presented token and issues a new
 * pair — the same single-use rotation the server performs.
 */
function matchBackend(options: { refreshFailures?: number } = {}) {
  let seq = 1
  let validAccess = 'access-1'
  let refreshFailures = options.refreshFailures ?? 0
  const liveRefreshTokens = new Set(['refresh-1'])
  const applied: unknown[] = []
  const counters = { refreshes: 0, rejectedRefreshes: 0 }

  const requestRefresh = async (token: string): Promise<RefreshedTokens> => {
    counters.refreshes += 1
    if (refreshFailures > 0) {
      refreshFailures -= 1
      // No status: a network failure, which is transient rather than proof the
      // session is invalid.
      throw new Error('Network Error')
    }
    if (!liveRefreshTokens.delete(token)) {
      counters.rejectedRefreshes += 1
      throw Object.assign(new Error('refresh token rejected'), { status: 401 })
    }
    seq += 1
    validAccess = `access-${seq}`
    const pair = { accessToken: validAccess, refreshToken: `refresh-${seq}` }
    liveRefreshTokens.add(pair.refreshToken)
    return pair
  }

  /** The access token's lifetime runs out on the server. */
  const expireAccessToken = () => {
    validAccess = 'expired'
  }

  const handler = (config: InternalAxiosRequestConfig) => {
    if (authorization(config) !== `Bearer ${validAccess}`) return { status: 401 }

    if (config.method === 'post') {
      applied.push(JSON.parse(String(config.data ?? '{}')))
      return { status: 200, data: { code: 'success', data: { match_id: 'match-42', plies: applied.length } } }
    }
    return {
      status: 200,
      data: { code: 'success', data: { match_id: 'match-42', status: 'in_progress', plies: applied.length } },
    }
  }

  return {
    requestRefresh,
    handler,
    expireAccessToken,
    applied,
    counters,
    currentAccess: () => validAccess,
  }
}

test('a match in progress keeps the player signed in when the access token expires', async () => {
  const auth = signedInPlayer()
  const backend = matchBackend()
  const counters = { clears: 0, expiredNotices: 0 }

  const instance = axios.create({
    baseURL: 'http://test.local/api/v1',
    adapter: adapter(backend.handler),
  })
  installAuthInterceptors(instance, interceptorDeps(auth, counters, backend.requestRefresh))

  // One game, twelve polls and a move every fourth turn: long enough for the
  // short-lived access token to run out twice in the middle of play.
  for (let turn = 1; turn <= 12; turn += 1) {
    if (turn === 4 || turn === 9) backend.expireAccessToken()

    const state = await instance.get('/p2p/matches/match-42/state')
    assert.equal(state.data.data.match_id, 'match-42', `turn ${turn} still reaches the match`)

    if (turn % 4 === 0) {
      const move = await instance.post('/p2p/matches/match-42/move', { from: 35, to: 49 })
      assert.equal(move.status, 200, `turn ${turn} move is accepted after the renewal`)
    }
  }

  assert.deepEqual(backend.applied, [
    { from: 35, to: 49 },
    { from: 35, to: 49 },
    { from: 35, to: 49 },
  ], 'every move is delivered exactly once, none lost or duplicated by the retries')
  assert.equal(backend.counters.refreshes, 2, 'each expiry renews once')
  assert.equal(backend.counters.rejectedRefreshes, 0)

  assert.equal(counters.clears, 0, 'an expired access token must never sign the player out')
  assert.equal(counters.expiredNotices, 0, 'no "session expired" notice during a match')

  // The session left in storage belongs to the same player, renewed in place.
  assert.equal(auth.getToken(), backend.currentAccess())
  assert.equal(auth.getRefreshToken(), 'refresh-3')
  assert.deepEqual(JSON.parse(auth.getUser()!), { id: 'user-1', full_name: 'Player One' })
})

test('a network blip while renewing does not end an active match', async () => {
  const auth = signedInPlayer()
  const backend = matchBackend({ refreshFailures: 1 })
  const counters = { clears: 0, expiredNotices: 0 }

  const instance = axios.create({
    baseURL: 'http://test.local/api/v1',
    adapter: adapter(backend.handler),
  })
  installAuthInterceptors(instance, interceptorDeps(auth, counters, backend.requestRefresh))

  backend.expireAccessToken()

  // The renewal fails on the network, so this poll fails with the original 401
  // instead of a logout.
  await assert.rejects(instance.get('/p2p/matches/match-42/state'))
  assert.equal(counters.clears, 0)
  assert.equal(counters.expiredNotices, 0)
  assert.equal(auth.getToken(), 'access-1', 'a connectivity blip never touches the session')
  assert.equal(auth.getRefreshToken(), 'refresh-1')
  assert.ok(auth.getUser())

  // The next poll renews and the match carries on from the same board.
  const state = await instance.get('/p2p/matches/match-42/state')
  assert.equal(state.status, 200)
  assert.equal(state.data.data.match_id, 'match-42')
  assert.equal(auth.getToken(), backend.currentAccess())
  assert.equal(backend.counters.refreshes, 2)
  assert.equal(backend.counters.rejectedRefreshes, 0)
  assert.equal(counters.clears, 0)
  assert.equal(counters.expiredNotices, 0)
})
