/**
 * AIDEV-NOTE: Health-related API functions
 */

import { api } from './client'
import type { HealthSummary } from './types'

/**
 * Fetch overall health summary
 * AIDEV-NOTE: Guard the response shape so backend contract drift (or a proxy error
 * page parsed as JSON) fails loudly instead of rendering undefined counts. Matches
 * the structural-guard approach in groups.ts.
 */
export async function getHealth(): Promise<HealthSummary> {
  const data = await api.get<HealthSummary>('/health')
  // AIDEV-NOTE: by_status must be a non-null object — HealthStats reads by_status.success
  // etc., and its `by_status = {}` destructuring default only covers `undefined`, not
  // `null`, so a null here would crash the dashboard. Fail loudly at the boundary instead.
  if (
    !data ||
    typeof data.status !== 'string' ||
    typeof data.total_jobs !== 'number' ||
    typeof data.by_status !== 'object' ||
    data.by_status === null
  ) {
    throw new Error('Invalid response from /health endpoint: expected a health summary')
  }
  return data
}
