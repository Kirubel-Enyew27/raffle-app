/**
 * Pure helper functions for computing disconnect/reconnect banner visibility
 * and countdown display from the server-supplied match state.
 *
 * Keeping these as pure functions (no React state) makes them easy to unit-
 * test without a DOM or rendering environment.
 */

import type { P2PMatch } from '@/types/api'

/**
 * Returns true when the match is in progress and the **opponent** is the
 * disconnected player (i.e. the calling user is the connected one).
 */
export function isOpponentDisconnected(match: P2PMatch | undefined, currentUserId: string | undefined): boolean {
  if (!match || match.status !== 'in_progress') return false
  if (!match.disconnect_user_id) return false
  return match.disconnect_user_id !== currentUserId
}

/**
 * Returns true when the match is in progress and **this client's user** is
 * the disconnected player and needs to reconnect.
 */
export function isSelfDisconnected(match: P2PMatch | undefined, currentUserId: string | undefined): boolean {
  if (!match || match.status !== 'in_progress') return false
  if (!currentUserId) return false
  return match.disconnect_user_id === currentUserId
}

/**
 * Computes the local countdown value from the server-supplied
 * `disconnect_seconds_left` and the elapsed seconds since the server value
 * was received.
 *
 * @param serverSecondsLeft  The `disconnect_seconds_left` from the last fetch.
 * @param elapsedSeconds     Whole seconds elapsed since that fetch.
 * @returns Clamped-to-zero remaining seconds.
 */
export function computeDisconnectCountdown(serverSecondsLeft: number, elapsedSeconds: number): number {
  return Math.max(0, serverSecondsLeft - elapsedSeconds)
}

/**
 * Returns a human-readable countdown label suitable for the banner.
 * Returns an empty string when there is no active countdown.
 */
export function disconnectCountdownLabel(secondsLeft: number): string {
  if (secondsLeft <= 0) return ''
  return `${secondsLeft}s`
}

/**
 * Returns the disconnect banner variant for the connected player's UI.
 * - 'opponent': show the opponent-disconnected orange banner
 * - 'self': show the own-reconnecting amber banner
 * - null: no banner needed
 */
export type DisconnectBannerVariant = 'opponent' | 'self' | null

export function disconnectBannerVariant(
  match: P2PMatch | undefined,
  currentUserId: string | undefined,
): DisconnectBannerVariant {
  if (isOpponentDisconnected(match, currentUserId)) return 'opponent'
  if (isSelfDisconnected(match, currentUserId)) return 'self'
  return null
}
