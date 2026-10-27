import test from 'node:test'
import assert from 'node:assert/strict'

import {
  EMPTY_CELL,
  MARKER_THEME,
  markerLabel,
  seatForCell,
  seatForPlayerIndex,
  seatForSymbol,
} from './ticTacToe.ts'

test('seatForSymbol maps the server symbols X and O onto the two seats', () => {
  assert.equal(seatForSymbol('X'), 1)
  assert.equal(seatForSymbol('x'), 1)
  assert.equal(seatForSymbol(' O '), 2)
  assert.equal(seatForSymbol('o'), 2)
})

test('seatForSymbol ignores missing or unknown symbols instead of guessing', () => {
  assert.equal(seatForSymbol(undefined), null)
  assert.equal(seatForSymbol(null), null)
  assert.equal(seatForSymbol(''), null)
  assert.equal(seatForSymbol('RED'), null)
})

test('seatForPlayerIndex keeps player index 0 on seat one', () => {
  assert.equal(seatForPlayerIndex(0), 1)
  assert.equal(seatForPlayerIndex(1), 2)
  assert.equal(seatForPlayerIndex(2), null)
  assert.equal(seatForPlayerIndex(undefined), null)
})

test('seatForCell reads the board encoding and never treats an empty cell as a move', () => {
  assert.equal(EMPTY_CELL, 0)
  assert.equal(seatForCell(0), null)
  assert.equal(seatForCell(1), 1)
  assert.equal(seatForCell(2), 2)
  assert.equal(seatForCell(3), null)
  assert.equal(seatForCell(undefined), null)
})

test('the two seats are visually distinct and labelled by shape', () => {
  assert.notEqual(MARKER_THEME[1].shape, MARKER_THEME[2].shape)
  assert.notEqual(MARKER_THEME[1].colorClass, MARKER_THEME[2].colorClass)
  assert.equal(markerLabel(1), 'Diamond')
  assert.equal(markerLabel(2), 'Square')
})
