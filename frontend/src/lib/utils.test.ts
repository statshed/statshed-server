/**
 * AIDEV-NOTE: Utility function tests
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { cn, formatDate, formatRelativeTime } from './utils'

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('handles conditional classes', () => {
    const isActive = false
    expect(cn('foo', isActive && 'bar', 'baz')).toBe('foo baz')
  })

  it('merges Tailwind classes correctly', () => {
    expect(cn('px-4', 'px-6')).toBe('px-6')
    expect(cn('text-red-500', 'text-blue-500')).toBe('text-blue-500')
  })

  it('handles arrays of classes', () => {
    expect(cn(['foo', 'bar'], 'baz')).toBe('foo bar baz')
  })
})

describe('formatDate', () => {
  it('formats date strings correctly', () => {
    const date = '2024-01-15T12:30:00Z'
    const formatted = formatDate(date)
    expect(formatted).toContain('2024')
  })

  it('returns fallback for invalid dates', () => {
    expect(formatDate('not-a-date')).toBe('Invalid date')
    expect(formatDate('')).toBe('Invalid date')
  })
})

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns "just now" for recent times', () => {
    const now = new Date('2024-01-15T12:00:00Z')
    vi.setSystemTime(now)
    const recent = '2024-01-15T11:59:30Z'
    expect(formatRelativeTime(recent)).toBe('just now')
  })

  it('returns minutes ago', () => {
    const now = new Date('2024-01-15T12:00:00Z')
    vi.setSystemTime(now)
    const fiveMinutesAgo = '2024-01-15T11:55:00Z'
    expect(formatRelativeTime(fiveMinutesAgo)).toBe('5 minutes ago')
  })

  it('returns hours ago', () => {
    const now = new Date('2024-01-15T12:00:00Z')
    vi.setSystemTime(now)
    const twoHoursAgo = '2024-01-15T10:00:00Z'
    expect(formatRelativeTime(twoHoursAgo)).toBe('2 hours ago')
  })

  it('returns days ago', () => {
    const now = new Date('2024-01-15T12:00:00Z')
    vi.setSystemTime(now)
    const threeDaysAgo = '2024-01-12T12:00:00Z'
    expect(formatRelativeTime(threeDaysAgo)).toBe('3 days ago')
  })

  it('returns fallback for invalid dates', () => {
    expect(formatRelativeTime('not-a-date')).toBe('Invalid date')
    expect(formatRelativeTime('')).toBe('Invalid date')
  })
})
