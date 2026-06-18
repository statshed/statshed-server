/**
 * AIDEV-NOTE: Error boundary component
 * Catches React errors and displays fallback UI
 * Error messages are sanitized to avoid leaking sensitive info
 */

import { Component, type ReactNode, type ErrorInfo } from 'react'
import { Button } from '@/components/ui'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  errorId?: string
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(): State {
    // AIDEV-NOTE: Generate unique error ID for logging/support reference
    // Don't store the actual error in state to avoid leaking it to the UI
    const errorId = Math.random().toString(36).substring(2, 10).toUpperCase()
    return { hasError: true, errorId }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Log full error details to console for debugging (not shown to users)
    console.error('Error boundary caught an error:', error, errorInfo)
  }

  handleRetry = () => {
    this.setState({ hasError: false, errorId: undefined })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback
      }

      return (
        <div className="flex flex-col items-center justify-center p-8 text-center">
          <div className="text-red-500 mb-4">
            <svg
              className="w-12 h-12 mx-auto"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            Something went wrong
          </h2>
          <p className="text-gray-600 dark:text-gray-400 mb-4 max-w-md">
            An unexpected error occurred. Please try again or contact support.
          </p>
          {this.state.errorId && (
            <p className="text-xs text-gray-500 dark:text-gray-500 mb-4">
              Error ID: {this.state.errorId}
            </p>
          )}
          <Button onClick={this.handleRetry}>Try Again</Button>
        </div>
      )
    }

    return this.props.children
  }
}
