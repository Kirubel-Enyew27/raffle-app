/**
 * Presentation helpers for Rock-Paper-Scissors (Bo3).
 *
 * The server owns the rules (backend/internal/p2p/domain/rules.go): it validates
 * choices, decides the round winner, keeps the best-of-three score and clears
 * `choices` once both players have picked. Everything here only reads that state
 * so the board can describe it, never to re-decide an outcome.
 */
export type RpsChoice = 'rock' | 'paper' | 'scissors'

export interface RpsChoiceMeta {
  readonly id: RpsChoice
  readonly label: string
  /** The move this one defeats, used for the "Beats …" hint. */
  readonly beats: RpsChoice
  readonly beatsLabel: string
  /** Text colour for the SVG icon, tuned for light and dark themes. */
  readonly accentClass: string
  /** Tinted tile behind the icon. */
  readonly tileClass: string
}

export const RPS_CHOICE_MAP: Record<RpsChoice, RpsChoiceMeta> = {
  rock: {
    id: 'rock',
    label: 'Rock',
    beats: 'scissors',
    beatsLabel: 'Scissors',
    accentClass: 'text-slate-600 dark:text-slate-300',
    tileClass: 'bg-slate-500/10 border-slate-500/25',
  },
  paper: {
    id: 'paper',
    label: 'Paper',
    beats: 'rock',
    beatsLabel: 'Rock',
    accentClass: 'text-indigo-600 dark:text-indigo-400',
    tileClass: 'bg-indigo-500/10 border-indigo-500/25',
  },
  scissors: {
    id: 'scissors',
    label: 'Scissors',
    beats: 'paper',
    beatsLabel: 'Paper',
    accentClass: 'text-teal-600 dark:text-teal-400',
    tileClass: 'bg-teal-500/10 border-teal-500/25',
  },
}

/** The three picks in play order, for grids and legends. */
export const RPS_CHOICES: readonly RpsChoiceMeta[] = [
  RPS_CHOICE_MAP.rock,
  RPS_CHOICE_MAP.paper,
  RPS_CHOICE_MAP.scissors,
]

const CHOICE_LABELS: Record<RpsChoice, string> = {
  rock: 'Rock',
  paper: 'Paper',
  scissors: 'Scissors',
}

export function isRpsChoice(value: unknown): value is RpsChoice {
  return value === 'rock' || value === 'paper' || value === 'scissors'
}

/** The view masks the opponent's pick as "hidden" until the round resolves. */
export function parseRpsChoice(value: unknown): RpsChoice | null {
  return isRpsChoice(value) ? value : null
}

export function choiceLabel(choice: RpsChoice | null | undefined): string {
  return choice ? CHOICE_LABELS[choice] : ''
}

/** Shape of `board_state.last_round_result` produced by the engine. */
export interface RpsRoundResult {
  p0_choice?: string
  p1_choice?: string
  is_tie?: boolean
  winner?: number | null
}

export type RpsVerdict = 'win' | 'lose' | 'draw'

/** Round verdict from the local player's point of view. */
export function roundVerdict(
  result: RpsRoundResult | null | undefined,
  myIndex: number | undefined
): RpsVerdict | null {
  if (!result || (myIndex !== 0 && myIndex !== 1)) return null
  if (result.is_tie) return 'draw'
  if (result.winner === null || result.winner === undefined) return null
  return result.winner === myIndex ? 'win' : 'lose'
}

export interface RpsSeries {
  readonly myWins: number
  readonly opponentWins: number
  /** Wins required to take the match: 2 in a best-of-three. */
  readonly winsNeeded: number
  readonly roundsPlayed: number
}

/** Best-of-three progress, mirrored to the local seat. */
export function seriesProgress(
  scores: Record<string, number> | null | undefined,
  myIndex: number | undefined,
  currentRound: number | undefined,
  maxRounds: number | undefined
): RpsSeries | null {
  if (myIndex !== 0 && myIndex !== 1) return null
  const scoreFor = (seat: number) => {
    const value = scores?.[String(seat)]
    return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
  }
  const rounds = typeof currentRound === 'number' && currentRound > 0 ? Math.floor(currentRound) : 1
  const max = typeof maxRounds === 'number' && maxRounds > 0 ? Math.floor(maxRounds) : 3
  return {
    myWins: scoreFor(myIndex),
    opponentWins: scoreFor(1 - myIndex),
    winsNeeded: Math.floor(max / 2) + 1,
    roundsPlayed: Math.min(rounds, max),
  }
}
