/**
 * AIDEV-NOTE: Tests for useFadePercentage hook
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFadePercentage } from './useFadePercentage'

describe('useFadePercentage', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns full opacity when no expiration', () => {
    const { result } = renderHook(() =>
      useFadePercentage(null, '2024-01-01T00:00:00Z')
    )

    expect(result.current.opacity).toBe(1.0)
    expect(result.current.isFading).toBe(false)
    expect(result.current.timeRemainingMs).toBe(Infinity)
    expect(result.current.timeRemainingText).toBe('never')
    expect(result.current.isValid).toBe(true)
  })

  it('returns full opacity before fade window (more than 50% time remaining)', () => {
    // Job updated at midnight, expires at midnight tomorrow (24h window)
    // Current time: 6 hours into the window (25% elapsed, 75% remaining)
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'
    const now = new Date('2024-01-01T06:00:00Z').getTime()
    vi.setSystemTime(now)

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )

    expect(result.current.opacity).toBe(1.0)
    expect(result.current.isFading).toBe(false)
    expect(result.current.timeRemainingMs).toBe(18 * 60 * 60 * 1000) // 18 hours
  })

  it('starts fading at 50% time remaining', () => {
    // Job updated at midnight, expires at midnight tomorrow (24h window)
    // Current time: exactly 12 hours in (50% elapsed, at fade_start)
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'
    const now = new Date('2024-01-01T12:00:00Z').getTime()
    vi.setSystemTime(now)

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )

    // At exactly 50%, opacity should be 1.0 (start of fade window)
    expect(result.current.opacity).toBe(1.0)
    expect(result.current.isFading).toBe(true)
  })

  it('fades to 75% opacity at 75% elapsed time', () => {
    // Job updated at midnight, expires at midnight tomorrow (24h window)
    // Current time: 18 hours in (75% elapsed, halfway through fade window)
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'
    const now = new Date('2024-01-01T18:00:00Z').getTime()
    vi.setSystemTime(now)

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )

    // Halfway through fade window: opacity = 1.0 - (0.5 * 0.5) = 0.75
    expect(result.current.opacity).toBeCloseTo(0.75, 2)
    expect(result.current.isFading).toBe(true)
    expect(result.current.timeRemainingMs).toBe(6 * 60 * 60 * 1000) // 6 hours
  })

  it('fades to 50% opacity at expiration', () => {
    // Job updated at midnight, expires at midnight tomorrow
    // Current time: exactly at expiration
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'
    const now = new Date('2024-01-02T00:00:00Z').getTime()
    vi.setSystemTime(now)

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )

    expect(result.current.opacity).toBe(0.5)
    expect(result.current.isFading).toBe(true)
    expect(result.current.timeRemainingMs).toBe(0)
    expect(result.current.timeRemainingText).toBe('expired')
  })

  it('formats time remaining correctly - days', () => {
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'

    // Test days (24 hours remaining)
    vi.setSystemTime(new Date('2024-01-01T00:00:00Z').getTime())
    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )
    expect(result.current.timeRemainingText).toBe('1d')
  })

  it('formats time remaining correctly - hours and minutes', () => {
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'

    // Test hours + minutes (2.5 hours remaining)
    vi.setSystemTime(new Date('2024-01-01T21:30:00Z').getTime())
    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )
    expect(result.current.timeRemainingText).toBe('2h 30m')
  })

  it('formats time remaining correctly - minutes only', () => {
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'

    // Test minutes only (15 minutes remaining)
    vi.setSystemTime(new Date('2024-01-01T23:45:00Z').getTime())
    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )
    expect(result.current.timeRemainingText).toBe('15m')
  })

  it('updates opacity on interval', () => {
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'

    // Start at 6 hours in (no fading)
    vi.setSystemTime(new Date('2024-01-01T06:00:00Z').getTime())

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt, 1000) // 1 second interval for testing
    )

    expect(result.current.opacity).toBe(1.0)
    expect(result.current.isFading).toBe(false)

    // Jump to 18 hours in (75% elapsed, should be fading)
    act(() => {
      vi.setSystemTime(new Date('2024-01-01T18:00:00Z').getTime())
      vi.advanceTimersByTime(1000)
    })

    expect(result.current.opacity).toBeCloseTo(0.75, 2)
    expect(result.current.isFading).toBe(true)
  })

  it('handles invalid dates gracefully', () => {
    const { result } = renderHook(() =>
      useFadePercentage('invalid-date', 'also-invalid')
    )

    expect(result.current.opacity).toBe(1.0)
    expect(result.current.isFading).toBe(false)
    expect(result.current.timeRemainingText).toBe('unknown')
    expect(result.current.isValid).toBe(false)
  })

  it('returns isValid=true for valid dates', () => {
    const updatedAt = '2024-01-01T00:00:00Z'
    const expiresAt = '2024-01-02T00:00:00Z'
    vi.setSystemTime(new Date('2024-01-01T06:00:00Z').getTime())

    const { result } = renderHook(() =>
      useFadePercentage(expiresAt, updatedAt)
    )

    expect(result.current.isValid).toBe(true)
  })

  it('does not set up interval for invalid dates', () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')

    renderHook(() => useFadePercentage('invalid-date', 'also-invalid'))

    // The interval should not be set up for invalid dates
    expect(setIntervalSpy).not.toHaveBeenCalled()

    setIntervalSpy.mockRestore()
  })

  it('does not set up interval for already expired jobs', () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')

    // Set time to after expiration
    vi.setSystemTime(new Date('2024-01-03T00:00:00Z').getTime())

    renderHook(() =>
      useFadePercentage('2024-01-02T00:00:00Z', '2024-01-01T00:00:00Z')
    )

    // The interval should not be set up for expired jobs
    expect(setIntervalSpy).not.toHaveBeenCalled()

    setIntervalSpy.mockRestore()
  })
})
