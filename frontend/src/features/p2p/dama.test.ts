import test from 'node:test'
import assert from 'node:assert/strict'

import {
  isPlayableSquare,
  pieceSide,
  pieceIsKing,
  orientSquare,
  orientationFor,
  displaySquares,
  squareName,
  legalStepsFrom,
  legalTargetsFrom,
  findStep,
  moveNotation,
  resultLabel,
  type DamaBoardState,
} from './dama.ts'

const boardState: DamaBoardState = {
  turn: 0,
  turn_seq: 4,
  cells: new Array(64).fill(0),
  legal_moves: [
    { from: 35, to: 17, captures: [26], continues: true },
    { from: 35, to: 28 },
    { from: 40, to: 33 },
  ],
}

test('playable squares are the dark squares', () => {
  assert.equal(isPlayableSquare(0), false)
  assert.equal(isPlayableSquare(1), true)
  assert.equal(isPlayableSquare(35), true)
  assert.equal(isPlayableSquare(62), true)
  assert.equal(isPlayableSquare(-1), false)
  assert.equal(isPlayableSquare(64), false)
})

test('piece decoding distinguishes sides and kings', () => {
  assert.equal(pieceSide(0), null)
  assert.equal(pieceSide(1), 0)
  assert.equal(pieceSide(2), 0)
  assert.equal(pieceSide(3), 1)
  assert.equal(pieceSide(4), 1)
  assert.equal(pieceIsKing(1), false)
  assert.equal(pieceIsKing(2), true)
  assert.equal(pieceIsKing(4), true)
})

test('orientation flips the board for the dark player and is reversible', () => {
  assert.equal(orientationFor(0), 0)
  assert.equal(orientationFor(1), 1)
  assert.equal(orientationFor(undefined), 0)
  assert.equal(orientSquare(0, 0), 0)
  assert.equal(orientSquare(0, 1), 63)
  assert.equal(orientSquare(63, 1), 0)
  assert.equal(orientSquare(35, 1), 28)
  assert.equal(orientSquare(orientSquare(35, 1), 1), 35)
})

test('display squares render 32 playable cells and the 12+12 starting men', () => {
  const cells = new Array(64).fill(0)
  for (let row = 0; row < 8; row++) {
    for (let col = 0; col < 8; col++) {
      const index = row * 8 + col
      if ((row + col) % 2 !== 1) continue
      if (row < 3) cells[index] = 3
      if (row > 4) cells[index] = 1
    }
  }

  const squares = displaySquares(cells, 0)
  assert.equal(squares.length, 64)
  assert.equal(squares.filter((square) => square.playable).length, 32)
  assert.equal(squares.filter((square) => square.value === 1).length, 12)
  assert.equal(squares.filter((square) => square.value === 3).length, 12)

  // Rotating for the dark player reverses the visual order without changing
  // which squares/pieces exist.
  const rotated = displaySquares(cells, 1)
  assert.equal(rotated[0].index, 63)
  assert.deepEqual(
    rotated.filter((square) => square.playable).map((square) => square.index).sort((a, b) => a - b),
    squares.filter((square) => square.playable).map((square) => square.index).sort((a, b) => a - b)
  )
  assert.equal(rotated.filter((square) => square.value === 1).length, 12)
})

test('algebraic square names match a standard board', () => {
  assert.equal(squareName(0), 'a8')
  assert.equal(squareName(7), 'h8')
  assert.equal(squareName(56), 'a1')
  assert.equal(squareName(63), 'h1')
  assert.equal(squareName(40), 'a3')
  assert.equal(squareName(35), 'd4')
})

test('legal destinations come from the server move list', () => {
  const steps = legalStepsFrom(boardState, 35)
  assert.equal(steps.length, 2)
  assert.deepEqual(legalTargetsFrom(boardState, 35), [17, 28])
  assert.deepEqual(legalTargetsFrom(boardState, 40), [33])
  assert.deepEqual(legalTargetsFrom(boardState, 99), [])
  assert.deepEqual(legalTargetsFrom(undefined, 35), [])

  assert.equal(findStep(boardState, 35, 17)?.continues, true)
  assert.equal(findStep(boardState, 35, 28)?.continues, undefined)
  assert.equal(findStep(boardState, 35, 99), undefined)
})

test('notation shows captures and promotions', () => {
  assert.equal(moveNotation({ from: 40, to: 33 }), 'a3–b4')
  assert.equal(moveNotation({ from: 35, to: 17, captures: [26] }), 'd4×b6')
  assert.equal(moveNotation({ from: 17, to: 3, captures: [10], promotion: true }), 'b6×d8=K')
})

test('result labels are written from the local player perspective', () => {
  assert.equal(resultLabel(null, 0), 'Match in progress')
  assert.equal(resultLabel({ winner: 0, draw: false, reason: 'no_legal_moves' }, 0), 'Victory')
  assert.equal(resultLabel({ winner: 0, draw: false, reason: 'no_legal_moves' }, 1), 'Defeat')
  assert.equal(resultLabel({ winner: null, draw: true, reason: 'draw_repetition' }, 0), 'Draw — threefold repetition')
  assert.equal(resultLabel({ winner: null, draw: true, reason: 'draw_halfmove' }, 1), 'Draw — 50-move rule')
  assert.equal(resultLabel({ winner: null, draw: true, reason: 'other' }, 0), 'Draw')
})
