/**
 * AIDEV-NOTE: Groups query hook
 * Fetches list of all groups with health summary
 */

import { useQuery } from '@tanstack/react-query'
import { getGroups } from '@/api'
import { queryKeys } from '@/lib/constants'

export function useGroups() {
  return useQuery({
    queryKey: queryKeys.groups,
    queryFn: getGroups,
    // AIDEV-NOTE: Poll as a fallback so a wall-monitor dashboard that never loses focus
    // doesn't stay stale indefinitely if socket events are missed (matches health/jobs).
    refetchInterval: 60000, // Refetch every minute
  })
}
