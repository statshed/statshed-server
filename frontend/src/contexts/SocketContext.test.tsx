/**
 * AIDEV-NOTE: SocketContext realtime cache-invalidation tests.
 *
 * Mocks socket.io-client so we can capture the registered event handlers and invoke them
 * directly, then assert that each realtime event invalidates the correct React Query caches.
 * Regression focus: a `status_update` must also invalidate queryKeys.jobs so the Jobs page
 * (byStatus queries) refreshes in realtime instead of waiting for its polling interval.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { createTestQueryClient } from '@/test/utils'
import { queryKeys } from '@/lib/constants'

// Capture handlers registered via socket.on(...) so tests can fire events.
const { mockSocket, handlers } = vi.hoisted(() => {
  const handlers: Record<string, (...args: unknown[]) => void> = {}
  const mockSocket = {
    on: (event: string, cb: (...args: unknown[]) => void) => {
      handlers[event] = cb
    },
    removeAllListeners: () => {},
    disconnect: () => {},
  }
  return { mockSocket, handlers }
})

vi.mock('socket.io-client', () => ({
  io: () => mockSocket,
}))

// Import after the mock is registered.
import { SocketProvider } from './SocketContext'

function renderProvider() {
  const queryClient = createTestQueryClient()
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  render(
    <QueryClientProvider client={queryClient}>
      <SocketProvider>
        <div />
      </SocketProvider>
    </QueryClientProvider>
  )
  return { invalidateSpy }
}

describe('SocketContext realtime invalidation', () => {
  beforeEach(() => {
    for (const key of Object.keys(handlers)) delete handlers[key]
  })

  it('registers a status_update handler on connect', () => {
    renderProvider()
    expect(typeof handlers['status_update']).toBe('function')
  })

  it('status_update invalidates the Jobs page (byStatus) cache', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    handlers['status_update']({ group_name: 'backups', job: { id: 1 } })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
  })

  it('status_update also invalidates groupJobs, health and groups', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    handlers['status_update']({ group_name: 'backups', job: { id: 1 } })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groupJobs('backups') })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.health })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groups })
  })
})

describe('SocketContext reconnect resync', () => {
  beforeEach(() => {
    for (const key of Object.keys(handlers)) delete handlers[key]
  })

  it('does not resync caches on the initial connect (mount queries already fetch)', () => {
    const { invalidateSpy } = renderProvider()
    invalidateSpy.mockClear()

    handlers['connect']()

    expect(invalidateSpy).not.toHaveBeenCalled()
  })

  it('resyncs health, groups and jobs after a reconnect so the dashboard is not left stale', () => {
    const { invalidateSpy } = renderProvider()
    // First connect is the initial one — no resync.
    handlers['connect']()
    invalidateSpy.mockClear()

    // Transient drop, then socket.io re-fires 'connect' on reconnect.
    handlers['disconnect']()
    handlers['connect']()

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.health })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.groups })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
  })
})
