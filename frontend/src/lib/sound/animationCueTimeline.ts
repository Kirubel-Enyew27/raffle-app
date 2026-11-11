/**
 * Animation-driven sound cue timeline for the P2P games.
 *
 * Gameplay sounds used to be emitted by `SoundEventTracker` from the
 * authoritative match snapshot, which meant they waited for the REST response
 * or SSE event and then played in a React effect — on the network timeline,
 * not the animation timeline. This module gives the boards a small scheduler
 * so a cue can be fired at a specific animation milestone instead.
 *
 * The timeline is deliberately framework-free and side-effect-free apart from
 * its injected sink, which keeps the sequencing directly testable:
 *
 *  - `playOnce` is idempotent per key, so re-renders, optimistic→confirmed
 *    snapshots and duplicate SSE deliveries cannot replay a cue.
 *  - `schedule` derives a delayed milestone from the central `ANIMATION_MS`
 *    table instead of an arbitrary magic number.
 *  - `clearPending` / `reset` guarantee an interrupted animation (rollback,
 *    navigation, new match) never leaves a late sound queued.
 */

/** Durations of the board animations declared in `index.css` (ms). */
export const ANIMATION_MS = {
  tttMarkerPlace: 240,
  c4TokenDrop: 250,
  damaSlide: 210,
  damaArrive: 190,
  damaPromote: 260,
  rpsSelect: 180,
  rpsReveal: 220,
} as const

/**
 * Extra time kept after the Dama slide transition ends before the temporary
 * overlay is removed, so the arriving piece cannot be unmounted mid-transition.
 */
export const DAMA_HOP_CLEANUP_GRACE_MS = 15

export type AnimationCue =
  | 'move'
  | 'drop'
  | 'land'
  | 'capture'
  | 'promotion'
  | 'select'
  | 'reveal'

export interface AnimationCueSink {
  move(): void
  drop(): void
  land(): void
  capture(): void
  promotion(): void
  select(): void
  reveal(): void
}

type TimerHandle = ReturnType<typeof setTimeout>

interface TimelineOptions {
  setTimer?: (handler: () => void, delayMs: number) => TimerHandle
  clearTimer?: (handle: TimerHandle) => void
}

/** Keeps the idempotency set bounded across a long match. */
const MAX_REMEMBERED_KEYS = 256

/**
 * Returns `ms` as-is unless the user prefers reduced motion, in which case the
 * visual milestone happens immediately and the sound must not wait for an
 * animation that is not running.
 */
export function animationDelay(ms: number): number {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return ms
  try {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : ms
  } catch {
    return ms
  }
}

export class AnimationCueTimeline {
  private readonly sink: AnimationCueSink
  private readonly setTimer: (handler: () => void, delayMs: number) => TimerHandle
  private readonly clearTimer: (handle: TimerHandle) => void
  private readonly played = new Set<string>()
  private readonly pending = new Map<string, TimerHandle>()

  constructor(sink: AnimationCueSink, options: TimelineOptions = {}) {
    this.sink = sink
    this.setTimer =
      options.setTimer ?? ((handler, delayMs) => setTimeout(handler, delayMs))
    this.clearTimer = options.clearTimer ?? ((handle) => clearTimeout(handle))
  }

  /** Emits a cue immediately. Used for direct user gestures. */
  emit(cue: AnimationCue): void {
    this.run(cue)
  }

  /** Emits a cue once per key and returns whether it actually played. */
  playOnce(key: string, cue: AnimationCue): boolean {
    if (this.played.has(key)) return false
    this.remember(key)
    this.run(cue)
    return true
  }

  /**
   * Plays a cue after `delayMs`, unless this key already played. Re-scheduling
   * the same key replaces the pending timer instead of stacking sounds.
   */
  schedule(key: string, cue: AnimationCue, delayMs: number): boolean {
    if (this.played.has(key)) return false
    this.cancel(key)
    const timer = this.setTimer(() => {
      this.pending.delete(key)
      this.playOnce(key, cue)
    }, Math.max(0, delayMs))
    this.pending.set(key, timer)
    return true
  }

  /** Cancels one scheduled cue (e.g. a capture on a rolled-back move). */
  cancel(key: string): void {
    const timer = this.pending.get(key)
    if (timer === undefined) return
    this.clearTimer(timer)
    this.pending.delete(key)
  }

  /** Cancels every scheduled cue but keeps the idempotency history. */
  clearPending(): void {
    for (const timer of this.pending.values()) this.clearTimer(timer)
    this.pending.clear()
  }

  /** Drops scheduled cues and idempotency history (new match / unmount). */
  reset(): void {
    this.clearPending()
    this.played.clear()
  }

  hasPlayed(key: string): boolean {
    return this.played.has(key)
  }

  private remember(key: string): void {
    this.played.add(key)
    if (this.played.size <= MAX_REMEMBERED_KEYS) return
    const oldest = this.played.values().next().value
    if (oldest !== undefined) this.played.delete(oldest)
  }

  private run(cue: AnimationCue): void {
    try {
      this.sink[cue]()
    } catch {
      // Audio failures must never interrupt gameplay.
    }
  }
}
