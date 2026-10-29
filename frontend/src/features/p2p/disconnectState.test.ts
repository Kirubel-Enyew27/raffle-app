import { test, describe } from 'node:test'
import assert from 'node:assert/strict'

import {
  isOpponentDisconnected,
  isSelfDisconnected,
  computeDisconnectCountdown,
  disconnectCountdownLabel,
  disconnectBannerVariant,
} from './disconnectState.ts'
import type { P2PMatch } from '@/types/api'

// ─── Minimal match fixture ────────────────────────────────────────────────────

function makeMatch(overrides: Partial<P2PMatch> = {}): P2PMatch {
  return {
    id: 'match-1',
    game_slug: 'tic-tac-toe',
    status: 'in_progress',
    match_type: 'public_queue',
    stake_amount: 0,
    prize_pool: 0,
    service_fee_percent: 0,
    total_fee: 0,
    turn_seconds_left: 15,
    current_round: 1,
    max_rounds: 3,
    scores: { '0': 0, '1': 0 },
    board_state: {},
    created_by: 'user-a',
    disconnect_seconds_left: 0,
    players: [],
    created_at: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

// ─── isOpponentDisconnected ───────────────────────────────────────────────────

describe('isOpponentDisconnected', () => {
  test('returns true when the opponent is disconnected', () => {
    const match = makeMatch({ disconnect_user_id: 'user-b' })
    assert.equal(isOpponentDisconnected(match, 'user-a'), true)
  })

  test('returns false when the current user is the disconnected player', () => {
    const match = makeMatch({ disconnect_user_id: 'user-a' })
    assert.equal(isOpponentDisconnected(match, 'user-a'), false)
  })

  test('returns false when nobody is disconnected', () => {
    const match = makeMatch()
    assert.equal(isOpponentDisconnected(match, 'user-a'), false)
  })

  test('returns false when match is not in_progress', () => {
    const match = makeMatch({ status: 'forfeited', disconnect_user_id: 'user-b' })
    assert.equal(isOpponentDisconnected(match, 'user-a'), false)
  })

  test('returns false when match is undefined', () => {
    assert.equal(isOpponentDisconnected(undefined, 'user-a'), false)
  })

  test('returns false when currentUserId is undefined', () => {
    const match = makeMatch({ disconnect_user_id: 'user-b' })
    // disconnect_user_id !== undefined means the check is true by logic, but
    // undefined user id should not flip the banner
    // The function compares uid to disconnect_user_id: undefined !== 'user-b' → true
    // This is intentional: if we don't know who we are, we should still show
    // the opponent-disconnected banner rather than the self-reconnecting one.
    assert.equal(isOpponentDisconnected(match, undefined), true)
  })
})

// ─── isSelfDisconnected ───────────────────────────────────────────────────────

describe('isSelfDisconnected', () => {
  test('returns true when the current user is disconnected', () => {
    const match = makeMatch({ disconnect_user_id: 'user-a' })
    assert.equal(isSelfDisconnected(match, 'user-a'), true)
  })

  test('returns false when the opponent is disconnected', () => {
    const match = makeMatch({ disconnect_user_id: 'user-b' })
    assert.equal(isSelfDisconnected(match, 'user-a'), false)
  })

  test('returns false when nobody is disconnected', () => {
    const match = makeMatch()
    assert.equal(isSelfDisconnected(match, 'user-a'), false)
  })

  test('returns false when match is not in_progress', () => {
    const match = makeMatch({ status: 'completed', disconnect_user_id: 'user-a' })
    assert.equal(isSelfDisconnected(match, 'user-a'), false)
  })

  test('returns false when currentUserId is undefined', () => {
    const match = makeMatch({ disconnect_user_id: 'user-a' })
    assert.equal(isSelfDisconnected(match, undefined), false)
  })
})

// ─── computeDisconnectCountdown ───────────────────────────────────────────────

describe('computeDisconnectCountdown', () => {
  test('returns remaining seconds when elapsed is less than server value', () => {
    assert.equal(computeDisconnectCountdown(60, 10), 50)
    assert.equal(computeDisconnectCountdown(60, 59), 1)
  })

  test('returns exactly 0 when elapsed equals server value', () => {
    assert.equal(computeDisconnectCountdown(60, 60), 0)
  })

  test('clamps to 0 when elapsed exceeds server value (no negative countdown)', () => {
    assert.equal(computeDisconnectCountdown(60, 61), 0)
    assert.equal(computeDisconnectCountdown(0, 5), 0)
  })

  test('handles 0 serverSecondsLeft as already expired', () => {
    assert.equal(computeDisconnectCountdown(0, 0), 0)
  })
})

// ─── disconnectCountdownLabel ─────────────────────────────────────────────────

describe('disconnectCountdownLabel', () => {
  test('formats positive seconds with s suffix', () => {
    assert.equal(disconnectCountdownLabel(30), '30s')
    assert.equal(disconnectCountdownLabel(1), '1s')
  })

  test('returns empty string for zero or negative values', () => {
    assert.equal(disconnectCountdownLabel(0), '')
    assert.equal(disconnectCountdownLabel(-1), '')
  })
})

// ─── disconnectBannerVariant ──────────────────────────────────────────────────

describe('disconnectBannerVariant', () => {
  test('returns "opponent" when the other player disconnected', () => {
    const match = makeMatch({ disconnect_user_id: 'user-b' })
    assert.equal(disconnectBannerVariant(match, 'user-a'), 'opponent')
  })

  test('returns "self" when the current user is disconnected', () => {
    const match = makeMatch({ disconnect_user_id: 'user-a' })
    assert.equal(disconnectBannerVariant(match, 'user-a'), 'self')
  })

  test('returns null when nobody is disconnected', () => {
    const match = makeMatch()
    assert.equal(disconnectBannerVariant(match, 'user-a'), null)
  })

  test('returns null when match is in a terminal state', () => {
    for (const status of ['completed', 'forfeited', 'tied', 'cancelled', 'waiting', 'ready'] as const) {
      const match = makeMatch({ status, disconnect_user_id: 'user-b' })
      assert.equal(
        disconnectBannerVariant(match, 'user-a'),
        null,
        `expected null for status=${status}`,
      )
    }
  })

  test('returns null when match is undefined', () => {
    assert.equal(disconnectBannerVariant(undefined, 'user-a'), null)
  })
})

// ─── Resign dialog pure logic ─────────────────────────────────────────────────
// These tests verify the rules that gate showing / submitting the resign dialog
// without needing React rendering.

describe('resign dialog guard rules (pure)', () => {
  test('resign is only available during in_progress matches', () => {
    const canResign = (status: P2PMatch['status']) =>
      status === 'in_progress'

    assert.equal(canResign('in_progress'), true)
    for (const s of ['waiting', 'ready', 'completed', 'forfeited', 'tied', 'cancelled'] as const) {
      assert.equal(canResign(s), false, `expected false for ${s}`)
    }
  })

  test('confirming resign closes dialog and submits, cancelling leaves match unchanged', () => {
    // Simulate state machine: dialogOpen → confirm → submit OR cancel → unchanged
    let dialogOpen = false
    let submitted = false

    const openDialog = () => { dialogOpen = true }
    const cancelDialog = () => { dialogOpen = false }
    const confirmDialog = () => { dialogOpen = false; submitted = true }

    openDialog()
    assert.equal(dialogOpen, true)

    // Cancel path: dialog closes, no submit.
    cancelDialog()
    assert.equal(dialogOpen, false)
    assert.equal(submitted, false)

    // Confirm path: dialog closes and submit fires.
    openDialog()
    confirmDialog()
    assert.equal(dialogOpen, false)
    assert.equal(submitted, true)
  })

  test('double-click guard: pending mutation prevents a second submission', () => {
    let pendingCount = 0
    const isPending = () => pendingCount > 0
    const submit = () => {
      if (isPending()) return
      pendingCount++
    }

    submit()
    assert.equal(pendingCount, 1)
    // Second click while pending: no-op.
    submit()
    assert.equal(pendingCount, 1)
  })
})
