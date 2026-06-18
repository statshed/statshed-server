/**
 * AIDEV-NOTE: Theme toggle component
 * Cycles through light -> dark -> system modes
 * Displays appropriate icon for current theme state
 */

import { Sun, Moon, Monitor } from 'lucide-react'
import { useTheme } from '@/contexts/ThemeContext'
import { cn } from '@/lib/utils'

const THEME_CYCLE = ['light', 'dark', 'system'] as const

export default function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme()

  const cycleTheme = () => {
    const currentIndex = THEME_CYCLE.indexOf(theme)
    const nextIndex = (currentIndex + 1) % THEME_CYCLE.length
    setTheme(THEME_CYCLE[nextIndex])
  }

  const getIcon = () => {
    if (theme === 'system') {
      return <Monitor className="w-5 h-5" />
    }
    if (theme === 'dark') {
      return <Moon className="w-5 h-5" />
    }
    return <Sun className="w-5 h-5" />
  }

  const getLabel = () => {
    if (theme === 'system') {
      return `System (${resolvedTheme})`
    }
    return theme === 'dark' ? 'Dark' : 'Light'
  }

  return (
    <button
      onClick={cycleTheme}
      className={cn(
        'p-2 rounded-lg transition-colors',
        'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
        'hover:bg-gray-100 dark:hover:bg-gray-700'
      )}
      title={`Theme: ${getLabel()}. Click to change.`}
      aria-label={`Current theme: ${getLabel()}. Click to change theme.`}
    >
      {getIcon()}
    </button>
  )
}
