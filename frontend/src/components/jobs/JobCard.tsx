/**
 * AIDEV-NOTE: Job card component
 * Displays job name, status badge, message, and timestamp
 * Optionally shows group name with link when showGroup is true
 * Includes ack button for error/timeout/stale jobs and delete button
 * AIDEV-NOTE: An expiring/acked card RECEDES via a page-colored scrim over its chrome (not
 * whole-card opacity), so the text + status badge stay readable (WCAG AA). See below.
 * AIDEV-NOTE: Shows "View logs" button when job has attached log file
 */

import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Trash2, Clock, FileText } from 'lucide-react'
import JobStatusBadge from './JobStatusBadge'
import LogViewerModal from './LogViewerModal'
import { Card, CardBody, Button, Dialog } from '@/components/ui'
import { cn } from '@/lib/utils'
import type { Job, JobStatus } from '@/types'
import { formatRelativeTime } from '@/lib/utils'
import { useAckJob, useDeleteJob, useFadePercentage } from '@/hooks'

// AIDEV-NOTE: Statuses that can be acknowledged (unhealthy states)
const ACKABLE_STATUSES: JobStatus[] = ['error', 'timeout', 'stale']

interface JobCardProps {
  job: Job
  /** Show the group name as a link (useful when displaying jobs from multiple groups) */
  showGroup?: boolean
}

export default function JobCard({ job, showGroup = false }: JobCardProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showLogViewer, setShowLogViewer] = useState(false)
  const ackMutation = useAckJob()
  const deleteMutation = useDeleteJob()
  const { opacity, isFading, timeRemainingText, isValid: isExpirationValid } = useFadePercentage(
    job.expires_at,
    job.updated_at
  )

  const isAckable = ACKABLE_STATUSES.includes(job.status) && !job.acked
  const isAcked = job.acked

  const handleAck = () => {
    ackMutation.mutate(job.id)
  }

  const handleDelete = () => {
    deleteMutation.mutate(job.id, {
      onSuccess: () => {
        setShowDeleteConfirm(false)
      },
    })
  }

  // AIDEV-NOTE: De-emphasis (expiring + acked) is painted as a page-colored SCRIM over the
  // card chrome, NOT as opacity on the whole card. Whole-card opacity dimmed the text and
  // status badge below WCAG AA; a scrim tints only the background so the card visibly recedes
  // while its foreground stays fully readable. `opacity` (1.0→0.5, from useFadePercentage)
  // becomes scrim alpha = 1 - opacity; acked adds a fixed recede; the two take the max.
  // The scrim is clipped to the card's rounded corners by the Card's `overflow-hidden`.
  const fadeScrim = isFading ? 1 - opacity : 0
  const ackedScrim = isAcked ? 0.45 : 0
  const scrimOpacity = Math.max(fadeScrim, ackedScrim)

  return (
    <Card>
      {scrimOpacity > 0 && (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-0 bg-gray-50 dark:bg-gray-900"
          style={{ opacity: scrimOpacity }}
        />
      )}
      <div className="relative z-10">
      <CardBody className="py-3">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3
                className={cn(
                  'font-medium text-gray-900 dark:text-white truncate',
                  isAcked && 'line-through'
                )}
              >
                {job.name}
              </h3>
              <JobStatusBadge status={job.status} acked={job.acked} isExpiring={isFading} />
              {isAckable && (
                <button
                  onClick={handleAck}
                  disabled={ackMutation.isPending}
                  className={cn(
                    'inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded',
                    'text-gray-500 hover:text-green-600 hover:bg-green-50',
                    'dark:hover:text-green-400 dark:hover:bg-green-900/30',
                    'transition-colors',
                    ackMutation.isPending && 'opacity-50 cursor-not-allowed'
                  )}
                  title="Acknowledge this error"
                >
                  <Check className="w-3 h-3" />
                  Ack
                </button>
              )}
              <button
                onClick={() => setShowDeleteConfirm(true)}
                disabled={deleteMutation.isPending || showDeleteConfirm}
                className={cn(
                  'inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded',
                  'text-gray-400 hover:text-red-600 hover:bg-red-50',
                  'dark:hover:text-red-400 dark:hover:bg-red-900/30',
                  'transition-colors',
                  (deleteMutation.isPending || showDeleteConfirm) && 'opacity-50 cursor-not-allowed'
                )}
                title="Delete this job"
              >
                <Trash2 className="w-3 h-3" />
              </button>
              {/* AIDEV-NOTE: View logs button shown when job has attached log file */}
              {job.has_log && (
                <button
                  onClick={() => setShowLogViewer(true)}
                  className={cn(
                    'inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded',
                    'text-primary-600 hover:text-primary-700 hover:bg-primary-50',
                    'dark:text-primary-400 dark:hover:text-primary-300 dark:hover:bg-primary-900/30',
                    'transition-colors'
                  )}
                  title="View job logs"
                  data-testid="view-logs-button"
                >
                  <FileText className="w-3 h-3" />
                  Logs
                </button>
              )}
            </div>
            {showGroup && (
              <Link
                to={`/groups/${encodeURIComponent(job.group_name)}`}
                className="text-sm text-primary-600 dark:text-primary-400 hover:underline"
              >
                {job.group_name}
              </Link>
            )}
            {job.message && (
              <p className="text-sm text-gray-600 dark:text-gray-400 mt-1 line-clamp-2">
                {job.message}
              </p>
            )}
          </div>
          <div className="flex flex-col items-end gap-1 flex-shrink-0">
            <time
              className="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap"
              dateTime={job.updated_at}
              title={new Date(job.updated_at).toLocaleString()}
            >
              {formatRelativeTime(job.updated_at)}
            </time>
            {/* AIDEV-NOTE: Only show expiration if date is valid to avoid "Invalid Date" tooltips */}
            {job.expires_at && isExpirationValid && (
              <span
                className={cn(
                  'inline-flex items-center gap-1 text-xs whitespace-nowrap',
                  isFading
                    ? 'text-amber-700 dark:text-amber-400'
                    : 'text-gray-500 dark:text-gray-400'
                )}
                title={`Expires: ${new Date(job.expires_at).toLocaleString()}`}
              >
                <Clock className="w-3 h-3" />
                {timeRemainingText}
              </span>
            )}
          </div>
        </div>
      </CardBody>
      </div>

      <Dialog
        isOpen={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        title="Delete Job"
      >
        <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
          Are you sure you want to delete "{job.name}"? This action cannot be undone.
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setShowDeleteConfirm(false)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            onClick={handleDelete}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </div>
      </Dialog>

      {/* Log viewer modal - only rendered when job has logs */}
      {job.has_log && (
        <LogViewerModal
          isOpen={showLogViewer}
          onClose={() => setShowLogViewer(false)}
          groupName={job.group_name}
          jobName={job.name}
          totalLineCount={job.log_line_count}
        />
      )}
    </Card>
  )
}
