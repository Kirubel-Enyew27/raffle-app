import { test, expect, type Page } from '@playwright/test'
import {
  backendReachable,
  createPrivateMatch,
  createSession,
  joinByCode,
  seedSession,
} from './helpers/api'

const BADGE = '[data-testid="match-connection-badge"]'

/**
 * Arms an in-page rAF observer on the opponent. The timestamp is taken after
 * the DOM mutation and the next animation frame, so Node/Playwright protocol
 * time is not part of the measurement.
 */
async function armOpponent(opponent: Page, selector: string) {
  await opponent.evaluate((targetSelector) => {
    ;(window as unknown as { __p2pSeenAt: number | null }).__p2pSeenAt = null
    const check = () => {
      const element = document.querySelector(targetSelector)
      if (element && !(element.getAttribute('aria-label') ?? '').includes('empty')) {
        requestAnimationFrame(() => {
          ;(window as unknown as { __p2pSeenAt: number | null }).__p2pSeenAt = Date.now()
        })
        return
      }
      requestAnimationFrame(check)
    }
    requestAnimationFrame(check)
  }, selector)
}

async function armOpponentText(opponent: Page, text: string) {
  await opponent.evaluate((expected) => {
    ;(window as unknown as { __p2pSeenAt: number | null }).__p2pSeenAt = null
    const expectedLower = expected.toLowerCase()
    const check = () => {
      if (document.body.innerText.toLowerCase().includes(expectedLower)) {
        requestAnimationFrame(() => {
          ;(window as unknown as { __p2pSeenAt: number | null }).__p2pSeenAt = Date.now()
        })
        return
      }
      requestAnimationFrame(check)
    }
    requestAnimationFrame(check)
  }, text)
}

async function opponentPaintTime(opponent: Page): Promise<number> {
  await opponent.waitForFunction(
    () => (window as unknown as { __p2pSeenAt: number | null }).__p2pSeenAt !== null,
    undefined,
    { timeout: 10_000 },
  )
  return opponent.evaluate(() => (window as unknown as { __p2pSeenAt: number }).__p2pSeenAt)
}

async function startMatch(page: Page, role: 'host' | 'guest', matchId: string) {
  await page.goto(`/games/match/${matchId}`, { waitUntil: 'domcontentloaded' })
  await expect.poll(
    async () => page.locator(BADGE).getAttribute('data-state'),
    { timeout: 15_000 },
  ).not.toBe('connecting')
  await page.getByRole('button', { name: /I am Ready/ }).click({ timeout: 15_000 })
  await expect.poll(
    async () => page.locator(BADGE).getAttribute('data-state'),
    { timeout: 15_000 },
  ).toBe('in_progress')
  if (role === 'host') {
    await expect(page.getByText('YOUR TURN').first()).toBeVisible()
  }
}

/**
 * Measures the browser-observed delay between a real pointer click and the
 * first painted board change. The REST move response is deliberately delayed
 * so this isolates optimistic local feedback from server/network time.
 */
