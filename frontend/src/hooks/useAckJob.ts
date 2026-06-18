/**
 * AIDEV-NOTE: Acknowledge job mutation hook
 * POSTs ack requests to mark jobs as acknowledged
 * Uses optimistic updates for snappy UI feedback
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ackJob, type AckJobResponse } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'
import type { Job, JobsResponse, GroupWithHealth, HealthSummary } from '@/types'

// AIDEV-NOTE: Statuses that can be acknowledged
const ACKABLE_STATUSES = ['error', 'timeout', 'stale']

// AIDEV-NOTE: Helper to update a job's acked status in cached job arrays
function updateJobInCache(jobs: Job[], jobId: number, acked: boolean): Job[] {
  return jobs.map((job) =>
    job.id === jobId ? { ...job, acked, acked_at: acked ? new Date().toISOString() : null } : job
  )
}

export function useAckJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (jobId: number) => ackJob(jobId),
    // AIDEV-NOTE: Optimistic update - immediately show job as acked
    onMutate: async (jobId: number) => {
      // Cancel any outgoing refetches to avoid overwriting optimistic update
      await queryClient.cancelQueries({ queryKey: queryKeys.jobs })
      await queryClient.cancelQueries({ queryKey: queryKeys.groups })
      await queryClient.cancelQueries({ queryKey: queryKeys.health })

      // AIDEV-NOTE: Find the job in main jobs cache or group-specific caches
      // This ensures optimistic updates work on group detail pages too
      let jobGroupName: string | undefined
      let jobStatus: string | undefined

      // First try main jobs cache
      const jobsQueries = queryClient.getQueriesData<JobsResponse>({
        queryKey: queryKeys.jobs,
      })
      for (const [, data] of jobsQueries) {
        const foundJob = data?.jobs?.find((j) => j.id === jobId)
        if (foundJob) {
          jobGroupName = foundJob.group_name
          jobStatus = foundJob.status
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
            break
          }
        }
      }

      // Optimistically update jobs in all cached queries
      queryClient.setQueriesData<JobsResponse>({ queryKey: queryKeys.jobs }, (old) => {
        // AIDEV-NOTE: Guard against prefix-key collision — queryKeys.jobs (['jobs']) also
        // matches jobLog caches (['jobs','log',...]) whose value has no `jobs` array.
        if (!old || !Array.isArray(old.jobs)) return old
        return { ...old, jobs: updateJobInCache(old.jobs, jobId, true) }
      })

      // Update group jobs cache if we know the group
      if (jobGroupName) {
        queryClient.setQueryData<Job[]>(queryKeys.groupJobs(jobGroupName), (old) => {
          if (!old) return old
          return updateJobInCache(old, jobId, true)
        })

        // Update groups list optimistically
        queryClient.setQueryData<GroupWithHealth[]>(queryKeys.groups, (old) => {
          if (!old) return old
          return old.map((group) => {
            if (group.name === jobGroupName) {
              const isUnhealthy = ACKABLE_STATUSES.includes(jobStatus || '')
              return {
                ...group,
                unhealthy_count: isUnhealthy ? Math.max(0, group.unhealthy_count - 1) : group.unhealthy_count,
                acked_count: isUnhealthy ? group.acked_count + 1 : group.acked_count,
              }
            }
            return group
          })
        })
      }

      // Update health summary optimistically
      if (jobStatus && ACKABLE_STATUSES.includes(jobStatus)) {
        queryClient.setQueryData<HealthSummary>(queryKeys.health, (old) => {
          if (!old) return old
          return {
            ...old,
            unhealthy: Math.max(0, old.unhealthy - 1),
            acked: old.acked + 1,
            // AIDEV-NOTE: When the last unhealthy job is acked, fall to 'in_progress'
            // if any in-progress jobs remain (don't flash 'healthy') — mirrors the
            // backend precedence unhealthy > in_progress > healthy.
            status:
              old.unhealthy - 1 <= 0
                ? old.in_progress > 0
                  ? 'in_progress'
                  : 'healthy'
                : old.status,
          }
        })
      }

      return { jobGroupName }
    },
    onSuccess: (response: AckJobResponse) => {
      const { job } = response
      showSuccessToast('Acknowledged', `Job "${job.name}" acknowledged.`)
      // Invalidate to ensure server state is synced
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(job.group_name) })
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
      showErrorToast('Failed to acknowledge job', error.message)
    },
  })
}
