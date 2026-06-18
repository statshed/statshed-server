/**
 * AIDEV-NOTE: useFavicon tests.
 *
 * Regression: the favicon took a boolean `hasErrors`, so when health was undefined
 * (backend unreachable) it collapsed to "no errors" and showed GREEN during a total
 * outage. It now takes a 3-state status so an unknown/unreachable backend shows grey.
 */

import { describe, it, expect } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useFavicon } from './useFavicon'

function faviconHref(): string {
  return document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.href ?? ''
}

describe('useFavicon', () => {
  it('uses green for the healthy state', () => {
    renderHook(() => useFavicon('healthy'))
    expect(faviconHref()).toContain(encodeURIComponent('#22c55e'))
  })

  it('uses red for the error state', () => {
    renderHook(() => useFavicon('error'))
    expect(faviconHref()).toContain(encodeURIComponent('#ef4444'))
  })

  it('uses grey (not green) for the unknown state when the backend is unreachable', () => {
    renderHook(() => useFavicon('unknown'))
    const href = faviconHref()
    expect(href).toContain(encodeURIComponent('#9ca3af'))
    expect(href).not.toContain(encodeURIComponent('#22c55e'))
  })
})
