import { api } from '@/lib/api'
import type { ApiResponse, Raffle } from '@/types/api'

export interface DrawProof {
  commit_hash: string
  revealed_seed: string
  combined_hash: string
  winning_number: number
  verification_url: string
}

export interface DrawResult {
  id: string
  raffle_id: string
  draw_timestamp: string
  status: string
  winning_ticket_id: string
  winning_ticket_number: number
  proof: DrawProof
  created_at: string
}

export interface VerificationResult {
  verified: boolean
  seed_matches: boolean
  hash_matches: boolean
  commit_hash: string
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
  raffle_title: string
  ticket_number: number
  user_email: string
  draw_timestamp: string
  draw_proof: {
    commit_hash: string
    server_seed_hash: string
    revealed_seed: string
    combined_hash: string
    winning_number: number
    verification_url: string
  }
  created_at: string
}

export const drawApi = {
  getResult: (raffleId: string) =>
    api.get<ApiResponse<DrawResult>>(`/draw/${raffleId}/result`).then(r => r.data.data),

  verify: (raffleId: string) =>
    api.post<ApiResponse<VerificationResult>>('/draw/verify', { raffle_id: raffleId })
      .then(r => r.data.data),

  getWinners: (raffleId: string) =>
    api.get<ApiResponse<WinnerDetail[]>>(`/winners/raffle/${raffleId}`).then(r => r.data.data),

  listCompleted: (limit = 20, offset = 0) =>
    api.get<ApiResponse<{ raffles: Raffle[]; total: number }>>('/raffles', {
      params: { limit, offset, status: 'completed' },
    }).then(r => r.data.data),
}

// ── Client-side verification (mirrors backend pkg/crypto) ───────────────────

async function sha256hex(message: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(message))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

export async function verifyLocally(
  commitHash: string,
  revealedSeed: string,
  combinedHash: string,
): Promise<{ seedMatches: boolean; hashValid: boolean }> {
  const seedDigest = await sha256hex(revealedSeed)
  const seedMatches = seedDigest === commitHash

  // combined hash format: SHA256("serverSeed:clientSeed:nonce") — we can only
  // verify it's a valid SHA-256 hex (length/char check) without the client seed
  const hashValid = /^[0-9a-f]{64}$/.test(combinedHash)

  return { seedMatches, hashValid }
}
