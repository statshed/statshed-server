/**
 * AIDEV-NOTE: useHealth hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider } from '@/test/utils'
import { useHealth } from './useHealth'

describe('useHealth', () => {
  it('fetches health data successfully', async () => {
    const { result } = renderHook(() => useHealth(), {
      wrapper: TestQueryProvider,
    })

    // Initially loading
    expect(result.current.isLoading).toBe(true)

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the data - AIDEV-NOTE: Backend returns {status, total_jobs, by_status} format
    expect(result.current.data).toBeDefined()
    expect(result.current.data?.status).toBe('healthy')
    expect(result.current.data?.total_jobs).toBe(10)
  })

  it('returns health status counts', async () => {
    const { result } = renderHook(() => useHealth(), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // AIDEV-NOTE: Backend returns by_status directly, not nested
    const by_status = result.current.data?.by_status
    expect(by_status).toBeDefined()
    expect(by_status?.success).toBe(7)
    expect(by_status?.error).toBe(2)
    expect(by_status?.progress).toBe(1)
  })

  it('returns health status string', async () => {
    const { result } = renderHook(() => useHealth(), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // AIDEV-NOTE: Backend returns status as health status enum value
    expect(result.current.data?.status).toBe('healthy')
  })
})
