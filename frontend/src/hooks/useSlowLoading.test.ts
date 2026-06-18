/**
 * AIDEV-NOTE: useSlowLoading tests.
 * Surfaces a "taking longer than usual" signal after a backend has been loading for a while,
 * so a hung backend shows a hint instead of only skeletons. Deterministic via fake timers.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useSlowLoading } from './useSlowLoading'

describe('useSlowLoading', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('is false initially and becomes true after the delay while still loading', () => {
    const { result } = renderHook(() => useSlowLoading(true, 5000))
    expect(result.current).toBe(false)

    act(() => {
      vi.advanceTimersByTime(5000)
    })
    expect(result.current).toBe(true)
  })

  it('stays false when loading finishes before the delay', () => {
    const { result, rerender } = renderHook(
      ({ loading }) => useSlowLoading(loading, 5000),
      { initialProps: { loading: true } }
    )

    act(() => {
      vi.advanceTimersByTime(3000)
    })
    rerender({ loading: false })
    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(result.current).toBe(false)
  })

  it('resets to false once loading ends after having been slow', () => {
    const { result, rerender } = renderHook(
      ({ loading }) => useSlowLoading(loading, 5000),
      { initialProps: { loading: true } }
    )

    act(() => {
      vi.advanceTimersByTime(5000)
    })
    expect(result.current).toBe(true)

    rerender({ loading: false })
    expect(result.current).toBe(false)
  })
})
