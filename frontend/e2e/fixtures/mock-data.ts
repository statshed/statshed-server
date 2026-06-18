/**
 * AIDEV-NOTE: Deterministic fixture data for the hermetic e2e suite.
 *
 * These objects mirror the backend API response shapes (see src/types). They are
 * served by mock-api.ts via Playwright request interception so the e2e tests run
 * with NO live backend — fully deterministic, no DB seeding, CI-friendly.
 *
 * Type-only imports from src/types keep the fixtures honest against the real
 * contract without coupling the e2e build to the app's path aliases.
 */

import type {
  Config,
  Group,
  GroupConfig,
  GroupWithHealth,
  HealthSummary,
  Job,
  JobsResponse,
  JobStatus,
  LogResponse,
} from '../../src/types'

// Fixed timestamp so nothing in the suite depends on the wall clock.
const NOW = '2026-06-14T12:00:00.000Z'

const emptyStatusCounts: Record<JobStatus, number> = {
  success: 0,
  error: 0,
  progress: 0,
  timeout: 0,
  stale: 0,
}

/** Overall health summary served at GET /api/health. */
export const healthSummary: HealthSummary = {
  status: 'unhealthy',
  total_jobs: 12,
  unhealthy: 2,
  acked: 1,
  healthy: 7,
  in_progress: 3,
  by_status: { success: 7, error: 1, progress: 3, timeout: 1, stale: 0 },
}

/** Groups list served at GET /api/groups. The first entry is what the
 *  log-viewer/group-detail specs reach via `a[href^="/groups/"]`.first(). */
export const groups: GroupWithHealth[] = [
  {
    id: 1,
    name: 'backups',
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    created_at: NOW,
    job_count: 4,
    status_counts: { ...emptyStatusCounts, success: 2, error: 1, progress: 1 },
    health: 'unhealthy',
    unhealthy_count: 1,
    acked_count: 0,
  },
  {
    id: 2,
    name: 'web-servers',
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    created_at: NOW,
    job_count: 4,
    status_counts: { ...emptyStatusCounts, success: 2, progress: 2 },
    health: 'in_progress',
    unhealthy_count: 0,
    acked_count: 0,
  },
  {
    id: 3,
    name: 'cron-tasks',
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    created_at: NOW,
    job_count: 4,
    status_counts: { ...emptyStatusCounts, success: 3, timeout: 1 },
    health: 'healthy',
    unhealthy_count: 0,
    acked_count: 1,
  },
]

/** The `group` envelope returned alongside jobs at GET /api/groups/:name/jobs. */
export function groupStub(name: string): Group {
  return {
    id: 1,
    name,
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    created_at: NOW,
  }
}

/**
 * Deterministic job set for any group. Includes every status the group-detail
 * filter tests exercise (success/error/progress) plus jobs with logs attached so
 * the log-viewer specs find a `view-logs-button`.
 */
export function jobsForGroup(groupName: string): Job[] {
  const job = (
    id: number,
    name: string,
    status: JobStatus,
    overrides: Partial<Job> = {}
  ): Job => ({
    id,
    group_id: 1,
    group_name: groupName,
    name,
    status,
    message: null,
    acked: false,
    acked_at: null,
    updated_at: NOW,
    created_at: NOW,
    expires_at: null,
    has_log: false,
    log_line_count: null,
    log_truncated: false,
    log_updated_at: null,
    ...overrides,
  })

  return [
    job(1, 'nightly-backup', 'success', {
      message: 'Completed in 3m12s',
      has_log: true,
      log_line_count: 5000,
      log_truncated: true,
      log_updated_at: NOW,
    }),
    job(2, 'db-dump', 'error', {
      message: 'Exited with status 1',
      has_log: true,
      log_line_count: 5000,
      log_truncated: true,
      log_updated_at: NOW,
    }),
    job(3, 'sync-offsite', 'progress', { message: 'Uploading…' }),
    job(4, 'verify-archives', 'success', { message: 'OK' }),
  ]
}

/** Global config served at GET /api/config. */
export const config: Config = {
  progress_timeout_minutes: 30,
  staleness_timeout_hours: 24,
}

/**
 * Per-group config served at GET /api/groups/:name/config.
 * staleness_enabled is false (the staleness input is hidden by default) and
 * expiration is 48h, so the "staleness must be < expiration" validation test can
 * set staleness=30 > expiration=24 and reliably trigger the client-side error.
 */
export function groupConfig(): GroupConfig {
  return {
    progress_timeout_minutes: null,
    staleness_timeout_hours: null,
    staleness_enabled: false,
    expiration_timeout_hours: 48,
    effective_progress_timeout_minutes: 30,
    effective_staleness_timeout_hours: null,
    effective_expiration_timeout_hours: 48,
  }
}

/**
 * Log payload served at GET /api/groups/:name/jobs/:job/log.
 * Truncated (1000 of 5000 lines) so the "Show all" button appears, with periodic
 * ERROR markers so the error-navigation controls have targets.
 */
export function logResponse(): LogResponse {
  const lines = Array.from({ length: 1000 }, (_, i) => {
    const sec = String(i % 60).padStart(2, '0')
    const marker = i % 50 === 0 ? ' ERROR something went wrong' : ''
    return `2026-06-14 12:00:${sec} INFO line ${i + 1}${marker}`
  })
  return {
    log: lines.join('\n'),
    line_count: 1000,
    truncated: true,
    total_line_count: 5000,
  }
}

/** Jobs listing served at GET /api/jobs(?status=...), used by the Jobs page. */
export function jobsByStatus(statuses: string[]): JobsResponse {
  const all = jobsForGroup('backups')
  const jobs =
    statuses.length === 0 ? all : all.filter((j) => statuses.includes(j.status))
  return { jobs, total: jobs.length }
}
