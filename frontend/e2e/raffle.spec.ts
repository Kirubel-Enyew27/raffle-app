import { test, expect } from '@playwright/test'
import { loginUser, createRaffle, commitDraw, executeDraw, uid } from './helpers/api'

test.describe('Raffle Lifecycle, Draw, and Winner Verification', () => {
  let adminToken: string
  let raffleId: string
  let raffleTitle: string
  const userEmail = `raffle_user_${uid()}@example.com`
  const password = 'Password123!'

  test.beforeAll(async () => {
    // 1. Log in as admin via API to get token
    const adminLogin = await loginUser('admin@raffle.com', 'admin123')
    adminToken = adminLogin.token

    // 2. Create a new active raffle programmatically
    raffleTitle = `Grand E2E Raffle ${uid()}`
    const raffle = await createRaffle(adminToken, {
      title: raffleTitle,
      ticket_price: 10,
      max_tickets: 100,
      prize_pool: 250,
    })
    raffleId = raffle.id

    // 3. Commit the draw seed for this raffle programmatically
    await commitDraw(adminToken, raffleId)
  })

  test('should purchase a ticket, execute draw, and verify the cryptographic proof', async ({ page }) => {
    // ── STEP 1: Register a new user & Top up their wallet ──
    await page.goto('/register')
    await page.fill('#fullName', 'Test User')
    await page.fill('#email', userEmail)
    await page.fill('#password', password)
    await page.fill('#confirm', password)
    await page.click('button[type="submit"]')
    await page.waitForURL('/dashboard')

    // Navigate to wallet to deposit funds
    await page.goto('/wallet')
    await page.click('button:has-text("Add funds")')
    await page.fill('#amount', '50.00')
    await page.fill('#reference', `dep-${uid()}`)
    await page.fill('#description', 'E2E ticket buying funds')
    await page.click('button[type="submit"]:has-text("Deposit funds")')
    await expect(page.locator('p.text-3xl')).toHaveText('50.00 Br')

    // ── STEP 2: Purchase a ticket ──
    // Navigate directly to the raffle detail page
    await page.goto(`/raffles/${raffleId}`)
    await expect(page.locator('h1')).toHaveText(raffleTitle)

    // Purchase 1 ticket
    const buyButton = page.locator('button:has-text("Buy 1 ticket")')
    await expect(buyButton).toBeEnabled()
    await buyButton.click()

    // Verify purchase success message and ticket number
    await expect(page.locator('p:has-text("Tickets purchased!")')).toBeVisible()
    await expect(page.locator('span:has-text("#1")')).toBeVisible() // First ticket in the raffle

    // ── STEP 3: Execute the Draw programmatically ──
    // Trigger draw execution on the backend (selects the winner)
    const drawResult = await executeDraw(adminToken, raffleId)
    expect(drawResult.winning_ticket_number).toBe(1) // Since only 1 ticket was sold

    // ── STEP 4: Login as Admin in UI ──
    // Logout the current user by going to login page (auth context gets cleared or we just clear localStorage/cookies)
    await page.goto('/login')
    
    // Login as admin
    await page.fill('#identifier', 'admin@raffle.com')
    await page.fill('#password', 'admin123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/admin/dashboard')

    // ── STEP 5: Verify the Winner & Cryptographic Proof in Admin Panel ──
    // Navigate to Admin Winners page
    await page.goto('/admin/winners')
    await expect(page.locator('h1')).toHaveText('Winners')

    // Find the winner row for our raffle
    const raffleRow = page.locator('tr', { hasText: raffleTitle })
    await expect(raffleRow).toBeVisible()
    await expect(raffleRow.locator('td', { hasText: userEmail })).toBeVisible()

    // Click the "Verification details" button (shield icon)
    const verifyButton = raffleRow.locator('button[title="Verification details"]')
    await verifyButton.click()

    // Verify the "Draw Verification" modal is open and displays cryptographic proofs
    const modal = page.locator('div:has-text("Draw Verification")')
    await expect(modal).toBeVisible()
    await expect(page.locator('span:has-text("Commit Hash")')).toBeVisible()
    await expect(page.locator('span:has-text("Server Seed Hash")')).toBeVisible()
    await expect(page.locator('span:has-text("Revealed Seed")')).toBeVisible()
    await expect(page.locator('span:has-text("Combined Hash")')).toBeVisible()
    await expect(page.locator('span:has-text("Winning Number")')).toBeVisible()

    // Close the modal
    await page.click('button:has(svg.lucide-x)')
    await expect(modal).not.toBeVisible()
  })
})
