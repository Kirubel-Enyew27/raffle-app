import { useEffect, useMemo, useState, useCallback } from 'react'
import { soundManager } from '@/lib/sound/soundManager'
import {
  SoundEventTracker,
  DEFAULT_TIMER_THRESHOLD,
  type SoundSink,
} from '@/lib/sound/soundEventTracker'
import type { P2PMatch } from '@/types/api'

interface UseGameSoundsOptions {
  match: P2PMatch | undefined
  userId: string | undefined
  secondsLeft?: number
  timerThreshold?: number // Default: 5 seconds
}

/**
 * Plays match sound effects for genuine state transitions only.
 *
 * All deduplication lives in `SoundEventTracker`, so duplicate SSE events,
 * polling refreshes, React re-renders and loading a historical match cannot
 * retrigger a sound. Audio failures are swallowed and never block gameplay.
 *
 * Board cues (move, drop, landing, capture, promotion, select, reveal) are NOT
 * played here: they belong to the boards' animation timelines so they never
 * wait for the REST response or SSE event that produced this snapshot.
 */
export function useGameSounds({
  match,
  userId,
  secondsLeft = 0,
  timerThreshold = DEFAULT_TIMER_THRESHOLD,
}: UseGameSoundsOptions) {
  const [isMuted, setIsMuted] = useState<boolean>(() => soundManager.isMuted())

  const sink = useMemo<SoundSink>(
    () => ({
      playMatchStart: () => soundManager.playMatchStart(),
      playTurnAlert: () => soundManager.playTurnAlert(),
      playTimerTick: (sec?: number) => soundManager.playTimerTick(sec),
      playVictory: () => soundManager.playVictory(),
      playDefeat: () => soundManager.playDefeat(),
      playDraw: () => soundManager.playDraw(),
    }),
    []
  )

  // Lazy state initialisation keeps one tracker per mounted arena without
  // reading a ref during render.
  const [tracker] = useState(() => new SoundEventTracker(sink, timerThreshold))

  useEffect(() => {
    tracker.setUser(userId)
    tracker.update(match, secondsLeft, timerThreshold)
  }, [tracker, match, userId, secondsLeft, timerThreshold])

  // Reset when the arena unmounts so a later visit starts from a clean baseline.
  useEffect(() => {
    return () => tracker.reset()
  }, [tracker])

  const toggleMute = useCallback(() => {
    const newMuted = soundManager.toggleMute()
    setIsMuted(newMuted)
  }, [])

  const setMuted = useCallback((muted: boolean) => {
    soundManager.setMuted(muted)
    setIsMuted(muted)
  }, [])

  return {
    isMuted,
    toggleMute,
    setMuted,
  }
}
