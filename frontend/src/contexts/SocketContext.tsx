/**
 * AIDEV-NOTE: Socket.IO context for WebSocket connection management
 * - Connects directly to the backend Socket.IO server (bypasses Vite proxy)
 * - Provides connection status and socket instance to components
 * - Handles cache invalidation on real-time events
 */

import {
  createContext,
  useEffect,
  useState,
  useMemo,
  type ReactNode,
} from 'react'
import { io } from 'socket.io-client'
import { useQueryClient } from '@tanstack/react-query'
import { queryKeys, BACKEND_URL } from '@/lib/constants'
import type { StatusUpdateEvent, JobsAckedEvent, JobDeletedEvent, JobExpiredEvent } from '@/types'

// AIDEV-NOTE: Only isConnected is exposed. The raw socket was never a usable context
// value — socketRef.current was null on first render and assigning it later didn't
// re-render consumers — and nothing read it, so it's dropped.
interface SocketContextValue {
  isConnected: boolean
}

export const SocketContext = createContext<SocketContextValue>({
  isConnected: false,
})

interface SocketProviderProps {
  children: ReactNode
}

export function SocketProvider({ children }: SocketProviderProps) {
  const [isConnected, setIsConnected] = useState(false)
  const queryClient = useQueryClient()

  useEffect(() => {
    // AIDEV-NOTE: Track if cleanup has been called to handle React StrictMode
    // StrictMode double-invokes effects, causing socket disconnect during connection
    let isCleanedUp = false

    // AIDEV-NOTE: Distinguish the initial connect from reconnects. socket.io v4 re-fires
    // 'connect' (there is no separate 'reconnect' event) every time the socket comes back.
    // The initial connect needs no resync — the page's mount queries already fetch. But on a
    // RECONNECT we must invalidate, because any realtime events emitted during the outage were
    // missed, and useGroups/useGroupJobs have no refetchInterval, so the dashboard would stay
    // stale indefinitely. Local closure flag (not a ref) so a fresh socket starts over.
    let hasConnected = false

    // AIDEV-NOTE: Connect to BACKEND_URL (empty string = same origin)
    // In production/Docker: the unified statshed-server serves /socket.io same-origin
    // In development: set VITE_BACKEND_URL for direct connection, or use Vite proxy
    const socket = io(BACKEND_URL, {
      path: '/socket.io',
      transports: ['websocket', 'polling'],
    })

    // Connection event handlers
    socket.on('connect', () => {
      // Only update state if not cleaned up (handles StrictMode)
      if (isCleanedUp) return
      setIsConnected(true)

      // Resync after a reconnect to recover any events missed during the outage.
      // queryKeys.groups (['groups']) is a prefix that also covers per-group jobs
      // (['groups',name,'jobs']); queryKeys.jobs (['jobs']) covers the Jobs page
      // byStatus caches. Together they refresh every job-bearing view.
      if (hasConnected) {
        queryClient.invalidateQueries({ queryKey: queryKeys.health })
        queryClient.invalidateQueries({ queryKey: queryKeys.groups })
        queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      }
      hasConnected = true
    })

    socket.on('disconnect', () => {
      if (!isCleanedUp) {
        // isConnected drives the visible "Disconnected" indicator; no console noise needed.
        setIsConnected(false)
      }
    })

    socket.on('connect_error', (error) => {
      if (!isCleanedUp) {
        console.error('Socket connection error:', error)
        setIsConnected(false)
      }
    })

    // Real-time event handlers for cache invalidation
    socket.on('status_update', (data: StatusUpdateEvent) => {
      if (isCleanedUp) return
      // Invalidate the specific group's jobs query
      queryClient.invalidateQueries({
        queryKey: queryKeys.groupJobs(data.group_name),
      })
      // Also invalidate health since job status affects overall health
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      // And the groups list since it contains job counts
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      // AIDEV-NOTE: Also invalidate the Jobs-page byStatus queries (['jobs',...]) so a
      // status change appears/disappears there in realtime instead of waiting ~60s for poll.
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
    })

    socket.on('group_created', () => {
      if (isCleanedUp) return
      // Invalidate groups list to show the new group
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    })

    socket.on('health_update', () => {
      if (isCleanedUp) return
      // Invalidate health and groups queries
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    })

    // AIDEV-NOTE: Handle bulk ack events from other clients
    socket.on('jobs_acked', (data: JobsAckedEvent) => {
      if (isCleanedUp) return
      // Invalidate health and groups queries
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      // Invalidate all jobs queries
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      // Invalidate specific group's jobs if group_name is present
      if (data.group_name) {
        queryClient.invalidateQueries({
          queryKey: queryKeys.groupJobs(data.group_name),
        })
      }
    })

    // AIDEV-NOTE: Handle job deletion events from other clients
    socket.on('job_deleted', (data: JobDeletedEvent) => {
      if (isCleanedUp) return
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (data.group_name) {
        queryClient.invalidateQueries({
          queryKey: queryKeys.groupJobs(data.group_name),
        })
      }
    })

    // AIDEV-NOTE: Handle job expiration events from background expiration processor
    // Jobs are auto-deleted after expiration_timeout_hours (default 24h)
    socket.on('job_expired', (data: JobExpiredEvent) => {
      if (isCleanedUp) return
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (data.group_name) {
        queryClient.invalidateQueries({
          queryKey: queryKeys.groupJobs(data.group_name),
        })
      }
    })

    // Cleanup on unmount
    return () => {
      isCleanedUp = true
      // AIDEV-NOTE: Remove all listeners before disconnecting to prevent
      // state updates after cleanup (especially in StrictMode)
      socket.removeAllListeners()
      socket.disconnect()
    }
  }, [queryClient])

  // Memoize so consumers don't re-render on unrelated SocketProvider renders.
  const value = useMemo(() => ({ isConnected }), [isConnected])

  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>
}
