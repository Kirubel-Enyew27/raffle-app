/**
 * The single source of truth for "what is happening with this match".
 *
 * Transport connectivity, room membership, readiness and gameplay are four
 * different things. Conflating them is what made a client that had joined the
 * room successfully still render "Connecting…" while the server had already
 * started the match: the badge only knew about the SSE socket, and nothing
 * noticed that the board had stopped receiving state.
 *
 * This module derives one explicit state from those inputs so every component
 * agrees on it. It is pure so the transitions can be tested without a browser.
 */

import type { SSEStatus } from '@/lib/sseClient'
import type { TFunction } from 'i18next'

/** Transport status of the per-match realtime stream. */
export type MatchTransport = SSEStatus

/**
 * Effective lifecycle state of a match for one player.
 *
 * - `disconnected`  no usable transport and no authoritative state
 * - `connecting`    transport opening, no authoritative state yet
 * - `joining_room`  state loaded, our seat is not recorded yet
 * - `waiting_for_opponent` seated, nobody else has joined
 * - `ready`         both players seated, waiting for readiness
 * - `starting`      both players ready, start not yet observed
 * - `in_progress`   match running with a live transport
 * - `reconnecting`  match running, transport lost — state must be resynced
 * - `failed`        transport gave up; recovery action required
 * - `completed`     terminal match state (result wins over transport state)
 */
export type MatchConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'joining_room'
  | 'waiting_for_opponent'
  | 'ready'
  | 'starting'
  | 'in_progress'
  | 'reconnecting'
  | 'failed'
  | 'completed'

/** Statuses the backend uses on `p2p_matches.status`. */
export type BackendMatchStatus =
  | 'waiting'
  | 'ready'
  | 'in_progress'
  | 'completed'
  | 'tied'
  | 'forfeited'
  | 'cancelled'

export interface MatchConnectionInput {
  /** Stream status for `p2p_match:<id>`. */
  transport: MatchTransport
  /** True once an authoritative match snapshot has been loaded. */
  hasState: boolean
  /** Backend match status, or null before the first snapshot. */
  matchStatus: BackendMatchStatus | null | undefined
  /** True when the current user has a seat in the match. */
  isParticipant: boolean
  /** Number of players currently seated (0, 1 or 2). */
  playersPresent: number
  /** True when every seated player has readied up. */
  allReady: boolean
  /**
   * Age in milliseconds of the last authoritative update. Optional; used to
   * flag a stream that is technically "live" but has gone quiet mid-match.
   */
  msSinceLastSync?: number
  /**
   * Precomputed staleness flag, for callers that already run a measuring
   * timer (rendering itself must not read the clock). Wins over
   * `msSinceLastSync` when provided.
   */
  stale?: boolean
  /** A silent stream is considered stale after this long. */
  staleAfterMs?: number
}

export interface MatchConnectionView {
  state: MatchConnectionState
  /** Transport reached the server. Does NOT by itself mean the match is playable. */
  isTransportLive: boolean
  /** Both players are present and the transport is live. */
  isReady: boolean
  /** The board may accept moves. */
  canPlay: boolean
  /** Authoritative state must be refetched (stream lost, stream failed, or stale). */
  needsSync: boolean
  /** Human readable label for the connection badge. */
  label: string
  /** Explanation for the current state, shown in a banner when degraded. */
  description: string
}

const TERMINAL: BackendMatchStatus[] = ['completed', 'tied', 'forfeited', 'cancelled']

export function isTerminalStatus(status: BackendMatchStatus | null | undefined): boolean {
  return !!status && TERMINAL.includes(status)
}

const DEFAULT_STALE_AFTER_MS = 8_000

/**
 * Derives the effective connection state. Precedence, highest first:
 *   1. A finished match is finished, whatever the transport is doing.
 *   2. A live transport tells the truth about connectivity.
 *   3. Without a live transport, a match in progress is "reconnecting" (never
 *      "Connecting…" forever) and a dead transport is "failed".
 */
