/**
 * AIDEV-NOTE: Main App component
 * Sets up providers (Theme, TanStack Query, Router, Toast, Socket) and defines routes
 */

import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { Toaster } from 'sonner'

import { SocketProvider } from '@/contexts/SocketContext'
import { ThemeProvider } from '@/contexts/ThemeContext'
import { createQueryClient } from '@/lib/queryClient'

// Pages
import Dashboard from '@/pages/Dashboard'
import GroupDetail from '@/pages/GroupDetail'
import Jobs from '@/pages/Jobs'
import Settings from '@/pages/Settings'
import NotFound from '@/pages/NotFound'

// Layout
import Header from '@/components/layout/Header'
import Container from '@/components/layout/Container'

// Create a client (with the global query-error toast safety net)
const queryClient = createQueryClient()

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <SocketProvider>
          <BrowserRouter>
          <div className="min-h-screen flex flex-col">
            <Header />
            <main className="flex-1">
              <Container>
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/jobs" element={<Jobs />} />
                  <Route path="/groups/:groupName" element={<GroupDetail />} />
                  <Route path="/settings" element={<Settings />} />
                  {/* AIDEV-NOTE: Catch-all so unknown URLs show a recoverable 404, not a blank page. */}
                  <Route path="*" element={<NotFound />} />
                </Routes>
              </Container>
            </main>
          </div>
          </BrowserRouter>
          <Toaster
            position="top-right"
            richColors
            closeButton
            toastOptions={{
              duration: 4000,
            }}
          />
        </SocketProvider>
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </ThemeProvider>
  )
}

export default App
