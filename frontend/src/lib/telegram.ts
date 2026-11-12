/**
 * Telegram Mini App integration and deep-linking utilities.
 */

export type TelegramHapticStyle = 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'
export type TelegramNotificationType = 'error' | 'success' | 'warning'

/** The subset of the official Telegram WebApp API KWIK uses. */
export interface TelegramWebApp {
  initData: string
  platform: string
  version: string
  colorScheme: 'light' | 'dark'
  themeParams?: Record<string, string>
  isExpanded: boolean
  viewportHeight?: number
  viewportStableHeight?: number
  safeAreaInset?: { top: number; bottom: number; left: number; right: number }
  contentSafeAreaInset?: { top: number; bottom: number; left: number; right: number }
  initDataUnsafe?: {
    user?: {
      id: number
      first_name: string
      last_name?: string
      username?: string
      language_code?: string
      photo_url?: string
    }
    start_param?: string
  }
  ready: () => void
  expand: () => void
  close: () => void
  openLink: (url: string, options?: { try_instant_view?: boolean }) => void
  openTelegramLink: (url: string) => void
  setHeaderColor?: (color: string) => void
  setBackgroundColor?: (color: string) => void
  setBottomBarColor?: (color: string) => void
  onEvent?: (event: string, handler: () => void) => void
  offEvent?: (event: string, handler: () => void) => void
  BackButton?: {
    isVisible: boolean
    show: () => void
    hide: () => void
    onClick: (handler: () => void) => void
    offClick: (handler: () => void) => void
  }
  HapticFeedback?: {
    impactOccurred: (style: TelegramHapticStyle) => void
    notificationOccurred: (type: TelegramNotificationType) => void
    selectionChanged: () => void
  }
}

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp }
  }
}

/** The raw WebApp object, if the telegram.org script has loaded. */
export function getTelegramWebApp(): TelegramWebApp | undefined {
  if (typeof window === 'undefined') return undefined
  return window.Telegram?.WebApp
}

/**
 * The raw, signed initData string. This is the only Telegram value the backend
 * accepts as proof of identity; initDataUnsafe is never sent or trusted.
 *
 * When the SDK has not populated `initData` yet (older clients, script timing)
 * the launch parameters in the URL fragment are checked instead.
 */
