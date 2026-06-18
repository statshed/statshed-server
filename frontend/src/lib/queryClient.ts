/**
 * AIDEV-NOTE: App-wide TanStack Query client + a global query-error safety net.
 *
 * Pages surface INITIAL-load failures through their own `isError` UI (error cards
 * with a retry). The gap this fills is a BACKGROUND refetch that fails while cached
 * data is still on screen: the query keeps status 'success' (it has data), so
 * `isError` stays false and nothing tells the user the view is now stale. We toast
 * only in that case (cached data present) so initial-load errors aren't double-reported.
 */

import { QueryClient, QueryCache } from '@tanstack/react-query'
import { DEFAULT_QUERY_OPTIONS } from './constants'
import { showErrorToast } from '@/contexts/ToastContext'

// Minimal structural shape of what the gate inspects — avoids coupling to Query's
// generics (the QueryCache passes Query<unknown, unknown, ...>).
type QueryLike = { state: { data: unknown } }

export function handleQueryError(error: unknown, query: QueryLike): void {
  // Undefined data == initial load; the component's error UI handles that.
  if (query.state.data === undefined) return
  showErrorToast(
    'Could not refresh data',
    error instanceof Error ? error.message : 'Showing the last loaded values.'
  )
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => handleQueryError(error, query),
    }),
    defaultOptions: {
      queries: DEFAULT_QUERY_OPTIONS,
    },
  })
}
