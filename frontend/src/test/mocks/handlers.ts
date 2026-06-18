/**
 * AIDEV-NOTE: MSW request handlers for API mocking in tests
 */

import { http, HttpResponse } from 'msw'
import type {
  HealthSummary,
  GroupWithHealth,
  Job,
  Config,
  GroupConfig,
  LogResponse,
} from '@/types'

// Mock data factories
// AIDEV-NOTE: These match the actual backend response format
export function createMockHealthSummary(
  overrides?: Partial<HealthSummary>
): HealthSummary {
  return {
    status: 'healthy',
    total_jobs: 10,
    unhealthy: 2,
    acked: 0,
    healthy: 7,
    in_progress: 1,
    by_status: {
      success: 7,
      error: 2,
      progress: 1,
      timeout: 0,
      stale: 0,
    },
    ...overrides,
  }
}

export function createMockGroup(
  overrides?: Partial<GroupWithHealth>
): GroupWithHealth {
  return {
    id: 1,
    name: 'test-group',
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    created_at: '2024-01-01T00:00:00Z',
    job_count: 5,
    status_counts: { success: 3, error: 1, progress: 1, timeout: 0, stale: 0 },
    health: 'healthy',
    unhealthy_count: 1,
    acked_count: 0,
    ...overrides,
  }
}

export function createMockJob(overrides?: Partial<Job>): Job {
  return {
    id: 1,
    group_id: 1,
    group_name: 'test-group',
    name: 'test-job',
    status: 'success',
    message: null,
    acked: false,
    acked_at: null,
    updated_at: '2024-01-01T12:00:00Z',
    created_at: '2024-01-01T00:00:00Z',
    expires_at: '2024-01-02T12:00:00Z', // 24h after updated_at by default
    // AIDEV-NOTE: Log metadata fields default to no log attached
    has_log: false,
    log_line_count: null,
    log_truncated: false,
    log_updated_at: null,
    ...overrides,
  }
}

export function createMockConfig(overrides?: Partial<Config>): Config {
  return {
    progress_timeout_minutes: 30,
    staleness_timeout_hours: 24,
    ...overrides,
  }
}

export function createMockGroupConfig(
  overrides?: Partial<GroupConfig>
): GroupConfig {
  return {
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    staleness_enabled: false,
    expiration_timeout_hours: 24,
    effective_progress_timeout_minutes: 30,
    effective_staleness_timeout_hours: null,
    effective_expiration_timeout_hours: 24,
    ...overrides,
  }
}

// AIDEV-NOTE: Mock log content with some error lines for testing
export const MOCK_LOG_CONTENT = `Starting build process...
Compiling source files...
Processing module A...
Processing module B...
[ERROR] Failed to compile module C: syntax error
Processing module D...
Build completed with errors.
Error: 1 module failed to compile
Done.`

export function createMockLogResponse(
  overrides?: Partial<LogResponse>
): LogResponse {
  const log = overrides?.log ?? MOCK_LOG_CONTENT
  const lines = log.split('\n')
  return {
    log,
    line_count: lines.length,
    truncated: false,
    total_line_count: lines.length,
    ...overrides,
  }
}

