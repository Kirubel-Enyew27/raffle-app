import { test, expect, type Page, type TestInfo } from '@playwright/test'
import {
  backendReachable,
  createPrivateMatch,
  createSession,
  fetchMatch,
  joinByCode,
  seedSession,
  submitMove,
} from './helpers/api'

/**
 * Two real browser sessions play the same match.
 *
 * The regression under test: a player whose realtime stream could not be
 * established used to stay on "Connecting…" with a frozen board, while the
 * other player played and eventually won on a turn timeout.
 */

const BADGE = '[data-testid="match-connection-badge"]'

async function badgeState(page: Page): Promise<string | null> {
  const badge = page.locator(BADGE)
  if ((await badge.count()) === 0) return null
  return badge.getAttribute('data-state')
}

async function bodyText(page: Page): Promise<string> {
  return (await page.innerText('body')).replace(/\s+/g, ' ')
}

/** Clicks an empty Tic-Tac-Toe cell by its accessible name. */
async function clickCell(page: Page, index: number) {
  await page.getByRole('button', { name: new RegExp(`^Cell ${index + 1},`) }).click()
}

function percentile(values: number[], p: number): number {
  const ordered = [...values].sort((a, b) => a - b)
  return ordered[Math.max(0, Math.ceil((p / 100) * ordered.length) - 1)]
}

function latencyStats(values: number[]) {
  if (values.length === 0) return null
  return {
    samples: values.length,
    average_ms: values.reduce((sum, value) => sum + value, 0) / values.length,
    p50_ms: percentile(values, 50),
    p95_ms: percentile(values, 95),
    p99_ms: percentile(values, 99),
  }
}

