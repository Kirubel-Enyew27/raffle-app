import { api } from '@/lib/api'
import type {
  ApiResponse,
  P2PAdminMatch,
  P2PAdminSettings,
  P2PConfig,
  P2PGame,
  P2PMatch,
  P2PRematchResult,
  P2PStakeQuote,
  InviteSummary,
  LeaderboardData,
  LeaderboardParams,
} from '@/types/api'

export interface CreateMatchPayload {
  game_slug: string
  match_type?: 'public_queue' | 'private_room'
  competition_mode?: 'fun' | 'earn'
  stake_amount?: number
  /** Quick Play: join a waiting opponent when one exists instead of opening a new room. */
  auto_match?: boolean
}

/**
 * Editorial order for game catalogs (lobby, challenge/host pickers and the
 * leaderboard filter). The API lists games alphabetically; the product wants
 * this order instead. Unknown slugs keep their API order at the end.
 */
const GAME_DISPLAY_ORDER = [
  'connect-four',
  'ethiopian-dama',
  'tic-tac-toe',
  'rock-paper-scissors',
]

function inDisplayOrder(games: P2PGame[]): P2PGame[] {
  const rank = new Map(GAME_DISPLAY_ORDER.map((slug, index) => [slug, index]))
  return [...games].sort(
    (a, b) =>
      (rank.get(a.slug) ?? GAME_DISPLAY_ORDER.length) -
      (rank.get(b.slug) ?? GAME_DISPLAY_ORDER.length)
  )
}

export const p2pApi = {
  listGames: () =>
    api.get<ApiResponse<P2PGame[]>>('/p2p/games').then((r) => inDisplayOrder(r.data.data ?? [])),

  getGame: (slug: string) =>
    api.get<ApiResponse<P2PGame>>(`/p2p/games/${slug}`).then((r) => r.data.data),

  listLobby: (gameSlug?: string, limit = 20, mode?: 'fun' | 'earn') =>
    api
      .get<ApiResponse<P2PMatch[]>>('/p2p/matches/lobby', {
        params: { game_slug: gameSlug, limit, mode },
      })
      .then((r) => r.data.data ?? []),

  /** Backend-authoritative paid-play limits, fee and spendable balance. */
  getConfig: () =>
    api.get<ApiResponse<P2PConfig>>('/p2p/config').then((r) => r.data.data),

  /** Exact stake/pool/fee/payout quote; never calculate money on the client. */
  quoteStake: (gameSlug: string, stakeAmount: number) =>
    api
      .get<ApiResponse<P2PStakeQuote>>('/p2p/stake-quote', {
        params: { game_slug: gameSlug, stake_amount: stakeAmount },
      })
      .then((r) => r.data.data),

  adminGetSettings: () =>
    api.get<ApiResponse<P2PAdminSettings>>('/admin/p2p/settings').then((r) => r.data.data),

  adminUpdateSettings: (settings: Record<string, string>) =>
    api
      .patch<ApiResponse<P2PAdminSettings>>('/admin/p2p/settings', { settings })
      .then((r) => r.data.data),

  adminListMatches: (params: { status?: string; game?: string; limit?: number; offset?: number }) =>
    api
      .get<ApiResponse<{ matches: P2PAdminMatch[]; total: number }>>('/admin/p2p/matches', {
        params,
      })
      .then((r) => ({ matches: r.data.data?.matches ?? [], total: r.data.data?.total ?? 0 })),

  listMyMatches: (limit = 20, offset = 0) =>
    api
      .get<ApiResponse<{ matches: P2PMatch[]; total: number }>>('/p2p/matches/my', {
        params: { limit, offset },
      })
      .then((r) => r.data.data ?? { matches: [], total: 0 }),

  createMatch: (payload: CreateMatchPayload) =>
    api.post<ApiResponse<P2PMatch>>('/p2p/matches', payload).then((r) => r.data.data),

  joinMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/join`).then((r) => r.data.data),

  joinMatchByCode: (inviteCode: string) =>
    api
      .post<ApiResponse<P2PMatch>>('/p2p/matches/join-code', { invite_code: inviteCode.trim().toUpperCase() })
      .then((r) => r.data.data),

  getInvite: (inviteCode: string) =>
    api
      .get<ApiResponse<InviteSummary>>(`/p2p/invites/${inviteCode.trim().toUpperCase()}`)
      .then((r) => r.data.data),

  cancelMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/cancel`).then((r) => r.data.data),

  readyUp: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/ready`).then((r) => r.data.data),

  submitMove: (matchId: string, action: Record<string, unknown>) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/move`, action).then((r) => r.data.data),

  resignMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/resign`).then((r) => r.data.data),

  /**
   * Notify the server that the client has lost its connection to the match.
   * The server starts the reconnection grace period (60 s).
   */
  disconnectFromMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/disconnect`).then((r) => r.data.data),

  /**
   * Notify the server that the client has re-established its connection.
   * Returns the current match state. If the grace period expired the server
   * returns 410 Gone — callers should refetch to show the final result.
   */
  reconnectToMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/reconnect`).then((r) => r.data.data),

  /** Ask for a return game: the first call waits for the opponent, the second starts it. */
  requestRematch: (matchId: string) =>
    api
      .post<ApiResponse<P2PRematchResult>>(`/p2p/matches/${matchId}/rematch`)
      .then((r) => r.data.data),

  getMatchState: (matchId: string) =>
    api.get<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/state`).then((r) => r.data.data),

  getLeaderboard: (params?: LeaderboardParams) =>
    api
      .get<ApiResponse<LeaderboardData>>('/p2p/leaderboard', { params })
      .then((r) => r.data.data),
}
