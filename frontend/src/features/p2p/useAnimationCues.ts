import { useEffect, useState } from 'react'

import { soundManager } from '@/lib/sound/soundManager'
import {
  AnimationCueTimeline,
  type AnimationCueSink,
} from '@/lib/sound/animationCueTimeline'

const SINK: AnimationCueSink = {
  move: () => soundManager.playMove(),
  drop: () => soundManager.playDrop(),
  land: () => soundManager.playMove(),
  capture: () => soundManager.playCapture(),
  promotion: () => soundManager.playPromotion(),
  select: () => soundManager.playSelect(),
  reveal: () => soundManager.playReveal(),
}

/**
 * One animation cue timeline per mounted board.
 *
 * Boards call this at the exact milestone they animate (marker placement, drop
 * start, Dama hop start/arrival, reveal) so a cue is never tied to a network
 * response. Unmounting cancels every pending cue, which is what stops a sound
 * from a previous match surviving navigation.
 */
export function useAnimationCues(): AnimationCueTimeline {
  // Lazy state initialisation keeps the timeline stable across renders without
  // reading a ref during render.
  const [timeline] = useState(() => new AnimationCueTimeline(SINK))

  useEffect(() => {
    return () => timeline.reset()
  }, [timeline])

  return timeline
}
