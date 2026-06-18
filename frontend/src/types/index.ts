/**
 * AIDEV-NOTE: Shared TypeScript types for StatShed frontend
 * These types mirror the backend API responses and are used throughout the application
 */

// Job status values
export type JobStatus = 'success' | 'error' | 'progress' | 'timeout' | 'stale'

// Health status values
export type HealthStatus = 'healthy' | 'unhealthy' | 'in_progress' | 'empty'

// Group from API
export interface Group {
  id: number
  name: string
  progress_timeout_minutes: number | null
  staleness_timeout_hours: number | null
  created_at: string
}

// Group with health summary (from /groups endpoint)
// AIDEV-NOTE: Backend returns {job_count, status_counts, health, unhealthy_count, acked_count}
// unhealthy_count excludes acked jobs; acked_count is count of acknowledged jobs in the group
export interface GroupWithHealth extends Group {
  job_count: number
  status_counts: Record<JobStatus, number>
  health: HealthStatus
  unhealthy_count: number
  acked_count: number
}

// Job from API
// AIDEV-NOTE: expires_at is computed from updated_at + group's expiration_timeout_hours
// Used for client-side fade calculation (no fade_percentage field from API)
// AIDEV-NOTE: Log fields (has_log, log_line_count, etc.) are included when a log file
// was attached to the status update via CLI submit --log
export interface Job {
  id: number
  group_id: number
  group_name: string
  name: string
  status: JobStatus
  message: string | null
  acked: boolean
  acked_at: string | null
  updated_at: string
  created_at: string
  expires_at: string | null
  // Log metadata (populated when log file attached to status update)
  has_log: boolean
  log_line_count: number | null
  log_truncated: boolean
  log_updated_at: string | null
}

// Global configuration
export interface Config {
  progress_timeout_minutes: number
  staleness_timeout_hours: number
}

// Group-specific configuration (with effective values)
// AIDEV-NOTE: staleness is opt-in (staleness_enabled), expiration always applies
// staleness_timeout_hours must be < expiration_timeout_hours when staleness is enabled
export interface GroupConfig {
  progress_timeout_minutes: number | null
  staleness_timeout_hours: number | null
  staleness_enabled: boolean
  expiration_timeout_hours: number
  effective_progress_timeout_minutes: number
  effective_staleness_timeout_hours: number | null
  effective_expiration_timeout_hours: number
}

// Health summary from /health endpoint
// AIDEV-NOTE: Backend returns {status, total_jobs, unhealthy, acked, healthy, in_progress, by_status} format
// unhealthy excludes acked jobs; acked is count of acknowledged jobs
export interface HealthSummary {
  status: HealthStatus
  total_jobs: number
  unhealthy: number
  acked: number
  healthy: number
  in_progress: number
  by_status: Partial<Record<JobStatus, number>>
}

// Status update payload (for POST /status)
export interface StatusUpdatePayload {
  group: string
  job: string
  status: JobStatus
  message?: string
}

// API response wrapper
export interface ApiResponse<T> {
  data: T
  error?: string
}

// Jobs listing response (from GET /jobs endpoint)
export interface JobsResponse {
  jobs: Job[]
  total: number
}

// WebSocket event payloads
export interface StatusUpdateEvent {
  job: Job
  group_name: string
}

export interface GroupCreatedEvent {
  group: Group
}

export interface HealthUpdateEvent {
  health: HealthSummary
}

export interface JobsAckedEvent {
  job_ids: number[]
  group_id: number | null
  group_name: string | null
  acked_count: number
  timestamp: string
}

export interface JobDeletedEvent {
  job_id: number
  job_name: string
  group_id: number
  group_name: string
  timestamp: string
}

// AIDEV-NOTE: Emitted by backend when expiration processor auto-deletes jobs
export interface JobExpiredEvent {
  job_id: number
  group_name: string
}

// Log retrieval response from GET /groups/{name}/jobs/{job}/log
// AIDEV-NOTE: truncated indicates if tail was applied (default 1000 lines)
// Use all=true query param to get full log
export interface LogResponse {
  log: string
  line_count: number
  truncated: boolean
  total_line_count: number
}
