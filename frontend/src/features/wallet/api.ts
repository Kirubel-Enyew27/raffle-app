import { api } from '@/lib/api'
import type { ApiResponse, Wallet, WalletTransaction } from '@/types/api'

export const mapTransaction = (tx: any): WalletTransaction => ({
  ...tx,
  type: (tx.type === 'deposit' || tx.type === 'sms_deposit') ? 'deposit' : 'withdraw',
})

export const walletApi = {
  balance: () =>
    api.get<ApiResponse<Wallet>>('/wallets/balance').then(r => r.data.data),

  transactions: (params: { limit: number; offset: number; type?: string }) => {
    // Map the frontend filter type ('withdraw') back to backend expectation ('withdrawal') if present
    const apiParams = { ...params }
    if (apiParams.type === 'withdraw') {
      apiParams.type = 'withdrawal'
    }
    return api.get<ApiResponse<{ transactions: any[]; total: number }>>(
      '/wallets/transactions',
      { params: apiParams },
    ).then(r => ({
      transactions: (r.data.data?.transactions ?? []).map(mapTransaction),
      total: r.data.data?.total ?? 0,
    }))
  },

  deposit: (amount: number, reference: string, description: string) =>
    api.post<ApiResponse<any>>('/wallets/deposit', { amount, reference, description })
      .then(r => mapTransaction(r.data.data)),

  withdraw: (amount: number, reference: string, description: string) =>
    api.post<ApiResponse<any>>('/wallets/withdraw', { amount, reference, description })
      .then(r => mapTransaction(r.data.data)),
}
