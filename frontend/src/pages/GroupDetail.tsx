/**
 * AIDEV-NOTE: Group Detail page
 * Displays jobs for a specific group with search/filter and configuration options
 */

import React, { useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Settings, Plus, AlertCircle, RefreshCw, Search } from 'lucide-react'
import { useGroupJobs, useGroupConfig, useFilterParam, useSearchParam } from '@/hooks'
import { JobList, JobSubmitForm } from '@/components/jobs'
import { GroupConfigForm } from '@/components/groups'
import { Button, Skeleton, Input } from '@/components/ui'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import type { JobStatus } from '@/types'
import { JOB_STATUS_LABELS } from '@/lib/constants'
import { cn } from '@/lib/utils'

type FilterOption = 'all' | JobStatus

const STATUS_FILTER_OPTIONS: Array<{ value: FilterOption; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'success', label: JOB_STATUS_LABELS.success },
  { value: 'error', label: JOB_STATUS_LABELS.error },
  { value: 'progress', label: JOB_STATUS_LABELS.progress },
  { value: 'timeout', label: JOB_STATUS_LABELS.timeout },
  { value: 'stale', label: JOB_STATUS_LABELS.stale },
]

const VALID_STATUS_VALUES = ['all', 'success', 'error', 'progress', 'timeout', 'stale'] as const

function GroupDetailContent() {
  const { groupName } = useParams<{ groupName: string }>()
  const decodedGroupName = groupName ? decodeURIComponent(groupName) : ''

  // UI-only state (not persisted to URL)
  const [isConfigOpen, setIsConfigOpen] = React.useState(false)
  const [isSubmitOpen, setIsSubmitOpen] = React.useState(false)

  // URL-synced filter state for persistence and shareability
  const [searchQuery, setSearchQuery] = useSearchParam('search')
  const [statusFilter, setStatusFilter] = useFilterParam<FilterOption>({
    paramName: 'status',
    defaultValue: 'all',
    validValues: VALID_STATUS_VALUES,
  })

  const {
    data: jobs,
    isLoading: jobsLoading,
    error: jobsError,
    refetch: refetchJobs,
  } = useGroupJobs(decodedGroupName)

  const {
    data: config,
    isLoading: configLoading,
  } = useGroupConfig(decodedGroupName)

  // AIDEV-NOTE: The favicon is driven globally from overall system health in
  // <Header> (single source of truth), not from this one group's job list.

  // Filter and sort jobs by most recent
  const filteredJobs = useMemo(() => {
    if (!jobs) return undefined

    const filtered = jobs.filter((job) => {
      // Search filter
      const matchesSearch = searchQuery === '' ||
        job.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (job.message && job.message.toLowerCase().includes(searchQuery.toLowerCase()))

      // Status filter
      const matchesStatus = statusFilter === 'all' || job.status === statusFilter

      return matchesSearch && matchesStatus
    })

    const sorted = [...filtered].sort((a, b) => {
      const timeA = new Date(a.updated_at).getTime()
      const timeB = new Date(b.updated_at).getTime()
      return timeB - timeA  // Descending (newest first)
    })

    return sorted
  }, [jobs, searchQuery, statusFilter])

  if (!decodedGroupName) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 dark:text-gray-400">Invalid group name</p>
      </div>
    )
  }

  if (jobsError) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <div className="p-4 bg-red-100 dark:bg-red-900/30 rounded-full mb-4">
          <AlertCircle className="w-8 h-8 text-red-600 dark:text-red-400" />
        </div>
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
          Failed to load jobs
        </h3>
        <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-4">
          {jobsError.message || 'An error occurred while fetching jobs.'}
        </p>
        <Button onClick={() => refetchJobs()} variant="secondary">
          <RefreshCw className="w-4 h-4" />
          Try Again
        </Button>
      </div>
    )
  }

  const showingFilteredCount = filteredJobs && jobs && filteredJobs.length !== jobs.length

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
              {decodedGroupName}
            </h1>
            {configLoading ? (
              <Skeleton className="h-4 w-48 mt-1" />
            ) : config ? (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Timeout: {config.effective_progress_timeout_minutes}m progress
                {config.effective_staleness_timeout_hours !== null && (
                  <>, {config.effective_staleness_timeout_hours}h stale</>
                )}
              </p>
            ) : null}
          </div>
        </div>
        <div className="flex gap-2 self-start sm:self-auto">
          <Button onClick={() => setIsSubmitOpen(true)}>
            <Plus className="w-4 h-4" />
            Submit Status
          </Button>
          <Button
            variant="secondary"
            onClick={() => setIsConfigOpen(true)}
          >
            <Settings className="w-4 h-4" />
            Configure
          </Button>
        </div>
      </div>

      {/* Job count and filters */}
      <div className="space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            {showingFilteredCount ? (
              <>Showing {filteredJobs.length} of {jobs.length} jobs</>
            ) : jobs ? (
              <>{jobs.length} job{jobs.length !== 1 ? 's' : ''}</>
            ) : null}
          </p>
        </div>

        {/* Search and Filter Controls */}
        <div className="flex flex-col sm:flex-row gap-4">
          {/* Search Input */}
          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <Input
              type="search"
              placeholder="Search jobs..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>

          {/* Status Filter Buttons */}
          <div className="flex gap-1 flex-wrap">
            {STATUS_FILTER_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => setStatusFilter(option.value)}
                className={cn(
                  'px-3 py-1.5 text-sm font-medium rounded-lg transition-colors',
                  statusFilter === option.value
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Jobs list */}
      <JobList jobs={filteredJobs} isLoading={jobsLoading} />

      {/* Config dialog */}
      <GroupConfigForm
        groupName={decodedGroupName}
        isOpen={isConfigOpen}
        onClose={() => setIsConfigOpen(false)}
      />

      {/* Job Submit dialog with pre-filled group */}
      <JobSubmitForm
        isOpen={isSubmitOpen}
        onClose={() => setIsSubmitOpen(false)}
        defaultGroup={decodedGroupName}
      />
    </div>
  )
}

export default function GroupDetail() {
  return (
    <ErrorBoundary>
      <GroupDetailContent />
    </ErrorBoundary>
  )
}
