/**
 * A finished match can have at most one rematch request. Use that stable
 * identity to make notification handling idempotent if SSE redelivers an
 * event during reconnect or a duplicate subscription window.
 */
export function consumeRematchRequest(
  seen: Set<string>,
  matchId: unknown,
  requesterId: unknown,
): boolean {
  if (typeof matchId !== 'string' || !matchId || typeof requesterId !== 'string' || !requesterId) {
    return false
  }

  const key = `${matchId}:${requesterId}`
  if (seen.has(key)) return false
  seen.add(key)
  return true
}
