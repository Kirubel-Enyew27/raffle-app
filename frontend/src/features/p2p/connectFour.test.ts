import test from 'node:test'
import assert from 'node:assert/strict'

import {
  EMPTY_SLOT,
  dropFallOffset,
  isColumnFull,
  isWinningIndex,
  landingIndex,
  seatForSymbol,
  tokenSeat,
} from './connectFour.ts'

const COLS = 7
const ROWS = 6

function emptyBoard() {
  return new Array(COLS * ROWS).fill(EMPTY_SLOT)
}

test('tokenSeat maps board cells and never guesses', () => {
  assert.equal(EMPTY_SLOT, 0)
  assert.equal(tokenSeat(0), null)
  assert.equal(tokenSeat(1), 1)
  assert.equal(tokenSeat(2), 2)
  assert.equal(tokenSeat(3), null)
  assert.equal(tokenSeat(undefined), null)
})

test('seatForSymbol maps the server seat names to the drawn tokens', () => {
  assert.equal(seatForSymbol('RED'), 1)
  assert.equal(seatForSymbol('red'), 1)
  assert.equal(seatForSymbol(' YELLOW '), 2)
  assert.equal(seatForSymbol('GREEN'), null)
  assert.equal(seatForSymbol(undefined), null)
})

test('isColumnFull reacts to the top slot only', () => {
  const cells = emptyBoard()
  assert.equal(isColumnFull(cells, 0, COLS), false)

  cells[0] = 1 // top of column 0
  assert.equal(isColumnFull(cells, 0, COLS), true)
  assert.equal(isColumnFull(cells, 1, COLS), false)

  // A token lower down does not fill the column.
  cells[5 * COLS + 1] = 2
  assert.equal(isColumnFull(cells, 1, COLS), false)

  assert.equal(isColumnFull(cells, 9, COLS), false)
  assert.equal(isColumnFull(undefined, 0, COLS), false)
})

test('landingIndex finds the lowest empty slot in a column', () => {
  const cells = emptyBoard()
  assert.equal(landingIndex(cells, 3, COLS, ROWS), (ROWS - 1) * COLS + 3)

  cells[(ROWS - 1) * COLS + 3] = 1
  assert.equal(landingIndex(cells, 3, COLS, ROWS), (ROWS - 2) * COLS + 3)

  const full = emptyBoard()
  for (let row = 0; row < ROWS; row += 1) full[row * COLS + 2] = 1
  assert.equal(landingIndex(full, 2, COLS, ROWS), null)
})

test('isWinningIndex only matches the four tokens the server flagged', () => {
  const line = [30, 31, 32, 33]
  assert.equal(isWinningIndex(line, 32), true)
  assert.equal(isWinningIndex(line, 34), false)
  assert.equal(isWinningIndex(null, 32), false)
})

test('dropFallOffset starts every token above the board and clamps bad rows', () => {
  assert.equal(dropFallOffset(ROWS - 1, ROWS), `${-ROWS * 108}%`)
  assert.equal(dropFallOffset(0, ROWS), '-108%')
  assert.equal(dropFallOffset(-4, ROWS), '-108%')
  assert.equal(dropFallOffset(99, ROWS), `${-ROWS * 108}%`)
})
