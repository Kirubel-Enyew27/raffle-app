import type { P2PMatch } from '@/types/api'
import type { TFunction } from 'i18next'

const GAME_NAMES: Record<string, string> = {
  'tic-tac-toe': 'Tic-Tac-Toe',
  'connect-four': 'Connect Four',
  'rock-paper-scissors': 'Rock Paper Scissors',
  'ethiopian-dama': 'Ethiopian Dama',
  'crash': 'Crash',
}

const GAME_NAME_KEYS: Record<string, string> = {
  'tic-tac-toe': 'p2p.ticTacToe',
  'connect-four': 'p2p.connectFour',
  'rock-paper-scissors': 'p2p.rockPaperScissors',
  'ethiopian-dama': 'p2p.ethiopianDama',
  'crash': 'p2p.crash',
}

export const TERMINAL_MATCH_STATUSES: P2PMatch['status'][] = [
  'completed',
  'tied',
  'forfeited',
  'cancelled',
]

export const RECENT_HISTORY_LIMIT = 5

export function isTerminalMatch(match: P2PMatch): boolean {
  return TERMINAL_MATCH_STATUSES.includes(match.status)
}

export function gameName(slug: string, t?: TFunction): string {
  if (t && GAME_NAME_KEYS[slug]) {
    return t(GAME_NAME_KEYS[slug], { defaultValue: GAME_NAMES[slug] })
  }
  return (
    GAME_NAMES[slug] ??
    slug.replace(/-/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase())
  )
}

export function formatMatchDate(value?: string, locale?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(locale ?? undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function compactReference(reference?: string): string {
  if (!reference) return '—'
  return reference.length > 18
    ? `${reference.slice(0, 8)}…${reference.slice(-4)}`
    : reference
}

export function matchOutcome(
  match: P2PMatch,
  currentUserId?: string,
  t?: TFunction
): {
  label: string
  className: string
} {
  if (match.status === 'tied') {
    return {
      label: t ? t('p2p.history.draw', 'Draw') : 'Draw',
      className: 'border-border bg-muted/60 text-muted-foreground',
    }
  }
  if (match.status === 'cancelled') {
    return {
      label: t ? t('p2p.history.cancelled', 'Cancelled') : 'Cancelled',
      className: 'border-border bg-muted/60 text-muted-foreground',
    }
  }
  if (currentUserId && match.winner_user_id === currentUserId) {
    return {
      label: t ? t('p2p.history.won', 'Won') : 'Won',
      className: 'border-success/30 bg-success/10 text-success',
    }
  }
  if (currentUserId && match.winner_user_id && match.winner_user_id !== currentUserId) {
    return {
      label: t ? t('p2p.history.lost', 'Lost') : 'Lost',
      className: 'border-destructive/30 bg-destructive/10 text-destructive',
    }
  }
  return {
    label: t ? t('p2p.history.completed', 'Completed') : 'Completed',
    className: 'border-border bg-muted/60 text-muted-foreground',
  }
}
