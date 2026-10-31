import { useEffect, useMemo, useRef, useState } from 'react'
import { Lock } from 'lucide-react'

import { DamaPiece } from './DamaPiece'
import {
  displaySquares,
  findStep,
  isCaptureStep,
  legalTargetsFrom,
  moveNotation,
  orientationFor,
  orientSquare,
  pieceIsKing,
  pieceSide,
  squareName,
  type DamaBoardState,
  type DamaHistoryEntry,
  type DamaSide,
} from './dama'
import type { GameOutcome } from './outcome/outcomeTypes'

interface DamaBoardProps {
  boardState: DamaBoardState
  isMyTurn: boolean
  myPlayerIndex?: number
  onMove: (action: Record<string, unknown>) => void
  disabled?: boolean
  isGameOver?: boolean
  outcome?: GameOutcome | null
}

interface DamaVisualMove {
  seq: number
  from: number
  to: number
  player: number
  captures: number[]
  king: boolean
  promotion: boolean
}

const LIGHT_SQUARE = 'bg-[#e9d6b8]'
const DARK_SQUARE = 'bg-gradient-to-br from-[#7a4a25] to-[#4b2a13]'

function pieceLabel(side: DamaSide, king: boolean): string {
  const color = side === 0 ? 'ivory' : 'obsidian'
  return king ? `${color} king` : `${color} checker`
}

