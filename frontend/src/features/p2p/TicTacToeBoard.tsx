import { TicTacToeMarker } from './TicTacToeMarkers'
import { EMPTY_CELL, markerLabel, seatForCell, seatForSymbol } from './ticTacToe'

interface TicTacToeBoardProps {
  boardState: {
    cells?: number[]
    last_move?: number | null
    winning_line?: number[] | null
    move_count?: number
  }
  isMyTurn: boolean
  mySymbol?: string
  onMove: (cellIndex: number) => void
  disabled?: boolean
  isGameOver?: boolean
  isTie?: boolean
}

// One size ladder for both seats so the two shapes keep identical bounds.
const MARKER_SIZE = 'h-8 w-8 sm:h-10 sm:w-10 md:h-11 md:w-11'

function getWinningLineCoords(winningLine: number[]): { x1: number; y1: number; x2: number; y2: number } | null {
  if (winningLine.length !== 3) return null
  const sorted = [...winningLine].sort((a, b) => a - b)
  const [first, , last] = sorted

  const c1 = first % 3
  const r1 = Math.floor(first / 3)
  const c2 = last % 3
  const r2 = Math.floor(last / 3)

  if (r1 === r2) {
    const y = r1 * 33.333 + 16.666
    return { x1: 8, y1: y, x2: 92, y2: y }
  }
  if (c1 === c2) {
    const x = c1 * 33.333 + 16.666
    return { x1: x, y1: 8, x2: x, y2: 92 }
  }
  if (first === 0 && last === 8) {
    return { x1: 9, y1: 9, x2: 91, y2: 91 }
  }
  if (first === 2 && last === 6) {
    return { x1: 91, y1: 9, x2: 9, y2: 91 }
  }
  return null
}

export function TicTacToeBoard({
  boardState,
  isMyTurn,
  mySymbol,
  onMove,
  disabled = false,
  isGameOver = false,
  isTie = false,
}: TicTacToeBoardProps) {
  const cells = boardState?.cells || [0, 0, 0, 0, 0, 0, 0, 0, 0]
  const winningLine = boardState?.winning_line || null
  const lastMove = boardState?.last_move ?? null
  const mySeat = seatForSymbol(mySymbol)

  const lineCoords = winningLine ? getWinningLineCoords(winningLine) : null
  const isDraw = isTie || (isGameOver && !winningLine && cells.every((c) => c !== EMPTY_CELL))

  const handleClick = (index: number) => {
    if (disabled || !isMyTurn || isGameOver || cells[index] !== 0) return
    onMove(index)
  }

  const isWinningCell = (index: number) => winningLine?.includes(index) ?? false
  const winningCellOrder = (index: number) => (winningLine ? winningLine.indexOf(index) : -1)

  return (
    <div className="flex w-full flex-col items-center justify-center p-4">
      <div
        className={[
          'relative grid grid-cols-3 gap-3 p-4 max-w-sm w-full aspect-square rounded-2xl',
          'border shadow-xl backdrop-blur-sm transition-colors duration-300',
          winningLine
            ? 'bg-emerald-500/[0.06] border-emerald-500/40 ring-2 ring-emerald-500/20'
            : isDraw
              ? 'bg-amber-500/[0.05] border-amber-500/30'
              : isMyTurn && !isGameOver
                ? 'bg-primary/[0.04] border-primary/40 ring-1 ring-primary/20'
                : 'bg-muted/30 border-border/80',
        ].join(' ')}
      >
        {cells.map((val, idx) => {
          const isWin = isWinningCell(idx)
          const winOrder = winningCellOrder(idx)
          const isLast = lastMove === idx
          const canClick = isMyTurn && !disabled && !isGameOver && val === EMPTY_CELL
          const seat = seatForCell(val)

          return (
            <button
              key={idx}
              type="button"
              onClick={() => handleClick(idx)}
              disabled={!canClick}
              aria-label={
                seat
                  ? `Cell ${idx + 1}, ${markerLabel(seat)}${isWin ? ', part of the winning line' : ''}`
                  : `Cell ${idx + 1}, empty${canClick ? ', your move' : ''}`
              }
              style={isWin && winOrder >= 0 ? { animationDelay: `${winOrder * 140}ms` } : undefined}
              className={[
                'group relative flex items-center justify-center rounded-xl border transition-all select-none',
                'border-border/60 shadow-sm',
                val === EMPTY_CELL
                  ? canClick
                    ? 'bg-background hover:bg-accent/40 hover:border-primary/40 hover:scale-[1.02] cursor-pointer active:scale-95'
                    : 'bg-background/40 cursor-not-allowed opacity-60'
                  : isWin
                    ? 'ttt-win-cell bg-emerald-500/20 border-emerald-400 ring-2 ring-emerald-400/50 scale-[1.04] z-10 ttt-win-cell-seq shadow-lg shadow-emerald-500/20'
                    : isDraw
                      ? 'bg-amber-500/10 border-amber-500/30 text-amber-500'
                      : isLast
                        ? 'bg-muted/70 border-primary/50 shadow-md'
                        : winningLine
                          ? 'bg-muted/30 opacity-70 border-border/40' // keep losing player's marks visible with subtle de-emphasis
                          : 'bg-muted/50',
              ].join(' ')}
            >
              {seat && <TicTacToeMarker seat={seat} variant="placed" className={MARKER_SIZE} />}

              {/* Ghost of the current player's marker */}
              {!seat && canClick && mySeat && (
                <span className="pointer-events-none absolute inset-0 flex items-center justify-center opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
                  <TicTacToeMarker seat={mySeat} variant="preview" className={MARKER_SIZE} />
                </span>
              )}
            </button>
          )
        })}

        {/* Dynamic laser connecting line across the 3 winning cells */}
        {lineCoords && (
          <svg
            className="pointer-events-none absolute inset-0 z-20 h-full w-full"
            viewBox="0 0 100 100"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            {/* Halo pulse */}
            <line
              x1={`${lineCoords.x1}%`}
              y1={`${lineCoords.y1}%`}
              x2={`${lineCoords.x2}%`}
              y2={`${lineCoords.y2}%`}
              stroke="rgb(16 185 129 / 0.45)"
              strokeWidth="7"
              strokeLinecap="round"
              className="ttt-line-stroke"
            />
            {/* Core beam */}
            <line
              x1={`${lineCoords.x1}%`}
              y1={`${lineCoords.y1}%`}
              x2={`${lineCoords.x2}%`}
              y2={`${lineCoords.y2}%`}
              stroke="#10b981"
              strokeWidth="3"
              strokeLinecap="round"
              className="ttt-line-stroke"
            />
          </svg>
        )}
      </div>

      {/* Footer indication */}
      {isDraw ? (
        <div className="mt-3 flex items-center gap-2 text-xs font-bold text-amber-600 dark:text-amber-400">
          <span>Grid is Full — No winning line (Stalemate)</span>
        </div>
      ) : winningLine ? (
        <div className="mt-3 flex items-center gap-2 text-xs font-bold text-emerald-600 dark:text-emerald-400 animate-pulse">
          <span>Decisive 3 in a Row Connected!</span>
        </div>
      ) : mySeat ? (
        <div className="mt-3 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          <TicTacToeMarker seat={mySeat} variant="plain" className="h-4 w-4" />
          <span>You play the {markerLabel(mySeat)}</span>
        </div>
      ) : null}
    </div>
  )
}
