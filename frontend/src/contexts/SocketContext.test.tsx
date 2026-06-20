/**
 * AIDEV-NOTE: SocketContext realtime cache-invalidation tests (EventSource).
 *
 * jsdom has no EventSource, so we install a mock that captures the named-event listeners and
 * the onopen/onerror callbacks, then invoke them directly. Each realtime event must
 * invalidate the correct React Query caches; a `status_update` must also invalidate
 * queryKeys.jobs so the Jobs page (byStatus queries) refreshes in realtime.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { type ReactNode } from 'react'
import { render, act } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { createTestQueryClient } from '@/test/utils'
import { queryKeys } from '@/lib/constants'
import { useSocket } from '@/hooks/useSocket'

// Capture the listeners + lifecycle callbacks the provider registers on its EventSource.
const { mock } = vi.hoisted(() => {
  const mock: {
    handlers: Record<string, (e: { data: string }) => void>
    onopen?: () => void
    onerror?: () => void
    closed: boolean
  } = { handlers: {}, closed: false }

  class MockEventSource {
    constructor(public url: string) {}
    addEventListener(event: string, cb: (e: { data: string }) => void) {
      mock.handlers[event] = cb
    }
    set onopen(cb: () => void) {
      mock.onopen = cb
    }
    set onerror(cb: () => void) {
      mock.onerror = cb
    }
    close() {
      mock.closed = true
    }
  }
  ;(globalThis as unknown as { EventSource: unknown }).EventSource = MockEventSource
  return { mock }
})

// Import after the global mock is installed.
import { SocketProvider } from './SocketContext'

function resetMock() {
  for (const key of Object.keys(mock.handlers)) delete mock.handlers[key]
  mock.onopen = undefined
  mock.onerror = undefined
}

function renderProvider(children?: ReactNode) {
  const queryClient = createTestQueryClient()
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  const result = render(
    <QueryClientProvider client={queryClient}>
      <SocketProvider>{children ?? <div />}</SocketProvider>
    </QueryClientProvider>
  )
  return { invalidateSpy, ...result }
}

function fire(event: string, data: unknown) {
  mock.handlers[event]({ data: JSON.stringify(data) })
}

describe('SocketContext realtime invalidation', () => {
  beforeEach(resetMock)

  it('registers a status_update listener on /api/events', () => {
    renderProvider()
    expect(typeof mock.handlers['status_update']).toBe('function')
  })

  it('status_update invalidates the Jobs page (byStatus) cache', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    fire('status_update', { group_name: 'backups', job: { id: 1 } })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
  })

  it('status_update also invalidates groupJobs, health and groups', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    fire('status_update', { group_name: 'backups', job: { id: 1 } })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groupJobs('backups') })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.health })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groups })
  })
})

describe('SocketContext reconnect resync', () => {
  beforeEach(resetMock)

  it('does not resync caches on the initial open (mount queries already fetch)', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    act(() => mock.onopen?.())

    expect(invalidateSpy).not.toHaveBeenCalled()
  })

  it('resyncs health, groups and jobs after a reconnect so the dashboard is not left stale', () => {
    const { invalidateSpy } = renderProvider()
    act(() => mock.onopen?.()) // initial open — no resync
    invalidateSpy.mockClear()

    // Transient drop, then EventSource reconnects and fires `open` again.
    act(() => mock.onerror?.())
    act(() => mock.onopen?.())

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.health })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groups })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
  })
})

describe('SocketContext connection badge', () => {
  beforeEach(resetMock)

  it('reflects open/error in isConnected', () => {
    function Status() {
      const { isConnected } = useSocket()
      return <span data-testid="status">{isConnected ? 'on' : 'off'}</span>
    }
    const { getByTestId } = renderProvider(<Status />)

    expect(getByTestId('status').textContent).toBe('off')
    act(() => mock.onopen?.())
    expect(getByTestId('status').textContent).toBe('on')
    act(() => mock.onerror?.())
    expect(getByTestId('status').textContent).toBe('off')
  })
})
