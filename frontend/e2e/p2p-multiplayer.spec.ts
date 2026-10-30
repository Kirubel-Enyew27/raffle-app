import { test, expect, type Page } from '@playwright/test'
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

test.describe('P2P multiplayer', () => {
  test.beforeEach(async () => {
    test.skip(!(await backendReachable()), 'backend is not reachable on localhost:8080')
  })

  test('two sessions join, both leave Connecting, and moves stay in sync', async ({ browser }) => {
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
    await host.goto(`/games/match/${match.id}`)
    await guest.goto(`/games/match/${match.id}`)

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

    // The host opens; once they move, the guest's board and turn update.
    await expect(host.getByText('YOUR TURN')).toBeVisible()
    await clickCell(host, 0)

    await expect
      .poll(async () => (await bodyText(guest)).includes('YOUR TURN'), { timeout: 10_000 })
      .toBe(true)
    await expect(guest.getByRole('button', { name: /^Cell 1, / })).toBeVisible()

    // The guest replies; the host sees it and it becomes the host's turn again.
    await clickCell(guest, 4)
    await expect
      .poll(async () => (await bodyText(host)).includes('YOUR TURN'), { timeout: 10_000 })
      .toBe(true)

    // Both clients agree with the server about the turn.
    const state = await fetchMatch(hostSession, match.id)
    expect(state.current_turn_user_id).toBe(hostSession.user.id)
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
    await host.goto(`/games/match/${match.id}`)
    await guest.goto(`/games/match/${match.id}`)

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
    await host.goto(`/games/match/${match.id}`)
    await guest.goto(`/games/match/${match.id}`)

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
    await host.goto(`/games/match/${match.id}`)
    await guest.goto(`/games/match/${match.id}`)

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

        await expect
          .poll(async () => (await fetchMatch(hostSession, match.id)).current_turn_user_id, {
            timeout: 10_000,
          })
          .toBeTruthy()
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
