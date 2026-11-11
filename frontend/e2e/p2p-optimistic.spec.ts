import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test'
import {
  backendReachable,
  createPrivateMatch,
  createSession,
  fetchMatch,
  joinByCode,
  seedSession,
  submitMove,
  type TestSession,
} from './helpers/api'

const BADGE = '[data-testid="match-connection-badge"]'

interface GameSetup {
  match: { id: string; invite_code: string }
  hostSession: TestSession
  guestSession: TestSession
  hostContext: BrowserContext
  guestContext: BrowserContext
  host: Page
  guest: Page
}

async function waitForBadge(page: Page, state: string) {
  await expect.poll(
    async () => page.locator(BADGE).getAttribute('data-state'),
    { timeout: 15_000 },
  ).toBe(state)
}

async function setupGame(browser: Browser, slug: string): Promise<GameSetup> {
  const hostSession = await createSession(`${slug}-host`)
  const guestSession = await createSession(`${slug}-guest`)
  const match = await createPrivateMatch(hostSession, slug)

  const hostContext = await browser.newContext()
  const guestContext = await browser.newContext()
  await seedSession(hostContext, hostSession)
  await seedSession(guestContext, guestSession)
  const host = await hostContext.newPage()
  const guest = await guestContext.newPage()

  await joinByCode(guestSession, match.invite_code)
  await Promise.all([
    host.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' }),
    guest.goto(`/games/match/${match.id}`, { waitUntil: 'domcontentloaded' }),
  ])
  await expect(host.locator(BADGE)).toBeVisible()
  await expect(guest.locator(BADGE)).toBeVisible()
  await host.getByRole('button', { name: /I am Ready/ }).click()
  await guest.getByRole('button', { name: /I am Ready/ }).click()
  await waitForBadge(host, 'in_progress')
  await waitForBadge(guest, 'in_progress')
  return { match, hostSession, guestSession, hostContext, guestContext, host, guest }
}

async function closeGame(game: GameSetup) {
  await Promise.all([game.hostContext.close(), game.guestContext.close()])
}

async function delayMoveRequest(context: BrowserContext, delayMs = 300) {
  await context.route('**/api/v1/p2p/matches/*/move', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, delayMs))
    await route.continue()
  })
}

function boardState(state: Awaited<ReturnType<typeof fetchMatch>>) {
  return state.board_state as Record<string, unknown>
}

