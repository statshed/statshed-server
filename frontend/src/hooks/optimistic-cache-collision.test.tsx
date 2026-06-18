/**
 * AIDEV-NOTE: Regression tests for the prefix-key cache collision in optimistic mutations.
 *
 * queryKeys.jobLog(...) = ['jobs','log',...] is a prefix-CHILD of queryKeys.jobs = ['jobs'].
 * The ack/delete mutations call setQueriesData({queryKey: queryKeys.jobs}, ...) which
 * partial-matches the jobLog cache (a LogResponse with no `.jobs` array). The updaters did
 * `old.jobs.map(...)` unconditionally, throwing inside onMutate when a user had opened a job
 * log (LogViewerModal -> useJobLog caches it) and then clicked Ack/Delete. The throw aborted
 * the mutation entirely, so the action silently failed with a confusing error toast.
 *
 * Each test seeds a jobLog cache entry (the poison) plus a valid jobs-by-status entry, then
 * fires the mutation and asserts it SUCCEEDS (onMutate did not throw). Before the guard these
 * fail (mutation goes to error); after the guard they pass.
 */

import { describe, it, expect } from 'vitest'
import { type ReactNode } from 'react'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient } from '@tanstack/react-query'
import { createTestQueryClient, TestQueryProvider } from '@/test/utils'
import { queryKeys } from '@/lib/constants'
import { createMockJob, createMockLogResponse } from '@/test/mocks/handlers'
import { useAckJob } from './useAckJob'
import { useDeleteJob } from './useDeleteJob'
import { useAckGroup } from './useAckGroup'
import { useAckAll } from './useAckAll'

const GROUP = 'backups'
const JOB_ID = 99

/** Seed a client with a poison jobLog cache entry + a valid jobs-by-status entry + group caches. */
function seedClient(): QueryClient {
  const client = createTestQueryClient()

  // The poison: a LogResponse cached under ['jobs','log',...], which the ['jobs'] prefix matches.
  client.setQueryData(
    queryKeys.jobLog(GROUP, 'daily-backup', false),
    createMockLogResponse({ log: 'starting\nERROR boom\ndone' })
  )

  // A valid jobs-by-status cache (['jobs','byStatus',...]) that the updaters should still touch.
  const errorJob = createMockJob({ id: JOB_ID, group_name: GROUP, name: 'daily-backup', status: 'error' })
  client.setQueryData(queryKeys.jobsByStatus(['error']), { jobs: [errorJob], total: 1 })

  // Group-specific caches so optimistic updates have something to operate on.
  client.setQueryData(queryKeys.groupJobs(GROUP), [errorJob])
  client.setQueryData(queryKeys.groups, [
    {
      id: 1,
      name: GROUP,
      progress_timeout_minutes: null,
      staleness_timeout_hours: null,
      created_at: '2024-01-01T00:00:00Z',
      job_count: 1,
      status_counts: { success: 0, error: 1, progress: 0, timeout: 0, stale: 0 },
      health: 'unhealthy' as const,
      unhealthy_count: 1,
      acked_count: 0,
    },
  ])
  client.setQueryData(queryKeys.health, {
    status: 'unhealthy' as const,
    total_jobs: 1,
    unhealthy: 1,
    acked: 0,
    healthy: 0,
    in_progress: 0,
    by_status: { error: 1 },
  })

  return client
}

function wrapperFor(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <TestQueryProvider queryClient={client}>{children}</TestQueryProvider>
  }
}

describe('optimistic mutation prefix-key cache collision', () => {
  it('useAckJob does not crash when a jobLog cache entry is present', async () => {
    const client = seedClient()
    const { result } = renderHook(() => useAckJob(), { wrapper: wrapperFor(client) })

    result.current.mutate(JOB_ID)

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.isError).toBe(false)
    // The jobLog cache must be left untouched by the jobs updater.
    expect(client.getQueryData(queryKeys.jobLog(GROUP, 'daily-backup', false))).toMatchObject({
      log: 'starting\nERROR boom\ndone',
    })
  })

  it('useDeleteJob does not crash when a jobLog cache entry is present', async () => {
    const client = seedClient()
    const { result } = renderHook(() => useDeleteJob(), { wrapper: wrapperFor(client) })

    result.current.mutate(JOB_ID)

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.isError).toBe(false)
    expect(client.getQueryData(queryKeys.jobLog(GROUP, 'daily-backup', false))).toMatchObject({
      log: 'starting\nERROR boom\ndone',
    })
  })

  it('useAckGroup does not crash when a jobLog cache entry is present', async () => {
    const client = seedClient()
    const { result } = renderHook(() => useAckGroup(), { wrapper: wrapperFor(client) })

    result.current.mutate(GROUP)

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.isError).toBe(false)
    expect(client.getQueryData(queryKeys.jobLog(GROUP, 'daily-backup', false))).toMatchObject({
      log: 'starting\nERROR boom\ndone',
    })
  })

  it('useAckAll does not crash when a jobLog cache entry is present', async () => {
    const client = seedClient()
    const { result } = renderHook(() => useAckAll(), { wrapper: wrapperFor(client) })

    result.current.mutate()

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.isError).toBe(false)
    expect(client.getQueryData(queryKeys.jobLog(GROUP, 'daily-backup', false))).toMatchObject({
      log: 'starting\nERROR boom\ndone',
    })
  })
})
