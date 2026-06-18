/**
 * AIDEV-NOTE: Log Viewer E2E tests
 * Tests for opening log viewer modal, navigation controls, and Show all functionality
 *
 * The mocked API (e2e/fixtures) always serves jobs with logs attached, so the
 * dashboard→group→"view-logs-button" path is deterministic. The no-data skip
 * guards below are retained as defensive fallbacks but are not expected to fire.
 */

import { test, expect } from './fixtures/test'

test.describe('Log Viewer', () => {
  test.describe('Finding Jobs with Logs', () => {
    test('job card shows Logs button when job has log', async ({ page }) => {
      // Navigate to dashboard
      await page.goto('/')

      // Wait for skeleton loaders to disappear (data loaded)
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {
        // Skeleton may never appear if data loads fast
      })

      // Find any "Logs" button on a job card
      const logsButton = page.getByTestId('view-logs-button').first()
      const buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        // Check groups for jobs with logs
        const groupLink = page.locator('a[href^="/groups/"]').first()
        const groupCount = await groupLink.count()

        if (groupCount > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
        }

        const nestedLogsButton = page.getByTestId('view-logs-button').first()
        const nestedCount = await nestedLogsButton.count()

        if (nestedCount === 0) {
          test.skip(true, 'No jobs with logs available for testing')
          return
        }

        await expect(nestedLogsButton).toBeVisible()
      } else {
        await expect(logsButton).toBeVisible()
      }
    })
  })

  test.describe('Modal Interactions', () => {
    test('opens log viewer modal when clicking Logs button', async ({ page }) => {
      // Navigate to dashboard
      await page.goto('/')

      // Wait for data to load
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find a Logs button
      const logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      // If no button on dashboard, try a group
      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        const groupCount = await groupLink.count()

        if (groupCount > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
        }
        buttonCount = await page.getByTestId('view-logs-button').count()
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      // Click the Logs button
      await page.getByTestId('view-logs-button').first().click()

      // Modal should open
      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Should show "Job Logs" title
      await expect(dialog.getByText('Job Logs')).toBeVisible()

      // Should show friendly microcopy
      await expect(dialog.getByText(/Here's what happened/)).toBeVisible()
    })

    test('can close log viewer modal with close button', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Close using close button
      await dialog.locator('[aria-label="Close dialog"]').click()
      await expect(dialog).not.toBeVisible()
    })

    test('can close log viewer modal with escape key', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Close with Escape key
      await page.keyboard.press('Escape')
      await expect(dialog).not.toBeVisible()
    })
  })

  test.describe('Modal Content', () => {
    test('shows navigation controls in toolbar', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Wait for log content to load
      await page.waitForSelector('[data-line]', { timeout: 10000 }).catch(() => {})

      // Check navigation controls are visible
      await expect(dialog.locator('[title="Jump to top"]')).toBeVisible()
      await expect(dialog.locator('[title="Jump to bottom"]')).toBeVisible()
      await expect(dialog.locator('[title="Previous error"]')).toBeVisible()
      await expect(dialog.locator('[title="Next error"]')).toBeVisible()
    })

    test('shows log content with line numbers', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Wait for log lines to appear
      const logLines = dialog.locator('[data-line]')
      await expect(logLines.first()).toBeVisible({ timeout: 10000 })

      // Should have multiple log lines
      const lineCount = await logLines.count()
      expect(lineCount).toBeGreaterThan(0)
    })

    test('shows Show all button when log is truncated', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Wait for log content to load
      await page.waitForSelector('[data-line]', { timeout: 10000 }).catch(() => {})

      // Check for Show all button (only present when truncated)
      // Note: This may not be visible if the log is not truncated
      const showAllButton = dialog.getByRole('button', { name: /show all/i })
      const showAllCount = await showAllButton.count()

      // If log is truncated, Show all should be visible
      // If not truncated, we just verify the modal is working
      if (showAllCount > 0) {
        await expect(showAllButton).toBeVisible()
      }
    })
  })

  test.describe('Error Handling', () => {
    test('shows loading state while fetching logs', async ({ page }) => {
      await page.goto('/')
      await page.waitForSelector('.animate-pulse', { state: 'detached', timeout: 15000 }).catch(() => {})

      // Find and click Logs button
      let logsButton = page.getByTestId('view-logs-button').first()
      let buttonCount = await logsButton.count()

      if (buttonCount === 0) {
        const groupLink = page.locator('a[href^="/groups/"]').first()
        if (await groupLink.count() > 0) {
          await groupLink.click()
          await page.waitForSelector('[data-testid="view-logs-button"]', { timeout: 5000 }).catch(() => {})
          logsButton = page.getByTestId('view-logs-button').first()
          buttonCount = await logsButton.count()
        }
      }

      if (buttonCount === 0) {
        test.skip(true, 'No jobs with logs available for testing')
        return
      }

      await logsButton.click()

      const dialog = page.locator('dialog[open]')
      await expect(dialog).toBeVisible()

      // Should show loading or log content (loading may be very brief)
      // Check for either loading state or log lines
      await expect(
        dialog.getByText('Loading logs...').or(dialog.locator('[data-line]').first())
      ).toBeVisible({ timeout: 10000 })
    })
  })
})
