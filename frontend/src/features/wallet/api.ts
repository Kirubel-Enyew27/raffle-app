import { api } from '@/lib/api'
import type { ApiResponse, Wallet, WalletTransaction } from '@/types/api'

export const walletApi = {
  balance: () =>
    api.get<ApiResponse<Wallet>>('/wallets/balance').then(r => r.data.data),

  transactions: (params: { limit: number; offset: number; type?: string }) =>
    api.get<ApiResponse<{ transactions: WalletTransaction[]; total: number }>>(
      '/wallets/transactions',
      { params },
    ).then(r => r.data.data),

  deposit: (amount: number, reference: string, description: string) =>
    api.post<ApiResponse<WalletTransaction>>('/wallets/deposit', { amount, reference, description })
      .then(r => r.data.data),

  withdraw: (amount: number, reference: string, description: string) =>
    api.post<ApiResponse<WalletTransaction>>('/wallets/withdraw', { amount, reference, description })
      .then(r => r.data.data),
}
