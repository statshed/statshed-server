/**
 * AIDEV-NOTE: Acknowledge all jobs mutation hook
 * POSTs ack requests to mark all unhealthy jobs globally as acknowledged
 * Uses optimistic updates for snappy UI feedback
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ackAll, type AckAllResponse } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'
import type { Job, JobsResponse, GroupWithHealth, HealthSummary } from '@/types'


// AIDEV-NOTE: Statuses that can be acknowledged
const ACKABLE_STATUSES = ['error', 'timeout', 'stale']

export function useAckAll() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => ackAll(),
    // AIDEV-NOTE: Optimistic update - immediately show all unhealthy jobs as acked
    onMutate: async () => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: queryKeys.jobs })
      await queryClient.cancelQueries({ queryKey: queryKeys.groups })
      await queryClient.cancelQueries({ queryKey: queryKeys.health })

      // Get groups and health for optimistic updates
      const groups = queryClient.getQueryData<GroupWithHealth[]>(queryKeys.groups)
      const health = queryClient.getQueryData<HealthSummary>(queryKeys.health)
      const ackedCount = health?.unhealthy ?? 0

      // Optimistically update all jobs queries
      queryClient.setQueriesData<JobsResponse>({ queryKey: queryKeys.jobs }, (old) => {
        // AIDEV-NOTE: Guard against prefix-key collision — queryKeys.jobs (['jobs']) also
        // matches jobLog caches (['jobs','log',...]) whose value has no `jobs` array.
        if (!old || !Array.isArray(old.jobs)) return old
        return {
          ...old,
          jobs: old.jobs.map((job) => {
            if (ACKABLE_STATUSES.includes(job.status) && !job.acked) {
              return { ...job, acked: true, acked_at: new Date().toISOString() }
            }
            return job
          }),
        }
      })

      // AIDEV-NOTE: Update all groups optimistically - set all to healthy since this is global ack
      queryClient.setQueryData<GroupWithHealth[]>(queryKeys.groups, (old) => {
        if (!old) return old
        return old.map((group) => ({
          ...group,
          acked_count: group.acked_count + group.unhealthy_count,
          unhealthy_count: 0,
          // Set all groups to healthy since ack-all is global operation
          health: group.health === 'unhealthy' ? ('healthy' as const) : group.health,
        }))
      })

      // Update health summary optimistically
      queryClient.setQueryData<HealthSummary>(queryKeys.health, (old) => {
        if (!old) return old
        return {
          ...old,
          acked: old.acked + old.unhealthy,
          unhealthy: 0,
          // AIDEV-NOTE: ack-all clears unhealthy, but in-progress jobs keep the
          // overall status at 'in_progress' — don't optimistically flash 'healthy'.
          status: old.in_progress > 0 ? ('in_progress' as const) : ('healthy' as const),
        }
      })

      // Also update any cached group jobs
      if (groups) {
        for (const group of groups) {
          queryClient.setQueryData<Job[]>(queryKeys.groupJobs(group.name), (old) => {
            if (!old) return old
            return old.map((job) => {
              if (ACKABLE_STATUSES.includes(job.status) && !job.acked) {
                return { ...job, acked: true, acked_at: new Date().toISOString() }
              }
              return job
            })
          })
        }
      }

      return { ackedCount }
    },
    onSuccess: (response: AckAllResponse, _vars, context) => {
      const { acked_count } = response
      // Invalidate to sync with server state
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })

      // Use server response for accurate count, fallback to optimistic count
      const displayCount = acked_count ?? context?.ackedCount ?? 0
      if (displayCount > 0) {
        showSuccessToast('Acknowledged', `${displayCount} job${displayCount !== 1 ? 's' : ''} acknowledged.`)
      } else {
        showSuccessToast('No jobs to acknowledge', 'All jobs are already acknowledged.')
      }
    },
    onError: (error: Error) => {
      // AIDEV-NOTE: On error, invalidate queries to refetch from server
      // rather than restoring snapshots which could clobber concurrent updates
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      showErrorToast('Failed to acknowledge all jobs', error.message)
    },
  })
}
