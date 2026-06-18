/**
 * AIDEV-NOTE: useGroups hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider } from '@/test/utils'
import { useGroups } from './useGroups'

describe('useGroups', () => {
  it('fetches groups list successfully', async () => {
    const { result } = renderHook(() => useGroups(), {
      wrapper: TestQueryProvider,
    })

    // Initially loading
    expect(result.current.isLoading).toBe(true)

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the data
    expect(result.current.data).toBeDefined()
    expect(Array.isArray(result.current.data)).toBe(true)
    expect(result.current.data?.length).toBe(2)
  })

  it('returns groups with correct structure', async () => {
    const { result } = renderHook(() => useGroups(), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const groups = result.current.data
    expect(groups?.[0]).toHaveProperty('id')
    expect(groups?.[0]).toHaveProperty('name')
    expect(groups?.[0]).toHaveProperty('job_count')
    expect(groups?.[0]).toHaveProperty('status_counts')
    expect(groups?.[0]).toHaveProperty('health')
  })

  it('returns groups with health status', async () => {
    const { result } = renderHook(() => useGroups(), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const groups = result.current.data
    expect(groups?.[0].health).toBeDefined()
    expect(groups?.[0].status_counts).toBeDefined()
    expect(groups?.[0].status_counts.success).toBeDefined()
  })
})
