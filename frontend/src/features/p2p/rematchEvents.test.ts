import assert from 'node:assert/strict'
import test from 'node:test'

import { consumeRematchRequest } from './rematchEvents.ts'

test('a rematch request event is consumed only once for its match and requester', () => {
  const seen = new Set<string>()

  assert.equal(consumeRematchRequest(seen, 'match-1', 'player-a'), true)
  assert.equal(consumeRematchRequest(seen, 'match-1', 'player-a'), false)
  assert.equal(seen.size, 1)
})

test('separate rematches and requesters retain independent event identities', () => {
  const seen = new Set<string>()

  assert.equal(consumeRematchRequest(seen, 'match-1', 'player-a'), true)
  assert.equal(consumeRematchRequest(seen, 'match-1', 'player-b'), true)
  assert.equal(consumeRematchRequest(seen, 'match-2', 'player-a'), true)
  assert.equal(consumeRematchRequest(seen, undefined, 'player-a'), false)
})
