/**
 * AIDEV-NOTE: Job-related API functions
 */

import { api } from './client'
import type { Job, JobsResponse, StatusUpdatePayload, LogResponse } from './types'

/**
 * Submit a job status update
 */
export function submitStatus(payload: StatusUpdatePayload): Promise<Job> {
  return api.post<Job>('/status', payload)
}

/**
 * Get jobs filtered by status(es)
 * AIDEV-NOTE: Used by health card click-through to show jobs with specific statuses
 *
 * @param statuses - Array of status values to filter by (e.g., ['success'] or ['error', 'timeout'])
 * @returns Promise with jobs array and total count
 */
export async function getJobsByStatus(statuses: string[]): Promise<JobsResponse> {
  // URL-encode each status value to handle special characters safely
  const url =
    statuses.length === 0
      ? '/jobs'
      : `/jobs?status=${statuses.map((s) => encodeURIComponent(s)).join(',')}`
  const data = await api.get<JobsResponse>(url)
  // AIDEV-NOTE: Guard the shape so contract drift fails loudly (matches groups.ts).
  if (!data || !Array.isArray(data.jobs) || typeof data.total !== 'number') {
    throw new Error('Invalid response from /jobs endpoint: expected {jobs: [...], total}')
  }
  return data
}

// Response type for single job ack
export interface AckJobResponse {
  job: Job
}

/**
 * Acknowledge a single job (mark it as acked)
 * AIDEV-NOTE: Only jobs with error/timeout/stale status can be acked
 *
 * @param jobId - The ID of the job to acknowledge
 * @returns Promise with the updated job
 */
export function ackJob(jobId: number): Promise<AckJobResponse> {
  return api.post<AckJobResponse>(`/jobs/${jobId}/ack`)
}

// Response type for bulk ack operations
export interface AckGroupResponse {
  acked_count: number
  group: string
}

export interface AckAllResponse {
  acked_count: number
}

/**
 * Acknowledge all errored jobs in a group
 * AIDEV-NOTE: Acks all jobs with error/timeout/stale status that are not already acked
 *
 * @param groupName - The name of the group
 * @returns Promise with the count of acked jobs
 */
export function ackGroup(groupName: string): Promise<AckGroupResponse> {
  return api.post<AckGroupResponse>(`/groups/${encodeURIComponent(groupName)}/ack`)
}

/**
 * Acknowledge all errored jobs globally
 * AIDEV-NOTE: Acks all jobs with error/timeout/stale status that are not already acked
 *
 * @returns Promise with the count of acked jobs
 */
export function ackAll(): Promise<AckAllResponse> {
  return api.post<AckAllResponse>('/ack-all')
}

// Response type for delete job
export interface DeleteJobResponse {
  deleted_job: Job
  group_id: number
  group_name: string
}

/**
 * Delete a single job
 * AIDEV-NOTE: Permanently removes the job from the database
 *
 * @param jobId - The ID of the job to delete
 * @returns Promise with the deleted job data
 */
export function deleteJob(jobId: number): Promise<DeleteJobResponse> {
  return api.delete<DeleteJobResponse>(`/jobs/${jobId}`)
}

// Options for log retrieval
export interface GetJobLogOptions {
  /** Number of lines from the end (default: 1000) */
  tail?: number
  /** If true, return the full log (ignores tail) */
  all?: boolean
}

/**
 * Retrieve log content for a job
 * AIDEV-NOTE: Returns last 1000 lines by default. Use all=true for full log.
 * Returns 404 if job or log doesn't exist.
 *
 * @param groupName - The name of the group
 * @param jobName - The name of the job
 * @param options - Optional parameters for tail/all
 * @returns Promise with log content and metadata
 */
export async function getJobLog(
  groupName: string,
  jobName: string,
  options?: GetJobLogOptions
): Promise<LogResponse> {
  const params = new URLSearchParams()
  if (options?.all) {
    params.set('all', 'true')
  } else if (options?.tail !== undefined) {
    params.set('tail', options.tail.toString())
  }

  const queryString = params.toString()
  const url = `/groups/${encodeURIComponent(groupName)}/jobs/${encodeURIComponent(jobName)}/log${queryString ? `?${queryString}` : ''}`
  const data = await api.get<LogResponse>(url)
  // AIDEV-NOTE: Guard all required fields — LogViewerModal keys "Show all"/truncation
  // off `truncated` and `total_line_count`, so validate them too, not just `log`.
  if (
    !data ||
    typeof data.log !== 'string' ||
    typeof data.line_count !== 'number' ||
    typeof data.truncated !== 'boolean' ||
    typeof data.total_line_count !== 'number'
  ) {
    throw new Error('Invalid response from job log endpoint: expected log content')
  }
  return data
}
