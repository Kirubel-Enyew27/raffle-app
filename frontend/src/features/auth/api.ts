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

  /**
   * Revokes the refresh token server-side so a signed-out session cannot be
   * replayed. Idempotent: an unknown or already-expired token is a no-op.
   */
  logout: (refreshToken: string) =>
    api.post('/auth/logout', { refresh_token: refreshToken }).then(r => r.data),

  /** Which external sign-in providers this server has configured. */
  providers: () =>
    api.get<ApiResponse<{ google: boolean }>>('/auth/providers')
      .then(r => r.data.data),

  /**
   * Redeems the one-time code from a Google callback for the session it holds.
   * The code is single use and expires after a minute.
   */
  exchangeGoogleCode: (code: string) =>
    api.post<ApiResponse<LoginResponse & { redirect?: string }>>('/auth/google/exchange', { code })
      .then(r => r.data.data),

  /** Whether the signed-in account has Google connected. */
  googleLinkStatus: () =>
    api.get<ApiResponse<{ linked: boolean }>>('/auth/google/link')
      .then(r => r.data.data),

  /**
   * Starts connecting Google to the signed-in account. The current password is
   * required: a session alone is not enough to add a new sign-in method.
   */
  startGoogleLink: (password: string) =>
    api.post<ApiResponse<{ url: string }>>('/auth/google/link', { password })
      .then(r => r.data.data),
}