export function getTelegramInitData(): string {
  const webApp = getTelegramWebApp()
  if (webApp?.initData) return webApp.initData

  if (typeof window !== 'undefined') {
    for (const source of [window.location.hash.replace(/^#/, ''), window.location.search.replace(/^\?/, '')]) {
      if (!source) continue
      const params = new URLSearchParams(source)
      const raw = params.get('tgWebAppData')
      if (raw) return raw
    }
  }
  return ''
}

/** Check if this page was launched inside Telegram with a signed payload. */
export function isTelegramWebApp(): boolean {
  if (getTelegramInitData() !== '') return true
  // A payload is present but the SDK could not read it yet.
  const webApp = getTelegramWebApp()
  return !!webApp?.initDataUnsafe?.user
}

const KWIK_CHROME = { dark: '#0a0a0b', light: '#f7f8fa' } as const

/** Keep Telegram's header/background in step with KWIK's resolved theme. */
export function syncTelegramChrome(isDark: boolean): void {
  const webApp = getTelegramWebApp()
  if (!webApp || !isTelegramWebApp()) return
  const color = isDark ? KWIK_CHROME.dark : KWIK_CHROME.light
  try {
    webApp.setHeaderColor?.(color)
    webApp.setBackgroundColor?.(color)
    webApp.setBottomBarColor?.(color)
  } catch {
    // Unsupported in older clients; the CSS theme still applies.
  }
}

type TelegramInsets = { top: number; bottom: number; left: number; right: number }

function safeInsets(value?: Partial<TelegramInsets> | null): TelegramInsets {
  return {
    top: value?.top ?? 0,
    bottom: value?.bottom ?? 0,
    left: value?.left ?? 0,
    right: value?.right ?? 0,
  }
}

/**
 * Combine device and Telegram UI insets. Exported so the sum can be tested
 * without a Telegram client; the layout depends on it being a sum, not a
 * choice between the two measurements.
 */
export function computeTelegramInsets(webApp: Pick<TelegramWebApp, 'safeAreaInset' | 'contentSafeAreaInset'>): {
  safe: TelegramInsets
  content: TelegramInsets
  total: TelegramInsets
} {
  const safe = safeInsets(webApp.safeAreaInset)
  const content = safeInsets(webApp.contentSafeAreaInset)
  return {
    safe,
    content,
    total: {
      top: safe.top + content.top,
      bottom: safe.bottom + content.bottom,
      left: safe.left + content.left,
      right: safe.right + content.right,
    },
  }
}

/**
 * Publish the viewport and both Telegram safe areas as CSS variables.
 *
 * They are two different measurements and must be added together:
 * - safeAreaInset covers device UI (status bar, notch, gesture bar);
 * - contentSafeAreaInset covers Telegram's own controls drawn over the WebView
 *   (the Close / expand header in fullscreen mode).
 *
 * Older clients (pre Bot API 8.0) do not expose either object, so both fall
 * back to zero and the Mini App keeps the regular viewport. The `tma` class is
 * the switch that tells the stylesheet to consume these values, so normal
 * browser sessions are untouched.
 */
function applyViewportVariables(webApp: TelegramWebApp): void {
  if (typeof document === 'undefined' || !isTelegramWebApp()) return
  const root = document.documentElement
  root.classList.add('tma')
  if (webApp.viewportHeight) root.style.setProperty('--tg-viewport-height', `${webApp.viewportHeight}px`)
  if (webApp.viewportStableHeight) {
    root.style.setProperty('--tg-viewport-stable-height', `${webApp.viewportStableHeight}px`)
  }

  const { safe, content, total } = computeTelegramInsets(webApp)
  const sides = ['top', 'bottom', 'left', 'right'] as const

  for (const side of sides) {
    root.style.setProperty(`--tg-safe-area-inset-${side}`, `${safe[side]}px`)
    root.style.setProperty(`--tg-content-safe-area-inset-${side}`, `${content[side]}px`)
    root.style.setProperty(`--tg-total-inset-${side}`, `${total[side]}px`)
  }
}

/**
 * Subscribe to viewport/theme changes. Returns an unsubscribe function.
 * Called from the shell so the layout follows expansion, rotation and the
 * Telegram keyboard without a reload.
 */
export function subscribeTelegramViewport(onChange?: () => void): () => void {
  const webApp = getTelegramWebApp()
  if (!webApp || !isTelegramWebApp() || !webApp.onEvent) return () => undefined

  const update = () => {
    applyViewportVariables(webApp)
    onChange?.()
  }
  update()
  webApp.onEvent('viewportChanged', update)
  webApp.onEvent('themeChanged', update)
  webApp.onEvent('safeAreaChanged', update)
  webApp.onEvent('contentSafeAreaChanged', update)
  webApp.onEvent('fullscreenChanged', update)

  return () => {
    webApp.offEvent?.('viewportChanged', update)
    webApp.offEvent?.('themeChanged', update)
    webApp.offEvent?.('safeAreaChanged', update)
    webApp.offEvent?.('contentSafeAreaChanged', update)
    webApp.offEvent?.('fullscreenChanged', update)
  }
}

/**
 * Initialize the Telegram Mini App shell. Runs before React paints so the app
 * never renders at Telegram's collapsed height first.
 */
export function initTelegramWebApp(): void {
  const webApp = getTelegramWebApp()
  // The SDK script is loaded on every page, including regular browsers, where
  // it exposes a WebApp object with no initData. Only enter Mini App mode when
  // Telegram actually launched the page.
  if (!webApp || !isTelegramWebApp()) return
  try {
    webApp.ready()
    // Expanded — not fullscreen. Fullscreen lets the WebView paint underneath
    // Telegram's Close/expand controls, and KWIK has its own sticky header and
    // bottom navigation that need a predictable viewport. The safe-area code
    // below still keeps the layout correct if the client hands us insets.
    webApp.expand()

    applyViewportVariables(webApp)
    const isDark = webApp.colorScheme !== 'light' || document.documentElement.classList.contains('dark')
    syncTelegramChrome(isDark)
  } catch {
    // Ignore when not in Telegram
  }
}

/** Light haptic tap for selection changes. No-op outside Telegram. */
export function hapticSelection(): void {
  if (!isTelegramWebApp()) return
  try {
    getTelegramWebApp()?.HapticFeedback?.selectionChanged()
  } catch {
    // Haptics are best effort.
  }
}

/** Haptic impact for moves and drops. */
export function hapticImpact(style: TelegramHapticStyle = 'light'): void {
  if (!isTelegramWebApp()) return
  try {
    getTelegramWebApp()?.HapticFeedback?.impactOccurred(style)
  } catch {
    // Haptics are best effort.
  }
}

/** Haptic notification for wins, losses and errors. */
export function hapticNotification(type: TelegramNotificationType): void {
  if (!isTelegramWebApp()) return
  try {
    getTelegramWebApp()?.HapticFeedback?.notificationOccurred(type)
  } catch {
    // Haptics are best effort.
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