export function DamaBoard({
  boardState,
  isMyTurn,
  myPlayerIndex,
  onMove,
  disabled = false,
  isGameOver = false,
  outcome = null,
}: DamaBoardProps) {
  const [selected, setSelected] = useState<number | null>(null)
  const [movingPiece, setMovingPiece] = useState<DamaVisualMove | null>(null)
  const [moveQueue, setMoveQueue] = useState<DamaVisualMove[]>([])
  const [moveArrived, setMoveArrived] = useState(false)
  const seenMoveSeq = useRef(0)
  const initializedMoveHistory = useRef(false)

  const orientation = orientationFor(myPlayerIndex)
  const cells = useMemo(() => boardState?.cells ?? [], [boardState?.cells])
  const lastMove = boardState?.last_move ?? null
  const chainFrom = typeof boardState?.must_continue_from === 'number' ? boardState.must_continue_from : null
  const gameOver = isGameOver || !!boardState?.result
  const boardTurn = typeof boardState?.turn === 'number' ? boardState.turn : undefined

  const canInteract =
    !gameOver &&
    !!isMyTurn &&
    !disabled &&
    myPlayerIndex !== undefined &&
    (boardTurn === undefined || boardTurn === myPlayerIndex)

  const activeSelection = chainFrom ?? selected

  const targets = useMemo(
    () => (activeSelection === null ? [] : legalTargetsFrom(boardState, activeSelection)),
    [boardState, activeSelection]
  )

  const moveNumber = boardState?.move_count ?? 0
  const turnSeq = boardState?.turn_seq ?? 0
  const latestHistoryMove = boardState?.move_history?.[boardState.move_history.length - 1]

  // The server commits the board first. Animate that accepted history entry as
  // a temporary, transform-only overlay so the logical board stays immediate.
  useEffect(() => {
    if (!initializedMoveHistory.current) {
      initializedMoveHistory.current = true
      seenMoveSeq.current = latestHistoryMove?.seq ?? 0
      return
    }
    const newMoves = (boardState?.move_history ?? []).filter((move) => move.seq > seenMoveSeq.current)
    if (newMoves.length === 0) return
    seenMoveSeq.current = newMoves[newMoves.length - 1].seq
    setMoveQueue((queue) => queue.concat(newMoves.map((move) => ({
      seq: move.seq,
      from: move.from,
      to: move.to,
      player: move.player,
      captures: move.captures ?? [],
      king: !move.promotion && pieceIsKing(cells[move.to] ?? 0),
      promotion: !!move.promotion,
    }))))
  }, [latestHistoryMove?.seq, boardState?.move_history, cells])

  useEffect(() => {
    if (movingPiece || moveQueue.length === 0) return
    const frame = requestAnimationFrame(() => {
      setMovingPiece(moveQueue[0])
      setMoveQueue((queue) => queue.slice(1))
      setMoveArrived(false)
    })
    return () => cancelAnimationFrame(frame)
  }, [movingPiece, moveQueue])

  useEffect(() => {
    if (!movingPiece) return
    const frame = requestAnimationFrame(() => setMoveArrived(true))
    const timer = window.setTimeout(() => {
      setMovingPiece(null)
      setMoveArrived(false)
    }, 225)
    return () => {
      cancelAnimationFrame(frame)
      window.clearTimeout(timer)
    }
  }, [movingPiece])

  // Count active pieces on the board
  const { lightCount, darkCount } = useMemo(() => {
    let light = 0
    let dark = 0
    for (const val of cells) {
      if (val === 1 || val === 2) light++
      if (val === 3 || val === 4) dark++
    }
    return { lightCount: light, darkCount: dark }
  }, [cells])

  const isEliminationWin = gameOver && (lightCount === 0 || darkCount === 0)
  const isImmobilization = gameOver && boardState?.result?.reason === 'no_legal_moves' && !isEliminationWin

  const trappedIndices = outcome?.secondaryIndices ?? []

  const handleSquare = (index: number) => {
    if (!canInteract) return

    const value = cells[index] ?? 0
    const side = pieceSide(value)
    const maySelect = side === myPlayerIndex && (chainFrom === null || index === chainFrom)

    if (maySelect) {
      setSelected(index)
      return
    }

    if (activeSelection === null) return
    const step = findStep(boardState, activeSelection, index)
    if (!step) {
      if (chainFrom === null) setSelected(null)
      return
    }

    onMove({ from: activeSelection, to: index, turn_seq: turnSeq })
    setSelected(null)
  }

  return (
    <div className="flex w-full max-w-[560px] flex-col items-center gap-3 select-none">
      {/* Turn or Decisive Game-Ending Status */}
      <div
        role="status"
        aria-live="polite"
        className="flex min-h-6 items-center justify-center px-2 text-center text-xs font-semibold text-muted-foreground"
      >
        {chainFrom !== null ? (
          <span className="text-rose-500 font-bold">
            Capture chain — keep jumping with the highlighted piece
          </span>
        ) : gameOver ? (
          <span className="font-extrabold text-foreground">
            {outcome?.subtitle || 'Match finished'}
          </span>
        ) : isMyTurn ? (
          'Your turn — select a piece'
        ) : (
          'Opponent is thinking…'
        )}
      </div>

      {chainFrom !== null && (
        <div className="w-full rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-center text-[11px] font-semibold text-rose-600 dark:text-rose-300">
          Captures are mandatory: continue from {squareName(chainFrom)}
        </div>
      )}

      {/* Piece Counts Bar */}
      <div className="flex w-full items-center justify-between px-2 text-xs font-bold text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded-full bg-[#f4ebd9] border border-amber-900/30" />
          <span>Ivory (P1): {lightCount}</span>
          {lightCount === 0 && gameOver && <span className="text-rose-500 text-[10px] uppercase">(Eliminated)</span>}
        </span>
        <span className="flex items-center gap-1.5">
          <span>Obsidian (P2): {darkCount}</span>
          {darkCount === 0 && gameOver && <span className="text-rose-500 text-[10px] uppercase">(Eliminated)</span>}
          <span className="h-3 w-3 rounded-full bg-[#2a1d17] border border-amber-500/30" />
        </span>
      </div>

      {/* 8x8 Board */}
      <div className="relative w-full rounded-2xl border-4 border-[#4a2c17] bg-[#4a2c17] p-1.5 shadow-2xl shadow-black/40">
        <div className="relative">
        <div className="grid grid-cols-8 overflow-hidden rounded-lg" role="grid" aria-label="Ethiopian Dama board">
          {displaySquares(cells, orientation).map(({ index, playable, value }) => {
            const side = pieceSide(value)
            const king = pieceIsKing(value)
            const isSelected = activeSelection === index
            const isTarget = targets.includes(index)
            const targetStep = activeSelection === null ? undefined : findStep(boardState, activeSelection, index)
            const captureTarget = isTarget && isCaptureStep(targetStep)
            const isLastFrom = lastMove?.from === index
            const isLastTo = lastMove?.to === index
            const wasCaptured = !!lastMove?.captures?.includes(index) || !!movingPiece?.captures.includes(index)
            const selectable = canInteract && side === myPlayerIndex && (chainFrom === null || chainFrom === index)

            // Winning / decisive states:
            const isTrappedPiece = isImmobilization && trappedIndices.includes(index)
            const isVictoriousRemainingPiece = isEliminationWin && value !== 0 && (
              (lightCount > 0 && side === 0) || (darkCount > 0 && side === 1)
            )

            if (!playable) {
              return <div key={`light-${index}`} aria-hidden="true" className={`aspect-square w-full ${LIGHT_SQUARE}`} />
            }

            return (
              <button
                key={`dark-${index}`}
                type="button"
                onClick={() => handleSquare(index)}
                disabled={!canInteract}
                aria-label={`${squareName(index)}${value ? `, ${pieceLabel(side ?? 0, king)}` : ', empty'}${
                  isTarget ? ', legal destination' : ''
                }${isTrappedPiece ? ', immobilized piece' : ''}`}
                aria-pressed={isSelected}
                className={[
                  'group relative aspect-square w-full transition-colors duration-150',
                  DARK_SQUARE,
                  selectable ? 'cursor-pointer hover:brightness-110' : '',
                  canInteract
                    ? 'focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300 focus-visible:ring-offset-1 focus-visible:ring-offset-[#4a2c17]'
                    : '',
                  isSelected ? 'z-10 ring-2 ring-inset ring-amber-300' : '',
                  isLastFrom && !isSelected ? 'ring-2 ring-inset ring-amber-200/50' : '',
                  isLastTo && !isSelected ? 'ring-2 ring-inset ring-emerald-300/80 shadow-md' : '',
                  isTarget ? 'cursor-pointer' : '',
                  isTrappedPiece ? 'ring-2 ring-inset ring-rose-500/80 bg-rose-950/40' : '',
                  isVictoriousRemainingPiece ? 'ring-2 ring-inset ring-emerald-400/80 bg-emerald-950/20' : '',
                ].join(' ')}
              >
                <span className="pointer-events-none absolute bottom-0.5 left-1 text-[8px] font-semibold uppercase tracking-tight text-amber-100/50">
                  {squareName(index)}
                </span>

                {/* Destination markers */}
                {isTarget && (
                  <span
                    aria-hidden="true"
                    className={[
                      'pointer-events-none absolute rounded-full border-2 shadow-sm',
                      captureTarget
                        ? 'inset-[34%] border-rose-100/85 bg-rose-500/35 motion-safe:animate-pulse'
                        : 'inset-[37%] border-emerald-100/85 bg-emerald-400/30',
                    ].join(' ')}
                  />
                )}

                {/* Final Move / Capture Indicator */}
                {wasCaptured && (
                  <span
                    key={`capture-${movingPiece?.seq ?? moveNumber}`}
                    aria-hidden="true"
                    className="pointer-events-none absolute inset-1 rounded-md ring-4 ring-rose-400/90 motion-safe:animate-ping motion-reduce:hidden"
                  />
                )}

                {/* Trapped Piece Indicator Icon */}
                {isTrappedPiece && (
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute top-1 right-1 z-20 flex h-4 w-4 items-center justify-center rounded-full bg-rose-500 text-white shadow"
                  >
                    <Lock className="h-2.5 w-2.5" />
                  </span>
                )}

                {/* Victorious Piece Sparkle */}
                {isVictoriousRemainingPiece && (
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute -top-1 -right-1 z-20 flex h-3.5 w-3.5 items-center justify-center text-emerald-400 animate-pulse"
                  >
                  </span>
                )}

                {value !== 0 && !(movingPiece && movingPiece.to === index && movingPiece.seq === latestHistoryMove?.seq) && (
                  <span
                    className={[
                      'pointer-events-none absolute inset-[11%] block rounded-full',
                      'transition-transform duration-150 motion-safe:group-hover:scale-105 motion-reduce:transition-none',
                      isSelected ? 'scale-105 drop-shadow-[0_0_7px_rgba(252,211,77,0.65)]' : '',
                      selectable && !isSelected ? 'ring-1 ring-amber-200/45' : '',
                      isVictoriousRemainingPiece ? 'drop-shadow-[0_0_8px_rgba(52,211,153,0.8)] scale-105' : '',
                      isTrappedPiece ? 'opacity-85 scale-95' : '',
                    ].join(' ')}
                  >
                    <DamaPiece side={side ?? 0} king={king} arriving={isLastTo} className="h-full w-full" />
                    {king && isLastTo && lastMove?.promotion && (
                      <span className="absolute -right-1 -top-1 rounded-full bg-amber-300 px-1 text-[8px] font-black text-black motion-safe:animate-bounce motion-reduce:animate-none">
                        K
                      </span>
                    )}
                  </span>
                )}
              </button>
            )
          })}
        </div>
        {movingPiece && (() => {
          const from = orientSquare(movingPiece.from, orientation)
          const to = orientSquare(movingPiece.to, orientation)
          const row = Math.floor(from / 8)
          const col = from % 8
          const deltaRow = Math.floor(to / 8) - row
          const deltaCol = (to % 8) - col
          return (
            <span
              key={movingPiece.seq}
              aria-hidden="true"
              className="dama-piece-slide pointer-events-none absolute z-30 block"
              style={{
                left: `${col * 12.5}%`,
                top: `${row * 12.5}%`,
                width: '12.5%',
                height: '12.5%',
                transform: moveArrived ? `translate3d(${deltaCol * 100}%, ${deltaRow * 100}%, 0)` : 'translate3d(0, 0, 0)',
              }}
            >
              <span className={movingPiece.promotion && moveArrived ? 'dama-piece-promote block h-full w-full' : 'block h-full w-full'}>
                <DamaPiece side={movingPiece.player === 0 ? 0 : 1} king={movingPiece.king || (movingPiece.promotion && moveArrived)} className="h-full w-full" />
              </span>
            </span>
          )
        })()}
        </div>
      </div>

      <div className="flex w-full items-center justify-between px-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
        <span>{orientation === 1 ? 'Light at top' : 'Light at bottom'}</span>
        <span>Move {moveNumber}</span>
      </div>
    </div>
  )
}

