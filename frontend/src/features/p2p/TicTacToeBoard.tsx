
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

  const handleClick = (index: number) => {
    if (disabled || !isMyTurn || cells[index] !== 0) return
    onMove(index)
  }

  const isWinningCell = (index: number) => winningLine?.includes(index)

  return (
    <div className="flex w-full flex-col items-center justify-center p-4">
      <div className="grid grid-cols-3 gap-3 p-4 bg-muted/30 border border-border/80 rounded-2xl shadow-xl backdrop-blur-sm max-w-sm w-full aspect-square">
        {cells.map((val, idx) => {
          const isWin = isWinningCell(idx)
          const isLast = lastMove === idx
          const canClick = isMyTurn && !disabled && val === 0

          return (
            <button
              key={idx}
              type="button"
              onClick={() => handleClick(idx)}
              disabled={!canClick}
              aria-label={`Cell ${idx + 1}`}
              className={[
                'relative flex items-center justify-center rounded-xl font-black transition-all select-none',
                'border border-border/60 shadow-sm text-3xl sm:text-4xl md:text-5xl',
                val === 0
                  ? canClick
                    ? 'bg-background hover:bg-accent/40 hover:scale-[1.02] cursor-pointer active:scale-95 group'
                    : 'bg-background/40 cursor-not-allowed opacity-60'
                  : isWin
                    ? 'bg-emerald-500/20 border-emerald-500 shadow-emerald-500/30 scale-105 z-10 animate-pulse'
                    : isLast
                      ? 'bg-muted/70 border-primary/50 shadow-md'
                      : 'bg-muted/50',
              ].join(' ')}
            >
              {val === 1 && (
                <span className="text-sky-500 drop-shadow-[0_2px_8px_rgba(14,165,233,0.5)]">
                  ✕
                </span>
              )}
              {val === 2 && (
                <span className="text-amber-500 drop-shadow-[0_2px_8px_rgba(245,158,11,0.5)]">
                  ◯
                </span>
              )}
              {val === 0 && canClick && (
                <span className="opacity-0 group-hover:opacity-30 transition-opacity text-foreground">
                  {mySymbol === 'O' ? '◯' : '✕'}
                </span>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
