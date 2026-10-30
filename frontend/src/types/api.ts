// Shared API response envelope
export interface ApiResponse<T> {
  code: string
  data: T
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

// Auth
export interface User {
  id: string
  email: string
  full_name?: string
  phone?: string
  avatar_url?: string
  role: 'user' | 'admin'
  is_banned: boolean
  created_at: string
}

export interface LoginResponse {
  token: string
  /** Long-lived token used by the API client to renew `token` without a new login. */
  refresh_token?: string
  /** Access-token lifetime in seconds. */
  expires_in?: number
  user: User
}

// Raffle
export interface Raffle {
  id: string
  title: string
  description: string
  ticket_price: number
  total_tickets: number
  sold_tickets: number
  status: 'scheduled' | 'active' | 'completed' | 'cancelled' | 'draft' | 'closed'
  draw_date: string
  prize_pool: number
  currency: string
  created_at: string
}

// Ticket
export interface Ticket {
  id: string
  raffle_id: string
  user_id: string
  ticket_number: number
  created_at: string
}

// Wallet
export interface Wallet {
  id: string
  user_id: string
  balance: number
  currency: string
}

export interface WalletTransaction {
  id: string
  wallet_id?: string
  user_id?: string
  type: string
  status: string
  amount: number
  balance_before: number
  balance_after: number
  reference: string
  description?: string
  metadata?: Record<string, any>
  created_at: string
}

// P2P Multiplayer Games
export interface P2PGame {
  slug: string
  title: string
  description: string
  min_players: number
  max_players: number
  is_active: boolean
  min_stake_cents: number
  max_stake_cents: number
  default_fee_bp: number
  default_turn_seconds: number
  rules_json?: Record<string, any>
}

export interface P2PMatchPlayer {
  id: string
  match_id: string
  user_id: string
  display_name?: string
  avatar_url?: string
  player_index: number
  symbol: string
  status: 'reserved' | 'ready' | 'active' | 'forfeited' | 'disconnected'
  score: number
  debit_tx_id?: string
  credit_tx_id?: string
  fee_tx_id?: string
  joined_at: string
  ready_at?: string
}

export interface P2PMatch {
  id: string
  game_slug: string
  status: 'waiting' | 'ready' | 'in_progress' | 'completed' | 'cancelled' | 'tied' | 'forfeited'
  match_type: 'public_queue' | 'private_room'
  invite_code?: string
  stake_amount: number
  prize_pool: number
  service_fee_percent: number
  total_fee: number
  winner_user_id?: string
  current_turn_user_id?: string
  turn_seconds_left: number
  current_round: number
  max_rounds: number
  scores: Record<string, number>
  board_state: Record<string, any>
  created_by: string
  /** Set when a player asked for a return game; the opponent accepts from the result screen. */
  rematch_requested_by?: string
  /** The return match created once both players agreed to a rematch. */
  rematch_match_id?: string
  /** The user_id of the player who has temporarily disconnected. */
  disconnect_user_id?: string
  /** Seconds remaining in the reconnection grace period (0 if none active). */
  disconnect_seconds_left: number
  /**
   * Absolute turn deadline on the server, with the server's clock at the time
   * of the snapshot. Counting down from these instead of from a rendered
   * `turn_seconds_left` keeps the display honest across refetches and drift.
   */
  turn_deadline_at?: string
  /** Absolute reconnection-grace deadline on the server. */
  disconnect_deadline_at?: string
  /** Server clock at snapshot time (reference point for the deadlines above). */
  server_time?: string
  my_player_index?: number
  players: P2PMatchPlayer[]
  started_at?: string
  completed_at?: string
  expires_at?: string
  created_at: string
}

export interface P2PRematchResult {
  /** `requested`: waiting for the opponent. `started`: the return match is live. */
  status: 'requested' | 'started'
  match: P2PMatch
}

export interface InviteSummary {
  invite_code: string
  match_id: string
  game_slug: string
  game_title: string
  game_description: string
  status: 'waiting' | 'ready' | 'in_progress' | 'completed' | 'cancelled' | 'expired'
  is_joinable: boolean
  is_expired: boolean
  expires_at: string
  seconds_left: number
  creator_id: string
  creator_name: string
  creator_avatar_url?: string
  match_type: 'public_queue' | 'private_room'
  max_rounds: number
  default_turn_seconds: number
  created_at: string
}

export type LeaderboardPeriod = 'all_time' | 'daily' | 'weekly' | 'monthly'
export type LeaderboardSortBy = 'wins' | 'win_streak' | 'win_rate'

export interface LeaderboardPlayer {
  rank: number
  player_id: string
  display_name: string
  avatar_url?: string | null
  matches_played: number
  wins: number
  losses: number
  draws: number
  win_rate: number
  current_win_streak: number
  longest_win_streak: number
}

export interface LeaderboardPagination {
  page: number
  limit: number
  total_players: number
  total_pages: number
}

export interface LeaderboardData {
  period: LeaderboardPeriod
  game?: string | null
  sort_by: LeaderboardSortBy
  players: LeaderboardPlayer[]
  current_player?: LeaderboardPlayer | null
  pagination: LeaderboardPagination
}

export interface LeaderboardParams {
  period?: LeaderboardPeriod
  game?: string
  sort_by?: LeaderboardSortBy
  page?: number
  limit?: number
}

export interface ReceiptInfo {
  payment_method: string
  amount: number
  transaction_ref: string
  transfer_date: string
  receiver_name: string
  status: string
  receipt_url: string
}

export interface ReceiptDepositResult {
  transaction: WalletTransaction
  receipt: ReceiptInfo
}

export interface DepositIntent {
  id: string
  user_id: string
  wallet_id: string
  amount: number
  currency: string
  payment_method: string
  status: string
  external_reference?: string
  receipt_token?: string
  receipt_url?: string
  payer_name?: string
  receiver_name?: string
  actual_amount_credited?: number
  fee_amount?: number
  error_message?: string
  expires_at: string
  created_at: string
}

// Winner
export interface Winner {
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

export interface WinningTicket {
  ticket_id: string
  ticket_number: number
  raffle_id: string
  user_id: string
  user_email: string
  draw_timestamp: string
}

export interface DrawVerification {
  draw_id: string
  raffle_id: string
  draw_timestamp: string
  commit_hash: string
  server_seed_hash: string
  revealed_seed: string
  combined_hash: string
  winning_number: number
  verification_url: string
  winner_id: string
  winning_ticket_id: string
}

// Notification
export interface Notification {
  id: string
  channel: 'email' | 'in_app'
  event: string
  subject: string
  body: string
  status: 'pending' | 'sent' | 'failed'
  read_at?: string
  created_at: string
}
