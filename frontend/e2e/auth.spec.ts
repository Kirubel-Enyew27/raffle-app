import { test, expect } from '@playwright/test'
import { uid } from './helpers/api'

test.describe('Authentication Flow', () => {
  const email = `user_${uid()}@example.com`
  const password = 'Password123!'

  test('should register a new user and auto-login', async ({ page }) => {
    // 1. Navigate to Register page
    await page.goto('/register')
    await expect(page).toHaveTitle(/Create account/i)

    // 2. Fill registration form
    await page.fill('#email', email)
    await page.fill('#password', password)
    await page.fill('#confirm', password)

    // 3. Submit
    await page.click('button[type="submit"]')

    // 4. Verify auto-login and redirect to dashboard
    await page.waitForURL('/dashboard')
    await expect(page.locator('h1')).toHaveText('Dashboard')
    await expect(page.getByText(email)).toBeVisible()
  })

  test('should login successfully with existing credentials', async ({ page }) => {
    // 1. Navigate to Login page
    await page.goto('/login')
    await expect(page).toHaveTitle(/Sign in/i)

    // 2. Fill login form
    await page.fill('#email', email)
    await page.fill('#password', password)

    // 3. Submit
    await page.click('button[type="submit"]')

    // 4. Verify login and redirect to dashboard
    await page.waitForURL('/dashboard')
    await expect(page.locator('h1')).toHaveText('Dashboard')
    await expect(page.getByText(email)).toBeVisible()
  })

  test('should show validation error on incorrect credentials', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', 'nonexistent@example.com')
    await page.fill('#password', 'wrongpassword')
    await page.click('button[type="submit"]')

    // Expect a validation or login error message
    const errorMsg = page.locator('p.text-destructive')
    await expect(errorMsg).toBeVisible()
  })
})
