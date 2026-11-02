/**
 * Where a user goes once they are signed in.
 *
 * Every authentication entry point (email/password sign-in, registration,
 * Google, session restoration, the root URL) resolves its destination here, so
 * the rule lives in one place instead of being repeated per page.
 *
 * The role comes from the user object issued by the backend and stored with the
 * session — never from a URL parameter or other client-side input. Anything
 * that is not explicitly `admin` lands on the games page.
 */

/** Landing page for players: the game lobby is the primary entry point. */
export const GAMES_PATH = '/games'

/** Landing page for administrators. */
export const ADMIN_DASHBOARD_PATH = '/admin/dashboard'

/** sessionStorage key holding a destination remembered across a sign-in redirect. */
export const PENDING_REDIRECT_KEY = 'pending_redirect'

/** Only same-origin absolute paths are followed after sign-in. */
export function sanitizeRedirect(value: string | null | undefined): string | null {
  if (!value) return null
  const trimmed = value.trim()
  if (!trimmed.startsWith('/')) return null
  // "//host" is protocol-relative and "/\host" is treated as such by some
  // browsers; both would leave the application.
  if (trimmed.startsWith('//') || trimmed.startsWith('/\\')) return null
  return trimmed
}

/** The default page for a role. Unknown or missing roles are treated as players. */
export function landingPathForRole(role: string | null | undefined): string {
  return role === 'admin' ? ADMIN_DASHBOARD_PATH : GAMES_PATH
}

/**
 * Resolves the destination after a successful sign-in: an explicitly requested
 * internal page if one was passed in, otherwise the role's default landing page.
 * An unsafe or external redirect is ignored rather than followed.
 */
export function resolvePostAuthPath(options: {
  redirect?: string | null
  role?: string | null
}): string {
  return sanitizeRedirect(options.redirect) ?? landingPathForRole(options.role)
}

/**
 * Reads a destination remembered before the user was bounced to the sign-in
 * page (`?redirect=` wins over the stored value). The value is still sanitised
 * by `resolvePostAuthPath` before use.
 */
export function readPendingRedirect(searchParam?: string | null): string | null {
  if (searchParam) return searchParam
  if (typeof sessionStorage === 'undefined') return null
  return sessionStorage.getItem(PENDING_REDIRECT_KEY)
}

/** Forgets the remembered destination once the sign-in has completed. */
export function clearPendingRedirect(): void {
  if (typeof sessionStorage === 'undefined') return
  sessionStorage.removeItem(PENDING_REDIRECT_KEY)
}