export function DamaMoveHistory({ history }: { history?: DamaHistoryEntry[] }) {
  if (!history || history.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border/80 p-3 text-center text-xs text-muted-foreground">
        No moves yet — the board is waiting for the first move.
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-border/80 bg-card/60">
      <div className="flex items-center justify-between border-b border-border/60 px-3 py-2">
        <h3 className="text-xs font-bold uppercase tracking-wide text-muted-foreground">Move history</h3>
        <span className="text-[10px] text-muted-foreground">{history.length} plies</span>
      </div>
      <ol className="max-h-40 space-y-1 overflow-y-auto p-2 text-xs" aria-label="Dama move history">
        {history.map((entry) => (
          <li key={`${entry.seq}-${entry.from}-${entry.to}`} className="flex items-center gap-2 rounded-md px-1.5 py-1 hover:bg-accent/40">
            <span className="w-6 shrink-0 text-right font-mono text-[10px] text-muted-foreground">{entry.seq}</span>
            <DamaPiece side={entry.player === 0 ? 0 : 1} className="h-3.5 w-3.5 shrink-0" />
            <span className="font-mono">{moveNotation(entry)}</span>
            {entry.captures && entry.captures.length > 0 && (
              <span className="ml-auto text-[10px] font-semibold text-rose-500">×{entry.captures.length}</span>
            )}
            {entry.promotion && <span className="ml-auto text-[10px] font-bold text-amber-500">King</span>}
          </li>
        ))}
      </ol>
    </div>
  )
}
