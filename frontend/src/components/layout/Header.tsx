/**
 * AIDEV-NOTE: Application header component
 * Contains app title, health indicator, connection status, theme toggle, and navigation
 *
 * AIDEV-NOTE: The header background stays opaque `bg-white dark:bg-gray-800` ON PURPOSE —
 * the MascotLogo's face/lamp negative space is knocked out in exactly that color
 * (see MascotLogo.tsx). Don't make this translucent or recolor it without updating the
 * mascot's KNOCKOUT constant to match, or the dog's face will show the wrong color.
 */

import { Link } from 'react-router-dom'
import { Settings, Wifi, WifiOff } from 'lucide-react'
import { useSocket } from '@/hooks/useSocket'
import { useHealth, useFavicon } from '@/hooks'
import { cn } from '@/lib/utils'
import { HEALTH_STATUS_LABELS } from '@/lib/constants'
import ThemeToggle from '@/components/ThemeToggle'
import MascotLogo from '@/components/layout/MascotLogo'

export default function Header() {
  const { isConnected } = useSocket()

  // AIDEV-NOTE: Single source of truth for the favicon. The Header renders on every
  // route, so driving it from overall /health here keeps it correct everywhere
  // (Dashboard, group pages, Settings, 404). 'unknown' (grey) while health is
  // unavailable — never green during an outage, which would falsely signal "all good".
  const { data: health } = useHealth()
  useFavicon(health ? (health.status === 'unhealthy' ? 'error' : 'healthy') : 'unknown')

  return (
    <header className="sticky top-0 z-50 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
      {/* Headlamp beam: a thin warm gradient riding the bottom edge of the header. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-primary-500/60 to-transparent"
      />
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo and Title — the mascot's headlamp LED reflects overall health */}
          <Link
            to="/"
            className="flex items-center gap-2.5 group"
            title={`StatShed — ${health ? HEALTH_STATUS_LABELS[health.status] : 'Status unavailable'}`}
          >
            <span className="lamp-glow inline-flex">
              <MascotLogo
                status={health ? health.status : 'unknown'}
                className="w-9 h-9 shrink-0 transition-transform duration-300 motion-safe:group-hover:scale-110 motion-safe:group-hover:-rotate-3"
              />
            </span>
            <span className="flex flex-col leading-none">
              <h1 className="text-xl font-semibold text-gray-900 dark:text-white tracking-tight">
                StatShed
              </h1>
              <span className="hidden sm:block font-display italic text-[0.7rem] text-primary-700 dark:text-primary-400 -mt-0.5">
                on watch
              </span>
            </span>
          </Link>

          {/* Right side: connection status and settings */}
          <div className="flex items-center gap-2 sm:gap-3">
            {/* Connection Status */}
            <div
              className={cn(
                'flex items-center gap-1.5 text-sm font-medium rounded-full px-2.5 py-1 transition-colors',
                isConnected
                  ? 'text-green-700 bg-green-100/70 dark:text-green-300 dark:bg-green-500/10'
                  : 'text-red-700 bg-red-100/70 dark:text-red-300 dark:bg-red-500/10'
              )}
              title={isConnected ? 'Connected to server' : 'Disconnected from server'}
            >
              {isConnected ? (
                <Wifi className="w-4 h-4" />
              ) : (
                <WifiOff className="w-4 h-4" />
              )}
              <span className="hidden sm:inline">
                {isConnected ? 'Connected' : 'Disconnected'}
              </span>
            </div>

            {/* Theme Toggle */}
            <ThemeToggle />

            {/* Settings Link */}
            <Link
              to="/settings"
              className="p-2 text-gray-500 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-500/10 transition-colors"
              title="Settings"
            >
              <Settings className="w-5 h-5 transition-transform duration-500 motion-safe:hover:rotate-90" />
            </Link>
          </div>
        </div>
      </div>
    </header>
  )
}
