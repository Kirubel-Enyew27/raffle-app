import type { P2PMatch } from '@/types/api'
import type { GameOutcome, DecisiveReason, OutcomeType } from './outcomeTypes'

const TERMINAL_STATUSES = ['completed', 'tied', 'forfeited', 'cancelled']

function rpsInteraction(c1: string, c2: string): string {
  if (c1 === c2) return `Both picked ${c1}`
  if ((c1 === 'rock' && c2 === 'scissors') || (c2 === 'rock' && c1 === 'scissors')) {
    return 'Rock crushes Scissors'
  }
  if ((c1 === 'scissors' && c2 === 'paper') || (c2 === 'scissors' && c1 === 'paper')) {
    return 'Scissors cut Paper'
  }
  if ((c1 === 'paper' && c2 === 'rock') || (c2 === 'paper' && c1 === 'rock')) {
    return 'Paper covers Rock'
  }
  return `${c1} vs ${c2}`
}

export function extractMatchOutcome(
  match: P2PMatch | undefined | null,
  currentUserId?: string
): GameOutcome | null {
  if (!match || !TERMINAL_STATUSES.includes(match.status)) {
    return null
  }

  const myPlayer = match.players?.find((p) => p.user_id === currentUserId)
  const opponentPlayer = match.players?.find((p) => p.user_id !== currentUserId)
  const myPlayerIndex = myPlayer?.player_index ?? 0
  const opponentName = opponentPlayer?.display_name || (opponentPlayer ? 'Opponent' : 'Challenger')
  const opponentAvatar = opponentPlayer?.avatar_url

  const isTie = match.status === 'tied' || (!match.winner_user_id && match.status === 'completed')
  const isWinner = !isTie && match.winner_user_id === currentUserId
  const outcomeType: OutcomeType = isTie ? 'draw' : isWinner ? 'win' : 'loss'

  const title = outcomeType === 'win' ? 'YOU WIN!' : outcomeType === 'loss' ? 'YOU LOSE!' : 'DRAW!'

  let reason: DecisiveReason = 'line_completion'
  let subtitle: string
  let detail: string
  let decisiveIndices: number[] | undefined
  let secondaryIndices: number[] | undefined

  // Common non-board outcomes:
  if (match.status === 'forfeited') {
    reason = 'forfeited'
    subtitle = isWinner ? 'Opponent surrendered the match' : 'You surrendered the match'
    detail = 'Match conceded by resignation'
    return {
      matchId: match.id,
      gameSlug: match.game_slug,
      type: outcomeType,
      reason,
      title,
      subtitle,
      detail,
      winnerUserId: match.winner_user_id ?? null,
      winnerPlayerIndex: isWinner ? myPlayerIndex : 1 - myPlayerIndex,
      opponentName,
      opponentAvatar,
    }
  }

  if (match.status === 'cancelled') {
    reason = 'cancelled'
    subtitle = 'Match was cancelled'
    detail = 'Session terminated'
    return {
      matchId: match.id,
      gameSlug: match.game_slug,
      type: outcomeType,
      reason,
      title,
      subtitle,
      detail,
      winnerUserId: match.winner_user_id ?? null,
      winnerPlayerIndex: null,
      opponentName,
      opponentAvatar,
    }
  }

  // Game-specific rules
  switch (match.game_slug) {
    case 'tic-tac-toe': {
      const winningLine = (match.board_state?.winning_line ?? null) as number[] | null
      const cells = (match.board_state?.cells ?? []) as number[]
      const p0Score = match.scores?.['0'] ?? 0
      const p1Score = match.scores?.['1'] ?? 0
      const myScore = myPlayerIndex === 0 ? p0Score : p1Score
      const opponentScore = myPlayerIndex === 0 ? p1Score : p0Score

      if (winningLine && Array.isArray(winningLine) && winningLine.length === 3) {
        reason = 'line_completion'
        decisiveIndices = winningLine
        subtitle = isWinner ? '3 in a Row — Winning Line Connected!' : 'Opponent completed 3 in a Row'
        detail = `Series Score: ${myScore} - ${opponentScore} (Round ${match.current_round} of ${match.max_rounds})`
      } else if (isTie || (cells.length === 9 && cells.every((c) => c !== 0))) {
        reason = 'draw_full_board'
        decisiveIndices = [0, 1, 2, 3, 4, 5, 6, 7, 8]
        subtitle = 'Grid is Full — No winning line formed'
        detail = `Series Score: ${myScore} - ${opponentScore} (Draw)`
      } else {
        reason = isTie ? 'draw_series' : 'line_completion'
        subtitle = isWinner ? 'Series Won' : isTie ? 'Series Tied' : 'Series Lost'
        detail = `Final Score: ${myScore} - ${opponentScore}`
      }
      break
    }

    case 'connect-four': {
      const winningLine = (match.board_state?.winning_line ?? null) as number[] | null
      const cells = (match.board_state?.cells ?? []) as number[]

      if (winningLine && Array.isArray(winningLine) && winningLine.length === 4) {
        reason = 'line_completion'
        decisiveIndices = winningLine
        subtitle = isWinner ? '4 in a Row — Decisive Connection!' : 'Opponent connected 4 Tokens'
        detail = 'First to align four chips wins'
      } else if (isTie || (cells.length > 0 && cells.slice(0, 7).every((c) => c !== 0))) {
        reason = 'draw_full_board'
        decisiveIndices = cells.map((_, i) => i)
        subtitle = 'Board is Full — Stalemate with no 4-in-a-row'
        detail = 'All columns filled'
      } else {
        reason = 'line_completion'
        subtitle = isWinner ? 'Victory on Connect Four' : isTie ? 'Draw on Connect Four' : 'Defeat'
        detail = 'Game completed'
      }
      break
    }

    case 'rock-paper-scissors': {
      const lastResult = match.board_state?.last_round_result as
        | { p0_choice?: string; p1_choice?: string; winner?: number | null; is_tie?: boolean }
        | undefined
      const p0Score = match.scores?.['0'] ?? 0
      const p1Score = match.scores?.['1'] ?? 0
      const myScore = myPlayerIndex === 0 ? p0Score : p1Score
      const opponentScore = myPlayerIndex === 0 ? p1Score : p0Score

      const p0Choice = lastResult?.p0_choice ?? ''
      const p1Choice = lastResult?.p1_choice ?? ''
      const myChoice = myPlayerIndex === 0 ? p0Choice : p1Choice
      const opponentChoice = myPlayerIndex === 0 ? p1Choice : p0Choice

      const interaction = rpsInteraction(myChoice, opponentChoice)
      const myWonRound = lastResult?.winner === myPlayerIndex
      const tieRound = !!lastResult?.is_tie

      if (isTie) {
        reason = 'draw_series'
        subtitle = 'Series Tied after max rounds'
        detail = `Final Score: ${myScore} - ${opponentScore}`
      } else {
        reason = 'rps_showdown'
        subtitle = isWinner ? `Championship Won — ${interaction}` : `Match Lost — ${interaction}`
        detail = `Final Series Score: ${myScore} - ${opponentScore}`
      }

      return {
        matchId: match.id,
        gameSlug: match.game_slug,
        type: outcomeType,
        reason,
        title,
        subtitle,
        detail,
        winnerUserId: match.winner_user_id ?? null,
        winnerPlayerIndex: isWinner ? myPlayerIndex : 1 - myPlayerIndex,
        opponentName,
        opponentAvatar,
        rpsData: {
          myChoice,
          opponentChoice,
          interactionText: interaction,
          myWonRound,
          tieRound,
          myWins: myScore,
          opponentWins: opponentScore,
        },
      }
    }

    case 'ethiopian-dama': {
      const damaResult = match.board_state?.result as
        | { winner?: number | null; draw?: boolean; reason?: string }
        | undefined
      const lastMove = match.board_state?.last_move as
        | { from?: number; to?: number; captures?: number[]; promotion?: boolean }
        | undefined
      const cells = ((match.board_state?.cells ?? []) as number[])

      let lightCount = 0
      let darkCount = 0
      const trappedPieces: number[] = []

      cells.forEach((val) => {
        if (val === 1 || val === 2) lightCount++
        if (val === 3 || val === 4) darkCount++
      })

      const friendlySide = myPlayerIndex === 1 ? 1 : 0
      const remainingFriendly = friendlySide === 0 ? lightCount : darkCount
      const remainingEnemy = friendlySide === 0 ? darkCount : lightCount

      // If immobilization: find opponent pieces if opponent lost, or our pieces if we lost
      if (damaResult?.reason === 'no_legal_moves') {
        const losingSide = isWinner ? (friendlySide === 0 ? 1 : 0) : friendlySide
        cells.forEach((val, idx) => {
          const side = val === 1 || val === 2 ? 0 : val === 3 || val === 4 ? 1 : null
          if (side === losingSide) {
            trappedPieces.push(idx)
          }
        })
      }

      if (damaResult?.draw) {
        if (damaResult.reason === 'draw_repetition') {
          reason = 'draw_repetition'
          subtitle = 'Draw — Threefold Repetition'
        } else {
          reason = 'draw_halfmove'
          subtitle = 'Draw — 50-Move Rule (Halfmove Clock)'
        }
        detail = `${match.board_state?.move_count ?? 0} moves played · Position locked`
      } else if (remainingEnemy === 0 || remainingFriendly === 0) {
        reason = 'elimination'
        subtitle = isWinner
          ? 'Total Elimination — All opponent pieces captured!'
          : 'Total Elimination — All your pieces were captured'
        detail = `${remainingFriendly} vs ${remainingEnemy} pieces remaining · ${match.board_state?.move_count ?? 0} moves`
      } else if (damaResult?.reason === 'no_legal_moves') {
        reason = 'immobilization'
        subtitle = isWinner
          ? 'Immobilization — Opponent has no legal moves remaining!'
          : 'Immobilization — No legal moves remain'
        detail = `${trappedPieces.length} piece${trappedPieces.length === 1 ? '' : 's'} immobilized · ${match.board_state?.move_count ?? 0} moves`
      } else {
        reason = 'elimination'
        subtitle = isWinner ? 'Victory on Ethiopian Dama' : 'Defeat'
        detail = `${match.board_state?.move_count ?? 0} moves played`
      }

      const decisive: number[] = []
      if (typeof lastMove?.from === 'number') decisive.push(lastMove.from)
      if (typeof lastMove?.to === 'number') decisive.push(lastMove.to)
      if (Array.isArray(lastMove?.captures)) decisive.push(...lastMove.captures)

      decisiveIndices = decisive
      secondaryIndices = trappedPieces

      return {
        matchId: match.id,
        gameSlug: match.game_slug,
        type: outcomeType,
        reason,
        title,
        subtitle,
        detail,
        winnerUserId: match.winner_user_id ?? null,
        winnerPlayerIndex: isWinner ? myPlayerIndex : 1 - myPlayerIndex,
        opponentName,
        opponentAvatar,
        decisiveIndices,
        secondaryIndices,
        damaData: {
          lastMoveFrom: lastMove?.from,
          lastMoveTo: lastMove?.to,
          captures: lastMove?.captures,
          promotion: lastMove?.promotion,
          remainingFriendly,
          remainingEnemy,
          trappedIndices: trappedPieces,
        },
      }
    }

    default: {
      subtitle = isWinner ? 'Victory!' : isTie ? 'Match Drawn' : 'Defeat'
      detail = 'Game completed'
      break
    }
  }

  return {
    matchId: match.id,
    gameSlug: match.game_slug,
    type: outcomeType,
    reason,
    title,
    subtitle,
    detail,
    winnerUserId: match.winner_user_id ?? null,
    winnerPlayerIndex: isWinner ? myPlayerIndex : 1 - myPlayerIndex,
    opponentName,
    opponentAvatar,
    decisiveIndices,
    secondaryIndices,
  }
}
