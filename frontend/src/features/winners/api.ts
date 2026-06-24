import { api } from '@/lib/api'
import type { ApiResponse, WinnerDetail, WinningTicket, DrawVerification } from '@/types/api'

export const winnerApi = {
  list: (limit = 20, offset = 0) =>
    api.get<ApiResponse<{ winners: WinnerDetail[]; total: number; limit: number; offset: number }>>(
      '/winners', { params: { limit, offset } },
    ).then(r => r.data.data),

  get: (id: string) =>
    api.get<ApiResponse<WinnerDetail>>(`/winners/${id}`).then(r => r.data.data),

  getWinningTicket: (id: string) =>
    api.get<ApiResponse<WinningTicket>>(`/winners/${id}/ticket`).then(r => r.data.data),

  getDrawVerification: (id: string) =>
    api.get<ApiResponse<DrawVerification>>(`/winners/${id}/verification`).then(r => r.data.data),
}
