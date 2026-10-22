import { useEffect, useMemo, useRef, useState, useCallback } from 'react'
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
      playMove: () => soundManager.playMove(),
      playTurnAlert: () => soundManager.playTurnAlert(),
      playTimerTick: (sec?: number) => soundManager.playTimerTick(sec),
      playVictory: () => soundManager.playVictory(),
      playDefeat: () => soundManager.playDefeat(),
      playDraw: () => soundManager.playDraw(),
    }),
    []
  )

  const trackerRef = useRef<SoundEventTracker | null>(null)
  if (!trackerRef.current) {
    trackerRef.current = new SoundEventTracker(sink, timerThreshold)
  }

  useEffect(() => {
    const tracker = trackerRef.current
    if (!tracker) return
    tracker.setUser(userId)
    tracker.update(match, secondsLeft, timerThreshold)
  }, [match, userId, secondsLeft, timerThreshold])

  // Reset when the arena unmounts so a later visit starts from a clean baseline.
  useEffect(() => {
    return () => trackerRef.current?.reset()
  }, [])

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
    playMove: useCallback(() => soundManager.playMove(), []),
    playTurnAlert: useCallback(() => soundManager.playTurnAlert(), []),
    playTimerTick: useCallback((sec?: number) => soundManager.playTimerTick(sec), []),
    playVictory: useCallback(() => soundManager.playVictory(), []),
    playDefeat: useCallback(() => soundManager.playDefeat(), []),
    playDraw: useCallback(() => soundManager.playDraw(), []),
  }
}
