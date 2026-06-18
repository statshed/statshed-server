/**
 * AIDEV-NOTE: useAckAll hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider } from '@/test/utils'
import { useAckAll } from './useAckAll'

describe('useAckAll', () => {
  it('successfully acks all jobs', async () => {
    const { result } = renderHook(() => useAckAll(), {
      wrapper: TestQueryProvider,
    })

    // Initially not loading
    expect(result.current.isPending).toBe(false)

    // Trigger the mutation
    result.current.mutate()

    // Wait for success
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the response
    expect(result.current.data).toEqual({
      acked_count: 5,
    })
  })

  it('returns acked_count', async () => {
    const { result } = renderHook(() => useAckAll(), {
      wrapper: TestQueryProvider,
    })

    result.current.mutate()

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(result.current.data?.acked_count).toBe(5)
  })
})
