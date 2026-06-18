/**
 * AIDEV-NOTE: Acknowledge group mutation hook
 * POSTs ack requests to mark all unhealthy jobs in a group as acknowledged
 * Uses optimistic updates for snappy UI feedback
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ackGroup, type AckGroupResponse } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'
import type { Job, JobsResponse, GroupWithHealth, HealthSummary } from '@/types'

// AIDEV-NOTE: Statuses that can be acknowledged
const ACKABLE_STATUSES = ['error', 'timeout', 'stale']

export function useAckGroup() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (groupName: string) => ackGroup(groupName),
    // AIDEV-NOTE: Optimistic update - immediately show group jobs as acked
    onMutate: async (groupName: string) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: queryKeys.jobs })
      await queryClient.cancelQueries({ queryKey: queryKeys.groups })
      await queryClient.cancelQueries({ queryKey: queryKeys.health })
      await queryClient.cancelQueries({ queryKey: queryKeys.groupJobs(groupName) })

      // Get group's unhealthy count for optimistic health update
      const groups = queryClient.getQueryData<GroupWithHealth[]>(queryKeys.groups)
      const group = groups?.find((g) => g.name === groupName)
      const ackedCount = group?.unhealthy_count ?? 0

      // Optimistically update group jobs
      queryClient.setQueryData<Job[]>(queryKeys.groupJobs(groupName), (old) => {
        if (!old) return old
        return old.map((job) => {
          if (ACKABLE_STATUSES.includes(job.status) && !job.acked) {
            return { ...job, acked: true, acked_at: new Date().toISOString() }
          }
          return job
        })
      })

      // Optimistically update jobs in all cached queries
      queryClient.setQueriesData<JobsResponse>({ queryKey: queryKeys.jobs }, (old) => {
        // AIDEV-NOTE: Guard against prefix-key collision — queryKeys.jobs (['jobs']) also
        // matches jobLog caches (['jobs','log',...]) whose value has no `jobs` array.
        if (!old || !Array.isArray(old.jobs)) return old
        return {
          ...old,
          jobs: old.jobs.map((job) => {
            if (job.group_name === groupName && ACKABLE_STATUSES.includes(job.status) && !job.acked) {
              return { ...job, acked: true, acked_at: new Date().toISOString() }
            }
            return job
          }),
        }
      })

      // Update groups list optimistically
      queryClient.setQueryData<GroupWithHealth[]>(queryKeys.groups, (old) => {
        if (!old) return old
        return old.map((g) => {
          if (g.name === groupName) {
            return {
              ...g,
              unhealthy_count: 0,
              acked_count: g.acked_count + g.unhealthy_count,
              health: 'healthy' as const,
            }
          }
          return g
        })
      })

      // Update health summary optimistically
      if (ackedCount > 0) {
        queryClient.setQueryData<HealthSummary>(queryKeys.health, (old) => {
          if (!old) return old
          const newUnhealthy = Math.max(0, old.unhealthy - ackedCount)
          return {
            ...old,
            unhealthy: newUnhealthy,
            acked: old.acked + ackedCount,
            // AIDEV-NOTE: Don't flash 'healthy' while in-progress jobs remain —
            // fall to 'in_progress' (backend precedence unhealthy > in_progress > healthy).
            status:
              newUnhealthy <= 0
                ? old.in_progress > 0
                  ? 'in_progress'
                  : 'healthy'
                : old.status,
          }
        })
      }

      return { groupName }
    },
    // AIDEV-NOTE: Use mutation variable (groupName) for cache invalidation, not server response.
    // Server may normalize the group name (e.g., lowercase), which could cause cache mismatch.
    // Use response.group only for display purposes as fallback.
    onSuccess: (response: AckGroupResponse, groupName: string) => {
      const { acked_count, group } = response
      const displayGroup = group || groupName
      // Invalidate to sync with server state
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(groupName) })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })

      if (acked_count > 0) {
        showSuccessToast(
          'Acknowledged',
          `${acked_count} job${acked_count !== 1 ? 's' : ''} acknowledged in "${displayGroup}".`
        )
      } else {
        showSuccessToast('No jobs to acknowledge', `All jobs in "${displayGroup}" are already acknowledged.`)
      }
    },
    onError: (error: Error, _groupName, context) => {
      // AIDEV-NOTE: On error, invalidate queries to refetch from server
      // rather than restoring snapshots which could clobber concurrent updates
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (context?.groupName) {
        queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(context.groupName) })
      }
      showErrorToast('Failed to acknowledge group', error.message)
    },
  })
}
