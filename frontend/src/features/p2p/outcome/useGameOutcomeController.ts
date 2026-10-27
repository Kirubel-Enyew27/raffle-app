import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import type { P2PMatch } from '@/types/api'
import { extractMatchOutcome } from './outcomeExtractor'
import type { OutcomePhase } from './outcomeTypes'

interface UseGameOutcomeControllerOptions {
  match?: P2PMatch | null
  currentUserId?: string
  decisiveDurationMs?: number
  effectsDurationMs?: number
}

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

export function useGameOutcomeController({
  match,
  currentUserId,
  decisiveDurationMs = 850,
  effectsDurationMs = 850,
}: UseGameOutcomeControllerOptions) {
  const [stagedPhase, setStagedPhase] = useState<OutcomePhase>('idle')
  const [isReviewingBoard, setIsReviewingBoard] = useState(false)
  const [skipped, setSkipped] = useState(false)

  const outcome = useMemo(
    () => extractMatchOutcome(match, currentUserId),
    [match, currentUserId]
  )

  const initialMountStatusRef = useRef<string | undefined>(match?.status)
  const lastMatchIdRef = useRef<string | undefined>(match?.id)

  // Reset review / skip when match ID changes
  useEffect(() => {
    if (match?.id !== lastMatchIdRef.current) {
      lastMatchIdRef.current = match?.id
      initialMountStatusRef.current = match?.status
      setIsReviewingBoard(false)
      setSkipped(false)
    }
  }, [match?.id, match?.status])

  // Staged transitions when outcome first triggers
  useEffect(() => {
    if (!outcome) return

    const wasAlreadyTerminal =
      initialMountStatusRef.current &&
      ['completed', 'tied', 'forfeited', 'cancelled'].includes(initialMountStatusRef.current)

    if (wasAlreadyTerminal || prefersReducedMotion()) {
      const timer = setTimeout(() => {
        setStagedPhase('announced')
      }, 0)
      return () => clearTimeout(timer)
    }

    const t0 = setTimeout(() => {
      setStagedPhase('decisive')
    }, 0)

    const t1 = setTimeout(() => {
      setStagedPhase('effects')
    }, decisiveDurationMs)

    const t2 = setTimeout(() => {
      setStagedPhase('announced')
    }, decisiveDurationMs + effectsDurationMs)

    return () => {
      clearTimeout(t0)
      clearTimeout(t1)
      clearTimeout(t2)
    }
  }, [outcome, decisiveDurationMs, effectsDurationMs])

  const phase: OutcomePhase = !outcome ? 'idle' : skipped ? 'announced' : stagedPhase

  const skipToAnnouncement = useCallback(() => {
    setSkipped(true)
  }, [])

  const toggleReviewBoard = useCallback(() => {
    setIsReviewingBoard((prev) => !prev)
  }, [])

  return {
    outcome,
    phase,
    isDecisive: phase === 'decisive' || phase === 'effects' || phase === 'announced',
    isEffectsActive: phase === 'effects' || phase === 'announced',
    isAnnounced: phase === 'announced',
    isReviewingBoard,
    skipToAnnouncement,
    toggleReviewBoard,
  }
}
