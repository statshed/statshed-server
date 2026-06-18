/**
 * AIDEV-NOTE: Regression tests for health-status-flash-ignores-in-progress.
 *
 * The optimistic health update in the ack hooks set status:'healthy' the moment
 * `unhealthy` hit 0 — even when in-progress jobs remained — so the favicon/health
 * pill briefly flashed green before the server invalidation corrected it. The fix
 * falls to 'in_progress' instead (backend precedence: unhealthy > in_progress > healthy).
 *
 * Each ack endpoint is held pending with delay('infinite') so the optimistic
 * onMutate value persists (no success/settle -> no invalidation/refetch to race),
 * making the assertion deterministic.
 */

import { describe, it, expect } from 'vitest'
import { type ReactNode } from 'react'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient } from '@tanstack/react-query'
import { http, HttpResponse, delay } from 'msw'
import { server } from '@/test/mocks/server'
import { createTestQueryClient, TestQueryProvider } from '@/test/utils'
import { queryKeys } from '@/lib/constants'
import { createMockJob } from '@/test/mocks/handlers'
import type { HealthSummary } from '@/types'
import { useAckJob } from './useAckJob'
import { useAckGroup } from './useAckGroup'
import { useAckAll } from './useAckAll'

const GROUP = 'backups'
const JOB_ID = 99

/** Seed: 1 unhealthy + 1 in-progress. Acking the unhealthy job must NOT flash 'healthy'. */
function seedClient(): QueryClient {
  const client = createTestQueryClient()
  const errorJob = createMockJob({
    id: JOB_ID,
    group_name: GROUP,
    name: 'daily-backup',
    status: 'error',
  })
  client.setQueryData(queryKeys.jobsByStatus(['error']), { jobs: [errorJob], total: 1 })
  client.setQueryData(queryKeys.groupJobs(GROUP), [errorJob])
  client.setQueryData(queryKeys.groups, [
    {
      id: 1,
      name: GROUP,
      progress_timeout_minutes: null,
      staleness_timeout_hours: null,
      created_at: '2024-01-01T00:00:00Z',
      job_count: 2,
      status_counts: { success: 0, error: 1, progress: 1, timeout: 0, stale: 0 },
      health: 'unhealthy' as const,
      unhealthy_count: 1,
      acked_count: 0,
    },
  ])
  client.setQueryData<HealthSummary>(queryKeys.health, {
    status: 'unhealthy',
    total_jobs: 2,
    unhealthy: 1,
    acked: 0,
    healthy: 0,
    in_progress: 1,
    by_status: { error: 1, progress: 1 },
  })
  return client
}

function wrapperFor(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <TestQueryProvider queryClient={client}>{children}</TestQueryProvider>
  }
}

const healthStatus = (client: QueryClient) =>
  client.getQueryData<HealthSummary>(queryKeys.health)?.status

describe('optimistic health status preserves in_progress', () => {
  it('useAckJob keeps status in_progress when in-progress jobs remain', async () => {
    server.use(
      http.post('/api/jobs/:id/ack', async () => {
        await delay('infinite')
        return HttpResponse.json({})
      })
    )
    const client = seedClient()
    const { result } = renderHook(() => useAckJob(), { wrapper: wrapperFor(client) })

    result.current.mutate(JOB_ID)

    await waitFor(() => expect(healthStatus(client)).toBe('in_progress'))
  })

  it('useAckGroup keeps status in_progress when in-progress jobs remain', async () => {
    server.use(
      http.post('/api/groups/:name/ack', async () => {
        await delay('infinite')
        return HttpResponse.json({})
      })
    )
    const client = seedClient()
    const { result } = renderHook(() => useAckGroup(), { wrapper: wrapperFor(client) })

    result.current.mutate(GROUP)

    await waitFor(() => expect(healthStatus(client)).toBe('in_progress'))
  })

  it('useAckAll keeps status in_progress when in-progress jobs remain', async () => {
    server.use(
      http.post('/api/ack-all', async () => {
        await delay('infinite')
        return HttpResponse.json({})
      })
    )
    const client = seedClient()
    const { result } = renderHook(() => useAckAll(), { wrapper: wrapperFor(client) })

    result.current.mutate()

    await waitFor(() => expect(healthStatus(client)).toBe('in_progress'))
  })
})
