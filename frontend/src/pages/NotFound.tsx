/**
 * AIDEV-NOTE: 404 page for the catch-all route in App.tsx.
 * Unknown URLs previously rendered a blank page; this gives a recoverable not-found state.
 */

import { Link } from 'react-router-dom'
import { FileQuestion, Home } from 'lucide-react'
import { Button } from '@/components/ui'

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="p-4 bg-gray-100 dark:bg-gray-800 rounded-full mb-4">
        <FileQuestion className="w-10 h-10 text-gray-400 dark:text-gray-500" />
      </div>
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-1">
        Page not found
      </h1>
      <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-6">
        The page you're looking for doesn't exist or may have moved.
      </p>
      <Link to="/">
        <Button>
          <Home className="w-4 h-4" />
          Back to dashboard
        </Button>
      </Link>
    </div>
  )
}
