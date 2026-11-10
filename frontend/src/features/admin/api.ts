import { api } from '@/lib/api'
import type { ApiResponse, Raffle, Ticket, WalletTransaction } from '@/types/api'
import { mapTransaction } from '../wallet/api'
import {
  asArray,
  normalizeGames,
  normalizeLive,
  normalizeOverview,
  normalizePage,
  normalizeUserDetail,
} from './normalize'

export interface RevenueRow {
  period: string
  ticket_revenue: number
  deposit_volume: number
  withdraw_volume: number
  prize_paid: number
  profit: number
}

export interface TicketSalesRow {
  period: string
  tickets_sold: number
  raffles_held: number
}

export interface ActiveUsersRow {
  period: string
  active_users: number
}

export interface ProfitSummary {
  total_ticket_revenue: number
  total_deposit_volume: number
  total_withdraw_volume: number
  total_prize_paid: number
  net_profit: number
  total_tickets_sold: number
  total_winners: number
  total_active_users: number
}

export interface CreateRaffleInput {
  title: string
  description: string
  ticket_price: number
  max_tickets: number
  draw_date: string
  status: string
  prize_pool: number
  currency: string
}

export interface DrawProof {
  commit_hash: string
  server_seed_hash: string
  revealed_seed: string
  combined_hash: string
  winning_number: number
  verification_url: string
}

export interface WinnerDetail {
  id: string
  raffle_id: string
  draw_id: string
  ticket_id: string
  user_id: string
  prize_amount: number
  prize_paid: boolean
  payment_date?: string
  payment_reference?: string
  created_at: string
  updated_at: string
  raffle_title: string
  ticket_number: number
  user_email: string
  draw_timestamp: string
  draw_proof: DrawProof
}

export interface WinnerSummaryRow {
  winner_id: string
  user_email: string
  raffle_title: string
  prize_amount: number
  prize_paid: boolean
  payment_date?: string
  payment_reference?: string
  created_at: string
}

export interface WinnerSummaryRow {
  winner_id: string
  user_email: string
  raffle_title: string
  prize_amount: number
  prize_paid: boolean
  payment_date?: string
  payment_reference?: string
  created_at: string
}

export interface AdminOverview {
  from: string
  to: string
  total_users: number
  new_users: number
  active_users: number
  total_matches: number
  matches_today: number
  completed_matches: number
  tied_matches: number
  forfeited_matches: number
  cancelled_matches: number
  live_matches: number
  live_players: number
  total_deposits: number
  total_withdrawals: number
  pending_deposits: number
  pending_withdrawals: number
  platform_revenue: number
  p2p_service_fees: number
  crash_service_fees: number
  raffle_profit: number
  new_users_change: number | null
  active_users_change: number | null
  total_matches_change: number | null
  deposits_change: number | null
  withdrawals_change: number | null
  platform_revenue_change: number | null
  registration_trend: { period: string; count: number }[]
  match_trend: { period: string; total: number; completed: number; free: number; paid: number }[]
  finance_trend: { period: string; deposits: number; withdrawals: number }[]
  revenue_trend: { period: string; p2p_fees: number; crash_fees: number; raffle_profit: number; total: number }[]
  games: AdminGame[]
}

export interface AdminGame {
  slug: string
  title: string
  description: string
  total_matches: number
  active_matches: number
  completed_matches: number
  unique_players: number
  free_matches: number
  paid_matches: number
  trend: { period: string; matches: number }[]
}

export interface AdminUser {
  id: string
  email: string
  phone?: string
  full_name: string
  avatar_url: string
  role: string
  is_banned: boolean
  created_at: string
  last_active_at?: string
  is_playing: boolean
  wallet_balance: number
  reserved_balance: number
  matches_played: number
  wins: number
  losses: number
  draws: number
  win_rate: number
}

