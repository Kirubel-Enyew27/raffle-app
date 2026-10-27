import { useState, type CSSProperties } from 'react'
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

  const mySeat = seatForSymbol(mySymbol)
  const canDropIn = (column: number) => isMyTurn && !disabled && !isColumnFull(cells, column, cols)
  const canDropHovered = hoveredCol !== null && canDropIn(hoveredCol)
  const ghostIndex = canDropHovered && hoveredCol !== null ? landingIndex(cells, hoveredCol, cols, rows) : null

  const handleColClick = (column: number) => {
    if (!canDropIn(column)) return
    onMove(column)
  }

  const gridStyle: CSSProperties = { gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))` }

  return (
    <div className="flex w-full select-none flex-col items-center justify-center p-2 sm:p-4">
      {/* Drop targets above the board; the token preview shows whose move it is. */}
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

      {/* Board: dark slots in a blue frame, tokens drawn on top of each slot. */}
      <div className="w-full max-w-md rounded-2xl border-4 border-[#0b2350] bg-gradient-to-b from-[#1e40af] to-[#152a63] p-2.5 shadow-2xl sm:rounded-3xl sm:p-4 dark:border-[#0a1733] dark:from-[#1b2f63] dark:to-[#0d1c3d]">
        <div className="grid gap-1.5 sm:gap-2.5" style={gridStyle}>
          {Array.from({ length: rows }).map((_, row) =>
            Array.from({ length: cols }).map((_, column) => {
              const index = row * cols + column
              const seat = tokenSeat(cells[index])
              const isWin = isWinningIndex(winningLine, index)
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
                  className={[
                    'relative aspect-square rounded-full bg-[#0a1733]/85',
                    'shadow-[inset_0_3px_7px_rgba(0,0,0,0.75),inset_0_-1px_2px_rgba(255,255,255,0.08)]',
                    'transition-shadow',
                    canClickColumn ? 'cursor-pointer' : 'cursor-default',
                    hoveredCol === column && canClickColumn ? 'ring-1 ring-inset ring-white/20' : '',
                    isWin ? 'ring-2 ring-inset ring-emerald-300/70' : '',
                  ].join(' ')}
                  aria-label={`Row ${row + 1}, column ${column + 1}: ${
                    seat ? (seat === 1 ? 'teal token' : 'amber token') : 'empty'
                  }${isWin ? ', part of the winning line' : ''}`}
                >
                  {seat && (
                    <span
                      className={[
                        'pointer-events-none absolute inset-[5%]',
                        isLast ? 'c4-token-drop' : '',
                      ].join(' ')}
                      style={isLast ? ({ '--c4-fall': dropFallOffset(row, rows) } as CSSProperties) : undefined}
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

      {/* Which token belongs to whom, plus whose turn it is. */}
      <div className="mt-3 flex w-full max-w-md items-center justify-between gap-2 px-1 text-[11px] font-semibold text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <ConnectFourToken seat={1} className="h-4 w-4" />
          <span className={mySeat === 1 ? 'text-foreground' : ''}>{mySeat === 1 ? 'You' : 'Opponent'}</span>
        </span>
        <span className={isMyTurn ? 'text-emerald-600 dark:text-emerald-400' : ''}>
          {isMyTurn ? 'Your turn' : 'Opponent’s turn'}
        </span>
        <span className="flex items-center gap-1.5">
          <span className={mySeat === 2 ? 'text-foreground' : ''}>{mySeat === 2 ? 'You' : 'Opponent'}</span>
          <ConnectFourToken seat={2} className="h-4 w-4" />
        </span>
      </div>
    </div>
  )
}
