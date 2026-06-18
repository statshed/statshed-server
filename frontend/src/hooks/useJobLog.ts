/**
 * AIDEV-NOTE: Job log query hook
 * Fetches log content for a specific job with tail/all options
 */

import { useQuery } from '@tanstack/react-query'
import { getJobLog } from '@/api'
import { queryKeys } from '@/lib/constants'

interface UseJobLogOptions {
  /** Group name */
  groupName: string
  /** Job name */
  jobName: string
  /** If true, fetch the full log instead of last 1000 lines */
  all?: boolean
  /** Enable/disable the query (useful for modal open state) */
  enabled?: boolean
}

/**
 * Hook for fetching job log content
 * AIDEV-NOTE: Fetches last 1000 lines by default. Use all=true for full log.
 * The query is disabled by default until enabled=true is passed.
 *
 * @returns React Query result with log content and metadata
 */
export function useJobLog({ groupName, jobName, all = false, enabled = true }: UseJobLogOptions) {
  return useQuery({
    queryKey: queryKeys.jobLog(groupName, jobName, all),
    queryFn: () => getJobLog(groupName, jobName, { all }),
    enabled,
    staleTime: 30000, // 30 seconds
  })
}
