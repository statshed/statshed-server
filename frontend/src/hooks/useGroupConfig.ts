/**
 * AIDEV-NOTE: Group config query and mutation hooks
 * Fetches and updates configuration for a specific group
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getGroupConfig, updateGroupConfig, type GroupConfig } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'

export function useGroupConfig(groupName: string) {
  return useQuery({
    queryKey: queryKeys.groupConfig(groupName),
    queryFn: () => getGroupConfig(groupName),
    enabled: !!groupName,
  })
}

export function useUpdateGroupConfig(groupName: string) {
  const queryClient = useQueryClient()

  return useMutation({
    // AIDEV-NOTE: Now includes staleness_enabled and expiration_timeout_hours for expiring status feature
    mutationFn: (
      config: Partial<Pick<GroupConfig, 'progress_timeout_minutes' | 'staleness_timeout_hours' | 'staleness_enabled' | 'expiration_timeout_hours'>>
    ) => updateGroupConfig(groupName, config),
    // Optimistic update: immediately update cache before server response
    onMutate: async (newConfig) => {
      const queryKey = queryKeys.groupConfig(groupName)

      // Cancel any outgoing refetches to avoid overwriting optimistic update
      await queryClient.cancelQueries({ queryKey })

      // Snapshot the previous value
      const previousConfig = queryClient.getQueryData<GroupConfig>(queryKey)

      // Optimistically update the override values only
      // AIDEV-NOTE: We intentionally do NOT update effective_* values optimistically
      // because they depend on global config which we can't accurately derive client-side.
      // The effective values will be corrected when the refetch completes in onSettled.
      if (previousConfig) {
        queryClient.setQueryData<GroupConfig>(queryKey, {
          ...previousConfig,
          ...newConfig,
        })
      }

      // Return context with the previous value for rollback
      return { previousConfig }
    },
    onSuccess: () => {
      showSuccessToast('Group configuration updated', `Settings for ${groupName} have been saved.`)
    },
    onError: (error: Error, _newConfig, context) => {
      // Rollback to previous value on error
      if (context?.previousConfig) {
        queryClient.setQueryData(queryKeys.groupConfig(groupName), context.previousConfig)
      }
      showErrorToast('Failed to update group configuration', error.message)
    },
    onSettled: () => {
      // Always refetch after mutation to ensure cache is in sync
      queryClient.invalidateQueries({ queryKey: queryKeys.groupConfig(groupName) })
      // Also invalidate group jobs since timeouts may affect status
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(groupName) })
    },
  })
}
