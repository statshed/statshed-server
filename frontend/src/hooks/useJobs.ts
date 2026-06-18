/**
 * AIDEV-NOTE: Jobs query hook
 * Fetches jobs filtered by status for the Jobs page
 */

import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getJobsByStatus } from '@/api'
import { queryKeys } from '@/lib/constants'

/**
 * Hook to fetch jobs by status(es)
 *
 * @param statuses - Array of status values to filter by
 * @returns TanStack Query result with jobs data
 */
export function useJobsByStatus(statuses: string[]) {
  // Canonicalize statuses: sort and dedupe to ensure stable cache keys
  // AIDEV-NOTE: This prevents cache fragmentation when same statuses are
  // passed in different orders (e.g., ['error', 'timeout'] vs ['timeout', 'error'])
  const canonicalStatuses = useMemo(
    () => [...new Set(statuses)].sort(),
    [statuses]
  )

  return useQuery({
    queryKey: queryKeys.jobsByStatus(canonicalStatuses),
    queryFn: () => getJobsByStatus(canonicalStatuses),
    refetchInterval: 60000, // Refetch every minute
  })
}
