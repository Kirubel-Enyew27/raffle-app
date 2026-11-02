import { test, expect } from '@playwright/test'
import { createSession, seedSession, uid } from './helpers/api'

test.describe('Authentication Flow', () => {
  const email = `user_${uid()}@example.com`
  const password = 'Password123!'

  test('should register a new user and auto-login', async ({ page }) => {
    // 1. Navigate to Register page
    await page.goto('/register', { waitUntil: 'domcontentloaded' })
    await expect(page).toHaveTitle(/Create account/i)

    // 2. Fill registration form
    await page.fill('#fullName', 'Test User')
    await page.fill('#email', email)
    await page.fill('#password', password)
    await page.fill('#confirm', password)

    // 3. Submit
    await page.click('button[type="submit"]')

    // 4. Verify auto-login and redirect to the games lobby (player landing page)
    await page.waitForURL('/games')
    await expect(page.locator('h1')).toHaveText('Play Live Against Other Players')
    await expect(page.getByRole('button', { name: 'Test User' })).toBeVisible()
  })

  test('should login successfully with existing credentials', async ({ page }) => {
    // Each Playwright test can run in a new worker after a failure, so create
    // this test's account independently instead of depending on the prior case.
    const session = await createSession('login_test_user')

    // 1. Navigate to Login page
    await page.goto('/login', { waitUntil: 'domcontentloaded' })
    await expect(page).toHaveTitle(/Sign in/i)

    // 2. Fill login form
    await page.fill('#identifier', session.email)
    await page.fill('#password', session.password)

    // 3. Submit
    await page.click('button[type="submit"]')

    // 4. Verify login and redirect to the games lobby
    await page.waitForURL('/games')
    await expect(page.locator('h1')).toHaveText('Play Live Against Other Players')
  })

  test('an authenticated player opening the root URL lands on the games lobby', async ({ page }) => {
    const session = await createSession('root_redirect')
    await seedSession(page.context(), session)

    await page.goto('/', { waitUntil: 'domcontentloaded' })

    await page.waitForURL('/games')
    await expect(page.locator('h1')).toHaveText('Play Live Against Other Players')
  })

  test('an authenticated player opening the login page is sent to the games lobby', async ({ page }) => {
    const session = await createSession('login_redirect')
    await seedSession(page.context(), session)

    await page.goto('/login', { waitUntil: 'domcontentloaded' })

    await page.waitForURL('/games')
  })

  test('should show validation error on incorrect credentials', async ({ page }) => {
    await page.goto('/login', { waitUntil: 'domcontentloaded' })
    await page.fill('#identifier', 'nonexistent@example.com')
    await page.fill('#password', 'wrongpassword')
    await page.click('button[type="submit"]')

    // Expect a validation or login error message
    const errorMsg = page.locator('p.text-destructive')
    await expect(errorMsg).toBeVisible()
  })
})