test.describe('P2P multiplayer', () => {
  test.beforeEach(async () => {
    test.skip(!(await backendReachable()), 'backend is not reachable on localhost:8080')
  })

  test('two sessions join, recover after refresh/network loss, and moves stay in sync', async ({ browser }, testInfo: TestInfo) => {
    const hostSession = await createSession('host')
    const guestSession = await createSession('guest')
    const match = await createPrivateMatch(hostSession)

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()

    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })

    // Both clients must leave "Connecting…" — the reported bug kept one there.
    await expect(host.locator(BADGE)).toBeVisible()
    await expect(guest.locator(BADGE)).toBeVisible()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).not.toBe('connecting')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).not.toBe('connecting')

    // Both see the same room with both players seated.
    await expect(host.getByText('Both Players Joined!')).toBeVisible()
    await expect(guest.getByText('Both Players Joined!')).toBeVisible()

    // Both ready up; each sees its own state and the opponent's readiness.
    await host.getByRole('button', { name: /I am Ready/ }).click()
    await expect(host.getByText(/You are ready!|Both players are ready/)).toBeVisible()
    await guest.getByRole('button', { name: /I am Ready/ }).click()

    // The match starts for both clients.
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')

    // A reload must recover the same active match from the authoritative API.
    await host.reload()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')

    // Measure browser-to-browser move-to-board latency in this Playwright run.
    // This includes REST, server validation/commit, Redis/SSE, React update and
    // the opponent DOM becoming observable. It is not a backend microbenchmark.
    const moveLatenciesMs: number[] = []

    // The host opens; once they move, the guest's board updates.
    await expect(host.getByText('YOUR TURN')).toBeVisible()
    const firstMoveStart = performance.now()
    await clickCell(host, 0)
    await expect
      .poll(async () => guest.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'), { timeout: 10_000 })
      .not.toContain('empty')
    moveLatenciesMs.push(performance.now() - firstMoveStart)

    // Simulate an internet outage while the players continue through several
    // turns. The host must recover all intervening moves on the same match.
    await hostContext.setOffline(true)
    await clickCell(guest, 4)
    expect(await submitMove(hostSession, match.id, { cell: 1 })).toBe(200)
    expect(await submitMove(guestSession, match.id, { cell: 3 })).toBe(200)
    const recoveryStart = performance.now()
    await hostContext.setOffline(false)
    await expect
      .poll(async () => host.getByRole('button', { name: /^Cell 5,/ }).getAttribute('aria-label'), { timeout: 15_000 })
      .not.toContain('empty')
    await expect
      .poll(async () => host.getByRole('button', { name: /^Cell 2,/ }).getAttribute('aria-label'), { timeout: 10_000 })
      .not.toContain('empty')
    await expect
      .poll(async () => host.getByRole('button', { name: /^Cell 4,/ }).getAttribute('aria-label'), { timeout: 10_000 })
      .not.toContain('empty')
    const recoveryLatencyMs = performance.now() - recoveryStart
    await expect(host.getByText('YOUR TURN')).toBeVisible()

    // The host completes round one now that the stale view is recovered.
    const roundOneMoves: Array<[Page, Page, number]> = [[host, guest, 2]]
    for (const [actor, opponent, cell] of roundOneMoves) {
      const startedAt = performance.now()
      await clickCell(actor, cell)
      await expect
        .poll(async () => opponent.getByRole('button', { name: new RegExp(`^Cell ${cell + 1},`) }).getAttribute('aria-label'), { timeout: 10_000 })
        .not.toContain('empty')
      moveLatenciesMs.push(performance.now() - startedAt)
    }

    // Round two opens with the guest. A second host win completes the best of
    // three, letting this same two-browser flow verify terminal agreement.
    await expect.poll(async () => (await fetchMatch(hostSession, match.id)).current_round, { timeout: 12_000 }).toBe(2)
    // Recover both views from the same authoritative match after the reveal;
    // this also covers a missing round-start notification.
    await Promise.all([host.reload(), guest.reload()])
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')
    await expect(guest.getByText('YOUR TURN')).toBeVisible({ timeout: 10_000 })
    const roundTwoMoves: Array<[Page, Page, number]> = [
      [guest, host, 0],
      [host, guest, 4],
      [guest, host, 1],
      [host, guest, 3],
      [guest, host, 8],
      [host, guest, 5],
    ]
    for (const [actor, opponent, cell] of roundTwoMoves) {
      const startedAt = performance.now()
      await clickCell(actor, cell)
      await expect
        .poll(async () => opponent.getByRole('button', { name: new RegExp(`^Cell ${cell + 1},`) }).getAttribute('aria-label'), { timeout: 10_000 })
        .not.toContain('empty')
      moveLatenciesMs.push(performance.now() - startedAt)
    }

    await expect.poll(() => badgeState(host), { timeout: 10_000 }).toBe('completed')
    await expect.poll(() => badgeState(guest), { timeout: 10_000 }).toBe('completed')

    const serverTimingSamples = await guest.evaluate((matchId) =>
      performance
        .getEntriesByType('mark')
        .filter((entry) => entry.name.startsWith(`p2p-move:${matchId}:`))
        .map((entry) => ({
          version: entry.name.slice(entry.name.lastIndexOf(':') + 1),
          ...(entry as PerformanceMark).detail as {
            server_timing_ms?: Record<string, number>
            server_delivery_latency_ms?: number
            event_payload_bytes?: number
          },
        })),
    match.id)
    const stageValues = (stage: string) => serverTimingSamples
      .map((sample) => sample.server_timing_ms?.[stage])
      .filter((value): value is number => typeof value === 'number')
    expect(stageValues('application_to_commit').length).toBeGreaterThan(0)
    const observedVersions = serverTimingSamples.map((sample) => sample.version)
    const duplicateMoveEvents = observedVersions.length - new Set(observedVersions).size
    let outOfOrderMoveEvents = 0
    let highestVersion = -Infinity
    for (const version of observedVersions) {
      const numericVersion = Number(version)
      if (!Number.isFinite(numericVersion)) continue
      if (numericVersion < highestVersion) outOfOrderMoveEvents++
      highestVersion = Math.max(highestVersion, numericVersion)
    }
    const summary = {
      browser_to_opponent_dom: latencyStats(moveLatenciesMs),
      backend_transaction_work: latencyStats(stageValues('transaction_work')),
      move_validation: latencyStats(stageValues('move_validation')),
      database_commit: latencyStats(stageValues('database_commit')),
      application_to_commit: latencyStats(stageValues('application_to_commit')),
      publish_to_sse_delivery: latencyStats(serverTimingSamples
        .map((sample) => sample.server_delivery_latency_ms)
        .filter((value): value is number => typeof value === 'number')),
      full_snapshot_payload_bytes: (() => {
        const values = serverTimingSamples
          .map((sample) => sample.event_payload_bytes)
          .filter((value): value is number => typeof value === 'number')
        if (values.length === 0) return null
        return {
          samples: values.length,
          average_bytes: values.reduce((sum, value) => sum + value, 0) / values.length,
          p50_bytes: percentile(values, 50),
          p95_bytes: percentile(values, 95),
          p99_bytes: percentile(values, 99),
        }
      })(),
      reconnect_recovery_ms: recoveryLatencyMs,
      duplicate_move_events: duplicateMoveEvents,
      out_of_order_move_events: outOfOrderMoveEvents,
    }
    await testInfo.attach('multiplayer-latency-summary.json', {
      body: Buffer.from(JSON.stringify(summary, null, 2)),
      contentType: 'application/json',
    })
    console.log(`[multiplayer-latency] ${JSON.stringify(summary)}`)

    // Both clients and the authoritative API agree the match completed.
    const state = await fetchMatch(hostSession, match.id)
    expect(state.status).toBe('completed')
  })

  test('Ethiopian Dama move is applied to the opponent board over SSE', async ({ browser }) => {
    const hostSession = await createSession('dama-host')
    const guestSession = await createSession('dama-guest')
    const match = await createPrivateMatch(hostSession, 'ethiopian-dama')

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')

    // The opening move is a3 → b4 for the first seat in the server's standard
    // Ethiopian Dama position.
    await host.getByRole('button', { name: /^a3, ivory checker/ }).click()
    await host.getByRole('button', { name: /^b4, empty, legal destination/ }).click()
    await expect
      .poll(async () => guest.getByRole('button', { name: /^b4, ivory checker/ }).count(), { timeout: 10_000 })
      .toBe(1)

    const state = await fetchMatch(hostSession, match.id)
    expect(state.status).toBe('in_progress')
    const board = state.board_state as { move_count?: number }
    expect(board.move_count).toBe(1)
    await hostContext.close()
    await guestContext.close()
  })

  test('Rock Paper Scissors choices stay private until both players lock in', async ({ browser }) => {
    const hostSession = await createSession('rps-host')
    const guestSession = await createSession('rps-guest')
    const match = await createPrivateMatch(hostSession, 'rock-paper-scissors')

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')

    await host.getByRole('button', { name: /^Rock/ }).click()
    await host.getByRole('button', { name: /Lock in Rock/ }).click()
    await expect(host.getByText('Locked in: Rock')).toBeVisible()

    const guestState = await fetchMatch(guestSession, match.id)
    const hiddenChoices = (guestState.board_state as { choices?: Record<string, string> }).choices
    expect(hiddenChoices?.['0']).toBe('hidden')
    await expect(guest.getByText('Locked in: Rock')).toHaveCount(0)

    await guest.getByRole('button', { name: /^Scissors/ }).click()
    await guest.getByRole('button', { name: /Lock in Scissors/ }).click()
    await expect(host.getByText('You Won the Round!')).toBeVisible({ timeout: 10_000 })
    await expect(guest.getByText('Opponent Won the Round!')).toBeVisible({ timeout: 10_000 })

    const state = await fetchMatch(hostSession, match.id)
    expect(state.status).toBe('in_progress')
    expect((state.scores as Record<string, number>)['0']).toBe(1)
    expect((state.board_state as { last_round_result?: { p0_choice?: string; p1_choice?: string } }).last_round_result)
      .toMatchObject({ p0_choice: 'rock', p1_choice: 'scissors' })
    await hostContext.close()
    await guestContext.close()
  })

  test('one click requests and accepts a rematch for both sessions', async ({ browser }) => {
    const hostSession = await createSession('rematch-host')
    const guestSession = await createSession('rematch-guest')
    const match = await createPrivateMatch(hostSession, 'connect-four')

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })

    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')

    const finishConnectFour = async (matchID: string) => {
      let state = await fetchMatch(hostSession, matchID)
      for (const column of [0, 1, 0, 1, 0, 1, 0]) {
        const actor = state.current_turn_user_id === hostSession.user.id ? hostSession : guestSession
        expect(await submitMove(actor, matchID, { column })).toBe(200)
        state = await fetchMatch(hostSession, matchID)
      }
      return state
    }
    await finishConnectFour(match.id)

    // A winner and loser returning to a terminal match must see the persisted
    // result, not a transient loading state or a resurrected live board.
    await host.reload()
    await guest.reload()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('completed')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('completed')

    await host.getByRole('button', { name: 'Play Again' }).click()
    await expect(host.getByText('Your opponent will be notified. Waiting for their response…')).toBeVisible()
    await expect(guest.getByText('Your opponent wants to play again.')).toHaveCount(1, { timeout: 10_000 })
    await expect(guest.getByRole('button', { name: 'Accept' })).toBeVisible()
    // Pending rematch consent is authoritative server state and survives a
    // refresh, including the notification's action button.
    await guest.reload()
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('completed')
    await expect(guest.getByRole('button', { name: 'Accept' })).toBeVisible()

    // One Accept click must be enough. Both the HTTP acknowledgement and the
    // rematch_created event can arrive, but they must lead to the same match.
    const rematchResponsePromise = guest.waitForResponse((response) =>
      response.request().method() === 'POST' && /\/p2p\/matches\/[^/]+\/rematch/.test(response.url()),
    )
    await guest.getByRole('button', { name: 'Accept' }).click()
    const rematchResponse = await rematchResponsePromise
    const rematchAcknowledgement = await rematchResponse.json()
    expect(rematchResponse.ok()).toBe(true)
    expect(rematchAcknowledgement?.data?.status).toBe('started')
    expect(rematchAcknowledgement?.data?.match?.id).not.toBe(match.id)
    const rematchUrl = /\/games\/match\/([0-9a-f-]{36})$/i
    await expect(host).toHaveURL(rematchUrl, { timeout: 15_000 })
    await expect(guest).toHaveURL(rematchUrl, { timeout: 15_000 })
    const hostMatchId = host.url().match(rematchUrl)?.[1]
    const guestMatchId = guest.url().match(rematchUrl)?.[1]
    expect(hostMatchId).toBeTruthy()
    expect(guestMatchId).toBe(hostMatchId)
    expect(hostMatchId).not.toBe(match.id)

    const rematchState = await fetchMatch(hostSession, hostMatchId!)
    expect(rematchState.status).toBe('in_progress')
    const priorState = await fetchMatch(hostSession, match.id)
    expect(priorState.status).toBe('completed')
    expect(priorState.rematch_match_id).toBe(hostMatchId)

    // Complete that rematch and immediately start another. This catches stale
    // events or cached state from an earlier completed match overwriting the
    // current match after consecutive rematches.
    await finishConnectFour(hostMatchId!)
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('completed')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('completed')

    await host.getByRole('button', { name: 'Play Again' }).click()
    await expect(guest.getByRole('button', { name: 'Accept' })).toBeVisible({ timeout: 10_000 })
    await expect(guest.getByText('Your opponent wants to play again.')).toHaveCount(1)
    const secondRematchResponsePromise = guest.waitForResponse((response) =>
      response.request().method() === 'POST' && /\/p2p\/matches\/[^/]+\/rematch/.test(response.url()),
    )
    await guest.getByRole('button', { name: 'Accept' }).click()
    const secondRematchResponse = await secondRematchResponsePromise
    const secondRematchAcknowledgement = await secondRematchResponse.json()
    expect(secondRematchResponse.ok()).toBe(true)
    expect(secondRematchAcknowledgement?.data?.status).toBe('started')
    const secondRematchId = secondRematchAcknowledgement?.data?.match?.id
    expect(secondRematchId).toBeTruthy()
    expect(secondRematchId).not.toBe(hostMatchId)
    expect(secondRematchId).not.toBe(match.id)
    await expect(host).toHaveURL(rematchUrl, { timeout: 15_000 })
    await expect(guest).toHaveURL(rematchUrl, { timeout: 15_000 })
    expect(host.url().match(rematchUrl)?.[1]).toBe(secondRematchId)
    expect(guest.url().match(rematchUrl)?.[1]).toBe(secondRematchId)
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')
    const firstRematchTerminal = await fetchMatch(hostSession, hostMatchId!)
    expect(firstRematchTerminal.status).toBe('completed')
    expect(firstRematchTerminal.rematch_match_id).toBe(secondRematchId)
  })

  test('a player with a blocked stream still sees the board and can play', async ({ browser }) => {
    const hostSession = await createSession('host')
    const guestSession = await createSession('guest')
    const match = await createPrivateMatch(hostSession)

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    // The guest's realtime stream cannot be established (proxy blocks
    // streaming, captive portal, 429…). The board must still work.
    await guestContext.route('**/api/v1/realtime/stream*', (route) => route.abort('failed'))

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()

    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })

    // The guest is candidly degraded — never a bare, endless "Connecting…".
    await expect.poll(() => badgeState(guest), { timeout: 20_000 }).toMatch(/reconnecting|failed/)

    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()

    // The blocked player still receives the started match through the
    // authoritative polling fallback. The badge stays candidly "reconnecting",
    // but the board and the turn clock are live — before the fix this client sat
    // on "Both Players Joined" (or a bare "Connecting…") for the whole match.
    await expect(guest.getByText(/YOUR TURN|OPPONENT TURN/), {
      timeout: 25_000,
    }).toBeVisible()
    await expect(guest.getByRole('button', { name: /^Cell 1, / })).toBeVisible()

    // The host moves; the guest's board catches up from the server.
    expect(await submitMove(hostSession, match.id, { cell: 0 })).toBe(200)

    await expect
      .poll(async () => (await bodyText(guest)).includes('YOUR TURN'), { timeout: 15_000 })
      .toBe(true)
    await expect(guest.getByRole('button', { name: /^Cell 1, / })).toBeVisible()

    // Crucially, the stuck client is not left to time out: the grace period
    // governs the match, so it is still in progress well past the turn clock.
    await guest.waitForTimeout(18_000)
    const state = await fetchMatch(guestSession, match.id)
    expect(state.status, 'a disconnected client must not lose on the turn clock').toBe('in_progress')
  })

  test('connect four marks all four winning tokens on both clients', async ({ browser }) => {
    const hostSession = await createSession('c4host')
    const guestSession = await createSession('c4guest')
    const match = await createPrivateMatch(hostSession, 'connect-four')

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()

    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })

    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')
    await expect.poll(() => badgeState(guest), { timeout: 15_000 }).toBe('in_progress')

    // Host stacks column 0, the guest answers in column 1; the seventh drop
    // completes a vertical four and ends the match.
    const drops: Array<[typeof hostSession, number]> = [
      [hostSession, 0],
      [guestSession, 1],
      [hostSession, 0],
      [guestSession, 1],
      [hostSession, 0],
      [guestSession, 1],
      [hostSession, 0],
    ]
    for (const [session, column] of drops) {
      expect(await submitMove(session, match.id, { column })).toBe(200)
    }

    // Every token of the winning four is flagged for the win effect — not just
    // the last one dropped — on both clients, and it survives the terminal
    // state refetch that follows match completion.
    const winningTokens = '[aria-label*="part of the winning 4-in-a-row connection"]'
    await expect(host.locator(winningTokens)).toHaveCount(4, { timeout: 15_000 })
    await expect(guest.locator(winningTokens)).toHaveCount(4, { timeout: 15_000 })

    // All four carry the winning animation class, not only the last drop.
    await expect(host.locator(`${winningTokens}.c4-win-seq`)).toHaveCount(4)
    await expect(guest.locator(`${winningTokens}.c4-win-seq`)).toHaveCount(4)

    // The completed match must keep the full board (including the final drop).
    const terminal = await fetchMatch(hostSession, match.id)
    expect(terminal.status).toBe('completed')
    const line = (terminal.board_state as { winning_line?: number[] } | undefined)?.winning_line
    expect(line?.length, 'the terminal state must expose the whole winning line').toBe(4)

    // A stale refetch used to erase the winning line a moment after it appeared.
    await host.waitForTimeout(2_500)
    await expect(host.locator(winningTokens)).toHaveCount(4)
    await expect(guest.locator(winningTokens)).toHaveCount(4)
  })

  test('tic-tac-toe marks all three winning cells once the series is won', async ({ browser }) => {
    const hostSession = await createSession('ttthost')
    const guestSession = await createSession('tttguest')
    const match = await createPrivateMatch(hostSession, 'tic-tac-toe')

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()

    await joinByCode(guestSession, match.invite_code)
    await host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })
    await guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' })

    await host.getByRole('button', { name: /I am Ready/ }).click()
    await guest.getByRole('button', { name: /I am Ready/ }).click()
    await expect.poll(() => badgeState(host), { timeout: 15_000 }).toBe('in_progress')

    // Best of three: the host takes rounds one and three, the guest round two.
    // The starter alternates, so each round's moves are submitted in turn.
    const rounds: Array<Array<[typeof hostSession, number]>> = [
      [
        [hostSession, 0],
        [guestSession, 3],
        [hostSession, 1],
        [guestSession, 4],
        [hostSession, 2],
      ],
      [
        [guestSession, 0],
        [hostSession, 3],
        [guestSession, 1],
        [hostSession, 4],
        [guestSession, 2],
      ],
      [
        [hostSession, 0],
        [guestSession, 3],
        [hostSession, 1],
        [guestSession, 4],
        [hostSession, 2],
      ],
    ]
    // All three cells of the winning row carry the winning effect on both
    // clients, not just the last square played.
    const winningCells = '[aria-label*="part of the winning line"]'

    for (const [index, round] of rounds.entries()) {
      for (const [session, cell] of round) {
        expect(await submitMove(session, match.id, { cell })).toBe(200)
      }

      if (index < rounds.length - 1) {
        // A non-deciding round is held on screen briefly, so both players see
        // the winning line before the fresh board of the next round appears.
        await expect(host.locator(winningCells)).toHaveCount(3, { timeout: 10_000 })
        await expect(guest.locator(winningCells)).toHaveCount(3, { timeout: 10_000 })

        const nextStarter = index === 0 ? guestSession.user.id : hostSession.user.id
        await expect
          .poll(async () => (await fetchMatch(hostSession, match.id)).current_round, {
            timeout: 10_000,
          })
          .toBe(index + 2)
        await expect
          .poll(async () => (await fetchMatch(hostSession, match.id)).current_turn_user_id, {
            timeout: 10_000,
          })
          .toBe(nextStarter)
        await expect(host.locator(winningCells)).toHaveCount(0, { timeout: 10_000 })
        await expect(guest.locator(winningCells)).toHaveCount(0, { timeout: 10_000 })
      }
    }

    await expect(host.locator(winningCells)).toHaveCount(3, { timeout: 15_000 })
    await expect(guest.locator(winningCells)).toHaveCount(3, { timeout: 15_000 })

    // All three carry the winning animation class, not only the last square.
    await expect(host.locator(`${winningCells}.ttt-win-cell`)).toHaveCount(3)
    await expect(guest.locator(`${winningCells}.ttt-win-cell`)).toHaveCount(3)

    const terminal = await fetchMatch(guestSession, match.id)
    expect(terminal.status).toBe('completed')
    expect((terminal.scores as Record<string, number>)['0']).toBe(2)

    await host.waitForTimeout(2_500)
    await expect(host.locator(winningCells)).toHaveCount(3)
    await expect(guest.locator(winningCells)).toHaveCount(3)
  })
})
