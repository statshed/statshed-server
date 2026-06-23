/**
 * AIDEV-NOTE: Settings page
 * Global configuration management
 */

import { Link } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { GlobalConfigForm } from '@/components/config'
import { ErrorBoundary } from '@/components/ErrorBoundary'

function SettingsContent() {
  return (
    <div className="space-y-6 animate-rise">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link
          to="/"
          className="p-2 -ml-2 text-gray-500 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-500/10 transition-colors"
          aria-label="Back to dashboard"
        >
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
          Settings
        </h1>
      </div>

      {/* Global Config Form */}
      <div className="max-w-2xl">
        <GlobalConfigForm />
      </div>
    </div>
  )
}

export default function Settings() {
  return (
    <ErrorBoundary>
      <SettingsContent />
    </ErrorBoundary>
  )
}
