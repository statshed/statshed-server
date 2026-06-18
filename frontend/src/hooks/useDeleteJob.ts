/**
 * AIDEV-NOTE: Delete job mutation hook
 * DELETEs jobs to remove them permanently
 * Uses optimistic updates for snappy UI feedback
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteJob, type DeleteJobResponse } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'
import type { Job, JobsResponse, GroupWithHealth, HealthSummary } from '@/types'

// AIDEV-NOTE: Statuses that count as unhealthy
const UNHEALTHY_STATUSES = ['error', 'timeout', 'stale']

// AIDEV-NOTE: Helper to remove a job from cached job arrays
function removeJobFromCache(jobs: Job[], jobId: number): Job[] {
  return jobs.filter((job) => job.id !== jobId)
}

export function useDeleteJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (jobId: number) => deleteJob(jobId),
    // AIDEV-NOTE: Optimistic update - immediately remove job from caches
    onMutate: async (jobId: number) => {
      // Cancel any outgoing refetches to avoid overwriting optimistic update
      await queryClient.cancelQueries({ queryKey: queryKeys.jobs })
      await queryClient.cancelQueries({ queryKey: queryKeys.groups })
      await queryClient.cancelQueries({ queryKey: queryKeys.health })

      // AIDEV-NOTE: Find the job in main jobs cache or group-specific caches
      // This ensures optimistic updates work on group detail pages too
      let jobGroupName: string | undefined
      let jobStatus: string | undefined
      let jobAcked: boolean | undefined

      // First try main jobs cache
      const jobsQueries = queryClient.getQueriesData<JobsResponse>({
        queryKey: queryKeys.jobs,
      })
      for (const [, data] of jobsQueries) {
        const foundJob = data?.jobs?.find((j) => j.id === jobId)
        if (foundJob) {
          jobGroupName = foundJob.group_name
          jobStatus = foundJob.status
          jobAcked = foundJob.acked
          break
        }
      }

      // If not found in main cache, search group-specific caches
      if (!jobGroupName) {
        const groupJobsQueries = queryClient.getQueriesData<Job[]>({
          queryKey: ['groups'],
          predicate: (query) => query.queryKey.length === 3 && query.queryKey[2] === 'jobs',
        })
        for (const [, jobs] of groupJobsQueries) {
          const foundJob = jobs?.find((j) => j.id === jobId)
          if (foundJob) {
            jobGroupName = foundJob.group_name
            jobStatus = foundJob.status
            jobAcked = foundJob.acked
            break
          }
        }
      }

      // Optimistically remove job from all cached queries
      queryClient.setQueriesData<JobsResponse>({ queryKey: queryKeys.jobs }, (old) => {
        // AIDEV-NOTE: Guard against prefix-key collision — queryKeys.jobs (['jobs']) also
        // matches jobLog caches (['jobs','log',...]) whose value has no `jobs` array.
        if (!old || !Array.isArray(old.jobs)) return old
        const newJobs = removeJobFromCache(old.jobs, jobId)
        // Only decrement total if job was actually removed
        const removed = newJobs.length < old.jobs.length
        return { ...old, jobs: newJobs, total: removed ? Math.max(0, old.total - 1) : old.total }
      })

      // Update group jobs cache if we know the group
      if (jobGroupName) {
        queryClient.setQueryData<Job[]>(queryKeys.groupJobs(jobGroupName), (old) => {
          if (!old) return old
          return removeJobFromCache(old, jobId)
        })
      }

      // AIDEV-NOTE: Only update counts if we found the job in cache.
      // This prevents negative counts and "undefined" keys in status_counts.
      if (jobStatus && jobGroupName) {
        queryClient.setQueryData<GroupWithHealth[]>(queryKeys.groups, (old) => {
          if (!old) return old
          return old.map((group) => {
            if (group.name === jobGroupName) {
              const isUnhealthyUnacked = UNHEALTHY_STATUSES.includes(jobStatus) && !jobAcked
              const isAcked = jobAcked === true
              return {
                ...group,
                job_count: Math.max(0, group.job_count - 1),
                unhealthy_count: isUnhealthyUnacked
                  ? Math.max(0, group.unhealthy_count - 1)
                  : group.unhealthy_count,
                acked_count: isAcked ? Math.max(0, group.acked_count - 1) : group.acked_count,
                status_counts: {
                  ...group.status_counts,
                  [jobStatus]: Math.max(
                    0,
                    (group.status_counts[jobStatus as keyof typeof group.status_counts] || 0) - 1
                  ),
                },
              }
            }
            return group
          })
        })
      }

      // Update health summary optimistically (only if job was found)
      if (jobStatus) {
        queryClient.setQueryData<HealthSummary>(queryKeys.health, (old) => {
          if (!old) return old
          const isUnhealthyUnacked = UNHEALTHY_STATUSES.includes(jobStatus) && !jobAcked
          const isAcked = jobAcked === true
          return {
            ...old,
            total_jobs: Math.max(0, old.total_jobs - 1),
            unhealthy: isUnhealthyUnacked ? Math.max(0, old.unhealthy - 1) : old.unhealthy,
            acked: isAcked ? Math.max(0, old.acked - 1) : old.acked,
            healthy: jobStatus === 'success' ? Math.max(0, old.healthy - 1) : old.healthy,
            in_progress: jobStatus === 'progress' ? Math.max(0, old.in_progress - 1) : old.in_progress,
            by_status: {
              ...old.by_status,
              [jobStatus]: Math.max(
                0,
                (old.by_status[jobStatus as keyof typeof old.by_status] || 0) - 1
              ),
            },
          }
        })
      }

      return { jobGroupName }
    },
    onSuccess: (response: DeleteJobResponse) => {
      const { deleted_job, group_name } = response
      showSuccessToast('Deleted', `Job "${deleted_job.name}" deleted.`)
      // Invalidate to ensure server state is synced
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(group_name) })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
    },
    onError: (error: Error, _jobId, context) => {
      // AIDEV-NOTE: On error, invalidate queries to refetch from server
      // rather than restoring snapshots which could clobber concurrent updates
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (context?.jobGroupName) {
        queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(context.jobGroupName) })
      }
      showErrorToast('Failed to delete job', error.message)
    },
  })
}
