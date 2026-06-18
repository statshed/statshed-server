/**
 * AIDEV-NOTE: Toast notification context
 * Wraps Sonner's toast functionality for consistent usage across the app
 * Note: We use Sonner directly via its toast() function, so this is minimal
 */

import { toast } from 'sonner'

// Re-export Sonner's toast for direct usage
export { toast }

// Helper functions for common toast patterns
export const showSuccessToast = (message: string, description?: string) => {
  toast.success(message, { description })
}

export const showErrorToast = (message: string, description?: string) => {
  toast.error(message, { description })
}

export const showInfoToast = (message: string, description?: string) => {
  toast.info(message, { description })
}

export const showWarningToast = (message: string, description?: string) => {
  toast.warning(message, { description })
}

// For async operations with loading states
export const showPromiseToast = <T,>(
  promise: Promise<T>,
  messages: {
    loading: string
    success: string | ((data: T) => string)
    error: string | ((error: Error) => string)
  }
) => {
  return toast.promise(promise, messages)
}
