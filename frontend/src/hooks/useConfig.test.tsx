/**
 * AIDEV-NOTE: useConfig hook tests
 */

import { describe, it, expect } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider, createTestQueryClient } from '@/test/utils'
import { useConfig, useUpdateConfig } from './useConfig'

describe('useConfig', () => {
  it('fetches global config successfully', async () => {
    const { result } = renderHook(() => useConfig(), {
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
    expect(result.current.data?.progress_timeout_minutes).toBe(30)
    expect(result.current.data?.staleness_timeout_hours).toBe(24)
  })
})

describe('useUpdateConfig', () => {
  it('updates config successfully', async () => {
    const queryClient = createTestQueryClient()

    const { result } = renderHook(() => useUpdateConfig(), {
      wrapper: ({ children }) => (
        <TestQueryProvider queryClient={queryClient}>{children}</TestQueryProvider>
      ),
    })

    // Execute mutation
    result.current.mutate({ progress_timeout_minutes: 45 })

    // Wait for mutation to succeed
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the returned data
    expect(result.current.data?.progress_timeout_minutes).toBe(45)
  })
})
