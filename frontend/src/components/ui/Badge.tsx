/**
 * AIDEV-NOTE: Badge component
 * Used for status indicators and labels
 * Variants match job status colors
 */

import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import type { JobStatus } from '@/types'
import { JOB_STATUS_COLORS, JOB_STATUS_LABELS } from '@/lib/constants'

type BadgeVariant = 'success' | 'error' | 'warning' | 'progress' | 'neutral'

interface BadgeProps {
  variant?: BadgeVariant
  children: ReactNode
  className?: string
}

const variantClasses: Record<BadgeVariant, string> = {
  success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  warning: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  neutral: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
}

export function Badge({ variant = 'neutral', children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
        variantClasses[variant],
        className
      )}
    >
      {children}
    </span>
  )
}

// Convenience component for job status badges
interface JobStatusBadgeProps {
  status: JobStatus
  className?: string
}

export function JobStatusBadge({ status, className }: JobStatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
        JOB_STATUS_COLORS[status],
        className
      )}
    >
      {JOB_STATUS_LABELS[status]}
    </span>
  )
}
