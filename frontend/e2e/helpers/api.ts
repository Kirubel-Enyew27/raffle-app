/**
 * Shared helpers for the Playwright suite.
 *
 * The e2e tests run against the real backend (the Vite dev server proxies
 * `/api` to it). Everything here talks to that API directly so a test can set up
 * accounts and matches without clicking through unrelated screens.
 */

import type { BrowserContext } from '@playwright/test'

export const API_BASE = process.env.E2E_API_URL || 'http://localhost:8080/api/v1'

/** Short unique suffix for e-mail addresses and names. */
export function uid(): string {
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`
}

export interface ApiResult<T = unknown> {
  status: number
  data: T
}

/** Loosely typed `p2p_matches` snapshot; unknown fields stay reachable. */
export interface MatchState {
  status?: string
  current_turn_user_id?: string
  [key: string]: unknown
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  token?: string,
): Promise<ApiResult<T>> {
  const res = await fetch(API_BASE + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  let parsed: unknown = null
  try {
    if (text) parsed = JSON.parse(text)
  } catch {
    parsed = { raw: text }
  }
  const envelope =
    parsed && typeof parsed === 'object' && 'data' in parsed ? (parsed as { data: unknown }).data : undefined
  return { status: res.status, data: envelope ?? parsed }
}

/** True when the backend is reachable; e2e specs skip when it is not. */
export async function backendReachable(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/health`, { signal: AbortSignal.timeout(2000) })
    return res.ok
  } catch {
    return false
  }
}

/** Logs in an existing user for admin-only setup steps in browser tests. */
export async function loginUser(identifier: string, password: string): Promise<{ token: string }> {
  const res = await request<{ token: string }>('POST', '/auth/login', { identifier, password })
  if (res.status !== 200) {
    throw new Error(`login failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
  return res.data
}

/** Creates an active raffle with test-safe defaults for omitted fields. */
export async function createRaffle(
  token: string,
  raffle: {
    title: string
    ticket_price: number
    max_tickets: number
    prize_pool: number
    draw_date?: string
    status?: string
    currency?: string
  },
): Promise<{ id: string }> {
  const res = await request<{ id: string }>(
    'POST',
    '/raffles',
    {
      ...raffle,
      draw_date: raffle.draw_date ?? new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      status: raffle.status ?? 'active',
      currency: raffle.currency ?? 'ETB',
    },
    token,
  )
  if (res.status !== 201) {
    throw new Error(`create raffle failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
  return res.data
}

export async function commitDraw(token: string, raffleId: string): Promise<void> {
  const res = await request('POST', `/draw/commit/${raffleId}`, {}, token)
  if (res.status !== 200) {
    throw new Error(`commit draw failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
}

export async function executeDraw(
  token: string,
  raffleId: string,
): Promise<{ winning_ticket_number: number }> {
  const res = await request<{ winning_ticket_number: number }>(
    'POST',
    `/draw/execute/${raffleId}`,
    {},
    token,
  )
  if (res.status !== 200) {
    throw new Error(`execute draw failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
  return res.data
}

export interface TestSession {
  token: string
  refreshToken: string
  user: { id: string; email: string; full_name?: string; role?: string }
  email: string
  password: string
}

/** Registers (and logs in) a fresh user, returning a usable session. */
export async function createSession(name = 'player'): Promise<TestSession> {
  const email = `${name}_${uid()}@example.com`
  const password = 'Password123!'

  const registered = await request('POST', '/auth/register', {
    email,
    password,
    full_name: name,
  })
  if (registered.status !== 201) {
    throw new Error(`register failed (${registered.status}): ${JSON.stringify(registered.data)}`)
  }

  const login = await request<{ token: string; refresh_token: string; user: TestSession['user'] }>(
    'POST',
    '/auth/login',
    { identifier: email, password },
  )
  if (login.status !== 200) {
    throw new Error(`login failed (${login.status}): ${JSON.stringify(login.data)}`)
  }

  return {
    token: login.data.token,
    refreshToken: login.data.refresh_token,
    user: login.data.user,
    email,
    password,
  }
}

/** Seeds a browser context with the session (dev builds read sessionStorage). */
export async function seedSession(context: BrowserContext, session: TestSession): Promise<void> {
  await context.addInitScript(
    ([token, refresh, user]: [string, string, string]) => {
      window.sessionStorage.setItem('token', token)
      window.sessionStorage.setItem('refresh_token', refresh)
      window.sessionStorage.setItem('user', user)
    },
    [session.token, session.refreshToken, JSON.stringify(session.user)],
  )
}

export async function createPrivateMatch(
  session: TestSession,
  gameSlug = 'tic-tac-toe',
): Promise<{ id: string; invite_code: string }> {
  const res = await request<{ id: string; invite_code: string }>(
    'POST',
    '/p2p/matches',
    { game_slug: gameSlug, match_type: 'private_room', stake_amount: 0 },
    session.token,
  )
  if (res.status !== 201) {
    throw new Error(`create match failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
  return res.data
}

export async function joinByCode(session: TestSession, inviteCode: string): Promise<void> {
  const res = await request('POST', '/p2p/matches/join-code', { invite_code: inviteCode }, session.token)
  if (res.status !== 200) {
    throw new Error(`join failed (${res.status}): ${JSON.stringify(res.data)}`)
  }
}

export async function fetchMatch(session: TestSession, matchId: string): Promise<MatchState> {
  const res = await request('GET', `/p2p/matches/${matchId}/state`, undefined, session.token)
  return res.data
}

/** Submits a move as `session`; returns the HTTP status. */
export async function submitMove(
  session: TestSession,
  matchId: string,
  action: Record<string, unknown>,
): Promise<number> {
  const res = await request('POST', `/p2p/matches/${matchId}/move`, action, session.token)
  return res.status
}
