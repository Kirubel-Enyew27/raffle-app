import { api } from '@/lib/api'
import type { ApiResponse, Raffle, Ticket, WalletTransaction } from '@/types/api'

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

interface Page<T> { items: T[]; total: number; limit: number; offset: number }

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
      .then(r => r.data.data),

  ticketSales: (from: string, to: string, period = 'daily', limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<TicketSalesRow>>>('/reports/tickets', { params: { from, to, period, limit, offset } })
      .then(r => r.data.data),

  activeUsers: (from: string, to: string, period = 'daily', limit = 30, offset = 0) =>
    api.get<ApiResponse<Page<ActiveUsersRow>>>('/reports/active-users', { params: { from, to, period, limit, offset } })
      .then(r => r.data.data),

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
      .then(r => r.data.data),

  approveWithdrawal: (id: string) =>
    api.post<ApiResponse<WalletTransaction>>(`/admin/wallets/approve-withdrawal/${id}`)
      .then(r => r.data.data),

  rejectWithdrawal: (id: string) =>
    api.post<ApiResponse<WalletTransaction>>(`/admin/wallets/reject-withdrawal/${id}`)
      .then(r => r.data.data),
}
