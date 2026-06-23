/**
 * AIDEV-NOTE: Dialog component
 * Modal wrapper using native dialog element
 */

import {
  useRef,
  useEffect,
  useId,
  type ReactNode,
  type MouseEvent,
} from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import Button from './Button'

interface DialogProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: ReactNode
  className?: string
}

export default function Dialog({
  isOpen,
  onClose,
  title,
  children,
  className,
}: DialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  // AIDEV-NOTE: Link the dialog to its title so screen readers announce an accessible name.
  const titleId = useId()

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return

    // AIDEV-NOTE: Guard against DOMException when dialog is already open/closed
    // This can happen in React StrictMode or with rapid state toggles
    if (isOpen && !dialog.open) {
      try {
        dialog.showModal()
      } catch {
        // Dialog may already be open in some edge cases
      }
    } else if (!isOpen && dialog.open) {
      dialog.close()
    }
  }, [isOpen])

  // Handle backdrop click
  const handleBackdropClick = (e: MouseEvent<HTMLDialogElement>) => {
    const dialog = dialogRef.current
    if (!dialog) return

    const rect = dialog.getBoundingClientRect()
    const isInDialog =
      e.clientX >= rect.left &&
      e.clientX <= rect.right &&
      e.clientY >= rect.top &&
      e.clientY <= rect.bottom

    if (!isInDialog) {
      onClose()
    }
  }

  // Handle escape key
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return

    const handleCancel = (e: Event) => {
      e.preventDefault()
      onClose()
    }

    dialog.addEventListener('cancel', handleCancel)
    return () => dialog.removeEventListener('cancel', handleCancel)
  }, [onClose])

  return (
    <dialog
      ref={dialogRef}
      onClick={handleBackdropClick}
      aria-labelledby={titleId}
      className={cn(
        // AIDEV-NOTE: `dialog-animate` (index.css) plays a gentle entrance when the [open]
        // attribute is set by showModal() — reduced-motion users get it instantly.
        'dialog-animate',
        'backdrop:bg-gray-950/60 backdrop:backdrop-blur-sm',
        'bg-white dark:bg-gray-800 rounded-2xl shadow-2xl shadow-gray-950/40',
        'p-0 max-w-lg w-full mx-auto',
        'border border-gray-200 dark:border-gray-700',
        className
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h2
          id={titleId}
          className="text-lg font-semibold text-gray-900 dark:text-white"
        >
          {title}
        </h2>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          className="p-1"
          aria-label="Close dialog"
        >
          <X className="h-5 w-5" />
        </Button>
      </div>

      {/* Content */}
      <div className="p-4">{children}</div>
    </dialog>
  )
}
