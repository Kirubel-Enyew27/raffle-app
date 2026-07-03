import { api } from '@/lib/api'
import type { ApiResponse, Wallet, Raffle, Winner } from '@/types/api'
import { mapTransaction } from '../wallet/api'

export const dashboardApi = {
  wallet: () =>
    api.get<ApiResponse<Wallet>>('/wallets/balance').then(r => r.data.data),

  transactions: (limit = 5) =>
    api.get<ApiResponse<{ transactions: any[]; total: number }>>('/wallets/transactions', { params: { limit } })
      .then(r => ({
        transactions: (r.data.data?.transactions ?? []).map(mapTransaction),
        total: r.data.data?.total ?? 0,
      })),

  raffles: (limit = 6) =>
    api.get<ApiResponse<{ raffles: Raffle[]; total: number }>>('/raffles', { params: { limit } })
      .then(r => r.data.data),

  winnersByRaffle: (raffleId: string) =>
    api.get<ApiResponse<Winner[]>>(`/winners/raffle/${raffleId}`).then(r => r.data.data),
}