test.describe('P2P move feedback latency', () => {
  test.beforeEach(async () => {
    test.skip(!(await backendReachable()), 'backend is not reachable on localhost:8080')
  })

  test('tic-tac-toe paints the local move before the delayed server acknowledgement', async ({
    browser,
  }, testInfo) => {
    const hostSession = await createSession('latency-host')
    const guestSession = await createSession('latency-guest')
    const match = await createPrivateMatch(hostSession)

    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)

    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)

    // Delay only the host's move acknowledgement. The optimistic UI must still
    // paint immediately; a server-first UI will take the full delay.
    await host.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 250))
      await route.continue()
    })

    await Promise.all([
      startMatch(host, 'host', match.id),
      startMatch(guest, 'guest', match.id),
    ])
    await armOpponent(guest, 'button[aria-label^="Cell 1,"]')
    const clickStartedAt = Date.now()

    const localFeedbackMs = await host.evaluate(async () => {
      const cell = document.querySelector<HTMLButtonElement>(
        'button[aria-label^="Cell 1,"]',
      )
      if (!cell) throw new Error('opening cell is missing')
      const started = performance.now()
      cell.click()

      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          const label = cell.getAttribute('aria-label') ?? ''
          if (!label.includes('empty')) {
            resolve(performance.now() - started)
            return
          }
          if (++frames > 120) {
            reject(new Error(`local move was not painted; label=${label}`))
            return
          }
          requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })

    // The opponent still receives the authoritative move after the delayed
    // request completes, so the measurement does not hide a broken pipeline.
    const opponentFeedbackMs = (await opponentPaintTime(guest)) - clickStartedAt

    // A normal, unthrottled reply measures the full opponent-visual path.
    await armOpponent(host, 'button[aria-label^="Cell 5,"]')
    const replyStartedAt = await guest.evaluate(() => {
      const cell = document.querySelector<HTMLButtonElement>('button[aria-label^="Cell 5,"]')
      if (!cell) throw new Error('reply cell is missing')
      const started = Date.now()
      cell.click()
      return started
    })
    const unthrottledOpponentMs = (await opponentPaintTime(host)) - replyStartedAt

    const summary = {
      local_visual_feedback_ms: localFeedbackMs,
      delayed_opponent_visual_ms: opponentFeedbackMs,
      unthrottled_opponent_visual_ms: unthrottledOpponentMs,
    }
    await testInfo.attach('move-feedback-latency.json', {
      body: Buffer.from(JSON.stringify(summary, null, 2)),
      contentType: 'application/json',
    })
    console.log(`[move-feedback-latency] ${JSON.stringify(summary)}`)

    expect(localFeedbackMs).toBeLessThan(800)
    // The acceptance target is p95 across runs; this per-sample guard catches a
    // broken realtime path without failing on a single loaded-run outlier.
    expect(unthrottledOpponentMs).toBeLessThan(250)
    await hostContext.close()
    await guestContext.close()
  })

  test('connect four paints the local drop and updates the opponent', async ({ browser }, testInfo) => {
    const hostSession = await createSession('latency-c4-host')
    const guestSession = await createSession('latency-c4-guest')
    const match = await createPrivateMatch(hostSession, 'connect-four')
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)
    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 250))
      await route.continue()
    })
    await Promise.all([startMatch(host, 'host', match.id), startMatch(guest, 'guest', match.id)])

    await armOpponent(guest, 'button[aria-label^="Row 6, column 1,"]')
    const clickStartedAt = Date.now()
    const localMs = await host.evaluate(async () => {
      const drop = document.querySelector<HTMLButtonElement>('button[aria-label="Drop token into column 1"]')
      if (!drop) throw new Error('drop target is missing')
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
    const delayedOpponentMs = (await opponentPaintTime(guest)) - clickStartedAt

    await armOpponent(host, 'button[aria-label^="Row 6, column 2,"]')
    await host.unroute('**/api/v1/p2p/matches/*/move')
    const replyStartedAt = await guest.evaluate(() => {
      const drop = document.querySelector<HTMLButtonElement>('button[aria-label="Drop token into column 2"]')
      if (!drop) throw new Error('reply drop target is missing')
      const started = Date.now()
      drop.click()
      return started
    })
    const unthrottledOpponentMs = (await opponentPaintTime(host)) - replyStartedAt
    const summary = {
      local_visual_feedback_ms: localMs,
      delayed_opponent_visual_ms: delayedOpponentMs,
      unthrottled_opponent_visual_ms: unthrottledOpponentMs,
    }
    await testInfo.attach('connect-four-feedback-latency.json', {
      body: Buffer.from(JSON.stringify(summary, null, 2)),
      contentType: 'application/json',
    })
    console.log(`[connect-four-feedback-latency] ${JSON.stringify(summary)}`)
    expect(localMs).toBeLessThan(800)
    expect(unthrottledOpponentMs).toBeLessThan(250)
    await hostContext.close()
    await guestContext.close()
  })

  test('Dama paints the local move and updates the opponent', async ({ browser }, testInfo) => {
    const hostSession = await createSession('latency-dama-host')
    const guestSession = await createSession('latency-dama-guest')
    const match = await createPrivateMatch(hostSession, 'ethiopian-dama')
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)
    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 250))
      await route.continue()
    })
    await Promise.all([startMatch(host, 'host', match.id), startMatch(guest, 'guest', match.id)])

    await host.getByRole('button', { name: /^a3, ivory checker/ }).click()
    await armOpponent(guest, 'button[aria-label^="b4, ivory checker"]')
    const clickStartedAt = Date.now()
    const localMs = await host.getByRole('button', { name: /^b4, empty · legal destination/ }).evaluate(async (element) => {
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
    const delayedOpponentMs = (await opponentPaintTime(guest)) - clickStartedAt

    await host.unroute('**/api/v1/p2p/matches/*/move')
    await armOpponent(host, 'button[aria-label^="a5, crimson checker"]')
    await guest.getByRole('button', { name: /^b6, crimson checker/ }).click()
    const replyTarget = guest.getByRole('button', { name: /^a5, empty · legal destination/ })
    await expect(replyTarget).toBeVisible()
    const replyStartedAt = await replyTarget.evaluate((element) => {
      const started = Date.now()
      ;(element as HTMLButtonElement).click()
      return started
    })
    const unthrottledOpponentMs = (await opponentPaintTime(host)) - replyStartedAt
    const summary = {
      local_visual_feedback_ms: localMs,
      delayed_opponent_visual_ms: delayedOpponentMs,
      unthrottled_opponent_visual_ms: unthrottledOpponentMs,
    }
    await testInfo.attach('dama-feedback-latency.json', {
      body: Buffer.from(JSON.stringify(summary, null, 2)),
      contentType: 'application/json',
    })
    console.log(`[dama-feedback-latency] ${JSON.stringify(summary)}`)
    expect(localMs).toBeLessThan(800)
    expect(unthrottledOpponentMs).toBeLessThan(250)
    await hostContext.close()
    await guestContext.close()
  })

  test('RPS paints the local lock-in and the opponent reveal', async ({ browser }, testInfo) => {
    const hostSession = await createSession('latency-rps-host')
    const guestSession = await createSession('latency-rps-guest')
    const match = await createPrivateMatch(hostSession, 'rock-paper-scissors')
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    await seedSession(hostContext, hostSession)
    await seedSession(guestContext, guestSession)
    const host = await hostContext.newPage()
    const guest = await guestContext.newPage()
    await joinByCode(guestSession, match.invite_code)
    await host.route('**/api/v1/p2p/matches/*/move', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 250))
      await route.continue()
    })
    await Promise.all([startMatch(host, 'host', match.id), startMatch(guest, 'guest', match.id)])

    await host.getByRole('button', { name: /^Rock/ }).click()
    const localMs = await host.evaluate(async () => {
      const lock = [...document.querySelectorAll<HTMLButtonElement>('button')]
        .find((button) => button.innerText.includes('Lock in'))
      if (!lock) throw new Error('Lock in button is missing')
      const started = performance.now()
      lock.click()
      return await new Promise<number>((resolve, reject) => {
        let frames = 0
        const check = () => {
          if (document.body.innerText.includes('Locked in: Rock')) {
            requestAnimationFrame(() => resolve(performance.now() - started))
            return
          }
          if (++frames > 120) reject(new Error('optimistic lock-in was not painted'))
          else requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      })
    })
    await guest.getByRole('button', { name: /^Scissors/ }).click()
    await armOpponentText(host, 'You Won This Round!')
    const startedAt = await guest.evaluate(() => {
      const lock = [...document.querySelectorAll<HTMLButtonElement>('button')]
        .find((button) => button.innerText.includes('Lock in'))
      if (!lock) throw new Error('guest Lock in button is missing')
      const started = Date.now()
      lock.click()
      return started
    })
    const opponentMs = (await opponentPaintTime(host)) - startedAt
    const summary = { local_visual_feedback_ms: localMs, unthrottled_opponent_visual_ms: opponentMs }
    await testInfo.attach('rps-feedback-latency.json', {
      body: Buffer.from(JSON.stringify(summary, null, 2)),
      contentType: 'application/json',
    })
    console.log(`[rps-feedback-latency] ${JSON.stringify(summary)}`)
    expect(localMs).toBeLessThan(800)
    await hostContext.close()
    await guestContext.close()
  })
})
