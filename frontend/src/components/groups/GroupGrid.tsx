/**
 * AIDEV-NOTE: Group grid component
 * Responsive grid layout for group cards
 */

import GroupCard from './GroupCard'
import { SkeletonCard } from '@/components/ui'
import type { GroupWithHealth } from '@/types'
import { Inbox } from 'lucide-react'

interface GroupGridProps {
  groups?: GroupWithHealth[]
  isLoading?: boolean
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="p-4 bg-gray-100 dark:bg-gray-800 rounded-full mb-4">
        <Inbox className="w-8 h-8 text-gray-400" />
      </div>
      <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
        No groups yet
      </h3>
      <p className="text-gray-500 dark:text-gray-400 max-w-sm">
        Groups will appear here once jobs are submitted to the system.
      </p>
    </div>
  )
}

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {[1, 2, 3, 4, 5, 6].map((i) => (
        <SkeletonCard key={i} />
      ))}
    </div>
  )
}

export default function GroupGrid({ groups, isLoading }: GroupGridProps) {
  if (isLoading) {
    return <LoadingSkeleton />
  }

  if (!groups || groups.length === 0) {
    return <EmptyState />
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {groups.map((group) => (
        <GroupCard key={group.id} group={group} />
      ))}
    </div>
  )
}
