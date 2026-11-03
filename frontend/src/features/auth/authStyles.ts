/**
 * Shared dark-surface styles for the authentication screens.
 *
 * Login, register, forgot-password and reset-password all render inside
 * `AuthShell`; keeping the control styling here means the fields, primary
 * action, alerts and links stay identical across those pages.
 */

/** Dark field: 52px tall, hairline border, slightly lighter than the page. */
export const authInputClass =
  'h-[52px] rounded-xl border-[#26282e] bg-[#101113] px-4 text-[15px] text-white shadow-none placeholder:text-[#6f747c] transition-colors focus-visible:border-[#3d4149] focus-visible:ring-1 focus-visible:ring-[#3d4149]'

/** Brand-red primary action ("Enter the arena", "Create account", …). */
export const authPrimaryButtonClass =
  'h-[52px] w-full rounded-xl bg-[#ff5b55] text-[15px] font-bold text-white shadow-none transition-colors hover:bg-[#f44a43] focus-visible:ring-2 focus-visible:ring-[#ff5b55]/50 disabled:pointer-events-none disabled:opacity-80'

/** Form-level errors and OAuth failures. */
export const authAlertClass =
  'rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive'

/** Neutral link for the opposite auth action ("Sign in", "Create an account"). */
export const authLinkClass = 'font-semibold text-white transition-colors hover:text-[#ff6d64]'

/** Brand-red inline link ("Forgot password?"). */
export const authAccentLinkClass =
  'font-semibold text-[#f2473f] transition-colors hover:text-[#ff6d64]'
