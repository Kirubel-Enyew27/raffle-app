export type OutcomeType = 'win' | 'loss' | 'draw'

export type DecisiveReason =
  | 'line_completion'
  | 'elimination'
  | 'immobilization'
  | 'rps_showdown'
  | 'draw_full_board'
  | 'draw_repetition'
  | 'draw_halfmove'
  | 'draw_series'
  | 'forfeited'
  | 'cancelled'

export type OutcomePhase = 'idle' | 'decisive' | 'effects' | 'announced'

export interface RpsOutcomeData {
  myChoice: string
  opponentChoice: string
  interactionText: string
  myWonRound: boolean
  tieRound: boolean
  myWins: number
  opponentWins: number
}

export interface DamaOutcomeData {
  lastMoveFrom?: number
  lastMoveTo?: number
  captures?: number[]
  promotion?: boolean
  remainingFriendly: number
  remainingEnemy: number
  trappedIndices?: number[]
}

export interface GameOutcome {
  matchId: string
  gameSlug: string
  type: OutcomeType
  reason: DecisiveReason
  title: string
  subtitle: string
  detail: string
  winnerUserId: string | null
  winnerPlayerIndex: number | null
  opponentName: string
  opponentAvatar?: string
  decisiveIndices?: number[]
  secondaryIndices?: number[]
  rpsData?: RpsOutcomeData
  damaData?: DamaOutcomeData
}
