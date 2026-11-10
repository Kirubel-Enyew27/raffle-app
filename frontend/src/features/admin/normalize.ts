import type {
  AdminGame,
  AdminLiveSnapshot,
  AdminOverview,
  AdminUserDetail,
  Page,
} from './api'

/**
 * The admin screens treat collection fields as arrays (`rows.length`,
 * `rows.map(...)`). A payload where such a field is JSON `null` therefore
 * crashes the page during render — exactly what happened when the overview
 * stopped populating `revenue_trend` and Go encoded the nil slice as `null`.
 *
 * These helpers re-establish the array contract at the API boundary so one
 * malformed field cannot take down a whole screen. They only fill in
 * collections; scalar values, counts and money fields are passed through
 * untouched so real data is never replaced with a made-up default.
 *
 * A contract violation is still logged (once per field) instead of being
 * silently swallowed, so the underlying API bug remains visible.
 */

const warnedFields = new Set<string>()

function warnContract(field: string, value: unknown): void {
  if (warnedFields.has(field)) return
  warnedFields.add(field)
  const received = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value
  console.warn(`[admin-api] Expected an array for "${field}", received ${received}; using an empty list.`)
}

export function asArray<T>(value: unknown, field: string): T[] {
  if (Array.isArray(value)) return value as T[]
  warnContract(field, value)
  return []
}

function normalizeGame(game: AdminGame, field: string): AdminGame {
  return { ...game, trend: asArray(game.trend, `${field}.trend`) }
}

export function normalizeGames(games: unknown): AdminGame[] {
  return asArray<AdminGame>(games, 'games').map(game => normalizeGame(game, `games.${game.slug}`))
}

export function normalizeOverview(overview: AdminOverview): AdminOverview {
  return {
    ...overview,
    registration_trend: asArray(overview.registration_trend, 'overview.registration_trend'),
    match_trend: asArray(overview.match_trend, 'overview.match_trend'),
    finance_trend: asArray(overview.finance_trend, 'overview.finance_trend'),
    revenue_trend: asArray(overview.revenue_trend, 'overview.revenue_trend'),
    games: asArray<AdminGame>(overview.games, 'overview.games')
      .map(game => normalizeGame(game, `overview.games.${game.slug}`)),
  }
}

export function normalizeLive(snapshot: AdminLiveSnapshot): AdminLiveSnapshot {
  return {
    ...snapshot,
    matches: asArray<AdminLiveSnapshot['matches'][number]>(snapshot.matches, 'live.matches')
      .map(match => ({
        ...match,
        players: asArray(match.players, `live.matches.${match.id}.players`),
      })),
    crash_rounds: asArray<AdminLiveSnapshot['crash_rounds'][number]>(snapshot.crash_rounds, 'live.crash_rounds')
      .map(round => ({
        ...round,
        bets: asArray(round.bets, `live.crash_rounds.${round.id}.bets`),
      })),
  }
}

export function normalizePage<T>(page: Page<T>, field: string): Page<T> {
  return { ...page, items: asArray<T>(page.items, `${field}.items`) }
}

export function normalizeUserDetail(user: AdminUserDetail): AdminUserDetail {
  return {
    ...user,
    recent_matches: asArray(user.recent_matches, 'user.recent_matches'),
    recent_transactions: asArray(user.recent_transactions, 'user.recent_transactions'),
  }
}
