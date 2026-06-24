/**
 * AIDEV-NOTE: Job list component
 * List layout with proper spacing for jobs
 */

import JobCard from './JobCard'
import { SkeletonCard } from '@/components/ui'
import type { Job } from '@/types'
import { Inbox } from 'lucide-react'

interface JobListProps {
  jobs?: Job[]
  isLoading?: boolean
  /** Show the group name for each job (useful when displaying jobs from multiple groups) */
  showGroup?: boolean
  /** Custom empty state message */
  emptyMessage?: string
}

interface EmptyStateProps {
  message?: string
}

function EmptyState({ message }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center animate-rise">
      <div className="p-4 bg-primary-100 dark:bg-primary-500/10 rounded-2xl ring-1 ring-inset ring-primary-600/15 dark:ring-primary-400/20 mb-4">
        <Inbox className="w-8 h-8 text-primary-600 dark:text-primary-400" />
      </div>
      <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
        Nothing to report
      </h3>
      <p className="text-gray-500 dark:text-gray-400 max-w-sm">
        {message || 'Jobs will appear here once status updates are submitted for this group.'}
      </p>
    </div>
  )
}

function LoadingSkeleton() {
  return (
    <div className="space-y-3">
      {[1, 2, 3, 4, 5].map((i) => (
        <SkeletonCard key={i} />
      ))}
    </div>
  )
}

export default function JobList({ jobs, isLoading, showGroup = false, emptyMessage }: JobListProps) {
  if (isLoading) {
    return <LoadingSkeleton />
  }

  if (!jobs || jobs.length === 0) {
    return <EmptyState message={emptyMessage} />
  }

  return (
    <div className="stagger space-y-3">
      {jobs.map((job) => (
        <JobCard key={job.id} job={job} showGroup={showGroup} />
      ))}
    </div>
  )
}
