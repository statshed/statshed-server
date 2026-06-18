/**
 * AIDEV-NOTE: Unified API exports
 * All API functions and types are re-exported from here
 */

// Re-export all types
export * from './types'

// Re-export API functions
export { getHealth } from './health'
export { getGroups, getGroupJobs, getGroupConfig, updateGroupConfig } from './groups'
export {
  submitStatus,
  getJobsByStatus,
  ackJob,
  ackGroup,
  ackAll,
  deleteJob,
  getJobLog,
  type AckJobResponse,
  type AckGroupResponse,
  type AckAllResponse,
  type DeleteJobResponse,
  type GetJobLogOptions,
} from './jobs'
export { getConfig, updateConfig } from './config'

// Re-export client utilities
export { api, apiRequest, ApiError } from './client'
