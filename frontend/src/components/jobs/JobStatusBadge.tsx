/**
 * AIDEV-NOTE: Job status badge component
 * Colored badge for each job status
 * AIDEV-NOTE: Badge stays solid (no fade) - only the parent card fades
 */

import { cn } from '@/lib/utils'
import type { JobStatus } from '@/types'
import { JOB_STATUS_COLORS, JOB_STATUS_LABELS } from '@/lib/constants'

interface JobStatusBadgeProps {
  status: JobStatus
  acked?: boolean
  /** Whether the job is in the expiring fade window */
  isExpiring?: boolean
  className?: string
}

export default function JobStatusBadge({
  status,
  acked = false,
  isExpiring = false,
  className,
}: JobStatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
        JOB_STATUS_COLORS[status],
        status === 'progress' && 'animate-pulse',
        // AIDEV-NOTE: acked is signalled by line-through + the " (acked)" suffix below — NOT by
        // dimming. opacity here would drop the small text-xs chip below WCAG AA; keep it opaque.
        acked && 'line-through',
        // AIDEV-NOTE: Subtle visual indicator for expiring jobs - dashed border
        isExpiring && !acked && 'ring-1 ring-amber-400/50 dark:ring-amber-500/50',
        className
      )}
    >
      {JOB_STATUS_LABELS[status]}
      {acked && ' (acked)'}
    </span>
  )
}
