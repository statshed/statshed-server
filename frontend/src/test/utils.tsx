/**
 * AIDEV-NOTE: Test utilities for React Query hooks testing
 */

import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, type RenderOptions } from '@testing-library/react'

/**
 * Create a fresh QueryClient for each test to ensure isolation
 */
export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Turn off retries for tests to fail fast
        retry: false,
        // Disable refetch on window focus in tests
        refetchOnWindowFocus: false,
        // Set stale time to 0 for immediate refetches in tests
        staleTime: 0,
      },
      mutations: {
        // Turn off retries for mutations in tests
        retry: false,
      },
    },
  })
}

/**
 * Wrapper component that provides QueryClient context
 */
interface TestQueryProviderProps {
  children: ReactNode
  queryClient?: QueryClient
}

export function TestQueryProvider({
  children,
  queryClient,
}: TestQueryProviderProps) {
  const client = queryClient ?? createTestQueryClient()
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

/**
 * Custom render function that wraps components with QueryClientProvider
 */
export function renderWithQueryClient(
  ui: React.ReactElement,
  options?: Omit<RenderOptions, 'wrapper'> & { queryClient?: QueryClient }
) {
  const { queryClient, ...renderOptions } = options ?? {}
  const testQueryClient = queryClient ?? createTestQueryClient()

  return {
    ...render(ui, {
      wrapper: ({ children }) => (
        <TestQueryProvider queryClient={testQueryClient}>
          {children}
        </TestQueryProvider>
      ),
      ...renderOptions,
    }),
    queryClient: testQueryClient,
  }
}