export interface AdminUserDetail extends AdminUser {
  recent_matches: {
    id: string
    game_slug: string
    status: string
    competition_mode: string
    result: string
    stake_amount: number
    prize_pool: number
    created_at: string
    completed_at?: string
  }[]
  recent_transactions: {
    id: string
    type: string
    status: string
    amount: number
    reference: string
    created_at: string
  }[]
}

export interface AdminMatchPlayer {
  user_id: string
  display_name: string
  avatar_url: string
  player_index: number
  status: string
  score: number
}

export interface AdminMatch {
  id: string
  game_slug: string
  game_title: string
  status: string
  match_type: string
  competition_mode: string
  stake_amount: number
  prize_pool: number
  service_fee: number
  winner_user_id?: string
  winner_name?: string
  duration_seconds: number
  financial_status: string
  players: AdminMatchPlayer[]
  created_at: string
  started_at?: string
  completed_at?: string
}

export interface AdminDeposit {
  id: string
  user_id: string
  user_email: string
  user_name: string
  amount: number
  currency: string
  payment_method: string
  status: string
  external_reference?: string
  receipt_url?: string
  created_at: string
  credited_at?: string
}

export interface AdminWithdrawal {
  id: string
  user_id: string
  user_email: string
  user_name: string
  amount: number
  currency: string
  status: string
  reference: string
  description: string
  created_at: string
  updated_at: string
}

export interface AuditLog {
  id: string
  actor_id?: string | null
  actor_type: string
  action: string
  resource_type: string
  resource_id?: string | null
  old_value?: string | null
  new_value?: string | null
  ip_address: string
  user_agent?: string | null
  created_at: string
}

export interface AdminLiveSnapshot {
  generated_at: string
  live_players: number
  active_matches: number
  active_crash_bets: number
  matches: AdminMatch[]
  crash_rounds: {
    id: string
    round_number: number
    state: string
    players: number
    total_stake: number
    crash_multiplier?: number
    created_at: string
    bets: { user_id: string; display_name: string; avatar_url: string; stake_amount: number }[]
  }[]
}

export interface Page<T> { items: T[]; total: number; limit: number; offset: number }

const fmt = (d: Date) => d.toISOString().split('T')[0]

export function defaultRange() {
  const to = new Date()
  const from = new Date(to); from.setDate(from.getDate() - 29)
  return { from: fmt(from), to: fmt(to) }
}

