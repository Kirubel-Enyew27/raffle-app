import { useState } from 'react'

interface ConnectFourBoardProps {
  boardState: {
    cells?: number[]
    columns?: number
    rows?: number
    last_move?: { row: number; col: number; index: number } | null
    winning_line?: number[] | null
  }
  isMyTurn: boolean
  mySymbol?: string
  onMove: (columnIndex: number) => void
  disabled?: boolean
}

export function ConnectFourBoard({
  boardState,
  isMyTurn,
  mySymbol,
  onMove,
  disabled = false,
}: ConnectFourBoardProps) {
  const cols = boardState?.columns || 7
  const rows = boardState?.rows || 6
  const cells = boardState?.cells || new Array(cols * rows).fill(0)
  const winningLine = boardState?.winning_line || null
  const lastMove = boardState?.last_move ?? null

  const [hoveredCol, setHoveredCol] = useState<number | null>(null)

  const isColFull = (c: number) => cells[c] !== 0 // top row full

  const handleColClick = (c: number) => {
    if (disabled || !isMyTurn || isColFull(c)) return
    onMove(c)
  }

  const isWinningCell = (idx: number) => winningLine?.includes(idx)

  return (
    <div className="flex w-full flex-col items-center justify-center p-2 sm:p-4 select-none">
      {/* Column drop indicators */}
      <div className="grid grid-cols-7 gap-1.5 sm:gap-2.5 max-w-md w-full mb-2 h-7">
        {Array.from({ length: cols }).map((_, c) => {
          const canDrop = isMyTurn && !disabled && !isColFull(c)
          const isHovered = hoveredCol === c && canDrop

          return (
            <button
              key={`indicator-${c}`}
              type="button"
              onClick={() => handleColClick(c)}
              onMouseEnter={() => setHoveredCol(c)}
              onMouseLeave={() => setHoveredCol(null)}
              disabled={!canDrop}
              className={[
                'flex items-center justify-center rounded-lg transition-all',
                canDrop
                  ? 'cursor-pointer hover:bg-accent/40 text-muted-foreground'
                  : 'cursor-default opacity-0',
              ].join(' ')}
              aria-label={`Drop chip into column ${c + 1}`}
            >
              {isHovered && (
                <div
                  className={[
                    'w-4 h-4 sm:w-5 sm:h-5 rounded-full shadow-md animate-bounce',
                    mySymbol === 'YELLOW' ? 'bg-amber-400' : 'bg-rose-500',
                  ].join(' ')}
                />
              )}
            </button>
          )
        })}
      </div>

      {/* Main 7x6 Board */}
      <div className="bg-blue-600 dark:bg-blue-800 p-2.5 sm:p-4 rounded-2xl sm:rounded-3xl shadow-2xl border-4 border-blue-700 dark:border-blue-900 max-w-md w-full">
        <div className="grid grid-cols-7 gap-1.5 sm:gap-2.5">
          {Array.from({ length: rows }).map((_, r) =>
            Array.from({ length: cols }).map((_, c) => {
              const idx = r * cols + c
              const val = cells[idx]
              const isWin = isWinningCell(idx)
              const isLast = lastMove?.index === idx
              const canClickCol = isMyTurn && !disabled && !isColFull(c)

              return (
                <button
                  key={`cell-${r}-${c}`}
                  type="button"
                  onClick={() => handleColClick(c)}
                  onMouseEnter={() => setHoveredCol(c)}
                  onMouseLeave={() => setHoveredCol(null)}
                  disabled={!canClickCol}
                  className={[
                    'aspect-square rounded-full flex items-center justify-center transition-transform relative',
                    canClickCol ? 'cursor-pointer' : 'cursor-default',
                    val === 0
                      ? 'bg-background/90 dark:bg-slate-900 shadow-inner'
                      : val === 1
                        ? 'bg-gradient-to-tr from-rose-600 to-rose-400 shadow-lg shadow-rose-900/40 border-2 border-rose-400'
                        : 'bg-gradient-to-tr from-amber-500 to-yellow-300 shadow-lg shadow-amber-900/40 border-2 border-yellow-200',
                    isWin ? 'ring-4 ring-emerald-400 ring-offset-2 animate-pulse scale-105 z-10' : '',
                    isLast && !isWin ? 'ring-2 ring-white/70' : '',
                  ].join(' ')}
                  aria-label={`Row ${r + 1}, Column ${c + 1}`}
                >
                  {/* Subtle 3D shine effect inside chips */}
                  {val !== 0 && (
                    <div className="w-1/2 h-1/2 rounded-full bg-white/20 blur-[1px] absolute top-1 left-1 pointer-events-none" />
                  )}
                </button>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
