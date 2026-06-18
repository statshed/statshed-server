/**
 * AIDEV-NOTE: Global configuration API functions
 */

import { api } from './client'
import type { Config } from './types'

/**
 * Fetch global configuration
 * AIDEV-NOTE: Guard the shape so contract drift fails loudly rather than feeding
 * undefined timeouts into the settings form (matches groups.ts).
 */
export async function getConfig(): Promise<Config> {
  const data = await api.get<Config>('/config')
  if (
    !data ||
    typeof data.progress_timeout_minutes !== 'number' ||
    typeof data.staleness_timeout_hours !== 'number'
  ) {
    throw new Error('Invalid response from /config endpoint: expected timeout settings')
  }
  return data
}

/**
 * Update global configuration
 */
export function updateConfig(config: Partial<Config>): Promise<Config> {
  return api.put<Config>('/config', config)
}
