/**
 * AIDEV-NOTE: Health data query hook
 * Fetches overall system health summary
 */

import { useQuery } from '@tanstack/react-query'
import { getHealth } from '@/api'
import { queryKeys } from '@/lib/constants'

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health,
    queryFn: getHealth,
    refetchInterval: 60000, // Refetch every minute
  })
}
