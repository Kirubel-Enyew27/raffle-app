import { TicTacToeMarkerPair } from '@/features/p2p/TicTacToeMarkers'
import { RpsChoiceTrio } from '@/features/p2p/RockPaperScissorsIcons'
import { ConnectFourTokenPair } from '@/features/p2p/ConnectFourToken'
import { DamaPiecePair } from '@/features/p2p/DamaPiece'

const ICON_SIZES = {
  sm: { text: 'text-lg', marker: 'h-4 w-4' },
  md: { text: 'text-xl', marker: 'h-5 w-5' },
  lg: { text: 'text-3xl', marker: 'h-7 w-7' },
  xl: { text: 'text-4xl', marker: 'h-9 w-9' },
} as const

export function GameIcon({
  slug,
  size = 'md',
  className,
}: {
  slug: string
  size?: keyof typeof ICON_SIZES
  className?: string
}) {
  const markerClass = className || ICON_SIZES[size].marker

  switch (slug) {
    case 'tic-tac-toe':
      return <TicTacToeMarkerPair className={markerClass} />
    case 'rock-paper-scissors':
      return <RpsChoiceTrio className={markerClass} />
    case 'connect-four':
      return <ConnectFourTokenPair className={markerClass} />
    case 'ethiopian-dama':
      return <DamaPiecePair className={markerClass} />
    default:
      return <span className={ICON_SIZES[size].text}>🎮</span>
  }
}
