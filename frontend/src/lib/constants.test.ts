/**
 * AIDEV-NOTE: Tests for DEFAULT_QUERY_OPTIONS.retry — deterministic 4xx client errors must
 * NOT be retried (they only delay the error UI), while network/5xx failures keep retrying.
 */

import { describe, it, expect } from 'vitest'
import { DEFAULT_QUERY_OPTIONS } from './constants'
import { ApiError } from '@/api/client'

// retry is a predicate (failureCount, error) => boolean
const retry = DEFAULT_QUERY_OPTIONS.retry as unknown as (
  failureCount: number,
  error: unknown
) => boolean

describe('DEFAULT_QUERY_OPTIONS.retry', () => {
  it('does not retry 4xx client errors', () => {
    expect(retry(0, new ApiError('Not found', 404))).toBe(false)
    expect(retry(0, new ApiError('Bad request', 400))).toBe(false)
    expect(retry(0, new ApiError('Conflict', 409))).toBe(false)
  })

  it('retries 5xx server errors up to 3 attempts', () => {
    expect(retry(0, new ApiError('Server error', 500))).toBe(true)
    expect(retry(2, new ApiError('Server error', 503))).toBe(true)
    expect(retry(3, new ApiError('Server error', 500))).toBe(false)
  })

  it('retries network errors (status 0)', () => {
    expect(retry(0, new ApiError('Network failure', 0))).toBe(true)
    expect(retry(3, new ApiError('Network failure', 0))).toBe(false)
  })

  it('retries unknown (non-ApiError) errors', () => {
    expect(retry(0, new Error('boom'))).toBe(true)
    expect(retry(3, new Error('boom'))).toBe(false)
  })
})
