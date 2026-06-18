/**
 * AIDEV-NOTE: Group-related API functions
 */

import { api } from './client'
import type { GroupWithHealth, Job, GroupConfig } from './types'

/**
 * Groups API response wrapper
 */
interface GroupsResponse {
  groups: GroupWithHealth[]
}

/**
 * Fetch all groups with health summary
 * AIDEV-NOTE: Backend returns {groups: [...]} so we unwrap it
 */
export async function getGroups(): Promise<GroupWithHealth[]> {
  const response = await api.get<GroupsResponse>('/groups')
  // AIDEV-NOTE: Validate response shape to avoid silent failures
  if (!response || !Array.isArray(response.groups)) {
    throw new Error('Invalid response from /groups endpoint: expected {groups: [...]}')
  }
  return response.groups
}

/**
 * Group jobs API response wrapper
 */
interface GroupJobsResponse {
  group: {
    id: number
    name: string
    progress_timeout_minutes: number | null
    staleness_timeout_hours: number | null
    created_at: string
  }
  jobs: Job[]
}

/**
 * Fetch jobs for a specific group
 * AIDEV-NOTE: Backend returns {group: {...}, jobs: [...]} so we unwrap it
 */
export async function getGroupJobs(groupName: string): Promise<Job[]> {
  const response = await api.get<GroupJobsResponse>(
    `/groups/${encodeURIComponent(groupName)}/jobs`
  )
  // AIDEV-NOTE: Validate response shape to avoid silent failures
  if (!response || !Array.isArray(response.jobs)) {
    throw new Error(`Invalid response from /groups/${groupName}/jobs endpoint: expected {jobs: [...]}`)
  }
  return response.jobs
}

/**
 * Fetch configuration for a specific group
 */
export function getGroupConfig(groupName: string): Promise<GroupConfig> {
  return api.get<GroupConfig>(`/groups/${encodeURIComponent(groupName)}/config`)
}

/**
 * Update configuration for a specific group
 * AIDEV-NOTE: Accepts progress_timeout_minutes, staleness_enabled, staleness_timeout_hours, expiration_timeout_hours
 * Backend validates staleness_timeout_hours < expiration_timeout_hours when staleness is enabled
 */
export function updateGroupConfig(
  groupName: string,
  config: Partial<Pick<GroupConfig, 'progress_timeout_minutes' | 'staleness_timeout_hours' | 'staleness_enabled' | 'expiration_timeout_hours'>>
): Promise<GroupConfig> {
  return api.put<GroupConfig>(
    `/groups/${encodeURIComponent(groupName)}/config`,
    config
  )
}
