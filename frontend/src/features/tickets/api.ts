import { api } from '@/lib/api'
import type { ApiResponse, Raffle, Ticket } from '@/types/api'

export interface PurchaseResult {
  tickets: Ticket[]
  wallet_tx_id: string
  total_spent: number
}

export const ticketApi = {
  listRaffles: (params: { limit: number; offset: number; status?: string }) =>
    api.get<ApiResponse<{ raffles: Raffle[]; total: number }>>('/raffles', { params })
      .then(r => r.data.data),

  getRaffle: (id: string) =>
    api.get<ApiResponse<Raffle>>(`/raffles/${id}`).then(r => r.data.data),

  purchase: (raffle_id: string, quantity: number) =>
    api.post<ApiResponse<PurchaseResult>>(
      '/tickets/purchase',
      { raffle_id, quantity },
      { headers: { 'Idempotency-Key': `${raffle_id}-${quantity}-${Date.now()}` } },
    ).then(r => r.data.data),
}
