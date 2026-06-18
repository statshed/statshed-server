/**
 * AIDEV-NOTE: Log viewer modal component
 * Displays job log content with error highlighting and navigation controls.
 *
 * Features:
 * - Monospace font with line numbers
 * - Highlight lines containing "error" (case-insensitive)
 * - Jump to next/prev error, top, bottom
 * - Default: last 1000 lines; "Show all" loads full log
 * - ANSI control sequence stripping
 * - React's built-in text escaping provides XSS safety
 *
 * AIDEV-NOTE: Modal visibility is controlled by conditional rendering (return null when !isOpen).
 * Native <dialog> elements remain in DOM even when close() is called, causing stacking issues
 * when multiple modals exist (one per job with logs). By returning null, we ensure only one
 * dialog is in the DOM at a time.
 */

import { useState, useRef, useCallback, useEffect, useId, useMemo } from 'react'
import {
  X,
  ChevronUp,
  ChevronDown,
  ArrowUp,
  ArrowDown,
  FileText,
  AlertCircle,
  Loader2,
} from 'lucide-react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui'
import { useJobLog } from '@/hooks'

interface LogViewerModalProps {
  isOpen: boolean
  onClose: () => void
  groupName: string
  jobName: string
  /** Total line count from job metadata (may differ from stored if truncated at upload) */
  totalLineCount?: number | null
}

// AIDEV-NOTE: Strip ANSI escape sequences for clean display
// Matches CSI sequences (colors, cursor, etc.) and OSC sequences.
// The \x1b / \x07 control characters are intentional (they ARE the ANSI escape
// bytes we strip), so the no-control-regex rule is disabled for this line only.
// eslint-disable-next-line no-control-regex
const ANSI_REGEX = /\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[PX^_][^\x1b]*\x1b\\|\x1b[@-Z\\-_]/g

function stripAnsi(text: string): string {
  return text.replace(ANSI_REGEX, '')
}

// AIDEV-NOTE: Check if a line contains "error" (case-insensitive)
// Used for error highlighting and navigation
function isErrorLine(line: string): boolean {
  return /error/i.test(line)
}

