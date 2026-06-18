/**
 * AIDEV-NOTE: Config query and mutation hooks
 * Fetches and updates global configuration
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getConfig, updateConfig, type Config } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'

export function useConfig() {
  return useQuery({
    queryKey: queryKeys.config,
    queryFn: getConfig,
  })
}

export function useUpdateConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (config: Partial<Config>) => updateConfig(config),
    // Optimistic update: immediately update cache before server response
    onMutate: async (newConfig) => {
      // Cancel any outgoing refetches to avoid overwriting optimistic update
      await queryClient.cancelQueries({ queryKey: queryKeys.config })

      // Snapshot the previous value
      const previousConfig = queryClient.getQueryData<Config>(queryKeys.config)

      // Optimistically update to the new value
      if (previousConfig) {
        queryClient.setQueryData<Config>(queryKeys.config, {
          ...previousConfig,
          ...newConfig,
        })
      }

      // Return context with the previous value for rollback
      return { previousConfig }
    },
    onSuccess: () => {
      showSuccessToast('Configuration updated', 'Global settings have been saved.')
    },
    onError: (error: Error, _newConfig, context) => {
      // Rollback to previous value on error
      if (context?.previousConfig) {
        queryClient.setQueryData(queryKeys.config, context.previousConfig)
      }
      showErrorToast('Failed to update configuration', error.message)
    },
    onSettled: () => {
      // Always refetch after mutation to ensure cache is in sync
      queryClient.invalidateQueries({ queryKey: queryKeys.config })
    },
  })
}
