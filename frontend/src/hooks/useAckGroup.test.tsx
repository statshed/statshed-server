/**
 * AIDEV-NOTE: useAckGroup hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider } from '@/test/utils'
import { useAckGroup } from './useAckGroup'

describe('useAckGroup', () => {
  it('successfully acks a group', async () => {
    const { result } = renderHook(() => useAckGroup(), {
      wrapper: TestQueryProvider,
    })

    // Initially not loading
    expect(result.current.isPending).toBe(false)

    // Trigger the mutation
    result.current.mutate('test-group')

    // Wait for success
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the response
    expect(result.current.data).toEqual({
      acked_count: 2,
      group: 'test-group',
    })
  })

  it('returns acked_count and group name', async () => {
    const { result } = renderHook(() => useAckGroup(), {
      wrapper: TestQueryProvider,
    })

    result.current.mutate('backups')

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.acked_count).toBe(2)
    expect(result.current.data?.group).toBe('backups')
  })
})
