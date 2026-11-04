/**
 * Shared theme-aware styles for the authentication screens.
 *
 * Login, register, forgot-password and reset-password all render inside
 * `AuthShell`; keeping the control styling here means the fields, primary
 * action, alerts and links stay identical across those pages.
 */

/** Field: 52px tall, hairline border, surface tone above the page. */
export const authInputClass =
  'h-[52px] rounded-xl border-input bg-surface px-4 text-[15px] text-foreground shadow-none placeholder:text-muted-foreground/70 transition-colors focus-visible:border-primary/50 focus-visible:ring-1 focus-visible:ring-ring/40'

/** Brand-red primary action ("Enter the arena", "Create account", …). */
export const authPrimaryButtonClass =
  'h-[52px] w-full rounded-xl bg-primary text-[15px] font-bold text-primary-foreground shadow-none transition-colors hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-80'

/** Form-level errors and OAuth failures. */
export const authAlertClass =
  'rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive'

/** Neutral link for the opposite auth action ("Sign in", "Create an account"). */
export const authLinkClass = 'font-semibold text-foreground transition-colors hover:text-primary'

/** Brand-red inline link ("Forgot password?"). */
export const authAccentLinkClass =
  'font-semibold text-primary transition-colors hover:text-primary/80'
