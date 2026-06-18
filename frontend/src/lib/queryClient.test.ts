/**
 * AIDEV-NOTE: Tests for the global query-error toast gate (handleQueryError).
 * Toast only on background-refetch failures (cached data present), not on
 * initial-load failures (data undefined) which the page's error UI already shows.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleQueryError } from './queryClient'
import { showErrorToast } from '@/contexts/ToastContext'

vi.mock('@/contexts/ToastContext', () => ({
  showErrorToast: vi.fn(),
}))

const mockShowErrorToast = vi.mocked(showErrorToast)

/** Minimal Query stand-in carrying just the state.data the gate inspects. */
const queryWithData = (data: unknown) => ({ state: { data } })

describe('handleQueryError', () => {
  beforeEach(() => mockShowErrorToast.mockClear())

  it('does NOT toast when the query has no data (initial load failure)', () => {
    handleQueryError(new Error('boom'), queryWithData(undefined))
    expect(mockShowErrorToast).not.toHaveBeenCalled()
  })

  it('toasts when a background refetch fails while cached data is shown', () => {
    handleQueryError(new Error('network down'), queryWithData([{ id: 1 }]))
    expect(mockShowErrorToast).toHaveBeenCalledTimes(1)
    expect(mockShowErrorToast).toHaveBeenCalledWith('Could not refresh data', 'network down')
  })

  it('falls back to a generic description for non-Error throwables', () => {
    handleQueryError('weird', queryWithData({ ok: true }))
    expect(mockShowErrorToast).toHaveBeenCalledWith(
      'Could not refresh data',
      'Showing the last loaded values.'
    )
  })
})
