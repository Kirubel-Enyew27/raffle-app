import { api } from '@/lib/api'
import type {
  ApiResponse,
  P2PGame,
  P2PMatch,
  P2PRematchResult,
  InviteSummary,
  LeaderboardData,
  LeaderboardParams,
} from '@/types/api'

export interface CreateMatchPayload {
  game_slug: string
  match_type?: 'public_queue' | 'private_room'
  stake_amount?: number
  /** Quick Play: join a waiting opponent when one exists instead of opening a new room. */
  auto_match?: boolean
}

export const p2pApi = {
  listGames: () =>
    api.get<ApiResponse<P2PGame[]>>('/p2p/games').then((r) => r.data.data),

  getGame: (slug: string) =>
    api.get<ApiResponse<P2PGame>>(`/p2p/games/${slug}`).then((r) => r.data.data),

  listLobby: (gameSlug?: string, limit = 20) =>
    api
      .get<ApiResponse<P2PMatch[]>>('/p2p/matches/lobby', {
        params: { game_slug: gameSlug, limit },
      })
      .then((r) => r.data.data ?? []),

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

  submitMove: (matchId: string, action: Record<string, any>) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/move`, action).then((r) => r.data.data),

  resignMatch: (matchId: string) =>
    api.post<ApiResponse<P2PMatch>>(`/p2p/matches/${matchId}/resign`).then((r) => r.data.data),

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
