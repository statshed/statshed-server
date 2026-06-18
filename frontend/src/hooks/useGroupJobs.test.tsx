/**
 * AIDEV-NOTE: useGroupJobs hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider } from '@/test/utils'
import { useGroupJobs } from './useGroupJobs'

describe('useGroupJobs', () => {
  it('fetches jobs for a group successfully', async () => {
    const { result } = renderHook(() => useGroupJobs('backups'), {
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

  it('returns jobs with correct structure', async () => {
    const { result } = renderHook(() => useGroupJobs('backups'), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const jobs = result.current.data
    expect(jobs?.[0]).toHaveProperty('id')
    expect(jobs?.[0]).toHaveProperty('name')
    expect(jobs?.[0]).toHaveProperty('status')
    expect(jobs?.[0]).toHaveProperty('group_name')
    expect(jobs?.[0]).toHaveProperty('updated_at')
  })

  it('returns jobs with the correct group name', async () => {
    const { result } = renderHook(() => useGroupJobs('test-group'), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const jobs = result.current.data
    expect(jobs?.[0].group_name).toBe('test-group')
  })

  it('returns different job statuses', async () => {
    const { result } = renderHook(() => useGroupJobs('backups'), {
      wrapper: TestQueryProvider,
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    const jobs = result.current.data
    const statuses = jobs?.map((j) => j.status)
    expect(statuses).toContain('success')
    expect(statuses).toContain('progress')
  })
})
