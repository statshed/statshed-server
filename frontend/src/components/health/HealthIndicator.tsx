/**
 * AIDEV-NOTE: Health indicator component
 * Displays pulsing dot with status message
 */

import { cn } from '@/lib/utils'
import type { HealthStatus } from '@/types'
import {
  HEALTH_STATUS_BG_COLORS,
  HEALTH_STATUS_LABELS,
} from '@/lib/constants'

interface HealthIndicatorProps {
  status: HealthStatus
  className?: string
}

export default function HealthIndicator({
  status,
  className,
}: HealthIndicatorProps) {
  const isAnimated = status === 'in_progress'

  return (
    <div className={cn('flex items-center gap-2', className)}>
      <span className="relative flex h-3 w-3">
        {isAnimated && (
          <span
            className={cn(
              'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75',
              HEALTH_STATUS_BG_COLORS[status]
            )}
          />
        )}
        <span
          className={cn(
            'relative inline-flex rounded-full h-3 w-3',
            HEALTH_STATUS_BG_COLORS[status]
          )}
        />
      </span>
      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
        {HEALTH_STATUS_LABELS[status]}
      </span>
    </div>
  )
}
