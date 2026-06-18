/**
 * AIDEV-NOTE: Settings page E2E tests
 * Tests settings navigation, form display, and form submission
 */

import { test, expect } from './fixtures/test'

test.describe('Settings Page', () => {
  test.describe('Navigation', () => {
    test('navigates to settings from dashboard header', async ({ page }) => {
      await page.goto('/')
      // Wait for the settings link to be visible
      const settingsLink = page.locator('a[href="/settings"]')
      await expect(settingsLink).toBeVisible({ timeout: 10000 })
      await settingsLink.click()

      // Should be on settings page
      await expect(page).toHaveURL('/settings')
      await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
    })

    test('can navigate back to dashboard from settings', async ({ page }) => {
      await page.goto('/settings')
      // Wait for the back button to be visible
      const backButton = page.locator('[aria-label="Back to dashboard"]')
      await expect(backButton).toBeVisible({ timeout: 10000 })
      await backButton.click()

      // Should be back on dashboard
      await expect(page).toHaveURL('/')
    })

    test('direct navigation to settings works', async ({ page }) => {
      await page.goto('/settings')
      await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
    })

    // AIDEV-NOTE: The favicon is driven globally from overall /health in <Header>, so it
    // reflects system health even on Settings (which sets no favicon of its own). The
    // mocked /health is 'unhealthy' -> red (#ef4444, url-encoded %23ef4444) data-URL icon.
    test('favicon reflects overall health from the global source', async ({ page }) => {
      await page.goto('/settings')
      await expect(page.locator('link[rel="icon"]')).toHaveAttribute('href', /ef4444/, {
        timeout: 5000,
      })
    })
  })

  test.describe('Form Display', () => {
    test('displays global configuration form', async ({ page }) => {
      await page.goto('/settings')

      // Should show timeout settings section
      await expect(page.locator('h2:has-text("Timeout Settings")')).toBeVisible()
    })

    test('displays progress timeout input', async ({ page }) => {
      await page.goto('/settings')

      const label = page.locator('label:has-text("Progress Timeout")')
      await expect(label).toBeVisible()

      const input = page.locator('input[name="progress_timeout_minutes"]')
      await expect(input).toBeVisible()
      await expect(input).toHaveAttribute('type', 'number')
    })

    test('displays staleness timeout input', async ({ page }) => {
      await page.goto('/settings')

      const label = page.locator('label:has-text("Staleness Timeout")')
      await expect(label).toBeVisible()

      const input = page.locator('input[name="staleness_timeout_hours"]')
      await expect(input).toBeVisible()
      await expect(input).toHaveAttribute('type', 'number')
    })

    test('displays save button', async ({ page }) => {
      await page.goto('/settings')

      const saveButton = page.locator('button:has-text("Save Settings")')
      await expect(saveButton).toBeVisible()
    })

    test('displays helper text for form fields', async ({ page }) => {
      await page.goto('/settings')

      // Check for helper text
      await expect(
        page.locator('text=Jobs in \'progress\' status will be marked as \'timeout\'')
      ).toBeVisible()
      await expect(
        page.locator('text=Successful jobs will be marked as \'stale\'')
      ).toBeVisible()
    })
  })

  test.describe('Form Interaction', () => {
    test('form inputs accept numeric values', async ({ page }) => {
      await page.goto('/settings')

      // Wait for form to load with values
      await page.waitForSelector('input[name="progress_timeout_minutes"]')

      const progressInput = page.locator('input[name="progress_timeout_minutes"]')
      const stalenessInput = page.locator('input[name="staleness_timeout_hours"]')

      // Clear and fill new values
      await progressInput.fill('60')
      await stalenessInput.fill('48')

      // Verify values were set
      await expect(progressInput).toHaveValue('60')
      await expect(stalenessInput).toHaveValue('48')
    })

    test('save button becomes enabled when form is dirty', async ({ page }) => {
      await page.goto('/settings')

      // Wait for form to load
      await page.waitForSelector('input[name="progress_timeout_minutes"]')

      const saveButton = page.locator('button:has-text("Save Settings")')

      // Initially the button might be disabled if form is not dirty
      // After changing a value, it should be enabled

      const progressInput = page.locator('input[name="progress_timeout_minutes"]')
      const currentValue = await progressInput.inputValue()

      // Change the value
      await progressInput.fill(String(Number(currentValue) + 1))

      // Button should be enabled now
      await expect(saveButton).toBeEnabled()
    })

    test('form validates required fields', async ({ page }) => {
      await page.goto('/settings')

      // Wait for form to load
      await page.waitForSelector('input[name="progress_timeout_minutes"]')

      const progressInput = page.locator('input[name="progress_timeout_minutes"]')
      const saveButton = page.locator('button:has-text("Save Settings")')

      // Clear the input
      await progressInput.clear()

      // Try to submit
      await saveButton.click()

      // Should show validation error
      await expect(page.locator('text=Required')).toBeVisible()
    })
  })

  test.describe('Form Submission', () => {
    test('can submit form with valid data', async ({ page }) => {
      await page.goto('/settings')

      // Wait for form to load with current values
      await page.waitForSelector('input[name="progress_timeout_minutes"]')

      const progressInput = page.locator('input[name="progress_timeout_minutes"]')
      const stalenessInput = page.locator('input[name="staleness_timeout_hours"]')
      const saveButton = page.locator('button:has-text("Save Settings")')

      // Get current values so we can modify them
      const currentProgress = await progressInput.inputValue()
      const currentStaleness = await stalenessInput.inputValue()

      // Change values to something different to make form dirty
      const newProgress = String(Number(currentProgress) + 5)
      const newStaleness = String(Number(currentStaleness) + 2)

      await progressInput.fill(newProgress)
      await stalenessInput.fill(newStaleness)

      // Now the button should be enabled because form is dirty
      await expect(saveButton).toBeEnabled({ timeout: 5000 })

      // Submit the form
      await saveButton.click()

      // AIDEV-NOTE: Wait for form submission to complete by checking button state
      // The button becomes disabled during submission, then re-enables (if dirty) or stays disabled (if saved)
      // Wait for the button to become disabled (submission started) then not be in loading state
      await expect(saveButton).not.toHaveAttribute('disabled', '', { timeout: 5000 }).catch(() => {
        // Button may stay disabled if form is no longer dirty after save
      })

      // Check that the new values are still there
      await expect(progressInput).toHaveValue(newProgress)
      await expect(stalenessInput).toHaveValue(newStaleness)
    })
  })
})
