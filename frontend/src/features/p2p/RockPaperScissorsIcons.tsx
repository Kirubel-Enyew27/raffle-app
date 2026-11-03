import { RpsGlyph } from './GamePieces'
import { RPS_CHOICES, type RpsChoice } from './rockPaperScissors'

/**
 * Rock-Paper-Scissors move illustrations.
 *
 * These are the shared `RpsGlyph` objects — a faceted steel stone, a folded
 * sheet of paper and red steel scissors — so the selection tiles, reveal tiles,
 * locked-in badge, lobby icon and game card all show the same artwork.
 *
 * `accentClass` still tints the surrounding tile and captions; the artwork
 * itself carries its own material colours.
 */
export type RpsIconProps = { className?: string }

const DEFAULT_ICON_SIZE = 'h-8 w-8'

export function RockIcon({ className }: RpsIconProps) {
  return <RpsGlyph choice="rock" className={className ?? DEFAULT_ICON_SIZE} />
}

export function PaperIcon({ className }: RpsIconProps) {
  return <RpsGlyph choice="paper" className={className ?? DEFAULT_ICON_SIZE} />
}

export function ScissorsIcon({ className }: RpsIconProps) {
  return <RpsGlyph choice="scissors" className={className ?? DEFAULT_ICON_SIZE} />
}

export function RpsChoiceIcon({ choice, className }: RpsIconProps & { choice: RpsChoice }) {
  return <RpsGlyph choice={choice} className={className ?? DEFAULT_ICON_SIZE} />
}

/** Compact all-three badge used wherever the game itself is illustrated. */
export function RpsChoiceTrio({ className }: { className?: string }) {
  const size = className ?? 'h-4 w-4'
  return (
    <span className="inline-flex items-center gap-0.5">
      {RPS_CHOICES.map((choice) => (
        <RpsChoiceIcon key={choice.id} choice={choice.id} className={size} />
      ))}
    </span>
  )
}
