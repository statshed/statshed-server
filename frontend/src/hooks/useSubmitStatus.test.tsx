/**
 * AIDEV-NOTE: useSubmitStatus hook tests
 */

import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { TestQueryProvider, createTestQueryClient } from '@/test/utils'
import { useSubmitStatus } from './useSubmitStatus'

// Mock the toast functions
vi.mock('@/contexts/ToastContext', () => ({
  showSuccessToast: vi.fn(),
  showErrorToast: vi.fn(),
}))

describe('useSubmitStatus', () => {
  it('submits status successfully', async () => {
    const queryClient = createTestQueryClient()

    const { result } = renderHook(() => useSubmitStatus(), {
      wrapper: ({ children }) => (
        <TestQueryProvider queryClient={queryClient}>{children}</TestQueryProvider>
      ),
    })

    // Execute mutation
    result.current.mutate({
      group: 'backups',
      job: 'daily-backup',
      status: 'success',
      message: 'Backup completed successfully',
    })

    // Wait for mutation to succeed
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the returned data
    expect(result.current.data).toBeDefined()
    expect(result.current.data?.group_name).toBe('backups')
    expect(result.current.data?.name).toBe('daily-backup')
    expect(result.current.data?.status).toBe('success')
    expect(result.current.data?.message).toBe('Backup completed successfully')
  })

  it('submits status without message', async () => {
    const queryClient = createTestQueryClient()

    const { result } = renderHook(() => useSubmitStatus(), {
      wrapper: ({ children }) => (
        <TestQueryProvider queryClient={queryClient}>{children}</TestQueryProvider>
      ),
    })

    // Execute mutation without message
    result.current.mutate({
      group: 'reports',
      job: 'weekly-report',
      status: 'progress',
    })

    // Wait for mutation to succeed
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the returned data
    expect(result.current.data?.group_name).toBe('reports')
    expect(result.current.data?.name).toBe('weekly-report')
    expect(result.current.data?.status).toBe('progress')
    expect(result.current.data?.message).toBeNull()
  })

  it('supports different status values', async () => {
    const queryClient = createTestQueryClient()

    const { result } = renderHook(() => useSubmitStatus(), {
      wrapper: ({ children }) => (
        <TestQueryProvider queryClient={queryClient}>{children}</TestQueryProvider>
      ),
    })

    // Submit with error status
    result.current.mutate({
      group: 'alerts',
      job: 'monitoring-check',
      status: 'error',
      message: 'Connection failed',
    })

    // Wait for mutation to succeed
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    // Check the returned data
    expect(result.current.data?.status).toBe('error')
    expect(result.current.data?.message).toBe('Connection failed')
  })
})
