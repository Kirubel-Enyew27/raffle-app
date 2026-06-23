import { api } from '@/lib/api'
import type { ApiResponse, LoginResponse, User } from '@/types/api'

export const authApi = {
  login: (email: string, password: string) =>
    api.post<ApiResponse<LoginResponse>>('/auth/login', { email, password })
      .then(r => r.data.data),

  register: (email: string, password: string) =>
    api.post<ApiResponse<User>>('/auth/register', { email, password })
      .then(r => r.data.data),

  changePassword: (old_password: string, new_password: string) =>
    api.post('/auth/change-password', { old_password, new_password }),
}
