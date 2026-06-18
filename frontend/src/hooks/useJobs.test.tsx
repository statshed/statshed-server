/**
 * AIDEV-NOTE: Tests for useJobsByStatus hook
 */

import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { useJobsByStatus } from './useJobs'
import { TestQueryProvider } from '@/test/utils'

describe('useJobsByStatus', () => {
  it('fetches jobs for single status', async () => {
    const { result } = renderHook(
      () => useJobsByStatus(['success']),
      { wrapper: TestQueryProvider }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toBeDefined()
    expect(result.current.data?.jobs).toBeDefined()
    // Mock returns 2 success jobs
    expect(result.current.data?.jobs.every(j => j.status === 'success')).toBe(true)
  })

  it('fetches jobs for multiple statuses', async () => {
    const { result } = renderHook(
      () => useJobsByStatus(['error', 'timeout']),
      { wrapper: TestQueryProvider }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toBeDefined()
    expect(result.current.data?.jobs).toBeDefined()
    // All jobs should be error or timeout
    expect(result.current.data?.jobs.every(j => ['error', 'timeout'].includes(j.status))).toBe(true)
  })

  it('fetches all jobs when no status filter', async () => {
    const { result } = renderHook(
      () => useJobsByStatus([]),
      { wrapper: TestQueryProvider }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toBeDefined()
    expect(result.current.data?.jobs).toBeDefined()
    // Mock returns 5 total jobs
    expect(result.current.data?.total).toBe(5)
  })

  it('returns correct total count', async () => {
    const { result } = renderHook(
      () => useJobsByStatus(['progress']),
      { wrapper: TestQueryProvider }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toBeDefined()
    // Mock returns 1 progress job
    expect(result.current.data?.total).toBe(1)
    expect(result.current.data?.jobs.length).toBe(1)
  })
})
