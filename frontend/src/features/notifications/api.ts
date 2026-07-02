import { api } from '@/lib/api'
import type { ApiResponse } from '@/types/api'

export interface NotificationItem {
  ID: string
  UserID: string
  Channel: 'email' | 'in_app'
  Event: string
  Subject: string
  Body: string
  Status: 'pending' | 'sent' | 'failed'
  ReadAt?: string | null
  CreatedAt: string
}

export const notificationsApi = {
  list: (params: { limit?: number; offset?: number } = {}) =>
    api.get<ApiResponse<{ items: NotificationItem[]; total: number }>>('/notifications', { params }).then(r => r.data.data),

  unread: () =>
    api.get<ApiResponse<{ unread: number }>>('/notifications/unread').then(r => r.data.data),

  markRead: (id: string) =>
    api.post<ApiResponse<null>>(`/notifications/${id}/read`).then(r => r.data.data),
}
