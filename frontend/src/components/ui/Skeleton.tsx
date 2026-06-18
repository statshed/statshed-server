/**
 * AIDEV-NOTE: Skeleton component
 * Loading placeholder with pulse animation
 */

import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
  className?: string
}

// AIDEV-NOTE: Expose a polite loading status so screen-reader users know content is loading.
// Defaults are spread-overridable: pass `aria-hidden` to make a skeleton decorative (e.g. the
// inner lines of a composite, so only ONE status is announced per loading region).
export function Skeleton({ className, ...rest }: SkeletonProps) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Loading"
      className={cn(
        'animate-pulse rounded-md bg-gray-200 dark:bg-gray-700',
        className
      )}
      {...rest}
    />
  )
}

// Pre-built skeleton variants for common use cases
export function SkeletonText({ className }: SkeletonProps) {
  return <Skeleton className={cn('h-4 w-full', className)} />
}

export function SkeletonTitle({ className }: SkeletonProps) {
  return <Skeleton className={cn('h-6 w-48', className)} />
}

export function SkeletonCard({ className }: SkeletonProps) {
  return (
    // AIDEV-NOTE: One status for the whole card; the inner lines are decorative (aria-hidden)
    // so a screen reader announces "Loading" once, not once per line.
    <div
      role="status"
      aria-busy="true"
      aria-label="Loading"
      className={cn(
        'rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3',
        className
      )}
    >
      <Skeleton className="h-5 w-24" aria-hidden />
      <Skeleton className="h-4 w-full" aria-hidden />
      <Skeleton className="h-4 w-3/4" aria-hidden />
    </div>
  )
}

export function SkeletonBadge({ className }: SkeletonProps) {
  return <Skeleton className={cn('h-5 w-16 rounded-full', className)} />
}
