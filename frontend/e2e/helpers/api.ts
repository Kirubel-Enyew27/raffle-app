/**
 * Direct API client for seeding / teardown — bypasses the UI.
 * All requests go to the backend API (port 8080 in CI, proxied via vite in dev).
 */
const BASE = process.env.API_URL ?? 'http://localhost:8080/api/v1'

interface LoginResponse { token: string; user: { id: string; email: string; role: string } }
interface WalletTx       { id: string; amount: number; balance_after: number }
interface Raffle         { id: string; title: string; status: string; ticket_price: number; sold_tickets: number }
interface Ticket         { id: string; ticket_number: number }
interface DrawResult     { id: string; raffle_id: string; winning_ticket_number: number; winning_ticket_id: string; proof: { commit_hash: string; revealed_seed: string } }

async function req<T>(method: string, path: string, body?: unknown, token?: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  const json = await res.json()
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status}: ${JSON.stringify(json)}`)
  return (json.data ?? json) as T
}

export function uid() {
  return Math.random().toString(36).slice(2, 10)
}

export async function registerUser(email: string, password: string) {
  return req<{ id: string; email: string }>('POST', '/auth/register', { email, password })
}

export async function loginUser(email: string, password: string) {
  return req<LoginResponse>('POST', '/auth/login', { email, password })
}

export async function deposit(token: string, amount: number) {
  return req<WalletTx>('POST', '/wallets/deposit', { amount, reference: `seed-${uid()}`, description: 'Test deposit' }, token)
}

export async function createRaffle(token: string, overrides: Partial<{
  title: string; ticket_price: number; max_tickets: number; draw_date: string; status: string; prize_pool: number
}> = {}) {
  const draw = new Date(Date.now() + 86_400_000).toISOString()
  return req<Raffle>('POST', '/raffles', {
    title: `Test Raffle ${uid()}`,
    description: 'E2E test raffle',
    ticket_price: 10,
    max_tickets: 100,
    draw_date: draw,
    status: 'active',
    prize_pool: 500,
    currency: 'USD',
    ...overrides,
  }, token)
}

export async function activateRaffle(token: string, raffleId: string) {
  return req<Raffle>('PUT', `/raffles/${raffleId}`, { status: 'active' }, token)
}

export async function commitDraw(token: string, raffleId: string) {
  return req<{ commit_hash: string }>('POST', `/draw/commit/${raffleId}`, {}, token)
}

export async function executeDraw(token: string, raffleId: string) {
  return req<DrawResult>('POST', `/draw/execute/${raffleId}`, {}, token)
}

export async function getDrawResult(raffleId: string) {
  return req<DrawResult>('GET', `/draw/${raffleId}/result`)
}
