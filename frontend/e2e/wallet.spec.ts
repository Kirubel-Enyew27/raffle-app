import { test, expect } from '@playwright/test'
import { uid } from './helpers/api'

test.describe('Wallet Transactions', () => {
  test('should handle deposit and withdrawal successfully', async ({ page }) => {
    const email = `wallet_${uid()}@example.com`
    const password = 'Password123!'

    // 1. Register and auto-login
    await page.goto('/register')
    await page.fill('#email', email)
    await page.fill('#password', password)
    await page.fill('#confirm', password)
    await page.click('button[type="submit"]')
    await page.waitForURL('/dashboard')

    // 2. Navigate to Wallet
    await page.goto('/wallet')
    await expect(page.locator('h1')).toHaveText('Wallet')

    // 3. Verify initial balance is $0.00
    const balanceLocator = page.locator('p.text-3xl')
    await expect(balanceLocator).toHaveText('$0.00')

    // 4. Perform Deposit of $100.00
    await page.click('button:has-text("Add funds")')
    await page.fill('#amount', '100.00')
    await page.fill('#reference', `dep-${uid()}`)
    await page.fill('#description', 'Test E2E deposit')
    await page.click('button[type="submit"]:has-text("Deposit funds")')

    // 5. Verify balance updates to $100.00
    await expect(balanceLocator).toHaveText('$100.00')

    // 6. Perform Withdrawal of $40.00
    await page.click('button:has-text("Withdraw funds")') // Click to open form
    await page.fill('#amount', '40.00')
    await page.fill('#reference', `wit-${uid()}`)
    await page.fill('#description', 'Test E2E withdrawal')
    await page.click('button[type="submit"]:has-text("Withdraw funds")') // Click to submit

    // 7. Verify balance updates to $60.00
    await expect(balanceLocator).toHaveText('$60.00')

    // 8. Verify transactions are listed in the history table
    await expect(page.locator('table tbody tr').first()).toBeVisible()
    const firstRowText = await page.locator('table tbody tr').first().innerText()
    expect(firstRowText).toContain('$40.00') // should show withdrawal of $40.00
  })
})
