import { api } from '@/lib/api'
import type { ApiResponse } from '@/types/api'

/** Server-authoritative round states. The client renders these; it never decides them. */
export type CrashRoundState =
  | 'WAITING'
  | 'BETTING'
  | 'RUNNING'
  | 'CRASHED'
  | 'SETTLEMENT'
  | 'NEXT_ROUND'

export type CrashBetStatus = 'active' | 'cashed_out' | 'lost'

export interface CrashRound {
  id: string
  round_number: number
  state: CrashRoundState
  betting_starts_at: string
  betting_ends_at: string
  started_at?: string
  crashed_at?: string
  settled_at?: string
  crash_multiplier: number
  server_seed_hash: string
  commitment_published_at: string
  /** Only present once the round is final (commit-reveal). */
  server_seed?: string
  nonce: number
  total_stake: number
  total_payout: number
  bet_count: number
  cashout_count: number
  /** P2P fields */
  p2p_mode: boolean
  prize_pool: number
  /** Service fee in percent (e.g. 5.00 means 5%). */
  service_fee_percent: number
  min_participants: number
  total_fee: number
}

export interface CrashBet {
  id: string
  round_id: string
  user_id: string
  /** Masked public name; the server never exposes emails or account details. */
  display_name: string
  stake: number
  status: CrashBetStatus
  auto_cashout_multiplier?: number
  cashout_multiplier?: number
  payout: number
  cashed_out_at?: string
  created_at: string
  /** P2P settlement fields — populated only after settlement. */
  gross_return?: number
  gross_profit?: number
  service_fee?: number
  service_fee_pct?: number
  net_payout?: number
}

export interface CrashConfig {
  growth_rate: number
  house_edge_percent: number
  min_bet: number
  max_bet: number
  betting_seconds: number
  max_multiplier: number
  currency: string
  /** P2P fields */
  service_fee_percent: number
  min_participants: number
  max_participants: number
  p2p_mode: boolean
}

/** Everything the game screen needs to render before any realtime event arrives. */
export interface CrashState {
  server_time: string
  round: CrashRound | null
  multiplier: number
  my_bet?: CrashBet
  live_bets: CrashBet[]
  recent: CrashRound[]
  previous_round?: CrashRound
  config: CrashConfig
}

export interface CrashVerification {
  round_id: string
  round_number: number
  server_seed: string
  server_seed_hash: string
  nonce: number
  crash_multiplier: number
  recomputed_multiplier: number
  seed_matches_commitment: boolean
  verified: boolean
  algorithm: string
  note: string
}

export interface PlaceBetPayload {
  amount: number
  auto_cashout_multiplier?: number
}

/** A P2P winner result as returned in the crash.p2p_settled event. */
export interface P2PWinner {
  user_id: string
  display_name: string
  stake: number
  gross_return: number
  gross_profit: number
  service_fee: number
  service_fee_pct: number
  net_payout: number
  cashout_multiplier: number
}

export const crashApi = {
  state: () => api.get<ApiResponse<CrashState>>('/crash/state').then(r => r.data.data),

  history: (limit = 20) =>
    api.get<ApiResponse<CrashRound[]>>('/crash/history', { params: { limit } }).then(r => r.data.data),

  roundBets: (roundId: string) =>
    api.get<ApiResponse<CrashBet[]>>(`/crash/rounds/${roundId}/bets`).then(r => r.data.data),

  myBets: (limit = 20, offset = 0) =>
    api
      .get<ApiResponse<{ bets: CrashBet[]; total: number }>>('/crash/my-bets', {
        params: { limit, offset },
      })
      .then(r => r.data.data),

  verifyRound: (roundId: string) =>
    api.get<ApiResponse<CrashVerification>>(`/crash/rounds/${roundId}/verify`).then(r => r.data.data),

  /**
   * Places a bet. The stake is debited server-side, so a retry of the same
   * logical action must carry the same idempotency key.
   */
  placeBet: (payload: PlaceBetPayload, idempotencyKey: string) =>
    api
      .post<ApiResponse<CrashBet>>('/crash/bets', payload, {
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .then(r => r.data.data),

  /** Cash-out is always priced by the server from its own clock.
   *  In P2P mode the server records the multiplier; payout is deferred to settlement. */
  cashOut: () => api.post<ApiResponse<CrashBet>>('/crash/bets/cashout').then(r => r.data.data),

  /** Admin: get all configurable settings. */
  getSettings: () =>
    api.get<ApiResponse<Record<string, string>>>('/crash/admin/settings').then(r => r.data.data),

  /** Admin: update a single setting. */
  updateSetting: (key: string, value: string) =>
    api.put<ApiResponse<void>>(`/crash/admin/settings/${key}`, { value }).then(r => r.data),
}
