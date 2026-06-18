/**
 * AIDEV-NOTE: Group jobs query hook
 * Fetches jobs for a specific group
 */

import { useQuery } from '@tanstack/react-query'
import { getGroupJobs } from '@/api'
import { queryKeys } from '@/lib/constants'

export function useGroupJobs(groupName: string) {
  return useQuery({
    queryKey: queryKeys.groupJobs(groupName),
    queryFn: () => getGroupJobs(groupName),
    enabled: !!groupName,
    // AIDEV-NOTE: Poll as a fallback so the group detail view doesn't stay stale
    // indefinitely if socket events are missed (matches health/jobs).
    refetchInterval: 60000, // Refetch every minute
  })
}
