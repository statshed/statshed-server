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

// reconnectDelayMs is how long we wait before recreating a permanently-closed EventSource.
const reconnectDelayMs = 1000

export function SocketProvider({ children }: SocketProviderProps) {
  const [isConnected, setIsConnected] = useState(false)
  const queryClient = useQueryClient()

  useEffect(() => {
    // AIDEV-NOTE: Guard against StrictMode's double-invoke / unmount races — never touch
    // state or reconnect after cleanup.
    let isCleanedUp = false

    // AIDEV-NOTE: Distinguish the initial open from a reconnect. The initial open needs no
    // resync — the page's mount queries already fetch — but a RECONNECT must invalidate,
    // because events emitted during the outage were missed; without an immediate resync the
    // dashboard would show stale data until the next 60s refetchInterval poll.
    let hasConnected = false
    // AIDEV-NOTE: hadError tracks whether the stream errored before its FIRST successful
    // open. If the app mounted during a backend/proxy outage, the mount queries may have
    // already failed and exhausted their retries; so once the stream finally opens we must
    // resync even though this is technically the first open, or the dashboard stays
    // stale/errored even though it is now live (codex review).
    let hadError = false

    let source: EventSource | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined

    const on = <T,>(event: string, handler: (data: T) => void) => {
      source?.addEventListener(event, (e: MessageEvent) => {
        if (isCleanedUp) return
        handler(JSON.parse(e.data) as T)
      })
    }

    const connect = () => {
      if (isCleanedUp) return
      // BACKEND_URL is '' (same-origin) in production/Docker; VITE_BACKEND_URL or the Vite
      // /api proxy in dev.
      source = new EventSource(`${BACKEND_URL}/api/events`)

      source.onopen = () => {
        if (isCleanedUp) return
        setIsConnected(true)
        // Resync on every (re)open EXCEPT the very first clean connect (mount queries already
        // fetch). A reconnect — OR a first open that only succeeded after earlier errors (the
        // app loaded during an outage; see hadError) — must invalidate, or the dashboard
        // stays stale.
        if (hasConnected || hadError) {
          queryClient.invalidateQueries({ queryKey: queryKeys.health })
          queryClient.invalidateQueries({ queryKey: queryKeys.groups })
          queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
        }
        hasConnected = true
        hadError = false
      }

      source.onerror = () => {
        if (isCleanedUp) return
        setIsConnected(false)
        hadError = true // the next successful (re)open must resync; see hadError above
        // AIDEV-NOTE: EventSource auto-reconnects on a network error (readyState stays
        // CONNECTING), but a NON-200 response — e.g. a reverse proxy or load balancer
        // returning 502/503/500 during a backend restart or deploy — closes it
        // PERMANENTLY. Without this the dashboard would be stuck "Disconnected" until a
        // manual reload. Recreate it ourselves so the app always recovers and resyncs.
        if (source && source.readyState === EventSource.CLOSED) {
          source.close()
          clearTimeout(reconnectTimer)
          reconnectTimer = setTimeout(connect, reconnectDelayMs)
        }
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
        // The background worker emits health_update on timeout/stale transitions; invalidate
        // jobs too so an open Jobs page (byStatus) reflects the new status without waiting for
        // its 60s poll. groups already covers groupJobs via query-key prefix matching.
        queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
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
    }

    connect()

    return () => {
      isCleanedUp = true
      clearTimeout(reconnectTimer)
      source?.close()
    }
  }, [queryClient])

  // Memoize so consumers don't re-render on unrelated SocketProvider renders.
  const value = useMemo(() => ({ isConnected }), [isConnected])

  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>
}
