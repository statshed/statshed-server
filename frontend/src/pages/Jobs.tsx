/**
 * AIDEV-NOTE: Jobs listing page
 * Displays jobs filtered by status, accessed via health card click-through
 *
 * URL format: /jobs?status=success or /jobs?status=error,timeout,stale
 * The status parameter determines which jobs to display
 *
 * Note: The Errors health card links here with error,timeout,stale statuses.
 * The card count shows only unacked jobs, but this page shows all matching jobs
 * (including acked ones) so users can see and manage acknowledgements.
 */

import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft, AlertCircle, RefreshCw } from 'lucide-react'
import { useJobsByStatus } from '@/hooks'
import { JobList } from '@/components/jobs'
import { Button } from '@/components/ui'
import { ErrorBoundary } from '@/components/ErrorBoundary'

/**
 * Map status filter to page title and description
 */
function getPageInfo(statuses: string[]): { title: string; description: string } {
  // Sort for consistent comparison
  const sorted = [...statuses].sort()

  if (sorted.length === 1) {
    switch (sorted[0]) {
      case 'success':
        return {
          title: 'Healthy Jobs',
          description: 'All jobs with successful status',
        }
      case 'progress':
        return {
          title: 'Jobs In Progress',
          description: 'All jobs currently in progress',
        }
      case 'error':
        return {
          title: 'Error Jobs',
          description: 'All jobs with error status',
        }
      case 'timeout':
        return {
          title: 'Timed Out Jobs',
          description: 'All jobs that have timed out',
        }
      case 'stale':
        return {
          title: 'Stale Jobs',
          description: 'All jobs with stale status',
        }
      default:
        return {
          title: 'Jobs',
          description: `Jobs with ${sorted[0]} status`,
        }
    }
  }

  // Check for error + timeout + stale combination (matches "Errors" health card)
  // AIDEV-NOTE: This is the primary link from the Errors health card
  if (
    sorted.length === 3 &&
    sorted.includes('error') &&
    sorted.includes('timeout') &&
    sorted.includes('stale')
  ) {
    return {
      title: 'Error Jobs',
      description: 'All jobs with error, timeout, or stale status',
    }
  }

  // Check for error + timeout combination (legacy/manual navigation)
  if (
    sorted.length === 2 &&
    sorted.includes('error') &&
    sorted.includes('timeout')
  ) {
    return {
      title: 'Error Jobs',
      description: 'All jobs with error or timeout status',
    }
  }

  // Generic multi-status
  return {
    title: 'Jobs',
    description: `Jobs with ${statuses.join(' or ')} status`,
  }
}

function JobsContent() {
  const [searchParams] = useSearchParams()
  const statusParam = searchParams.get('status') || ''

  // Parse comma-separated statuses
  const statuses = useMemo(() => {
    if (!statusParam) return []
    return statusParam
      .split(',')
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
  }, [statusParam])

  const { title, description } = useMemo(() => getPageInfo(statuses), [statuses])

  const {
    data,
    isLoading,
    error,
    refetch,
  } = useJobsByStatus(statuses)

  // AIDEV-NOTE: The favicon is driven globally from overall health in <Header>
  // (a single source of truth), not from this page's filtered job list.

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <div className="p-4 bg-red-100 dark:bg-red-900/30 rounded-full mb-4">
          <AlertCircle className="w-8 h-8 text-red-600 dark:text-red-400" />
        </div>
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
          Failed to load jobs
        </h3>
        <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-4">
          {error instanceof Error ? error.message : 'An error occurred while fetching jobs.'}
        </p>
        <Button onClick={() => refetch()} variant="secondary">
          <RefreshCw className="w-4 h-4" />
          Try Again
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="p-2 -ml-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            aria-label="Back to dashboard"
          >
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              {title}
            </h1>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {description}
            </p>
          </div>
        </div>
      </div>

      {/* Job count */}
      {data && !isLoading && (
        <p className="text-sm text-gray-600 dark:text-gray-400">
          {data.total} job{data.total !== 1 ? 's' : ''}
        </p>
      )}

      {/* Jobs list */}
      <JobList
        jobs={data?.jobs}
        isLoading={isLoading}
        showGroup
        emptyMessage={`No jobs with ${statuses.length > 0 ? statuses.join(' or ') : 'matching'} status found.`}
      />
    </div>
  )
}

export default function Jobs() {
  return (
    <ErrorBoundary>
      <JobsContent />
    </ErrorBoundary>
  )
}
