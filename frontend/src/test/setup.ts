/**
 * AIDEV-NOTE: Test setup file
 * Configures testing environment with jsdom and testing-library
 * Also sets up MSW for API mocking
 */

// AIDEV-NOTE: Must be first — installs undici Fetch globals before MSW patches fetch.
// See undici-globals.ts for why (jsdom/undici AbortSignal realm mismatch).
import './undici-globals'
import '@testing-library/jest-dom'
import { beforeAll, afterEach, afterAll } from 'vitest'
import { server } from './mocks/server'

// Start MSW server before all tests
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))

// Reset handlers after each test (important for test isolation)
afterEach(() => server.resetHandlers())

// Clean up after all tests are done
afterAll(() => server.close())

// Mock matchMedia for tests
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
})

// Mock ResizeObserver
class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

window.ResizeObserver = ResizeObserverMock

// Mock dialog element methods
HTMLDialogElement.prototype.showModal = function () {
  this.setAttribute('open', '')
}

HTMLDialogElement.prototype.close = function () {
  this.removeAttribute('open')
}
