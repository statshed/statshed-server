/**
 * AIDEV-NOTE: Application constants
 * Status colors, labels, and other configuration values
 */

import type { JobStatus, HealthStatus } from '@/types'
import { ApiError } from '@/api/client'

/**
 * AIDEV-NOTE: Backend URL for WebSocket and API connections
 *
 * Build-time configuration via VITE_BACKEND_URL environment variable:
 * - If set: Uses the specified URL (for direct connections bypassing proxy)
 * - If not set: Uses empty string (same origin); the unified server serves /api + /socket.io
 *
 * In Docker/production: the single statshed-server image serves the SPA, /api, and /socket.io same-origin
 * In local dev: Set VITE_BACKEND_URL to connect directly, or use Vite's proxy
 */
function getBackendUrl(): string {
  const envUrl = import.meta.env.VITE_BACKEND_URL as string | undefined

  // If no build-time URL specified, use same origin (let nginx proxy)
  if (!envUrl) {
    return ''
  }

  // Validate URL format and protocol
  try {
    const parsed = new URL(envUrl)
    if (!['http:', 'https:'].includes(parsed.protocol)) {
      console.error(`Invalid BACKEND_URL protocol: ${parsed.protocol}. Must be http: or https:`)
      return ''
    }
    return envUrl
  } catch {
    console.error(`Invalid BACKEND_URL format: ${envUrl}`)
    return ''
  }
}

export const BACKEND_URL = getBackendUrl()

/**
 * Status badge color classes for job statuses
 */
export const JOB_STATUS_COLORS: Record<JobStatus, string> = {
  success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  timeout: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  stale: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
}

/**
 * Human-readable labels for job statuses
 */
export const JOB_STATUS_LABELS: Record<JobStatus, string> = {
  success: 'Success',
  error: 'Error',
  progress: 'In Progress',
  timeout: 'Timeout',
  stale: 'Stale',
}

/**
 * Health status indicator colors
 */
export const HEALTH_STATUS_COLORS: Record<HealthStatus, string> = {
  healthy: 'text-green-500',
  unhealthy: 'text-red-500',
  in_progress: 'text-blue-500',
  empty: 'text-gray-400',
}

/**
 * Human-readable labels for health statuses
 */
export const HEALTH_STATUS_LABELS: Record<HealthStatus, string> = {
  healthy: 'All systems operational',
  unhealthy: 'Some systems have issues',
  in_progress: 'Operations in progress',
  empty: 'No jobs configured',
}

/**
 * Background indicator colors for health status
 */
export const HEALTH_STATUS_BG_COLORS: Record<HealthStatus, string> = {
  healthy: 'bg-green-500',
  unhealthy: 'bg-red-500',
  in_progress: 'bg-blue-500',
  empty: 'bg-gray-400',
}

/**
 * TanStack Query key factory for consistency
 */
export const queryKeys = {
  health: ['health'] as const,
  groups: ['groups'] as const,
  groupJobs: (name: string) => ['groups', name, 'jobs'] as const,
  config: ['config'] as const,
  groupConfig: (name: string) => ['groups', name, 'config'] as const,
  jobsByStatus: (statuses: string[]) => ['jobs', 'byStatus', statuses] as const,
  jobs: ['jobs'] as const,
  // AIDEV-NOTE: jobLog key includes groupName, jobName, and whether we're fetching all lines
  jobLog: (groupName: string, jobName: string, all?: boolean) =>
    ['jobs', 'log', groupName, jobName, all ?? false] as const,
}

/**
 * Default query options
 */
export const DEFAULT_QUERY_OPTIONS = {
  staleTime: 30 * 1000, // 30 seconds
  refetchOnWindowFocus: true,
  // AIDEV-NOTE: Don't retry deterministic 4xx client errors (400/404/etc., and our 408 timeout
  // sentinel) — retrying only delays the error UI by ~7s and multiplies request volume. Keep
  // retrying network failures (ApiError status 0), 5xx, and unknown errors up to 3 attempts.
  retry: (failureCount: number, error: unknown) => {
    if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
      return false
    }
    return failureCount < 3
  },
  retryDelay: (attemptIndex: number) => Math.min(1000 * 2 ** attemptIndex, 30000),
}
