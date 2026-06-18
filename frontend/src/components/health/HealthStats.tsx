/**
 * AIDEV-NOTE: Health stats component
 * Displays metric cards for overall health summary
 * Healthy, Errors, and In Progress cards are clickable and navigate to /jobs with status filter
 * Total Jobs card is not clickable
 */

import { Link } from 'react-router-dom'
import { CheckCircle, XCircle, Loader2, Layers, Check } from 'lucide-react'
import type { HealthSummary } from '@/types'
import { Skeleton, Button } from '@/components/ui'
import { cn } from '@/lib/utils'
import { useAckAll } from '@/hooks'

interface HealthStatsProps {
  health?: HealthSummary
  isLoading?: boolean
}

interface StatCardProps {
  label: string
  value: number
  icon: React.ReactNode
  color: string
  /** If provided, the card becomes a clickable link */
  href?: string
  /** Optional subtitle to show below the label */
  subtitle?: string
}

function StatCard({ label, value, icon, color, href, subtitle }: StatCardProps) {
  const cardContent = (
    <div className="flex items-center gap-3">
      <div className={`p-2 rounded-lg ${color}`}>{icon}</div>
      <div>
        <p className="text-2xl font-semibold text-gray-900 dark:text-white">
          {value}
        </p>
        <p className="text-sm text-gray-500 dark:text-gray-400">{label}</p>
        {subtitle && (
          <p className="text-xs text-gray-400 dark:text-gray-500">{subtitle}</p>
        )}
      </div>
    </div>
  )

  const baseClasses = 'bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4'
  const clickableClasses = 'hover:border-primary-300 dark:hover:border-primary-600 hover:shadow-sm transition-all cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900'

  if (href) {
    return (
      <Link
        to={href}
        className={cn(baseClasses, clickableClasses, 'block')}
        aria-label={`View ${label.toLowerCase()}`}
      >
        {cardContent}
      </Link>
    )
  }

  return (
    <div className={baseClasses}>
      {cardContent}
    </div>
  )
}

function StatCardSkeleton() {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
      <div className="flex items-center gap-3">
        <Skeleton className="w-10 h-10 rounded-lg" />
        <div className="space-y-2">
          <Skeleton className="h-7 w-12" />
          <Skeleton className="h-4 w-20" />
        </div>
      </div>
    </div>
  )
}

export default function HealthStats({ health, isLoading }: HealthStatsProps) {
  const ackAllMutation = useAckAll()

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCardSkeleton />
        <StatCardSkeleton />
        <StatCardSkeleton />
        <StatCardSkeleton />
      </div>
    )
  }

  if (!health) {
    return null
  }

  // AIDEV-NOTE: Backend returns {status, total_jobs, unhealthy, acked, healthy, in_progress, by_status} format
  // unhealthy excludes acked jobs; acked is the count of acknowledged jobs
  // Defensive defaults in case API returns null/undefined
  const { total_jobs = 0, by_status = {}, unhealthy = 0, acked = 0 } = health

  const handleAckAll = () => {
    ackAllMutation.mutate()
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          label="Total Jobs"
          value={total_jobs}
          icon={<Layers className="w-5 h-5 text-gray-600" />}
          color="bg-gray-100 dark:bg-gray-700"
        />
        <StatCard
          label="Healthy"
          value={by_status.success || 0}
          icon={<CheckCircle className="w-5 h-5 text-green-600" />}
          color="bg-green-100 dark:bg-green-900/30"
          href="/jobs?status=success"
        />
        <StatCard
          label="Errors"
          value={unhealthy}
          icon={<XCircle className="w-5 h-5 text-red-600" />}
          color="bg-red-100 dark:bg-red-900/30"
          href="/jobs?status=error,timeout,stale"
          subtitle={acked > 0 ? `${acked} acked` : undefined}
        />
        <StatCard
          label="In Progress"
          value={by_status.progress || 0}
          icon={<Loader2 className="w-5 h-5 text-blue-600" />}
          color="bg-blue-100 dark:bg-blue-900/30"
          href="/jobs?status=progress"
        />
      </div>

      {/* Global Ack All Errors button */}
      {unhealthy > 0 && (
        <div className="flex justify-end">
          <Button
            size="sm"
            variant="secondary"
            onClick={handleAckAll}
            disabled={ackAllMutation.isPending}
            className="text-gray-600 hover:text-green-600 dark:text-gray-400 dark:hover:text-green-400"
          >
            <Check className="w-4 h-4 mr-1.5" />
            Ack All Errors ({unhealthy})
          </Button>
        </div>
      )}
    </div>
  )
}
