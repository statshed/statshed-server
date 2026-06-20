/**
 * AIDEV-NOTE: Real-time connection management via Server-Sent Events (EventSource).
 * - Connects to GET /api/events. The Go server streams SSE (spec §7); this replaced the
 *   Socket.IO client. The provider/hook keep their historical names (SocketProvider,
 *   useSocket, isConnected) so the header badge and consumers are unchanged.
 * - Exposes the connection status; invalidates TanStack Query caches on each event so views
 *   stay in sync (the payloads are ignored beyond their group_name — we refetch, not patch).
 */

import {
  createContext,
  useEffect,
  useState,
  useMemo,
  type ReactNode,
} from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { queryKeys, BACKEND_URL } from '@/lib/constants'
import type { StatusUpdateEvent, JobsAckedEvent, JobDeletedEvent, JobExpiredEvent } from '@/types'

// AIDEV-NOTE: Only isConnected is exposed — it drives the header connection badge.
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
    // AIDEV-NOTE: Guard against StrictMode's double-invoke / unmount races — never touch
    // state after cleanup.
    let isCleanedUp = false

    // AIDEV-NOTE: Distinguish the initial open from a reconnect. EventSource reconnects on
    // its own after a drop and fires `open` again each time. The initial open needs no
    // resync — the page's mount queries already fetch — but a RECONNECT must invalidate,
    // because events emitted during the outage were missed and useGroups/useGroupJobs have
    // no refetchInterval, so the dashboard would stay stale indefinitely. A local closure
    // flag (not a ref) so a fresh EventSource starts over.
    let hasConnected = false

    // AIDEV-NOTE: BACKEND_URL is '' (same-origin) in production/Docker — the unified
    // statshed-server serves /api/events same-origin. In dev, set VITE_BACKEND_URL or rely
    // on the Vite /api proxy.
    const source = new EventSource(`${BACKEND_URL}/api/events`)

    source.onopen = () => {
      if (isCleanedUp) return
      setIsConnected(true)
      if (hasConnected) {
        queryClient.invalidateQueries({ queryKey: queryKeys.health })
        queryClient.invalidateQueries({ queryKey: queryKeys.groups })
        queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      }
      hasConnected = true
    }

    source.onerror = () => {
      // EventSource auto-reconnects after an error; reflect the outage in the badge
      // meanwhile (no console noise — a flaky link would spam it).
      if (!isCleanedUp) setIsConnected(false)
    }

    // on registers a named-event listener that parses the JSON payload before handling.
    const on = <T,>(event: string, handler: (data: T) => void) => {
      source.addEventListener(event, (e: MessageEvent) => {
        if (isCleanedUp) return
        handler(JSON.parse(e.data) as T)
      })
    }

    on<StatusUpdateEvent>('status_update', (data) => {
      // Invalidate the specific group's jobs, plus health + the groups list (job counts),
      // plus the Jobs-page byStatus caches so a status change appears there in realtime.
      queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(data.group_name) })
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
    })

    on('group_created', () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    })

    on('health_update', () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    })

    on<JobsAckedEvent>('jobs_acked', (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (data.group_name) {
        queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(data.group_name) })
      }
    })

    on<JobDeletedEvent>('job_deleted', (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (data.group_name) {
        queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(data.group_name) })
      }
    })

    on<JobExpiredEvent>('job_expired', (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.health })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
      if (data.group_name) {
        queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(data.group_name) })
      }
    })

    return () => {
      isCleanedUp = true
      source.close()
    }
  }, [queryClient])

  // Memoize so consumers don't re-render on unrelated SocketProvider renders.
  const value = useMemo(() => ({ isConnected }), [isConnected])

  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>
}