// Default handlers
export const handlers = [
  // Health endpoint
  http.get('/api/health', () => {
    return HttpResponse.json(createMockHealthSummary())
  }),

  // Groups list endpoint - AIDEV-NOTE: Backend returns {groups: [...]}
  http.get('/api/groups', () => {
    return HttpResponse.json({
      groups: [
        createMockGroup({ id: 1, name: 'backups' }),
        createMockGroup({ id: 2, name: 'reports' }),
      ],
    })
  }),

  // Group jobs endpoint - AIDEV-NOTE: Backend returns {group: {...}, jobs: [...]}
  http.get('/api/groups/:name/jobs', ({ params }) => {
    const { name } = params
    return HttpResponse.json({
      group: {
        id: 1,
        name: name as string,
        progress_timeout_minutes: null,
        staleness_timeout_hours: null,
        created_at: '2024-01-01T00:00:00Z',
      },
      jobs: [
        createMockJob({ id: 1, group_name: name as string, name: 'job-1' }),
        createMockJob({ id: 2, group_name: name as string, name: 'job-2', status: 'progress' }),
      ],
    })
  }),

  // Jobs listing endpoint - AIDEV-NOTE: Backend returns {jobs: [...], total: number}
  // Used by health card click-through feature
  http.get('/api/jobs', ({ request }) => {
    const url = new URL(request.url)
    const statusParam = url.searchParams.get('status') || ''
    const statuses = statusParam ? statusParam.split(',').map(s => s.trim().toLowerCase()) : []

    // Create mock jobs with various statuses
    const allJobs = [
      createMockJob({ id: 1, group_name: 'backups', name: 'daily-backup', status: 'success' }),
      createMockJob({ id: 2, group_name: 'backups', name: 'weekly-backup', status: 'success' }),
      createMockJob({ id: 3, group_name: 'reports', name: 'monthly-report', status: 'error', message: 'Connection failed' }),
      createMockJob({ id: 4, group_name: 'monitoring', name: 'health-check', status: 'progress' }),
      createMockJob({ id: 5, group_name: 'monitoring', name: 'api-check', status: 'timeout' }),
    ]

    // Filter by status if provided
    const filteredJobs = statuses.length > 0
      ? allJobs.filter(job => statuses.includes(job.status))
      : allJobs

    return HttpResponse.json({
      jobs: filteredJobs,
      total: filteredJobs.length,
    })
  }),

  // Group config endpoint
  http.get('/api/groups/:name/config', () => {
    return HttpResponse.json(createMockGroupConfig())
  }),

  // Update group config endpoint
  http.put('/api/groups/:name/config', async ({ request }) => {
    const body = (await request.json()) as Partial<GroupConfig>
    return HttpResponse.json(
      createMockGroupConfig({
        progress_timeout_minutes: body.progress_timeout_minutes ?? null,
        staleness_timeout_hours: body.staleness_timeout_hours ?? null,
      })
    )
  }),

  // Global config endpoint
  http.get('/api/config', () => {
    return HttpResponse.json(createMockConfig())
  }),

  // Update global config endpoint
  http.put('/api/config', async ({ request }) => {
    const body = (await request.json()) as Partial<Config>
    return HttpResponse.json(createMockConfig(body))
  }),

  // Submit status endpoint
  http.post('/api/status', async ({ request }) => {
    const body = (await request.json()) as {
      group: string
      job: string
      status: string
      message?: string
    }
    return HttpResponse.json(
      createMockJob({
        group_name: body.group,
        name: body.job,
        status: body.status as Job['status'],
        message: body.message || null,
      })
    )
  }),

  // Acknowledge job endpoint
  http.post('/api/jobs/:id/ack', ({ params }) => {
    const { id } = params
    return HttpResponse.json({
      job: createMockJob({
        id: parseInt(id as string, 10),
        status: 'error',
        acked: true,
        acked_at: new Date().toISOString(),
      }),
    })
  }),

  // Acknowledge group endpoint
  http.post('/api/groups/:name/ack', ({ params }) => {
    const { name } = params
    return HttpResponse.json({
      acked_count: 2,
      group: name as string,
    })
  }),

  // Acknowledge all endpoint
  http.post('/api/ack-all', () => {
    return HttpResponse.json({
      acked_count: 5,
    })
  }),

  // Delete job endpoint
  http.delete('/api/jobs/:id', ({ params }) => {
    const { id } = params
    return HttpResponse.json({
      deleted_job: createMockJob({
        id: parseInt(id as string, 10),
      }),
      group_id: 1,
      group_name: 'test-group',
    })
  }),

  // Job log endpoint
  // AIDEV-NOTE: Returns mock log content with error lines for testing
  http.get('/api/groups/:groupName/jobs/:jobName/log', ({ request }) => {
    const url = new URL(request.url)
    const returnAll = url.searchParams.get('all') === 'true'

    // Simulate truncation for large logs unless all=true
    if (!returnAll) {
      return HttpResponse.json(
        createMockLogResponse({
          truncated: true,
          line_count: 9,
          total_line_count: 50, // Simulate larger original log
        })
      )
    }

    return HttpResponse.json(createMockLogResponse())
  }),
]
