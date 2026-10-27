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
}

// One size ladder for both seats so the two shapes keep identical bounds.
const MARKER_SIZE = 'h-8 w-8 sm:h-10 sm:w-10 md:h-11 md:w-11'

export function TicTacToeBoard({
  boardState,
  isMyTurn,
  mySymbol,
  onMove,
  disabled = false,
}: TicTacToeBoardProps) {
  const cells = boardState?.cells || [0, 0, 0, 0, 0, 0, 0, 0, 0]
  const winningLine = boardState?.winning_line || null
  const lastMove = boardState?.last_move ?? null
  const mySeat = seatForSymbol(mySymbol)

  const handleClick = (index: number) => {
    if (disabled || !isMyTurn || cells[index] !== 0) return
    onMove(index)
  }

  const isWinningCell = (index: number) => winningLine?.includes(index) ?? false

  return (
    <div className="flex w-full flex-col items-center justify-center p-4">
      <div
        className={[
          'relative grid grid-cols-3 gap-3 p-4 max-w-sm w-full aspect-square rounded-2xl',
          'border shadow-xl backdrop-blur-sm transition-colors duration-300',
          isMyTurn
            ? 'bg-primary/[0.04] border-primary/40 ring-1 ring-primary/20'
            : 'bg-muted/30 border-border/80',
        ].join(' ')}
      >
        {cells.map((val, idx) => {
          const isWin = isWinningCell(idx)
          const isLast = lastMove === idx
          const canClick = isMyTurn && !disabled && val === EMPTY_CELL
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
              className={[
                'group relative flex items-center justify-center rounded-xl border transition-all select-none',
                'border-border/60 shadow-sm',
                val === EMPTY_CELL
                  ? canClick
                    ? 'bg-background hover:bg-accent/40 hover:border-primary/40 hover:scale-[1.02] cursor-pointer active:scale-95'
                    : 'bg-background/40 cursor-not-allowed opacity-60'
                  : isWin
                    ? 'ttt-win-cell bg-emerald-500/15 border-emerald-400/70 ring-1 ring-emerald-400/40 scale-[1.03] z-10'
                    : isLast
                      ? 'bg-muted/70 border-primary/50 shadow-md'
                      : 'bg-muted/50',
              ].join(' ')}
            >
              {seat && <TicTacToeMarker seat={seat} variant="placed" className={MARKER_SIZE} />}

              {/* Ghost of the current player's marker, so a move is previewed
                  with the shape it will actually leave on the board. */}
              {!seat && canClick && mySeat && (
                <span className="pointer-events-none absolute inset-0 flex items-center justify-center opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
                  <TicTacToeMarker seat={mySeat} variant="preview" className={MARKER_SIZE} />
                </span>
              )}
            </button>
          )
        })}
      </div>

      {mySeat && (
        <div className="mt-3 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          <TicTacToeMarker seat={mySeat} variant="plain" className="h-4 w-4" />
          <span>You play the {markerLabel(mySeat)}</span>
        </div>
      )}
    </div>
  )
}
