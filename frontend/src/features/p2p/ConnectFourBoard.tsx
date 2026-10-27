import { useState, useMemo, type CSSProperties } from 'react'
import { ChevronDown } from 'lucide-react'

import { ConnectFourToken } from './ConnectFourToken'
import {
  dropFallOffset,
  isColumnFull,
  isWinningIndex,
  landingIndex,
  seatForSymbol,
  tokenSeat,
} from './connectFour'

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
  isGameOver?: boolean
  isTie?: boolean
}

export function ConnectFourBoard({
  boardState,
  isMyTurn,
  mySymbol,
  onMove,
  disabled = false,
  isGameOver = false,
  isTie = false,
}: ConnectFourBoardProps) {
  const cols = boardState?.columns || 7
  const rows = boardState?.rows || 6
  const cells = boardState?.cells || new Array(cols * rows).fill(0)
  const winningLine = boardState?.winning_line || null
  const lastMove = boardState?.last_move ?? null

  const [hoveredCol, setHoveredCol] = useState<number | null>(null)

  const mySeat = seatForSymbol(mySymbol)
  const canDropIn = (column: number) =>
    isMyTurn && !disabled && !isGameOver && !isColumnFull(cells, column, cols)
  const canDropHovered = hoveredCol !== null && canDropIn(hoveredCol)
  const ghostIndex = canDropHovered && hoveredCol !== null ? landingIndex(cells, hoveredCol, cols, rows) : null

  // Sort winning line tokens along their geometric direction for a sequential reveal
  const sortedWinningLine = useMemo(() => {
    if (!winningLine || winningLine.length !== 4) return winningLine
    return [...winningLine].sort((a, b) => a - b)
  }, [winningLine])

  const handleColClick = (column: number) => {
    if (!canDropIn(column)) return
    onMove(column)
  }

  const isFullBoard = cells.length > 0 && cells.slice(0, cols).every((c) => c !== 0)
  const isDraw = isTie || (isGameOver && !winningLine && isFullBoard)

  const gridStyle: CSSProperties = { gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))` }

  return (
    <div className="flex w-full select-none flex-col items-center justify-center p-2 sm:p-4">
      {/* Drop targets above the board */}
      <div className="mb-2 grid h-8 w-full max-w-md gap-1.5 sm:gap-2.5" style={gridStyle}>
        {Array.from({ length: cols }).map((_, column) => {
          const canDrop = canDropIn(column)
          const isHovered = hoveredCol === column && canDrop

          return (
            <button
              key={`indicator-${column}`}
              type="button"
              onClick={() => handleColClick(column)}
              onMouseEnter={() => setHoveredCol(column)}
              onMouseLeave={() => setHoveredCol(null)}
              disabled={!canDrop}
              className={[
                'flex items-center justify-center rounded-lg transition-colors',
                canDrop ? 'cursor-pointer text-muted-foreground hover:bg-accent/40' : 'cursor-default opacity-0',
              ].join(' ')}
              aria-label={`Drop token into column ${column + 1}`}
            >
              {isHovered && mySeat ? (
                <span aria-hidden="true">
                  <ConnectFourToken seat={mySeat} ghost className="h-5 w-5" />
                </span>
              ) : canDrop ? (
                <ChevronDown className="h-4 w-4 opacity-40" aria-hidden="true" />
              ) : null}
            </button>
          )
        })}
      </div>

      {/* Board: dark slots in a blue frame, tokens drawn on top of each slot */}
      <div className="relative w-full max-w-md rounded-2xl border-4 border-[#0b2350] bg-gradient-to-b from-[#1e40af] to-[#152a63] p-2.5 shadow-2xl sm:rounded-3xl sm:p-4 dark:border-[#0a1733] dark:from-[#1b2f63] dark:to-[#0d1c3d]">
        <div className="grid gap-1.5 sm:gap-2.5" style={gridStyle}>
          {Array.from({ length: rows }).map((_, row) =>
            Array.from({ length: cols }).map((_, column) => {
              const index = row * cols + column
              const seat = tokenSeat(cells[index])
              const isWin = isWinningIndex(winningLine, index)
              const winIndex = sortedWinningLine ? sortedWinningLine.indexOf(index) : -1
              const isLast = lastMove?.index === index
              const canClickColumn = canDropIn(column)
              const showGhost = !seat && mySeat !== null && ghostIndex === index

              return (
                <button
                  key={`slot-${row}-${column}`}
                  type="button"
                  onClick={() => handleColClick(column)}
                  onMouseEnter={() => setHoveredCol(column)}
                  onMouseLeave={() => setHoveredCol(null)}
                  disabled={!canClickColumn}
                  style={isWin && winIndex >= 0 ? { animationDelay: `${winIndex * 150}ms` } : undefined}
                  className={[
                    'relative aspect-square rounded-full bg-[#0a1733]/85',
                    'shadow-[inset_0_3px_7px_rgba(0,0,0,0.75),inset_0_-1px_2px_rgba(255,255,255,0.08)]',
                    'transition-all duration-300',
                    canClickColumn ? 'cursor-pointer' : 'cursor-default',
                    hoveredCol === column && canClickColumn ? 'ring-1 ring-inset ring-white/20' : '',
                    isWin
                      ? 'ring-4 ring-emerald-400 ring-offset-2 ring-offset-[#0b2350] shadow-[0_0_20px_rgba(52,211,153,0.75)] z-10 c4-win-seq'
                      : '',
                    isDraw ? 'ring-1 ring-amber-400/40' : '',
                  ].join(' ')}
                  aria-label={`Row ${row + 1}, column ${column + 1}: ${
                    seat ? (seat === 1 ? 'teal token' : 'amber token') : 'empty'
                  }${isWin ? ', part of the winning 4-in-a-row connection' : ''}`}
                >
                  {seat && (
                    <span
                      className={[
                        'pointer-events-none absolute inset-[5%] transition-opacity duration-300',
                        isLast && !isGameOver ? 'c4-token-drop' : '',
                        winningLine && !isWin ? 'opacity-40' : 'opacity-100', // distinguish winning 4 from non-winning tokens
                      ].join(' ')}
                      style={
                        isLast && !isGameOver
                          ? ({ '--c4-fall': dropFallOffset(row, rows) } as CSSProperties)
                          : undefined
                      }
                    >
                      <ConnectFourToken
                        seat={seat}
                        className="h-full w-full"
                        emphasis={isWin ? 'win' : isLast ? 'last' : 'none'}
                      />
                    </span>
                  )}

                  {showGhost && mySeat && (
                    <span aria-hidden="true" className="pointer-events-none absolute inset-[7%]">
                      <ConnectFourToken seat={mySeat} ghost className="h-full w-full" />
                    </span>
                  )}
                </button>
              )
            })
          )}
        </div>
      </div>

      {/* Outcome / Turn status footer */}
      <div className="mt-3 flex w-full max-w-md items-center justify-between gap-2 px-1 text-[11px] font-semibold text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <ConnectFourToken seat={1} className="h-4 w-4" />
          <span className={mySeat === 1 ? 'text-foreground font-bold' : ''}>{mySeat === 1 ? 'You' : 'Opponent'}</span>
        </span>

        {winningLine ? (
          <span className="font-extrabold text-emerald-600 dark:text-emerald-400 animate-pulse text-xs">
            4-in-a-Row Connected!
          </span>
        ) : isDraw ? (
          <span className="font-extrabold text-amber-600 dark:text-amber-400 text-xs">
            Full Grid — Draw!
          </span>
        ) : (
          <span className={isMyTurn ? 'text-emerald-600 dark:text-emerald-400' : ''}>
            {isMyTurn ? 'Your turn' : 'Opponent’s turn'}
          </span>
        )}

        <span className="flex items-center gap-1.5">
          <span className={mySeat === 2 ? 'text-foreground font-bold' : ''}>{mySeat === 2 ? 'You' : 'Opponent'}</span>
          <ConnectFourToken seat={2} className="h-4 w-4" />
        </span>
      </div>
    </div>
  )
}
