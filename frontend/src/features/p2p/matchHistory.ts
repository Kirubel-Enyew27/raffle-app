import type { P2PMatch } from '@/types/api'

const GAME_NAMES: Record<string, string> = {
  'tic-tac-toe': 'Tic-Tac-Toe',
  'connect-four': 'Connect Four',
  'rock-paper-scissors': 'Rock Paper Scissors',
  'ethiopian-dama': 'Ethiopian Dama',
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

export function gameName(slug: string): string {
  return (
    GAME_NAMES[slug] ??
    slug.replace(/-/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase())
  )
}

export function formatMatchDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(undefined, {
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

export function matchOutcome(match: P2PMatch, currentUserId?: string): {
  label: string
  className: string
} {
  if (match.status === 'tied') {
    return {
      label: 'Draw',
      className: 'border-border bg-muted/60 text-muted-foreground',
    }
  }
  if (match.status === 'cancelled') {
    return {
      label: 'Cancelled',
      className: 'border-border bg-muted/60 text-muted-foreground',
    }
  }
  if (currentUserId && match.winner_user_id === currentUserId) {
    return {
      label: 'Won',
      className: 'border-success/30 bg-success/10 text-success',
    }
  }
  if (currentUserId && match.winner_user_id && match.winner_user_id !== currentUserId) {
    return {
      label: 'Lost',
      className: 'border-destructive/30 bg-destructive/10 text-destructive',
    }
  }
  return {
    label: 'Completed',
    className: 'border-border bg-muted/60 text-muted-foreground',
  }
}
