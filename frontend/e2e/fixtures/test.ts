/**
 * AIDEV-NOTE: Playwright test fixture with hermetic API mocking baked in.
 *
 * Import `test`/`expect` from here instead of '@playwright/test' and every test
 * gets the mocked `/api` (see mock-api.ts) installed before it runs — no live
 * backend required. New specs need no per-test setup.
 */

import { test as base, expect } from '@playwright/test'
import { mockApi } from './mock-api'

export const test = base.extend({
  // AIDEV-NOTE: the fixture's "provide" callback is named `provide` (not the
  // conventional `use`) so eslint's react-hooks/rules-of-hooks doesn't mistake
  // `use(page)` for the React `use` hook.
  page: async ({ page }, provide) => {
    await mockApi(page)
    await provide(page)
  },
})

export { expect }
