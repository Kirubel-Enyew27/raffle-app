/**
 * Sound event tracker for the P2P match view.
 *
 * The tracker is deliberately pure with respect to React: it is fed an
 * authoritative match snapshot (plus the locally ticking countdown) and calls a
 * sink only for *transitions*. That gives three guarantees the UI depends on:
 *
 *  1. Initial mount / loading a historical match never plays an outcome sound.
 *  2. The same event delivered twice (SSE + polling, React re-render) is ignored.
 *  3. Sound playback can never throw into the render path — the sink is called
 *     inside a try/catch.
 *
 * Keeping the logic here (instead of inside effects) makes it directly testable.
 */

export interface MatchSnapshot {
  id: string
  status: string
  current_turn_user_id?: string | null
  winner_user_id?: string | null
  board_state?: unknown
  current_round?: number
}

export interface SoundSink {
  playMatchStart(): void
  playMove(): void
  playTurnAlert(): void
  playTimerTick(secondsLeft?: number): void
  playVictory(): void
  playDefeat(): void
  playDraw(): void
}

const GAME_OVER_STATUSES = ['completed', 'tied', 'forfeited', 'cancelled']

export const DEFAULT_TIMER_THRESHOLD = 5

function moveSignature(match: MatchSnapshot): string | null {
  if (!match.board_state) return null
  const round = match.current_round ? `_r${match.current_round}` : ''
  try {
    return JSON.stringify(match.board_state) + round
  } catch {
    // Circular/degenerate payloads must not break gameplay sound handling.
    return null
  }
}

export class SoundEventTracker {
  private userId?: string
  private lastMatchId?: string
  private lastStatus?: string
  private lastTurnUserId?: string | null
  private lastMoveSignature: string | null = null
  private lastTickedSecond: number | null = null
  private playedEndSoundFor: string | null = null
  private readonly sink: SoundSink
  private readonly timerThreshold: number

  constructor(sink: SoundSink, timerThreshold: number = DEFAULT_TIMER_THRESHOLD) {
    this.sink = sink
    this.timerThreshold = timerThreshold
  }

  /** The local player id is used to decide alerts vs. victory/defeat. */
  setUser(userId: string | undefined): void {
    this.userId = userId
  }

  /**
   * Feed one authoritative snapshot. Safe to call on every render/poll: only
   * genuine state transitions emit sound.
   */
  update(
    match: MatchSnapshot | undefined,
    secondsLeft = 0,
    timerThreshold: number = this.timerThreshold
  ): void {
    if (!match?.id) return

    if (this.lastMatchId !== match.id) {
      this.adoptMatch(match)
      return
    }

    const previousStatus = this.lastStatus
    this.lastStatus = match.status

    if (previousStatus && previousStatus !== 'in_progress' && match.status === 'in_progress') {
      this.play(() => this.sink.playMatchStart())
    }

    this.handleOutcome(match)
    this.handleTurn(match)
    this.handleMove(match)
    this.handleTimer(match, secondsLeft, timerThreshold)
  }

  /** Forget all state (e.g. when leaving the arena). */
  reset(): void {
    this.lastMatchId = undefined
    this.lastStatus = undefined
    this.lastTurnUserId = undefined
    this.lastMoveSignature = null
    this.lastTickedSecond = null
    this.playedEndSoundFor = null
  }

  private adoptMatch(match: MatchSnapshot): void {
    this.lastMatchId = match.id
    this.lastStatus = match.status
    this.lastTurnUserId = match.current_turn_user_id
    this.lastMoveSignature = moveSignature(match)
    this.lastTickedSecond = null
    // A match that is already over when first opened (history) must stay silent.
    this.playedEndSoundFor = GAME_OVER_STATUSES.includes(match.status) ? match.id : null
  }

  private handleOutcome(match: MatchSnapshot): void {
    if (!GAME_OVER_STATUSES.includes(match.status)) return
    if (this.playedEndSoundFor === match.id) return
    this.playedEndSoundFor = match.id

    if (match.status === 'tied') {
      this.play(() => this.sink.playDraw())
    } else if (match.winner_user_id && match.winner_user_id === this.userId) {
      this.play(() => this.sink.playVictory())
    } else if (match.winner_user_id && match.winner_user_id !== this.userId) {
      this.play(() => this.sink.playDefeat())
    }
  }

  private handleTurn(match: MatchSnapshot): void {
    if (match.status !== 'in_progress') return

    const previousTurn = this.lastTurnUserId
    this.lastTurnUserId = match.current_turn_user_id

    if (
      this.userId &&
      match.current_turn_user_id === this.userId &&
      previousTurn !== this.userId
    ) {
      this.play(() => this.sink.playTurnAlert())
    }
  }

  private handleMove(match: MatchSnapshot): void {
    if (match.status !== 'in_progress') return

    const signature = moveSignature(match)
    const previous = this.lastMoveSignature
    this.lastMoveSignature = signature

    if (previous !== null && signature !== null && previous !== signature) {
      this.play(() => this.sink.playMove())
    }
  }

  private handleTimer(match: MatchSnapshot, secondsLeft: number, timerThreshold: number): void {
    if (match.status !== 'in_progress') {
      this.lastTickedSecond = null
      return
    }

    if (secondsLeft > 0 && secondsLeft <= timerThreshold) {
      if (this.lastTickedSecond !== secondsLeft) {
        this.lastTickedSecond = secondsLeft
        this.play(() => this.sink.playTimerTick(secondsLeft))
      }
      return
    }

    if (secondsLeft <= 0) {
      this.lastTickedSecond = null
    }
  }

  private play(effect: () => void): void {
    try {
      effect()
    } catch {
      // Audio failures must never interrupt gameplay.
    }
  }
}
