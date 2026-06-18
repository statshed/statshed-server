/**
 * AIDEV-NOTE: Group Detail page E2E tests
 * Tests navigation to groups, job listing, filtering, and configuration
 */

import { test, expect } from './fixtures/test'

test.describe('Group Detail Page', () => {
  // AIDEV-NOTE: Data comes from the hermetic mock (e2e/fixtures) — groups, jobs,
  // and per-group config are always present and deterministic.

  test.describe('Navigation', () => {
    test('navigates to group detail from dashboard', async ({ page }) => {
      await page.goto('/')

      // Mocked API always returns groups, so a group card must render.
      const groupCard = page.locator('[data-testid="group-card"]').first()
      await expect(groupCard).toBeVisible({ timeout: 10000 })

      const groupLink = page.locator('a[href^="/groups/"]').first()
      const href = await groupLink.getAttribute('href')
      expect(href).toBeTruthy()

      // Click the group card and confirm navigation to its detail page.
      await groupLink.click()
      await expect(page).toHaveURL(/\/groups\//)
      await expect(page.locator('[aria-label="Back to dashboard"]')).toBeVisible()
    })

    test('displays back to dashboard link', async ({ page }) => {
      // Navigate directly to a group page
      await page.goto('/groups/test-group')

      // Should have back button
      const backButton = page.locator('[aria-label="Back to dashboard"]')
      await expect(backButton).toBeVisible()

      // Clicking should navigate back to dashboard
      await backButton.click()
      await expect(page).toHaveURL('/')
    })

    test('handles URL encoded group names', async ({ page }) => {
      // Test with a URL encoded group name
      await page.goto('/groups/test%20group%20with%20spaces')

      // Should display the decoded group name (use more specific selector)
      await expect(page.getByRole('heading', { level: 1 }).filter({ hasText: 'test group with spaces' })).toBeVisible()
    })
  })

  test.describe('Page Elements', () => {
    test('displays group name in header', async ({ page }) => {
      await page.goto('/groups/backups')
      // Use specific heading selector to avoid matching header logo
      await expect(page.getByRole('heading', { level: 1 }).filter({ hasText: 'backups' })).toBeVisible()
    })

    test('shows Submit Status button', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Use first() to get the main page button, not the dialog button
      const submitButton = page.getByRole('button', { name: 'Submit Status' }).first()
      await expect(submitButton).toBeVisible()
    })

    test('shows Configure button', async ({ page }) => {
      await page.goto('/groups/test-group')
      const configButton = page.getByRole('button', { name: 'Configure' })
      await expect(configButton).toBeVisible()
    })

    test('shows search input for jobs', async ({ page }) => {
      await page.goto('/groups/test-group')
      await expect(page.locator('input[placeholder="Search jobs..."]')).toBeVisible()
    })

    test('shows status filter buttons', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Filter buttons are in main content, not in dialogs
      const filterSection = page.locator('main')
      await expect(filterSection.locator('button:has-text("All")')).toBeVisible()
      await expect(filterSection.locator('button:has-text("Success")')).toBeVisible()
      await expect(filterSection.locator('button:has-text("Error")')).toBeVisible()
      await expect(filterSection.locator('button:has-text("Progress")')).toBeVisible()
    })
  })

  test.describe('Job Submit Dialog', () => {
    test('opens submit status dialog with pre-filled group', async ({ page }) => {
      await page.goto('/groups/backups')
      // Wait for the page content to render by checking for specific element
      await expect(page.getByRole('button', { name: 'Submit Status' }).first()).toBeVisible({ timeout: 10000 })

      // Click submit status button (first one, the page button not the form button)
      await page.getByRole('button', { name: 'Submit Status' }).first().click()

      // Dialog should be visible
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()
      await expect(dialog.locator('h2')).toContainText('Submit Job Status')

      // Group field should be pre-filled or disabled
      const groupInput = dialog.locator('input[name="group"]')
      await expect(groupInput).toHaveValue('backups')
    })

    test('can close submit dialog', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Wait for the page content to render
      await expect(page.getByRole('button', { name: 'Submit Status' }).first()).toBeVisible({ timeout: 10000 })

      // Open dialog
      await page.getByRole('button', { name: 'Submit Status' }).first().click()
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Close using close button inside the dialog
      await dialog.locator('[aria-label="Close dialog"]').click()
      await expect(dialog).not.toBeVisible()
    })
  })

  test.describe('Group Configuration', () => {
    test('opens configuration dialog', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Wait for the page content to render
      await expect(page.getByRole('button', { name: 'Configure' })).toBeVisible({ timeout: 10000 })

      // Click configure button
      await page.getByRole('button', { name: 'Configure' }).click()

      // Configuration dialog should be visible
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()
      // Dialog title is "Configure {group-name}"
      await expect(dialog.locator('h2')).toContainText('Configure')
    })

    test('configuration dialog has timeout fields', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Wait for the page content to render
      await expect(page.getByRole('button', { name: 'Configure' })).toBeVisible({ timeout: 10000 })

      // Open config dialog
      await page.getByRole('button', { name: 'Configure' }).click()

      // Should have timeout input fields
      const dialog = page.locator('dialog[open]')
      await expect(dialog.locator('label:has-text("Progress Timeout")')).toBeVisible()
      // AIDEV-NOTE: Expiration is always visible, staleness timeout is conditional
      await expect(dialog.locator('label:has-text("Expiration Timeout")')).toBeVisible()
    })

    // AIDEV-NOTE: Phase 7 test - staleness input visibility is conditional on checkbox
    test('staleness timeout input shows/hides based on checkbox', async ({ page }) => {
      await page.goto('/groups/backups')
      await expect(page.getByRole('button', { name: 'Configure' })).toBeVisible({ timeout: 10000 })

      // Open config dialog
      await page.getByRole('button', { name: 'Configure' }).click()
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // AIDEV-NOTE: Use label-based selector for checkbox to be resilient to layout changes
      const checkbox = dialog.getByLabel('Enable staleness warnings')
      await expect(checkbox).toBeEnabled({ timeout: 15000 })

      // Initially staleness timeout should NOT be visible (staleness disabled by default)
      // Note: If the group already has staleness enabled, the input will be visible
      const stalenessLabel = dialog.locator('label:has-text("Staleness Timeout")')
      const isChecked = await checkbox.isChecked()

      if (isChecked) {
        // If staleness is already enabled, verify input is visible
        await expect(stalenessLabel).toBeVisible()
        // Uncheck to test hide behavior
        await checkbox.uncheck()
        await expect(stalenessLabel).not.toBeVisible()
        // Re-check to show again
        await checkbox.check()
        await expect(stalenessLabel).toBeVisible()
      } else {
        // Staleness disabled - input should be hidden
        await expect(stalenessLabel).not.toBeVisible()
        // Enable staleness warnings checkbox
        await checkbox.check()
        // Now staleness timeout input should be visible
        await expect(stalenessLabel).toBeVisible()
        // Uncheck to hide again
        await checkbox.uncheck()
        await expect(stalenessLabel).not.toBeVisible()
      }
    })

    // AIDEV-NOTE: Phase 7 test - form validation for staleness < expiration
    test('validates staleness must be less than expiration', async ({ page }) => {
      await page.goto('/groups/backups')
      await expect(page.getByRole('button', { name: 'Configure' })).toBeVisible({ timeout: 10000 })

      // Open config dialog
      await page.getByRole('button', { name: 'Configure' }).click()
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // AIDEV-NOTE: Use label-based selectors to be resilient to layout changes
      const expirationInput = dialog.getByLabel(/Expiration Timeout/i)
      await expect(expirationInput).toBeEnabled({ timeout: 15000 })

      // Set expiration to 24 hours
      await expirationInput.fill('24')

      // Enable staleness using label-based selector
      const checkbox = dialog.getByLabel('Enable staleness warnings')
      await checkbox.check()

      // Set staleness to 30 hours (greater than expiration)
      const stalenessInput = dialog.getByLabel(/Staleness Timeout/i)
      await stalenessInput.fill('30')

      // Try to submit
      await dialog.getByRole('button', { name: 'Save Changes' }).click()

      // Should show validation error
      const crossFieldError = dialog.getByText(
        'Staleness timeout must be less than expiration timeout'
      )
      await expect(crossFieldError).toBeVisible()

      // AIDEV-NOTE: Resolving the conflict by RAISING expiration must clear the
      // staleness error immediately (cross-field revalidation), not wait for resubmit.
      await expirationInput.fill('40')
      await expect(crossFieldError).toHaveCount(0)
    })

    test('can close configuration dialog', async ({ page }) => {
      await page.goto('/groups/test-group')
      // Wait for the page content to render
      await expect(page.getByRole('button', { name: 'Configure' })).toBeVisible({ timeout: 10000 })

      // Open dialog
      await page.getByRole('button', { name: 'Configure' }).click()
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Close using close button inside the dialog
      await dialog.locator('[aria-label="Close dialog"]').click()
      await expect(dialog).not.toBeVisible()
    })
  })

  test.describe('Job Filtering', () => {
    test('status filter buttons change active state', async ({ page }) => {
      await page.goto('/groups/test-group')

      // Filter buttons are in the main content area
      const filterSection = page.locator('main')

      // All button should be active by default
      const allButton = filterSection.locator('button:has-text("All")')
      await expect(allButton).toHaveClass(/bg-primary/)

      // Click on Success filter
      const successButton = filterSection.locator('button:has-text("Success")')
      await successButton.click()

      // Success should now be active
      await expect(successButton).toHaveClass(/bg-primary/)

      // All should no longer be active
      await expect(allButton).not.toHaveClass(/bg-primary/)
    })

    test('search input accepts text', async ({ page }) => {
      await page.goto('/groups/test-group')

      const searchInput = page.locator('input[placeholder="Search jobs..."]')
      await searchInput.fill('backup')
      await expect(searchInput).toHaveValue('backup')
    })
  })
})
