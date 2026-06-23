import { api } from '@/lib/api'
import type { ApiResponse, Wallet, WalletTransaction, Raffle, Winner } from '@/types/api'

export const dashboardApi = {
  wallet: () =>
    api.get<ApiResponse<Wallet>>('/wallets/balance').then(r => r.data.data),

  transactions: (limit = 5) =>
    api.get<ApiResponse<{ transactions: WalletTransaction[]; total: number }>>('/wallets/transactions', { params: { limit } })
      .then(r => r.data.data),

  raffles: (limit = 6) =>
    api.get<ApiResponse<{ raffles: Raffle[]; total: number }>>('/raffles', { params: { limit } })
      .then(r => r.data.data),

  winnersByRaffle: (raffleId: string) =>
    api.get<ApiResponse<Winner[]>>(`/winners/raffle/${raffleId}`).then(r => r.data.data),
}
