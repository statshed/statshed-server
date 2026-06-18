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
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link
          to="/"
          className="p-2 -ml-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          aria-label="Back to dashboard"
        >
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
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
