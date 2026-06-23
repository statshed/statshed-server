/**
 * AIDEV-NOTE: Group card component
 * Displays group name, job count, and mini status indicators
 * Includes an "Ack All" button when there are unacked errors
 */

import { ChevronRight, Check } from 'lucide-react'
import { Card, CardBody, Button } from '@/components/ui'
import type { GroupWithHealth, JobStatus } from '@/types'
import { JOB_STATUS_COLORS } from '@/lib/constants'
import { cn } from '@/lib/utils'
import { useAckGroup } from '@/hooks'

interface GroupCardProps {
  group: GroupWithHealth
}

// Mini badge for job count by status
function MiniStatusBadge({
  status,
  count,
}: {
  status: JobStatus
  count: number
}) {
  if (count === 0) return null

  return (
    <span
      className={cn(
        'inline-flex items-center justify-center min-w-[1.25rem] h-5 px-1.5 rounded-full text-xs font-medium',
        JOB_STATUS_COLORS[status]
      )}
      title={`${count} ${status}`}
    >
      {count}
    </span>
  )
}

export default function GroupCard({ group }: GroupCardProps) {
  const ackGroupMutation = useAckGroup()

  // AIDEV-NOTE: Defensive defaults in case API returns null/undefined
  const jobCount = group.job_count ?? 0
  const statusCounts = group.status_counts ?? {}
  const unhealthyCount = group.unhealthy_count ?? 0
  const ackedCount = group.acked_count ?? 0

  const handleAckAll = (e: React.MouseEvent) => {
    e.preventDefault() // Prevent card navigation
    e.stopPropagation()
    ackGroupMutation.mutate(group.name)
  }

  return (
    <Card
      to={`/groups/${encodeURIComponent(group.name)}`}
      status={group.health}
      className="group"
      data-testid="group-card"
    >
      <CardBody>
        <div className="flex items-center justify-between">
          <div className="flex-1 min-w-0">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate">
              {group.name}
            </h3>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {jobCount} job{jobCount !== 1 ? 's' : ''}
              {ackedCount > 0 && (
                <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                  ({ackedCount} acked)
                </span>
              )}
            </p>
          </div>
          <ChevronRight className="w-5 h-5 text-gray-400 flex-shrink-0 transition-all duration-200 motion-safe:group-hover:translate-x-0.5 group-hover:text-primary-500" />
        </div>

        {/* Mini status indicators and ack button row */}
        <div className="flex items-center justify-between mt-3">
          {/* Mini status indicators */}
          {jobCount > 0 && (
            <div className="flex flex-wrap gap-1.5">
              <MiniStatusBadge status="success" count={statusCounts.success || 0} />
              <MiniStatusBadge status="error" count={statusCounts.error || 0} />
              <MiniStatusBadge status="progress" count={statusCounts.progress || 0} />
              <MiniStatusBadge status="timeout" count={statusCounts.timeout || 0} />
              <MiniStatusBadge status="stale" count={statusCounts.stale || 0} />
            </div>
          )}

          {/* Ack All button when there are unacked errors */}
          {unhealthyCount > 0 && (
            <Button
              size="sm"
              variant="ghost"
              onClick={handleAckAll}
              disabled={ackGroupMutation.isPending}
              className="text-xs text-gray-500 hover:text-green-600 dark:hover:text-green-400 ml-2 flex-shrink-0"
              title={`Acknowledge ${unhealthyCount} error${unhealthyCount !== 1 ? 's' : ''}`}
            >
              <Check className="w-3 h-3 mr-1" />
              Ack ({unhealthyCount})
            </Button>
          )}
        </div>
      </CardBody>
    </Card>
  )
}
