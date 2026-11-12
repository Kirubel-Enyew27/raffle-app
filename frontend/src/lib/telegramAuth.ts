import { authApi } from '@/features/auth/api'
import type { LoginResponse } from '@/types/api'
import { getTelegramInitData } from './telegram'

/**
 * One Telegram sign-in per page load, shared by every caller. React StrictMode
 * mounts effects twice in development; two concurrent posts would create two
 * sessions (and, on a brand-new account, two server races) for one launch.
 */
let pending: Promise<LoginResponse> | null = null
let completed: LoginResponse | null = null

/** True when the page has a signed launch payload to authenticate with. */
export function hasTelegramLaunchData(): boolean {
  return getTelegramInitData() !== ''
}

/**
 * Exchanges the raw initData for a KWIK session. Callers should only invoke
 * this inside Telegram; outside it returns null without a request.
 *
 * The raw initData is never persisted or logged: it is a credential.
 */
export function telegramLoginOnce(): Promise<LoginResponse | null> {
  const initData = getTelegramInitData()
  if (!initData) return Promise.resolve(null)
  if (completed) return Promise.resolve(completed)
  if (pending) return pending

  pending = authApi
    .telegramLogin(initData)
    .then((session) => {
      completed = session
      return session
    })
    .catch((error: unknown) => {
      // Allow an explicit retry (for example from the sign-in screen) after a
      // transient failure; the initData itself is still allowed to be reused
      // until its auth_date expires.
      pending = null
      throw error
    })
    .finally(() => {
      pending = null
    })

  return pending
}

/** Test hook: forget the cached result between cases. */
export function resetTelegramAuthCache(): void {
  pending = null
  completed = null
}
