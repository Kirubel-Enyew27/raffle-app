import assert from 'node:assert/strict'
import test from 'node:test'
import {
  asArray,
  normalizeGames,
  normalizeLive,
  normalizeOverview,
  normalizePage,
  normalizeUserDetail,
} from './normalize.ts'
import type { AdminLiveSnapshot, AdminOverview, AdminUserDetail } from './api'

// The dashboard calls `.length` on these collections during render. Before the
// fix, the API returned `revenue_trend: null` and signing in as an admin crashed
// with "Cannot read properties of null (reading 'length')".
const overviewPayload = {
  from: '2026-11-01',
  to: '2026-11-09',
  total_users: 42,
  new_users: 3,
  platform_revenue: 123.45,
  new_users_change: null,
  registration_trend: null,
  match_trend: null,
  finance_trend: null,
  revenue_trend: null,
  games: null,
} as unknown as AdminOverview

test('asArray keeps valid arrays and replaces null/undefined with a new list', () => {
  const rows = [{ id: 'row-1' }]
  assert.equal(asArray(rows, 'test.valid'), rows)

  assert.deepEqual(asArray(null, 'test.null'), [])
  assert.deepEqual(asArray(undefined, 'test.undefined'), [])
  assert.deepEqual(asArray({ not: 'an array' }, 'test.object'), [])
})

test('asArray logs each contract violation once, naming the field', () => {
  const original = console.warn
  const messages: string[] = []
  console.warn = (message: string) => { messages.push(message) }

  try {
    asArray(null, 'test.warn.null')
    asArray(null, 'test.warn.null')
    asArray(undefined, 'test.warn.undefined')
  } finally {
    console.warn = original
  }

  assert.equal(messages.length, 2, 'repeated violations of the same field warn only once')
  assert.match(messages[0], /test\.warn\.null/)
  assert.match(messages[0], /null/)
})

test('normalizeOverview makes chart collections safe without touching real data', () => {
  const normalized = normalizeOverview(overviewPayload)

  // This is the exact expression that used to throw after admin login.
  assert.equal(normalized.revenue_trend.length, 0)
  assert.equal(normalized.registration_trend.length, 0)
  assert.equal(normalized.match_trend.length, 0)
  assert.equal(normalized.finance_trend.length, 0)
  assert.deepEqual(normalized.games, [])

  assert.equal(normalized.total_users, 42)
  assert.equal(normalized.new_users, 3)
  assert.equal(normalized.platform_revenue, 123.45)
  assert.equal(normalized.new_users_change, null, 'nullable scalars are preserved')
})

test('normalizeOverview keeps populated trends and normalizes each game trend', () => {
  const payload = {
    ...overviewPayload,
    revenue_trend: [{ period: '2026-11-08', p2p_fees: 10, crash_fees: 0, raffle_profit: 0, total: 10 }],
    games: [{ slug: 'dama', title: 'Dama', total_matches: 4, trend: null }],
  } as unknown as AdminOverview

  const normalized = normalizeOverview(payload)

  assert.equal(normalized.revenue_trend.length, 1)
  assert.equal(normalized.games.length, 1)
  assert.deepEqual(normalized.games[0].trend, [])
  assert.equal(normalized.games[0].total_matches, 4)
})

test('normalizeLive guards live matches, players and crash bets', () => {
  const snapshot = {
    generated_at: '2026-11-09T10:00:00Z',
    live_players: 7,
    active_matches: 2,
    active_crash_bets: 1,
    matches: [{ id: 'm-1', game_title: 'Dama', players: null }],
    crash_rounds: null,
  } as unknown as AdminLiveSnapshot

  const normalized = normalizeLive(snapshot)

  assert.equal(normalized.matches.length, 1)
  assert.deepEqual(normalized.matches[0].players, [])
  assert.deepEqual(normalized.crash_rounds, [])
  assert.equal(normalized.live_players, 7)
})

test('normalizePage, normalizeGames and normalizeUserDetail restore their arrays', () => {
  const page = normalizePage(
    { items: null, total: 0, limit: 30, offset: 0 } as unknown as Parameters<typeof normalizePage>[0],
    'test.page',
  )
  assert.deepEqual(page.items, [])
  assert.equal(page.total, 0)

  assert.deepEqual(normalizeGames(null), [])

  const detail = normalizeUserDetail({
    id: 'user-1',
    recent_matches: null,
    recent_transactions: null,
  } as unknown as AdminUserDetail)
  assert.deepEqual(detail.recent_matches, [])
  assert.deepEqual(detail.recent_transactions, [])
  assert.equal(detail.id, 'user-1')
})
