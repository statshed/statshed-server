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

// Capture the listeners + lifecycle callbacks the provider registers on its EventSource,
// plus how many EventSources it has created (to assert reconnection).
const { mock } = vi.hoisted(() => {
  const mock: {
    handlers: Record<string, (e: { data: string }) => void>
    onopen?: () => void
    onerror?: () => void
    readyState: number
    instances: number
  } = { handlers: {}, readyState: 1, instances: 0 }

  class MockEventSource {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSED = 2
    constructor(public url: string) {
      mock.instances++
    }
    get readyState() {
      return mock.readyState
    }
    addEventListener(event: string, cb: (e: { data: string }) => void) {
      mock.handlers[event] = cb
    }
    set onopen(cb: () => void) {
      mock.onopen = cb
    }
    set onerror(cb: () => void) {
      mock.onerror = cb
    }
    close() {}
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
  mock.readyState = 1 // OPEN
  mock.instances = 0
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

  // I16: health_update is emitted by the background worker on timeout/stale transitions. It must
  // invalidate queryKeys.jobs so an open Jobs page (byStatus) reflects the new status; groups
  // already covers groupJobs via prefix matching.
  it('health_update invalidates jobs (Jobs page) as well as health and groups', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    fire('health_update', {})

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
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

  it('resyncs on the FIRST open if it followed errors (app mounted during an outage)', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    // The stream errored before it ever opened — the backend/proxy was down at mount, so the
    // mount queries may have failed and exhausted their retries. When it finally opens, the
    // dashboard must resync even though this is technically the first successful open.
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

describe('SocketContext permanent-close recovery', () => {
  beforeEach(resetMock)

  it('recreates the EventSource after a permanent close (proxy 5xx during a backend restart)', () => {
    vi.useFakeTimers()
    try {
      renderProvider()
      expect(mock.instances).toBe(1)

      // The browser closed the stream permanently (a non-200 reconnect response).
      mock.readyState = 2 // CLOSED
      act(() => mock.onerror?.())

      // A reconnect is scheduled; advancing past the delay creates a fresh EventSource.
      act(() => {
        vi.advanceTimersByTime(1500)
      })
      expect(mock.instances).toBe(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not recreate while the browser is still auto-reconnecting (CONNECTING)', () => {
    vi.useFakeTimers()
    try {
      renderProvider()
      expect(mock.instances).toBe(1)

      mock.readyState = 0 // CONNECTING — the browser retries on its own
      act(() => mock.onerror?.())
      act(() => {
        vi.advanceTimersByTime(1500)
      })
      expect(mock.instances).toBe(1)
    } finally {
      vi.useRealTimers()
    }
  })
})
