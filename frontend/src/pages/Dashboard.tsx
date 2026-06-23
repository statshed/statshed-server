/**
 * AIDEV-NOTE: Dashboard page - Main view
 * Displays health summary and groups grid with search and filtering
 */

import React, { useMemo } from 'react'
import { Plus, AlertCircle, RefreshCw, Search } from 'lucide-react'
import { useHealth, useGroups, useFilterParam, useSearchParam, useSlowLoading } from '@/hooks'
import { HealthIndicator, HealthStats } from '@/components/health'
import { GroupGrid } from '@/components/groups'
import { JobSubmitForm } from '@/components/jobs'
import { Button, Input } from '@/components/ui'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import type { HealthStatus } from '@/types'
import { cn } from '@/lib/utils'

type FilterOption = 'all' | HealthStatus

const FILTER_OPTIONS: Array<{ value: FilterOption; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'healthy', label: 'Healthy' },
  { value: 'unhealthy', label: 'Unhealthy' },
  { value: 'in_progress', label: 'In Progress' },
]

const VALID_FILTER_VALUES = ['all', 'healthy', 'unhealthy', 'in_progress'] as const

function DashboardContent() {
  // UI-only state (not persisted to URL)
  const [isSubmitOpen, setIsSubmitOpen] = React.useState(false)

  // URL-synced filter state for persistence and shareability
  const [searchQuery, setSearchQuery] = useSearchParam('search')
  const [healthFilter, setHealthFilter] = useFilterParam<FilterOption>({
    paramName: 'health',
    defaultValue: 'all',
    validValues: VALID_FILTER_VALUES,
  })

  const {
    data: health,
    isLoading: healthLoading,
    error: healthError,
    refetch: refetchHealth,
  } = useHealth()

  const {
    data: groups,
    isLoading: groupsLoading,
    error: groupsError,
    refetch: refetchGroups,
  } = useGroups()

  // AIDEV-NOTE: The favicon is driven globally from overall health in <Header>
  // (a single source of truth), so it stays correct on every route — including
  // Settings/404, which set no favicon of their own.

  const isLoading = healthLoading || groupsLoading
  const error = healthError || groupsError
  // AIDEV-NOTE: After a few seconds of loading, hint that the backend may be slow/hung
  // instead of showing only skeletons indefinitely.
  const isSlow = useSlowLoading(isLoading)

  // Filter groups based on search and health status
  const filteredGroups = useMemo(() => {
    if (!groups) return undefined

    return groups.filter((group) => {
      // AIDEV-NOTE: Empty groups (job_count === 0) are hidden from dashboard
      // but preserved in backend. Users can still access via direct URL.
      if (group.job_count === 0) return false

      // Search filter
      const matchesSearch = searchQuery === '' ||
        group.name.toLowerCase().includes(searchQuery.toLowerCase())

      // Health status filter
      const matchesHealth = healthFilter === 'all' ||
        group.health === healthFilter

      return matchesSearch && matchesHealth
    })
  }, [groups, searchQuery, healthFilter])

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <div className="p-4 bg-red-100 dark:bg-red-900/30 rounded-full mb-4">
          <AlertCircle className="w-8 h-8 text-red-600 dark:text-red-400" />
        </div>
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
          Failed to load data
        </h3>
        <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-4">
          {error.message || 'An error occurred while fetching data.'}
        </p>
        <Button
          onClick={() => {
            refetchHealth()
            refetchGroups()
          }}
          variant="secondary"
        >
          <RefreshCw className="w-4 h-4" />
          Try Again
        </Button>
      </div>
    )
  }

  const nonEmptyGroupCount = groups ? groups.filter(g => g.job_count > 0).length : 0
  const showingFilteredCount = filteredGroups && filteredGroups.length !== nonEmptyGroupCount

  return (
    <div className="space-y-8 animate-rise">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
            Dashboard
          </h1>
          {health && !isLoading && (
            <HealthIndicator status={health.status} className="mt-1" />
          )}
        </div>
        <Button onClick={() => setIsSubmitOpen(true)}>
          <Plus className="w-4 h-4" />
          Submit Status
        </Button>
      </div>

      {/* Slow-backend hint: shown only after loading has dragged on, so a hung backend
          isn't just an indefinite wall of skeletons. */}
      {isLoading && isSlow && (
        <p
          role="status"
          className="text-sm text-amber-700 dark:text-amber-400"
        >
          Taking longer than usual… still trying to reach the server.
        </p>
      )}

      {/* Health Stats */}
      <section>
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          System Health
        </h2>
        <HealthStats health={health} isLoading={isLoading} />
      </section>

      {/* Groups Section with Search and Filter */}
      <section>
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Groups
            {showingFilteredCount && (
              <span className="ml-2 text-sm font-normal text-gray-500 dark:text-gray-400">
                (showing {filteredGroups.length} of {nonEmptyGroupCount})
              </span>
            )}
          </h2>
        </div>

        {/* Search and Filter Controls */}
        <div className="flex flex-col sm:flex-row gap-4 mb-4">
          {/* Search Input */}
          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <Input
              type="search"
              placeholder="Search groups..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>

          {/* Filter Buttons */}
          <div className="flex gap-1 flex-wrap">
            {FILTER_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => setHealthFilter(option.value)}
                className={cn(
                  'px-3.5 py-1.5 text-sm font-medium rounded-full transition-all duration-150 motion-safe:active:scale-[0.97]',
                  healthFilter === option.value
                    ? 'bg-primary-600 text-white shadow-sm shadow-primary-600/30'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 ring-1 ring-inset ring-gray-200 dark:ring-gray-700 hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white'
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        <GroupGrid groups={filteredGroups} isLoading={isLoading} />
      </section>

      {/* Job Submit Dialog */}
      <JobSubmitForm isOpen={isSubmitOpen} onClose={() => setIsSubmitOpen(false)} />
    </div>
  )
}

export default function Dashboard() {
  return (
    <ErrorBoundary>
      <DashboardContent />
    </ErrorBoundary>
  )
}