test.describe('optimistic gameplay across all games', () => {
  test.beforeEach(async () => {
    test.skip(!(await backendReachable()), 'backend is not reachable on localhost:8080')
  })

  test('tic-tac-toe paints the marker before the delayed REST acknowledgement', async ({
    browser,
  }, testInfo) => {
    const game = await setupGame(browser, 'tic-tac-toe')
    const { host, guest, hostContext } = game
    await delayMoveRequest(hostContext, 300)
    const ackStarted = Date.now()
    const ack = host.waitForResponse(
      (response) => response.request().method() === 'POST' && /\/p2p\/matches\/[^/]+\/move/.test(response.url()),
    )

    const localMs = await host.evaluate(async () => {
      const cell = document.querySelector<HTMLButtonElement>('button[aria-label^="Cell 1,"]')
      if (!cell) throw new Error('Cell 1 is missing')
      const started = performance.now()
      cell.click()
      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          if (!(cell.getAttribute('aria-label') ?? '').includes('empty')) {
            requestAnimationFrame(() => resolve(performance.now() - started))
            return
          }
          if (++frames > 120) reject(new Error('optimistic marker was not painted'))
          else requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })

    const ackMs = Date.now() - ackStarted
    expect((await ack).ok()).toBe(true)
    expect(localMs).toBeLessThan(800)
    expect(localMs).toBeLessThan(ackMs)
    await expect
      .poll(() => guest.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')

    const state = await fetchMatch(game.hostSession, game.match.id)
    const cells = boardState(state).cells as number[]
    expect(cells.filter((cell) => cell !== 0)).toHaveLength(1)
    expect(cells[0]).toBe(1)
    expect(await host.getByRole('button', { name: /^Cell 1,/ }).count()).toBe(1)

    await testInfo.attach('ttt-optimistic.json', {
      body: Buffer.from(JSON.stringify({ local_visual_feedback_ms: localMs, rest_ack_ms: ackMs }, null, 2)),
      contentType: 'application/json',
    })
    await closeGame(game)
  })

  test('connect four drops one token locally, then reconciles with the opponent', async ({
    browser,
  }, testInfo) => {
    const game = await setupGame(browser, 'connect-four')
    const { host, guest, hostContext } = game
    await delayMoveRequest(hostContext, 300)

    const localMs = await host.evaluate(async () => {
      const drop = document.querySelector<HTMLButtonElement>('button[aria-label="Drop token into column 1"]')
      if (!drop) throw new Error('column 1 drop target is missing')
      const started = performance.now()
      drop.click()
      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          const slot = document.querySelector<HTMLButtonElement>('button[aria-label^="Row 6, column 1,"]')
          if (slot && !(slot.getAttribute('aria-label') ?? '').includes('empty')) {
            requestAnimationFrame(() => resolve(performance.now() - started))
            return
          }
          if (++frames > 120) reject(new Error('optimistic token was not painted'))
          else requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })

    expect(localMs).toBeLessThan(800)
    await expect
      .poll(() => guest.getByRole('button', { name: /^Row 6, column 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    const state = await fetchMatch(game.hostSession, game.match.id)
    const cells = boardState(state).cells as number[]
    expect(cells.filter((cell) => cell !== 0)).toHaveLength(1)
    expect((boardState(state).move_count as number)).toBe(1)

    // A second, unthrottled move from the other seat must not replay the first.
    await guest.getByRole('button', { name: 'Drop token into column 2' }).click()
    await expect
      .poll(() => host.getByRole('button', { name: /^Row 6, column 2,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    const after = await fetchMatch(game.hostSession, game.match.id)
    expect(boardState(after).move_count).toBe(2)
    expect(await host.locator('button[aria-label*="red token"]').count()).toBe(1)
    expect(await host.locator('button[aria-label*="amber token"]').count()).toBe(1)

    await testInfo.attach('connect-four-optimistic.json', {
      body: Buffer.from(JSON.stringify({ local_visual_feedback_ms: localMs }, null, 2)),
      contentType: 'application/json',
    })
    await closeGame(game)
  })

  test('RPS locks the local choice immediately and reveals exactly once for both players', async ({
    browser,
  }) => {
    const game = await setupGame(browser, 'rock-paper-scissors')
    const { host, guest, hostContext, hostSession, guestSession } = game
    await delayMoveRequest(hostContext, 300)

    await host.getByRole('button', { name: /^Rock/ }).click()
    const localMs = await host.evaluate(async () => {
      const button = [...document.querySelectorAll<HTMLButtonElement>('button')]
        .find((candidate) => candidate.innerText.includes('Lock in'))
      if (!button) throw new Error('Lock in button is missing')
      const started = performance.now()
      button.click()
      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          if (document.body.innerText.includes('Locked in: Rock')) {
            requestAnimationFrame(() => resolve(performance.now() - started))
            return
          }
          if (++frames > 120) reject(new Error('optimistic choice was not painted'))
          else requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })

    expect(localMs).toBeLessThan(800)
    await expect(host.getByText('Locked in: Rock')).toBeVisible()
    await expect(guest.getByText('Locked in: Rock')).toHaveCount(0)
    await expect
      .poll(async () => ((await fetchMatch(guestSession, game.match.id)).board_state as Record<string, unknown>).choices)
      .toMatchObject({ '0': 'hidden' })

    await guest.getByRole('button', { name: /^Scissors/ }).click()
    await guest.getByRole('button', { name: /Lock in Scissors/ }).click()
    await expect(host.getByText('You Won This Round!')).toBeVisible({ timeout: 10_000 })
    await expect(guest.getByText('Opponent Won This Round')).toBeVisible({ timeout: 10_000 })
    const state = await fetchMatch(hostSession, game.match.id)
    expect((state.scores as Record<string, number>)['0']).toBe(1)
    expect(boardState(state).last_round_result).toMatchObject({ p0_choice: 'rock', p1_choice: 'scissors' })
    await expect(host.getByText('You Won This Round!')).toHaveCount(1)
    await expect(guest.getByText('Opponent Won This Round')).toHaveCount(1)
    await closeGame(game)
  })

  test('Dama paints a legal move and the opponent receives the same board', async ({
    browser,
  }, testInfo) => {
    const game = await setupGame(browser, 'ethiopian-dama')
    const { host, guest, hostContext } = game
    await delayMoveRequest(hostContext, 300)

    await host.getByRole('button', { name: /^a3, ivory checker/ }).click()
    const target = host.getByRole('button', { name: /^b4, empty · legal destination/ })
    await expect(target).toBeVisible()

    const localMs = await target.evaluate(async (element) => {
      const button = element as HTMLButtonElement
      const started = performance.now()
      button.click()
      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          if ((button.getAttribute('aria-label') ?? '').startsWith('b4, ivory checker')) {
            requestAnimationFrame(() => resolve(performance.now() - started))
            return
          }
          if (++frames > 120) reject(new Error('optimistic Dama move was not painted'))
          else requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })

    expect(localMs).toBeLessThan(800)
    await expect
      .poll(() => guest.getByRole('button', { name: /^b4, ivory checker/ }).count())
      .toBe(1)
    const state = await fetchMatch(game.hostSession, game.match.id)
    expect(boardState(state).move_count).toBe(1)
    expect((boardState(state).move_history as unknown[]).length).toBe(1)
    const cells = boardState(state).cells as number[]
    expect(cells[40]).toBe(0)
    expect(cells[33]).toBe(1)

    await testInfo.attach('dama-optimistic.json', {
      body: Buffer.from(JSON.stringify({ local_visual_feedback_ms: localMs }, null, 2)),
      contentType: 'application/json',
    })
    await closeGame(game)
  })
})

test.describe('optimistic synchronization edge cases', () => {
  test.beforeEach(async () => {
    test.skip(!(await backendReachable()), 'backend is not reachable on localhost:8080')
  })

  test('server rejection rolls the provisional piece back', async ({ browser }) => {
    const game = await setupGame(browser, 'tic-tac-toe')
    const { host, hostContext } = game
    await hostContext.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 200))
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'invalid_move', message: 'rejected for test' } }),
      })
    })

    const cell = host.getByRole('button', { name: /^Cell 1,/ })
    await cell.click()
    await expect.poll(() => cell.getAttribute('aria-label')).not.toContain('empty')
    await expect.poll(() => cell.getAttribute('aria-label')).toContain('empty')

    const state = await fetchMatch(game.hostSession, game.match.id)
    expect((boardState(state).cells as number[])[0]).toBe(0)
    expect(boardState(state).move_count).toBe(0)
    await closeGame(game)
  })

  test('rapid same-frame clicks submit only one move', async ({ browser }) => {
    const game = await setupGame(browser, 'tic-tac-toe')
    const { host, guest, hostContext } = game
    await delayMoveRequest(hostContext, 250)

    await host.evaluate(() => {
      const first = document.querySelector<HTMLButtonElement>('button[aria-label^="Cell 1,"]')
      const second = document.querySelector<HTMLButtonElement>('button[aria-label^="Cell 2,"]')
      if (!first || !second) throw new Error('cells are missing')
      first.click()
      second.click()
    })

    await expect
      .poll(() => guest.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    await host.waitForTimeout(500)
    const state = await fetchMatch(game.hostSession, game.match.id)
    const cells = boardState(state).cells as number[]
    expect(cells.filter((cell) => cell !== 0)).toHaveLength(1)
    expect(cells[0]).toBe(1)
    expect(cells[1]).toBe(0)
    expect(await host.locator('button[aria-label*="X"]').count()).toBe(1)
    await closeGame(game)
  })

  test('SSE delivery before the REST acknowledgement does not duplicate a move', async ({
    browser,
  }, testInfo) => {
    const game = await setupGame(browser, 'tic-tac-toe')
    const { host, guest, hostContext, match } = game
    let sseArrivedBeforeRest = false
    await hostContext.route('**/api/v1/p2p/matches/*/move', async (route) => {
      const response = await route.fetch()
      sseArrivedBeforeRest = await host.evaluate(async (matchID) => {
        const prefix = `p2p-move:${matchID}:`
        for (let attempt = 0; attempt < 40; attempt++) {
          if (performance.getEntriesByType('mark').some((entry) => entry.name.startsWith(prefix))) return true
          await new Promise((resolve) => setTimeout(resolve, 25))
        }
        return false
      }, match.id)
      await new Promise((resolve) => setTimeout(resolve, 250))
      await route.fulfill({ response })
    })

    const acknowledgement = host.waitForResponse(
      (response) => response.request().method() === 'POST' && /\/p2p\/matches\/[^/]+\/move/.test(response.url()),
    )
    await host.getByRole('button', { name: /^Cell 1,/ }).click()
    await acknowledgement
    await expect
      .poll(() => guest.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    const state = await fetchMatch(game.hostSession, game.match.id)
    expect((boardState(state).cells as number[])[0]).toBe(1)
    expect(boardState(state).move_count).toBe(1)
    expect(await host.getByRole('button', { name: /^Cell 1,/ }).count()).toBe(1)
    expect(sseArrivedBeforeRest, 'SSE should reach the actor while the REST response is held').toBe(true)
    await testInfo.attach('sse-first.json', {
      body: Buffer.from(JSON.stringify({ sse_arrived_before_rest: sseArrivedBeforeRest }, null, 2)),
      contentType: 'application/json',
    })
    await closeGame(game)
  })

  test('two players locking in back-to-back produce one reveal and one score', async ({
    browser,
  }) => {
    const game = await setupGame(browser, 'rock-paper-scissors')
    const { host, guest } = game
    await host.getByRole('button', { name: /^Rock/ }).click()
    const hostAck = host.waitForResponse(
      (response) => response.request().method() === 'POST' && /\/move$/.test(response.url()),
    )
    await host.getByRole('button', { name: /Lock in Rock/ }).click()
    await hostAck
    await guest.getByRole('button', { name: /^Scissors/ }).click()
    await guest.getByRole('button', { name: /Lock in Scissors/ }).click()
    await expect(host.getByText('You Won This Round!')).toBeVisible({ timeout: 10_000 })
    await expect(guest.getByText('Opponent Won This Round')).toBeVisible({ timeout: 10_000 })
    const state = await fetchMatch(game.hostSession, game.match.id)
    expect((state.scores as Record<string, number>)['0']).toBe(1)
    expect((state.scores as Record<string, number>)['1']).toBe(0)
    await expect(host.getByText('You Won This Round!')).toHaveCount(1)
    await closeGame(game)
  })

  test('a winning Connect Four drop stays terminal while its REST response is pending', async ({
    browser,
  }) => {
    const game = await setupGame(browser, 'connect-four')
    const { host, hostContext, hostSession, guestSession } = game
    for (const [session, column] of [
      [hostSession, 0],
      [guestSession, 1],
      [hostSession, 0],
      [guestSession, 1],
      [hostSession, 0],
      [guestSession, 1],
    ] as Array<[TestSession, number]>) {
      expect(await submitMove(session, game.match.id, { column })).toBe(200)
    }
    expect((await fetchMatch(hostSession, game.match.id)).current_turn_user_id).toBe(hostSession.user.id)
    await expect(host.getByRole('button', { name: 'Drop token into column 1' })).toBeEnabled({ timeout: 10_000 })

    await hostContext.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 350))
      await route.continue()
    })
    await host.getByRole('button', { name: 'Drop token into column 1' }).click()
    await expect
      .poll(async () => (await fetchMatch(hostSession, game.match.id)).status, { timeout: 10_000 })
      .toBe('completed')

    const terminal = await fetchMatch(hostSession, game.match.id)
    expect((boardState(terminal).winning_line as number[]).length).toBe(4)
    expect((boardState(terminal).cells as number[]).filter((cell) => cell === 1)).toHaveLength(4)
    await expect.poll(async () => host.locator('[aria-label*="winning 4-in-a-row"]').count()).toBe(4)
    await expect.poll(async () => host.locator('button[aria-label*="red token"]').count()).toBe(4)
    await closeGame(game)
  })

  test('disconnecting during an in-flight move recovers one authoritative state', async ({
    browser,
  }) => {
    const game = await setupGame(browser, 'tic-tac-toe')
    const { host, guest, hostContext } = game
    await hostContext.route('**/api/v1/p2p/matches/*/move', async (route) => {
      const response = await route.fetch()
      await new Promise((resolve) => setTimeout(resolve, 900))
      await route.fulfill({ response }).catch(() => {})
    })

    await host.getByRole('button', { name: /^Cell 1,/ }).click()
    await host.waitForTimeout(120)
    await hostContext.setOffline(true)
    await host.waitForTimeout(180)
    await hostContext.setOffline(false)
    await expect
      .poll(async () => (await fetchMatch(game.hostSession, game.match.id)).board_state, { timeout: 10_000 })
      .toMatchObject({ move_count: 1 })

    await host.reload({ waitUntil: 'domcontentloaded' })
    await waitForBadge(host, 'in_progress')
    await expect
      .poll(() => host.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    await expect
      .poll(() => guest.getByRole('button', { name: /^Cell 1,/ }).getAttribute('aria-label'))
      .not.toContain('empty')
    const state = await fetchMatch(game.hostSession, game.match.id)
    const cells = boardState(state).cells as number[]
    expect(cells.filter((cell) => cell !== 0)).toHaveLength(1)
    expect(await host.locator('button[aria-label*="X"]').count()).toBe(1)
    await closeGame(game)
  })
})
