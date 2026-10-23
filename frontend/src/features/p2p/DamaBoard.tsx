import { useMemo, useState } from 'react'
import { Crown } from 'lucide-react'

import {
  displaySquares,
  findStep,
  isCaptureStep,
  legalTargetsFrom,
  moveNotation,
  orientationFor,
  pieceIsKing,
  pieceSide,
  squareName,
  type DamaBoardState,
  type DamaHistoryEntry,
  type DamaSide,
} from './dama'

interface DamaBoardProps {
  boardState: DamaBoardState
  isMyTurn: boolean
  myPlayerIndex?: number
  onMove: (action: Record<string, unknown>) => void
  disabled?: boolean
}

const LIGHT_SQUARE = 'bg-[#e9d6b8]'
const DARK_SQUARE = 'bg-gradient-to-br from-[#7a4a25] to-[#4b2a13]'

function pieceLabel(side: DamaSide, king: boolean): string {
  const color = side === 0 ? 'light' : 'dark'
  return king ? `${color} king` : `${color} man`
}

export function DamaBoard({
  boardState,
  isMyTurn,
  myPlayerIndex,
  onMove,
  disabled = false,
}: DamaBoardProps) {
  const [selected, setSelected] = useState<number | null>(null)

  const orientation = orientationFor(myPlayerIndex)
  const cells = boardState?.cells ?? []
  const lastMove = boardState?.last_move ?? null
  const chainFrom = typeof boardState?.must_continue_from === 'number' ? boardState.must_continue_from : null
  const gameOver = !!boardState?.result
  const boardTurn = typeof boardState?.turn === 'number' ? boardState.turn : undefined

  const canInteract =
    !!isMyTurn &&
    !disabled &&
    !gameOver &&
    myPlayerIndex !== undefined &&
    (boardTurn === undefined || boardTurn === myPlayerIndex)

  // During a capture chain the server names the only movable square. Deriving
  // the active selection avoids a synchronisation effect and guarantees the
  // UI can never offer a different piece mid-chain.
  const activeSelection = chainFrom ?? selected

  const targets = useMemo(
    () => (activeSelection === null ? [] : legalTargetsFrom(boardState, activeSelection)),
    [boardState, activeSelection]
  )

  const moveNumber = boardState?.move_count ?? 0
  const turnSeq = boardState?.turn_seq ?? 0

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
      // Clicking an empty/non-legal square clears the selection, but a capture
      // chain square stays selected because the turn cannot be abandoned.
      if (chainFrom === null) setSelected(null)
      return
    }

    onMove({ from: activeSelection, to: index, turn_seq: turnSeq })
    setSelected(null)
  }

  return (
    <div className="flex w-full max-w-[560px] flex-col items-center gap-3 select-none">
      <div
        role="status"
        aria-live="polite"
        className="flex min-h-6 items-center justify-center px-2 text-center text-xs font-semibold text-muted-foreground"
      >
        {chainFrom !== null
          ? 'Capture chain — keep jumping with the highlighted piece'
          : gameOver
            ? 'Match finished'
            : isMyTurn
              ? 'Your turn — select a piece'
              : 'Opponent is thinking…'}
      </div>

      {chainFrom !== null && (
        <div className="w-full rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-center text-[11px] font-semibold text-rose-600 dark:text-rose-300">
          Captures are mandatory: continue from {squareName(chainFrom)}
        </div>
      )}

      <div className="w-full rounded-2xl border-4 border-[#4a2c17] bg-[#4a2c17] p-1.5 shadow-2xl shadow-black/40">
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
            const wasCaptured = !!lastMove?.captures?.includes(index)
            const selectable = canInteract && side === myPlayerIndex && (chainFrom === null || chainFrom === index)

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
                }`}
                aria-pressed={isSelected}
                className={[
                  'group relative aspect-square w-full transition-colors duration-150',
                  DARK_SQUARE,
                  selectable ? 'cursor-pointer hover:brightness-110' : '',
                  canInteract ? 'focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300 focus-visible:ring-offset-1 focus-visible:ring-offset-[#4a2c17]' : '',
                  isSelected ? 'z-10 ring-2 ring-inset ring-amber-300' : '',
                  isLastFrom && !isSelected ? 'ring-2 ring-inset ring-amber-200/50' : '',
                  isLastTo && !isSelected ? 'ring-2 ring-inset ring-emerald-300/60' : '',
                  isTarget ? 'cursor-pointer' : '',
                ].join(' ')}
              >
                <span className="pointer-events-none absolute bottom-0.5 left-1 text-[8px] font-semibold uppercase tracking-tight text-amber-100/50">
                  {squareName(index)}
                </span>

                {isTarget && (
                  <span
                    className={[
                      'pointer-events-none absolute inset-[30%] rounded-full',
                      captureTarget
                        ? 'bg-rose-500/70 ring-2 ring-rose-200 motion-safe:animate-pulse'
                        : 'bg-emerald-400/80 ring-2 ring-emerald-100/70',
                    ].join(' ')}
                  />
                )}

                {wasCaptured && (
                  <span
                    key={`capture-${moveNumber}`}
                    aria-hidden="true"
                    className="pointer-events-none absolute inset-1 rounded-md ring-4 ring-rose-400/80 motion-safe:animate-ping motion-reduce:hidden"
                  />
                )}

                {value !== 0 && (
                  <span
                    className={[
                      'pointer-events-none absolute inset-[12%] flex items-center justify-center rounded-full border shadow-lg',
                      'transition-transform duration-150 motion-safe:group-hover:scale-105 motion-reduce:transition-none',
                      side === 0
                        ? 'border-[#8a5a2b] bg-gradient-to-br from-[#fff4dc] via-[#ecd3a8] to-[#c59a62] text-[#6b4423] shadow-black/40'
                        : 'border-black/70 bg-gradient-to-br from-[#4b4b4f] via-[#232326] to-[#0b0b0d] text-[#f4e3c2] shadow-black/60',
                      isSelected ? 'ring-2 ring-amber-300 scale-105' : '',
                    ].join(' ')}
                  >
                    {king ? (
                      <Crown
                        className={[
                          'h-1/2 w-1/2 motion-safe:animate-none',
                          isLastTo && lastMove?.promotion ? 'motion-safe:animate-bounce' : '',
                        ].join(' ')}
                        strokeWidth={2.5}
                        aria-hidden="true"
                      />
                    ) : (
                      <span
                        className={[
                          'h-[28%] w-[28%] rounded-full',
                          side === 0 ? 'bg-[#8a5a2b]/40' : 'bg-[#f4e3c2]/25',
                        ].join(' ')}
                        aria-hidden="true"
                      />
                    )}
                    {king && isLastTo && lastMove?.promotion && (
                      <span className="absolute -right-1 -top-1 rounded-full bg-amber-300 px-1 text-[8px] font-black text-black motion-safe:animate-bounce motion-reduce:animate-none">
                        K
                      </span>
                    )}
                  </span>
                )}

                {isSelected && (
                  <span className="pointer-events-none absolute inset-0 rounded-sm ring-2 ring-inset ring-amber-200" />
                )}
              </button>
            )
          })}
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
            <span
              className={[
                'inline-block h-2.5 w-2.5 shrink-0 rounded-full border',
                entry.player === 0
                  ? 'border-[#8a5a2b] bg-[#f2ddb8]'
                  : 'border-black/60 bg-[#26262a]',
              ].join(' ')}
              aria-hidden="true"
            />
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
