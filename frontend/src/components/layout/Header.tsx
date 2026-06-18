/**
 * AIDEV-NOTE: Application header component
 * Contains app title, health indicator, connection status, theme toggle, and navigation
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
    <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo and Title — the mascot's headlamp LED reflects overall health */}
          <Link
            to="/"
            className="flex items-center gap-2 group"
            title={`StatShed — ${health ? HEALTH_STATUS_LABELS[health.status] : 'Status unavailable'}`}
          >
            <MascotLogo
              status={health ? health.status : 'unknown'}
              className="w-9 h-9 shrink-0 transition-transform group-hover:scale-105"
            />
            <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
              StatShed
            </h1>
          </Link>

          {/* Right side: connection status and settings */}
          <div className="flex items-center gap-4">
            {/* Connection Status */}
            <div
              className={cn(
                'flex items-center gap-1.5 text-sm',
                isConnected
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-red-600 dark:text-red-400'
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
              className="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title="Settings"
            >
              <Settings className="w-5 h-5" />
            </Link>
          </div>
        </div>
      </div>
    </header>
  )
}