export function deriveMatchConnection(input: MatchConnectionInput, t?: TFunction): MatchConnectionView {
  const {
    transport,
    hasState,
    matchStatus,
    isParticipant,
    playersPresent,
    allReady,
    msSinceLastSync,
    stale: staleOverride,
    staleAfterMs = DEFAULT_STALE_AFTER_MS,
  } = input

  const isTransportLive = transport === 'live'
  const terminal = isTerminalStatus(matchStatus)
  const stale = staleOverride ?? (isTransportLive && (msSinceLastSync ?? 0) > staleAfterMs)
  const needsSync = !isTransportLive || stale

  const tr = (key: string, fallback: string): string => (t ? t(key, fallback) : fallback)

  const build = (
    state: MatchConnectionState,
    label: string,
    description: string,
    opts: { canPlay?: boolean; isReady?: boolean } = {},
  ): MatchConnectionView => ({
    state,
    isTransportLive,
    isReady: opts.isReady ?? false,
    canPlay: opts.canPlay ?? false,
    needsSync,
    label,
    description,
  })

  // 1. A terminal match is terminal, even if the stream is down: the result is
  //    already known and the player must be able to see it.
  if (terminal) {
    return {
      ...build(
        'completed',
        tr('p2p.connection.finished', 'Match finished'),
        tr('p2p.connection.matchOver', 'This match is over.')
      ),
      // The result is already authoritative; a dropped stream must not keep
      // polling a finished match.
      needsSync: false,
    }
  }

  // 2. Transport is healthy: the match status drives the state.
  if (isTransportLive) {
    switch (matchStatus) {
      case undefined:
      case null:
        return build(
          'connecting',
          tr('p2p.connection.connecting', 'Connecting'),
          tr('p2p.connection.loadingMatch', 'Loading the match…')
        )
      case 'waiting':
        return isParticipant
          ? build(
              'waiting_for_opponent',
              tr('p2p.connection.live', 'Live'),
              tr('p2p.connection.waitingOpponent', 'Waiting for an opponent to join.')
            )
          : build(
              'joining_room',
              tr('p2p.connection.joining', 'Joining'),
              tr('p2p.connection.joiningRoom', 'Joining the room…')
            )
      case 'ready':
        if (playersPresent < 2) {
          return build(
            'waiting_for_opponent',
            tr('p2p.connection.live', 'Live'),
            tr('p2p.connection.waitingOpponent', 'Waiting for an opponent to join.')
          )
        }
        return allReady
          ? build(
              'starting',
              tr('p2p.connection.starting', 'Starting…'),
              tr('p2p.connection.bothReadyStarting', 'Both players are ready — the match is starting.')
            )
          : build(
              'ready',
              tr('p2p.connection.live', 'Live'),
              tr('p2p.connection.bothJoinedReadyUp', 'Both players have joined. Ready up to begin.')
            )
      case 'in_progress':
        return build(
          'in_progress',
          tr('p2p.connection.live', 'Live'),
          tr('p2p.connection.connectedLive', 'You are connected to the live match.'),
          {
            canPlay: isParticipant,
            isReady: playersPresent >= 2,
          }
        )
      default:
        return build(
          'ready',
          tr('p2p.connection.live', 'Live'),
          tr('p2p.connection.bothJoinedReadyUp', 'Waiting for the match to start.')
        )
    }
  }

  // 3. No live transport. Never claim the match is fine.
  if (transport === 'closed' && !hasState) {
    return build(
      'disconnected',
      tr('p2p.connection.disconnected', 'Disconnected'),
      tr('p2p.connection.noConnection', 'No connection to the match.')
    )
  }

  if (transport === 'failed') {
    return build(
      'failed',
      tr('p2p.connection.connectionFailed', 'Connection failed'),
      tr('p2p.connection.failedDesc', 'The live connection could not be established. Retry to resync the match.')
    )
  }

  if (matchStatus === 'in_progress') {
    return build(
      'reconnecting',
      tr('p2p.connection.reconnecting', 'Reconnecting'),
      tr('p2p.connection.reconnectingDesc', 'Reconnecting to the live match. Moves are safe on the server.')
    )
  }

  if (!hasState) {
    return build(
      'connecting',
      tr('p2p.connection.connecting', 'Connecting'),
      tr('p2p.connection.loadingMatch', 'Connecting to the match…')
    )
  }

  return build(
    'reconnecting',
    tr('p2p.connection.reconnecting', 'Reconnecting'),
    tr('p2p.connection.restoring', 'Restoring the connection to the match…')
  )
}