export const adminApi = {
  profit: (from: string, to: string) =>
    api.get<ApiResponse<ProfitSummary>>('/reports/profit', { params: { from, to } })
      .then(r => r.data.data),

  revenue: (from: string, to: string, period = 'daily', limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<RevenueRow>>>('/reports/revenue', { params: { from, to, period, limit, offset } })
      .then(r => r.data.data && normalizePage(r.data.data, 'reports.revenue')),

  ticketSales: (from: string, to: string, period = 'daily', limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<TicketSalesRow>>>('/reports/tickets', { params: { from, to, period, limit, offset } })
      .then(r => r.data.data && normalizePage(r.data.data, 'reports.tickets')),

  activeUsers: (from: string, to: string, period = 'daily', limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<ActiveUsersRow>>>('/reports/active-users', { params: { from, to, period, limit, offset } })
      .then(r => r.data.data && normalizePage(r.data.data, 'reports.active-users')),

  raffles: (limit = 10, offset = 0, status?: string) =>
    api.get<ApiResponse<{ raffles: Raffle[]; total: number }>>('/raffles', { params: { limit, offset, status } })
      .then(r => r.data.data),

  getRaffle: (id: string) =>
    api.get<ApiResponse<Raffle>>(`/raffles/${id}`).then(r => r.data.data),

  createRaffle: (input: CreateRaffleInput) =>
    api.post<ApiResponse<Raffle>>('/raffles', input).then(r => r.data.data),

  updateRaffle: (id: string, input: Partial<CreateRaffleInput>) =>
    api.put<ApiResponse<Raffle>>(`/raffles/${id}`, input).then(r => r.data.data),

  closeRaffle: (id: string) =>
    api.post<ApiResponse<void>>(`/raffles/${id}/close`).then(r => r.data),

  listRaffleTickets: (raffleId: string) =>
    api.get<ApiResponse<{ tickets: Ticket[]; total: number }>>(`/raffles/${raffleId}/tickets`)
      .then(r => r.data.data),

  winners: (limit = 20, offset = 0, paid?: boolean) =>
    api.get<ApiResponse<{ winners: WinnerDetail[]; total: number; limit: number; offset: number }>>(
      '/winners', { params: { limit, offset, ...(paid !== undefined && { paid }) } },
    ).then(r => r.data.data),

  getWinner: (id: string) =>
    api.get<ApiResponse<WinnerDetail>>(`/winners/${id}`).then(r => r.data.data),

  markPaid: (id: string, payment_reference: string) =>
    api.post<ApiResponse<WinnerDetail>>(`/winners/${id}/paid`, { payment_reference }).then(r => r.data.data),

  winnerSummary: (from: string, to: string, limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<WinnerSummaryRow>>>('/reports/winners', { params: { from, to, limit, offset } })
      .then(r => r.data.data),

  // ─── Withdrawal Management ───────────────────────────────────────────────────

  pendingWithdrawals: () =>
    api.get<ApiResponse<WalletTransaction[]>>('/admin/wallets/pending-withdrawals')
      .then(r => (r.data.data ?? []).map(mapTransaction)),

  approveWithdrawal: (id: string) =>
    api.post<ApiResponse<WalletTransaction>>(`/admin/wallets/approve-withdrawal/${id}`)
      .then(r => mapTransaction(r.data.data)),

  rejectWithdrawal: (id: string) =>
    api.post<ApiResponse<WalletTransaction>>(`/admin/wallets/reject-withdrawal/${id}`)
      .then(r => mapTransaction(r.data.data)),

  // ─── Platform administration ───────────────────────────────────────────────

  overview: (from: string, to: string, period = 'day') =>
    api.get<ApiResponse<AdminOverview>>('/admin/overview', { params: { from, to, period } })
      .then(r => r.data.data && normalizeOverview(r.data.data)),

  games: (from: string, to: string, period = 'day') =>
    api.get<ApiResponse<AdminGame[]>>('/admin/games', { params: { from, to, period } })
      .then(r => normalizeGames(r.data.data)),

  users: (params: { search?: string; status?: string; sort?: string; limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<Page<AdminUser>>>('/admin/users', { params })
      .then(r => r.data.data && normalizePage(r.data.data, 'users')),

  user: (id: string) =>
    api.get<ApiResponse<AdminUserDetail>>(`/admin/users/${id}`)
      .then(r => r.data.data && normalizeUserDetail(r.data.data)),

  matches: (params: { search?: string; game?: string; status?: string; mode?: string; sort?: string; from?: string; to?: string; limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<Page<AdminMatch>>>('/admin/matches', { params })
      .then(r => r.data.data && normalizePage(r.data.data, 'matches')),

  deposits: (params: { search?: string; status?: string; from?: string; to?: string; limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<Page<AdminDeposit>>>('/admin/deposits', { params })
      .then(r => r.data.data && normalizePage(r.data.data, 'deposits')),

  withdrawals: (params: { search?: string; status?: string; from?: string; to?: string; limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<Page<AdminWithdrawal>>>('/admin/withdrawals', { params })
      .then(r => r.data.data && normalizePage(r.data.data, 'withdrawals')),

  live: () =>
    api.get<ApiResponse<AdminLiveSnapshot>>('/admin/live')
      .then(r => r.data.data && normalizeLive(r.data.data)),

  audit: (params: { action?: string; resource_type?: string; user_id?: string; from?: string; to?: string; limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<{ logs: AuditLog[]; total: number; limit: number; offset: number }>>('/audit/logs', { params })
      .then(r => r.data.data
        ? { ...r.data.data, logs: asArray<AuditLog>(r.data.data.logs, 'audit.logs') }
        : r.data.data),
}
