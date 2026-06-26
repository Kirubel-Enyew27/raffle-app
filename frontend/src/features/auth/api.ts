import { api } from '@/lib/api'
import type { ApiResponse, LoginResponse, User } from '@/types/api'

export const authApi = {
  login: (identifier: string, password: string) =>
    api.post<ApiResponse<LoginResponse>>('/auth/login', { identifier, password })
      .then(r => r.data.data),

  register: (email: string, password: string, fullName?: string, phone?: string) =>
    api.post<ApiResponse<User>>('/auth/register', { email, password, full_name: fullName, phone })
      .then(r => r.data.data),

  changePassword: (old_password: string, new_password: string) =>
    api.post('/auth/change-password', { old_password, new_password }),
}
