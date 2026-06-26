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
  type: 'deposit' | 'withdrawal' | 'sms_deposit'
  status: string
  amount: number
  balance_before: number
  balance_after: number
  reference: string
  description?: string
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
