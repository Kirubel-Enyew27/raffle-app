import test from 'node:test'
import assert from 'node:assert/strict'

import {
  RPS_CHOICES,
  isRpsChoice,
  parseRpsChoice,
  roundVerdict,
  seriesProgress,
  type RpsRoundResult,
} from './rockPaperScissors.ts'

test('every choice declares the opponent move it beats', () => {
  const beats = Object.fromEntries(RPS_CHOICES.map((choice) => [choice.id, choice.beats]))
  assert.deepEqual(beats, { rock: 'scissors', paper: 'rock', scissors: 'paper' })
})

test('choice metadata stays in sync with the server vocabulary', () => {
  assert.deepEqual(
    RPS_CHOICES.map((choice) => choice.id),
    ['rock', 'paper', 'scissors']
  )
  for (const choice of RPS_CHOICES) {
    assert.equal(isRpsChoice(choice.id), true)
    assert.equal(isRpsChoice(choice.beats), true)
  }
  assert.equal(isRpsChoice('lizard'), false)
  assert.equal(isRpsChoice(undefined), false)
})

test('parseRpsChoice rejects the masked opponent entry instead of rendering it', () => {
  assert.equal(parseRpsChoice('rock'), 'rock')
  assert.equal(parseRpsChoice('hidden'), null)
  assert.equal(parseRpsChoice(undefined), null)
  assert.equal(parseRpsChoice(''), null)
})

test('roundVerdict reads the server result from either seat', () => {
  const p0Won: RpsRoundResult = { p0_choice: 'rock', p1_choice: 'scissors', is_tie: false, winner: 0 }
  assert.equal(roundVerdict(p0Won, 0), 'win')
  assert.equal(roundVerdict(p0Won, 1), 'lose')

  const p1Won: RpsRoundResult = { p0_choice: 'paper', p1_choice: 'scissors', is_tie: false, winner: 1 }
  assert.equal(roundVerdict(p1Won, 0), 'lose')
  assert.equal(roundVerdict(p1Won, 1), 'win')

  const tie: RpsRoundResult = { p0_choice: 'paper', p1_choice: 'paper', is_tie: true, winner: null }
  assert.equal(roundVerdict(tie, 0), 'draw')
  assert.equal(roundVerdict(tie, 1), 'draw')

  assert.equal(roundVerdict(null, 0), null)
  assert.equal(roundVerdict(p0Won, undefined), null)
})

test('seriesProgress reports best-of-three progress from the local seat', () => {
  assert.deepEqual(seriesProgress({ '0': 1, '1': 0 }, 0, 2, 3), {
    myWins: 1,
    opponentWins: 0,
    winsNeeded: 2,
    roundsPlayed: 2,
  })
  assert.deepEqual(seriesProgress({ '0': 1, '1': 2 }, 0, 3, 3), {
    myWins: 1,
    opponentWins: 2,
    winsNeeded: 2,
    roundsPlayed: 3,
  })
  // Seat one sees the same series mirrored.
  assert.equal(seriesProgress({ '0': 1, '1': 2 }, 1, 3, 3)?.myWins, 2)
  // Missing or malformed scores never render as NaN.
  assert.deepEqual(seriesProgress(undefined, 0, 1, 3), {
    myWins: 0,
    opponentWins: 0,
    winsNeeded: 2,
    roundsPlayed: 1,
  })
  assert.equal(seriesProgress({ '0': 0, '1': 0 }, undefined, 1, 3), null)
})
