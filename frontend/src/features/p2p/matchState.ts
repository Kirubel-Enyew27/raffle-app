import type { P2PMatch } from '@/types/api'

/**
 * Preserve the newest authoritative snapshot when concurrent REST responses
 * and SSE events arrive in a different order than the server committed them.
 */
export function mergeMatchSnapshot(
  current: P2PMatch | undefined,
  incoming: P2PMatch,
): P2PMatch {
  if (!current || current.id !== incoming.id) return incoming

  const currentVersion = current.state_version
  const incomingVersion = incoming.state_version
  if (typeof currentVersion === 'number') {
    if (typeof incomingVersion === 'number' && incomingVersion > currentVersion) return incoming
    if (typeof incomingVersion === 'number' && incomingVersion < currentVersion) return current

    const currentTimestamp = Date.parse(current.updated_at ?? current.server_time ?? '')
    const incomingTimestamp = Date.parse(incoming.updated_at ?? incoming.server_time ?? '')
    // Older app instances may still update a row without advancing (or knowing
    // about) state_version. Accept only a demonstrably newer DB snapshot then.
    if (
      !Number.isFinite(currentTimestamp) ||
      !Number.isFinite(incomingTimestamp) ||
      incomingTimestamp <= currentTimestamp
    ) {
      return current
    }
    return incoming
  }

  return incoming
}

/** True when an event snapshot proves that at least one committed version was missed. */
export function hasMatchVersionGap(
  current: P2PMatch | undefined,
  incoming: P2PMatch,
): boolean {
  return !!(
    current &&
    current.id === incoming.id &&
    typeof current.state_version === 'number' &&
    typeof incoming.state_version === 'number' &&
    incoming.state_version > current.state_version + 1
  )
}
