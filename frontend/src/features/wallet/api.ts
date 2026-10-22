import { api } from '@/lib/api'
import type { ApiResponse, ReceiptDepositResult, Wallet, WalletTransaction } from '@/types/api'

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

  extractReceipt: (file: File, method: string) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('method', method)
    return api.post<ApiResponse<{ transaction_id: string }>>('/wallets/deposit/extract', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then(r => r.data.data)
  },

  verifyAndDeposit: (params: { method: string; transaction_id?: string; file?: File }) => {
    if (params.file) {
      const formData = new FormData()
      formData.append('method', params.method)
      if (params.transaction_id) {
        formData.append('transaction_id', params.transaction_id)
      }
      formData.append('file', params.file)
      return api.post<ApiResponse<ReceiptDepositResult>>('/wallets/deposit/verify', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      }).then(r => ({
        transaction: mapTransaction(r.data.data.transaction),
        receipt: r.data.data.receipt,
      }))
    }

    return api.post<ApiResponse<ReceiptDepositResult>>('/wallets/deposit/verify', {
      method: params.method,
      transaction_id: params.transaction_id,
    }).then(r => ({
      transaction: mapTransaction(r.data.data.transaction),
      receipt: r.data.data.receipt,
    }))
  },

  createDepositIntent: (amount: number, payment_method: string) =>
    api.post<ApiResponse<any>>('/wallets/deposits/intent', { amount, payment_method })
      .then(r => r.data.data),

  getDeposit: (id: string) =>
    api.get<ApiResponse<any>>(`/wallets/deposits/${id}`)
      .then(r => r.data.data),

  verifyDepositIntent: (depositId: string, params: { transaction_id?: string; receipt_url?: string; file?: File }) => {
    if (params.file) {
      const formData = new FormData()
      if (params.transaction_id) formData.append('transaction_id', params.transaction_id)
      if (params.receipt_url) formData.append('receipt_url', params.receipt_url)
      formData.append('file', params.file)
      return api.post<ApiResponse<{ deposit: any; transaction: any }>>(`/wallets/deposits/${depositId}/verify`, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      }).then(r => ({
        deposit: r.data.data.deposit,
        transaction: mapTransaction(r.data.data.transaction),
      }))
    }
    return api.post<ApiResponse<{ deposit: any; transaction: any }>>(`/wallets/deposits/${depositId}/verify`, {
      transaction_id: params.transaction_id,
      receipt_url: params.receipt_url,
    }).then(r => ({
      deposit: r.data.data.deposit,
      transaction: mapTransaction(r.data.data.transaction),
    }))
  },

  withdraw: (amount: number, reference: string, description: string) =>
    api.post<ApiResponse<any>>('/wallets/withdraw', { amount, reference, description })
      .then(r => mapTransaction(r.data.data)),
}
