/**
 * AIDEV-NOTE: Button component
 * Variants: primary, secondary, ghost, danger
 * Sizes: sm, md, lg
 */

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  isLoading?: boolean
  children: ReactNode
}

const variantClasses: Record<ButtonVariant, string> = {
  // AIDEV-NOTE: hover DARKENS to primary-700 (keeps white text ≥ AA). Never hover to
  // primary-500 here — white-on-500 is ~2.8:1 and fails contrast.
  primary:
    'bg-primary-600 text-white shadow-sm shadow-primary-900/20 hover:bg-primary-700 hover:shadow-md hover:shadow-primary-600/30 focus:ring-primary-600 dark:focus:ring-primary-400 disabled:bg-primary-400 disabled:shadow-none',
  // AIDEV-NOTE: all variants share the amber focus ring (primary-600 light / primary-400 dark)
  // — gray rings (gray-400/500) are < 3:1 on the warm-light surfaces and fail the focus-
  // indicator contrast threshold once the browser outline is removed (see base classes).
  secondary:
    'bg-gray-100 text-gray-800 border border-gray-200 hover:bg-gray-200 hover:border-gray-300 focus:ring-primary-600 dark:focus:ring-primary-400 dark:bg-gray-700 dark:text-gray-100 dark:border-gray-600 dark:hover:bg-gray-600',
  ghost:
    'text-gray-600 hover:bg-gray-100 hover:text-gray-900 focus:ring-primary-600 dark:focus:ring-primary-400 dark:text-gray-300 dark:hover:bg-gray-700/60 dark:hover:text-white',
  // AIDEV-NOTE: like primary, danger hover DARKENS (red-700) — hovering to red-500 is
  // ~3.8:1 white-on-red and fails AA.
  danger:
    'bg-red-600 text-white shadow-sm shadow-red-900/20 hover:bg-red-700 hover:shadow-md hover:shadow-red-600/30 focus:ring-red-500 disabled:bg-red-400 disabled:shadow-none',
}

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-3 text-base',
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'primary',
    size = 'md',
    isLoading = false,
    disabled,
    className,
    children,
    ...props
  },
  ref
) {
  return (
    <button
      ref={ref}
      disabled={disabled || isLoading}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-all duration-150 ease-out',
        'focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-900',
        'motion-safe:active:scale-[0.98]',
        'disabled:cursor-not-allowed disabled:opacity-60 disabled:active:scale-100',
        variantClasses[variant],
        sizeClasses[size],
        className
      )}
      {...props}
    >
      {isLoading && (
        <svg
          className="animate-spin h-4 w-4"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      )}
      {children}
    </button>
  )
})

export default Button
