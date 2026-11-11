/**
 * Telegram Mini App integration and deep-linking utilities.
 */

declare global {
  interface Window {
    Telegram?: {
      WebApp?: {
        ready: () => void
        expand: () => void
        close: () => void
        openLink: (url: string, options?: { try_instant_view?: boolean }) => void
        openTelegramLink: (url: string) => void
        isExpanded?: boolean
        viewportHeight?: number
        colorScheme?: 'light' | 'dark'
        initDataUnsafe?: {
          user?: {
            id: number
            first_name: string
            last_name?: string
            username?: string
            language_code?: string
          }
          start_param?: string
        }
      }
    }
  }
}

/** Check if running inside Telegram WebApp */
export function isTelegramWebApp(): boolean {
  return typeof window !== 'undefined' && !!window.Telegram?.WebApp?.initDataUnsafe
}

/** Initialize Telegram WebApp viewport */
export function initTelegramWebApp(): void {
  try {
    if (window.Telegram?.WebApp) {
      window.Telegram.WebApp.ready()
      window.Telegram.WebApp.expand()
    }
  } catch {
    // Ignore when not in Telegram
  }
}

/**
 * Extract invite code or start parameter from:
 * 1. Telegram WebApp start_param (e.g. join_ABC9X2 or ABC9X2)
 * 2. URL search params (tgWebAppStartParam, join, code, invite)
 * 3. Hash params
 */
export function extractStartInviteCode(): string | null {
  if (typeof window === 'undefined') return null

  // 1. Telegram WebApp start parameter
  const tgParam = window.Telegram?.WebApp?.initDataUnsafe?.start_param
  if (tgParam) {
    const cleaned = cleanCodeParam(tgParam)
    if (cleaned) return cleaned
  }

  // 2. Search parameters in current URL
  const url = new URL(window.location.href)
  const candidates = [
    url.searchParams.get('tgWebAppStartParam'),
    url.searchParams.get('join'),
    url.searchParams.get('code'),
    url.searchParams.get('invite'),
  ]

  for (const c of candidates) {
    if (c) {
      const cleaned = cleanCodeParam(c)
      if (cleaned) return cleaned
    }
  }

  // 3. Fallback: check location hash (e.g. #join=ABC9X2 or #tgWebAppStartParam=...)
  if (window.location.hash) {
    const hashParams = new URLSearchParams(window.location.hash.replace(/^#/, ''))
    for (const key of ['tgWebAppStartParam', 'join', 'code', 'invite']) {
      const val = hashParams.get(key)
      if (val) {
        const cleaned = cleanCodeParam(val)
        if (cleaned) return cleaned
      }
    }
  }

  return null
}

function cleanCodeParam(param: string): string | null {
  const trimmed = param.trim()
  if (!trimmed) return null
  // Strip prefixes like "join_" or "invite_"
  const code = trimmed.replace(/^(join_|invite_)/i, '').trim().toUpperCase()
  if (code.length >= 4 && code.length <= 16 && /^[A-Z0-9]+$/.test(code)) {
    return code
  }
  return null
}

/**
 * Build a valid invitation URL.
 * Prefers Telegram Mini App deep link if bot username is configured,
 * otherwise produces canonical Web link.
 */
export function buildInviteUrl(inviteCode: string): string {
  const clean = inviteCode.trim().toUpperCase()
  const botUsername = (
    (typeof import.meta !== 'undefined' && import.meta.env?.VITE_TELEGRAM_BOT_USERNAME) ||
    ''
  ).replace(/^@/, '')
  const appName =
    (typeof import.meta !== 'undefined' && import.meta.env?.VITE_TELEGRAM_APP_NAME) || ''

  if (botUsername) {
    if (appName) {
      return `https://t.me/${botUsername}/${appName}?startapp=join_${clean}`
    }
    return `https://t.me/${botUsername}?startapp=join_${clean}`
  }

  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}/games?join=${clean}`
}

/**
 * Build the web fallback URL (always directly openable in browser).
 */
export function buildWebInviteUrl(inviteCode: string): string {
  const clean = inviteCode.trim().toUpperCase()
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}/games?join=${clean}`
}

/**
 * Generate formatted challenge message.
 */
export function buildInviteMessage(
  gameTitle: string,
  inviteCode: string,
  inviteUrl: string,
  t?: (key: string, options?: Record<string, unknown>) => string,
): string {
  const tr = (key: string, fallback: string, vars?: Record<string, unknown>) =>
    t ? t(key, { ...vars, defaultValue: fallback }) : fallback
  return [
    tr('invite.shareTitle', `🎮 Let's play ${gameTitle} together!`, { game: gameTitle }),
    '',
    tr('invite.shareBody', "I've challenged you to a match on KWIK."),
    tr('invite.joinHere', `Join me here: ${inviteUrl}`, { url: inviteUrl }),
    '',
    tr('invite.roomCode', `Room Code: ${inviteCode.trim().toUpperCase()}`, {
      code: inviteCode.trim().toUpperCase(),
    }),
    tr('invite.ready', 'Ready to play? ⚔️'),
  ].join('\n')
}

/**
 * Open the Telegram share dialogue in a new tab.
 *
 * The dialogue must never replace the current view: after sending the invite
 * the sender has to get back to the challenge modal they started from. So a new
 * browser tab is always preferred, and the Telegram in-app browser is used only
 * when the popup is blocked.
 */
export function shareViaTelegram(inviteUrl: string, message: string): void {
  if (typeof window === 'undefined') return

  const shareUrl = `https://t.me/share/url?url=${encodeURIComponent(inviteUrl)}&text=${encodeURIComponent(message)}`

  const tab = window.open(shareUrl, '_blank')
  if (tab) {
    try {
      // Detach the opener so the share page cannot navigate the app away.
      tab.opener = null
    } catch {
      // Cross-origin handle; nothing else to detach.
    }
    return
  }

  // Popup blocked: open the dialogue externally instead of navigating the
  // Mini App away from the challenge modal with openTelegramLink.
  if (window.Telegram?.WebApp?.openLink) {
    window.Telegram.WebApp.openLink(shareUrl)
    return
  }

  window.Telegram?.WebApp?.openTelegramLink?.(shareUrl)
}

/**
 * Try native Web Share API, with fallback to clipboard.
 */
export async function shareLinkOrCopy(options: {
  title: string
  text: string
  url: string
}): Promise<'shared' | 'copied' | 'failed'> {
  if (typeof navigator !== 'undefined' && navigator.share) {
    try {
      await navigator.share(options)
      return 'shared'
    } catch (err: unknown) {
      // User cancelled share dialog
      if ((err as Error)?.name === 'AbortError') {
        return 'failed'
      }
      // Other error: fallback to copy
    }
  }

  const copied = await copyToClipboard(options.url)
  return copied ? 'copied' : 'failed'
}

/**
 * Safe clipboard copy with fallback for older browsers.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Fall through to textarea execCommand fallback
    }
  }

  try {
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.left = '-999999px'
    textArea.style.top = '-999999px'
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()
    const successful = document.execCommand('copy')
    textArea.remove()
    return successful
  } catch {
    return false
  }
}