export default function LogViewerModal({
  isOpen,
  onClose,
  groupName,
  jobName,
  totalLineCount,
}: LogViewerModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const logContainerRef = useRef<HTMLDivElement>(null)
  // AIDEV-NOTE: Link the dialog to its title for an accessible name (screen readers).
  const titleId = useId()
  const [showAll, setShowAll] = useState(false)
  const [currentErrorIndex, setCurrentErrorIndex] = useState(-1)

  // AIDEV-NOTE: Reset state when job changes to avoid
  // fetching full logs by default on subsequent opens (perf/cost risk)
  useEffect(() => {
    setShowAll(false)
    setCurrentErrorIndex(-1)
  }, [groupName, jobName])

  // Fetch log content - only enabled when modal is open
  const {
    data: logData,
    isLoading,
    error,
    refetch,
  } = useJobLog({
    groupName,
    jobName,
    all: showAll,
    enabled: isOpen,
  })

  // Process log lines
  const { lines, errorLineIndices } = useMemo(() => {
    if (!logData?.log) return { lines: [], errorLineIndices: [] }

    const cleanLog = stripAnsi(logData.log)
    const splitLines = cleanLog.split('\n')
    // Remove trailing empty line if present
    if (splitLines.length > 0 && splitLines[splitLines.length - 1] === '') {
      splitLines.pop()
    }

    const errorIndices: number[] = []
    splitLines.forEach((line, index) => {
      if (isErrorLine(line)) {
        errorIndices.push(index)
      }
    })

    return { lines: splitLines, errorLineIndices: errorIndices }
  }, [logData?.log])

  // Reset error index when log changes
  useEffect(() => {
    setCurrentErrorIndex(-1)
  }, [lines])

  // AIDEV-NOTE: Virtualize the line list so "Show all" on a huge log renders only the visible
  // window (+overscan), not one DOM row per line — rendering every line froze/OOM'd the tab.
  // Rows keep wrapping (variable height), so measureElement dynamically measures each rendered
  // row; estimateSize is just the initial guess. Declared before the early return below to
  // satisfy the Rules of Hooks.
  const rowVirtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => logContainerRef.current,
    estimateSize: () => 24,
    overscan: 20,
  })

  // Dialog open/close handling - only runs when isOpen is true (since we return null otherwise)
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog || !isOpen) return

    if (!dialog.open) {
      try {
        dialog.showModal()
      } catch {
        // Dialog may already be open
      }
    }
  }, [isOpen])

  // Handle escape key
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog || !isOpen) return

    const handleCancel = (e: Event) => {
      e.preventDefault()
      onClose()
    }

    dialog.addEventListener('cancel', handleCancel)
    return () => dialog.removeEventListener('cancel', handleCancel)
  }, [onClose, isOpen])

  // Scroll to a specific line
  // AIDEV-NOTE: Scroll via the virtualizer — under virtualization the target row may be
  // outside the rendered window (not in the DOM), so a querySelector would silently no-op.
  const scrollToLine = useCallback(
    (lineIndex: number) => {
      rowVirtualizer.scrollToIndex(lineIndex, { align: 'center' })
    },
    [rowVirtualizer]
  )

  // Jump to top
  const jumpToTop = useCallback(() => {
    logContainerRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
    setCurrentErrorIndex(-1)
  }, [])

  // Jump to bottom
  const jumpToBottom = useCallback(() => {
    const container = logContainerRef.current
    if (container) {
      container.scrollTo({ top: container.scrollHeight, behavior: 'smooth' })
    }
    setCurrentErrorIndex(-1)
  }, [])

  // Jump to next error
  const jumpToNextError = useCallback(() => {
    if (errorLineIndices.length === 0) return

    const nextIndex =
      currentErrorIndex < errorLineIndices.length - 1 ? currentErrorIndex + 1 : 0
    setCurrentErrorIndex(nextIndex)
    scrollToLine(errorLineIndices[nextIndex])
  }, [errorLineIndices, currentErrorIndex, scrollToLine])

  // Jump to previous error
  const jumpToPrevError = useCallback(() => {
    if (errorLineIndices.length === 0) return

    const prevIndex =
      currentErrorIndex > 0 ? currentErrorIndex - 1 : errorLineIndices.length - 1
    setCurrentErrorIndex(prevIndex)
    scrollToLine(errorLineIndices[prevIndex])
  }, [errorLineIndices, currentErrorIndex, scrollToLine])

  // Handle "Show all" click
  // AIDEV-NOTE: No need to call refetch() - query will refetch automatically
  // when showAll changes because it's part of the query key
  const handleShowAll = useCallback(() => {
    setShowAll(true)
  }, [])

  // Backdrop click handler
  const handleBackdropClick = useCallback(
    (e: React.MouseEvent<HTMLDialogElement>) => {
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
    },
    [onClose]
  )

  // AIDEV-NOTE: Early return AFTER all hooks to comply with Rules of Hooks.
  // This prevents the <dialog> element from being in the DOM when not needed,
  // avoiding stacking issues when multiple jobs have logs.
  if (!isOpen) {
    return null
  }

  const isTruncated = logData?.truncated ?? false
  const displayedLineCount = logData?.line_count ?? 0
  const totalStoredLines = logData?.total_line_count ?? totalLineCount ?? 0

  return (
    <dialog
      ref={dialogRef}
      onClick={handleBackdropClick}
      aria-labelledby={titleId}
      className={cn(
        'backdrop:bg-black/50 backdrop:backdrop-blur-sm',
        'bg-white dark:bg-gray-900 rounded-xl shadow-2xl',
        'p-0 w-[90vw] max-w-4xl max-h-[85vh] mx-auto',
        'border border-gray-200 dark:border-gray-700',
        'flex flex-col overflow-hidden'
      )}
    >
      {/* Header with gradient */}
      <div
        className={cn(
          'flex items-center justify-between px-5 py-4',
          'bg-gradient-to-r from-primary-50 to-primary-100',
          'dark:from-primary-900/30 dark:to-primary-800/20',
          'border-b border-gray-200 dark:border-gray-700'
        )}
      >
        <div className="flex items-center gap-3">
          <div className="p-2 bg-primary-100 dark:bg-primary-800/40 rounded-lg">
            <FileText className="h-5 w-5 text-primary-600 dark:text-primary-400" />
          </div>
          <div>
            <h2
              id={titleId}
              className="text-lg font-semibold text-gray-900 dark:text-white"
            >
              Job Logs
            </h2>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              {jobName} &middot; Here's what happened
            </p>
          </div>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          className="p-2"
          aria-label="Close dialog"
        >
          <X className="h-5 w-5" />
        </Button>
      </div>

      {/* Toolbar */}
      <div
        className={cn(
          'flex items-center justify-between px-4 py-2',
          'bg-gray-50 dark:bg-gray-800/50',
          'border-b border-gray-200 dark:border-gray-700'
        )}
      >
        <div className="flex items-center gap-2">
          {/* Navigation controls */}
          <div className="flex items-center gap-1 border-r border-gray-300 dark:border-gray-600 pr-2 mr-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={jumpToTop}
              disabled={isLoading || lines.length === 0}
              title="Jump to top"
              className="p-1.5"
            >
              <ArrowUp className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={jumpToBottom}
              disabled={isLoading || lines.length === 0}
              title="Jump to bottom"
              className="p-1.5"
            >
              <ArrowDown className="h-4 w-4" />
            </Button>
          </div>

          {/* Error navigation */}
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={jumpToPrevError}
              disabled={isLoading || errorLineIndices.length === 0}
              title="Previous error"
              className="p-1.5"
            >
              <ChevronUp className="h-4 w-4 text-red-500" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={jumpToNextError}
              disabled={isLoading || errorLineIndices.length === 0}
              title="Next error"
              className="p-1.5"
            >
              <ChevronDown className="h-4 w-4 text-red-500" />
            </Button>
            {errorLineIndices.length > 0 && (
              <span className="text-xs text-red-600 dark:text-red-400 ml-1">
                {currentErrorIndex >= 0 ? currentErrorIndex + 1 : 0}/{errorLineIndices.length} errors
              </span>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          {isTruncated && !showAll && (
            <>
              <span>Showing last {displayedLineCount} of {totalStoredLines} lines</span>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleShowAll}
                disabled={isLoading}
                className="text-xs py-1 px-2"
              >
                Show all
              </Button>
            </>
          )}
          {!isTruncated && lines.length > 0 && (
            <span>{displayedLineCount} lines</span>
          )}
        </div>
      </div>

      {/* Log content */}
      <div
        ref={logContainerRef}
        className={cn(
          'flex-1 overflow-auto p-4',
          'bg-gray-900 dark:bg-gray-950',
          // Subtle glow effect
          'shadow-[inset_0_0_30px_rgba(59,130,246,0.05)]'
        )}
      >
        {isLoading && (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-gray-400">
            <Loader2 className="h-8 w-8 animate-spin" />
            <p className="text-sm">Loading logs...</p>
          </div>
        )}

        {error && (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-red-400">
            <AlertCircle className="h-8 w-8" />
            <p className="text-sm">Failed to load logs</p>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => refetch()}
              className="mt-2"
            >
              Retry
            </Button>
          </div>
        )}

        {!isLoading && !error && lines.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-gray-400">
            <FileText className="h-8 w-8" />
            <p className="text-sm">No log content available</p>
          </div>
        )}

        {!isLoading && !error && lines.length > 0 && (
          <pre
            className="font-mono text-sm leading-relaxed relative"
            style={{ height: rowVirtualizer.getTotalSize(), width: '100%' }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const index = virtualRow.index
              const line = lines[index]
              // AIDEV-NOTE: Clamp baseLine to 0 minimum to handle metadata mismatch
              const baseLine = isTruncated && !showAll ? Math.max(0, totalStoredLines - lines.length) : 0
              const lineNumber = baseLine + index + 1
              const hasError = isErrorLine(line)

              return (
                <div
                  key={virtualRow.key}
                  data-line={index}
                  // AIDEV-NOTE: data-index + measureElement let the virtualizer measure each
                  // (variable-height, wrapped) row; data-line is kept for the existing selector.
                  data-index={index}
                  ref={rowVirtualizer.measureElement}
                  className={cn(
                    'flex hover:bg-white/5 transition-colors absolute left-0 top-0 w-full',
                    hasError && 'bg-red-900/30 hover:bg-red-900/40'
                  )}
                  style={{ transform: `translateY(${virtualRow.start}px)` }}
                >
                  <span
                    className={cn(
                      'select-none text-right pr-4 min-w-[4ch]',
                      'text-gray-500 dark:text-gray-600',
                      hasError && 'text-red-400 dark:text-red-500'
                    )}
                  >
                    {lineNumber}
                  </span>
                  <span
                    className={cn(
                      'flex-1 text-gray-200 dark:text-gray-300 whitespace-pre-wrap break-all',
                      hasError && 'text-red-200 dark:text-red-300'
                    )}
                  >
                    {/* AIDEV-NOTE: React auto-escapes text content, providing XSS safety */}
                    {line}
                  </span>
                </div>
              )
            })}
          </pre>
        )}
      </div>

      {/* Footer with truncation warning */}
      {logData?.truncated && !showAll && (
        <div
          className={cn(
            'px-4 py-2 text-xs text-center',
            'bg-amber-50 dark:bg-amber-900/20',
            'text-amber-700 dark:text-amber-400',
            'border-t border-amber-200 dark:border-amber-800'
          )}
        >
          Showing last {displayedLineCount} lines. Click "Show all" to view the complete log.
        </div>
      )}
    </dialog>
  )
}
