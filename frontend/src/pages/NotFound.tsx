/**
 * AIDEV-NOTE: 404 page for the catch-all route in App.tsx.
 * Unknown URLs previously rendered a blank page; this gives a recoverable not-found state.
 *
 * AIDEV-NOTE: We reuse <MascotLogo> here, which knocks out its face/lamp in the HEADER
 * background color (text-white dark:text-gray-800 — see MascotLogo.tsx). To keep that
 * negative space correct off the header, the mascot sits inside a circle painted that same
 * `bg-white dark:bg-gray-800`. If the mascot's KNOCKOUT constant changes, update this too.
 */

import { Link } from 'react-router-dom'
import { Home } from 'lucide-react'
import { Button } from '@/components/ui'
import MascotLogo from '@/components/layout/MascotLogo'

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center animate-rise">
      <div className="lamp-glow mb-6">
        <div className="flex items-center justify-center w-28 h-28 rounded-full bg-white dark:bg-gray-800 ring-1 ring-gray-200 dark:ring-gray-700 shadow-lg shadow-gray-900/10">
          <MascotLogo status="unknown" className="w-20 h-20" />
        </div>
      </div>
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
        This trail's gone cold
      </h1>
      <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-6">
        The page you're after wandered off the path — or never existed. Let's head
        back to the watch.
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
