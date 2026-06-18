/**
 * AIDEV-NOTE: Dashboard E2E tests
 * Tests the main dashboard page functionality
 */

import { test, expect } from './fixtures/test'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate and wait for page content to load
    await page.goto('/')
    // Wait for either the Dashboard heading or an error message
    await page.waitForSelector('h1, [role="alert"]', { timeout: 15000 })
  })

  test('displays the dashboard header', async ({ page }) => {
    // StatShed in header should always be visible
    await expect(page.getByText('StatShed')).toBeVisible()
    // And "Dashboard" text should be visible (page title or heading)
    await expect(page.getByText('Dashboard')).toBeVisible()
  })

  test('has a settings link in the header', async ({ page }) => {
    const settingsLink = page.locator('a[href="/settings"]')
    await expect(settingsLink).toBeVisible()
  })

  test('shows the submit status button', async ({ page }) => {
    // Find any submit button
    const submitButton = page.locator('button', { hasText: 'Submit Status' }).first()
    await expect(submitButton).toBeVisible()
  })

  test('opens submit status dialog when clicked', async ({ page }) => {
    // Find and click the submit button
    const submitButton = page.locator('button', { hasText: 'Submit Status' }).first()
    await expect(submitButton).toBeVisible()
    await submitButton.click()
    // Wait for dialog to open
    await expect(page.locator('dialog[open]')).toBeVisible({ timeout: 5000 })
  })

  test('has search and filter controls', async ({ page }) => {
    // Wait for the page to load with filter controls
    await expect(page.locator('input[placeholder="Search groups..."]')).toBeVisible()
    // Use getByRole for filter button to avoid matching health stats label
    await expect(page.getByRole('button', { name: 'Healthy', exact: true })).toBeVisible()
  })
})

test.describe('Settings Page', () => {
  test('navigates to settings page', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
  })

  test('displays timeout settings form', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.locator('h2:has-text("Timeout Settings")')).toBeVisible()
    await expect(page.locator('label:has-text("Progress Timeout")')).toBeVisible()
    await expect(page.locator('label:has-text("Staleness Timeout")')).toBeVisible()
  })
})
