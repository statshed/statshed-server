/**
 * AIDEV-NOTE: Submit status mutation hook
 * POSTs job status updates to the backend
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { submitStatus, type StatusUpdatePayload } from '@/api'
import { queryKeys } from '@/lib/constants'
import { showSuccessToast, showErrorToast } from '@/contexts/ToastContext'

export function useSubmitStatus() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: StatusUpdatePayload) => submitStatus(payload),
    onSuccess: (job) => {
      // Invalidate relevant queries
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(job.group_name) })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      showSuccessToast('Status updated', `Job "${job.name}" in group "${job.group_name}" updated.`)
    },
    onError: (error: Error) => {
      showErrorToast('Failed to submit status', error.message)
    },
  })
}
