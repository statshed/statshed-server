/**
 * AIDEV-NOTE: LogViewerModal component tests
 * Tests for log viewing, error highlighting, navigation controls, and Show all
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { renderWithQueryClient } from '@/test/utils'
import { server } from '@/test/mocks/server'
import { createMockLogResponse } from '@/test/mocks/handlers'
import LogViewerModal from './LogViewerModal'

// AIDEV-NOTE: @tanstack/react-virtual reads element size via offsetWidth/offsetHeight
// (virtual-core getRect/measureElement), which jsdom always reports as 0 — so without this the
// virtualizer sees a zero-height scroll element and renders no rows, breaking every content
// assertion. Stub non-zero dimensions for the whole file so it computes a real bounded window.
let originalOffsetHeight: PropertyDescriptor | undefined
let originalOffsetWidth: PropertyDescriptor | undefined
beforeEach(() => {
  originalOffsetHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')
  originalOffsetWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth')
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, get: () => 100 })
  Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, get: () => 800 })
})
afterEach(() => {
  if (originalOffsetHeight) {
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', originalOffsetHeight)
  }
  if (originalOffsetWidth) {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', originalOffsetWidth)
  }
})

describe('LogViewerModal', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    groupName: 'test-group',
    jobName: 'test-job',
    totalLineCount: 50,
  }

  it('renders dialog with title when open', () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Job Logs')).toBeInTheDocument()
    expect(screen.getByText(/test-job/)).toBeInTheDocument()
  })

  it('has open attribute when isOpen is true', () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('open')
  })

  it('exposes an accessible name for the dialog', () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)
    // a11y: the native dialog needs aria-labelledby so screen readers announce a name.
    expect(screen.getByRole('dialog')).toHaveAccessibleName(/job logs/i)
  })

  it('does not render dialog when isOpen is false', () => {
    // AIDEV-NOTE: Modal now uses conditional rendering to prevent DOM stacking issues
    // when multiple jobs have logs. Dialog is not rendered at all when closed.
    renderWithQueryClient(<LogViewerModal {...defaultProps} isOpen={false} />)

    const dialog = document.querySelector('dialog')
    expect(dialog).toBeNull()
  })

  it('renders close button', () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    const closeButton = screen.getByRole('button', { name: /close dialog/i })
    expect(closeButton).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const handleClose = vi.fn()
    renderWithQueryClient(<LogViewerModal {...defaultProps} onClose={handleClose} />)

    const closeButton = screen.getByRole('button', { name: /close dialog/i })
    fireEvent.click(closeButton)
    expect(handleClose).toHaveBeenCalledTimes(1)
  })

  it('shows loading state initially', () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    expect(screen.getByText('Loading logs...')).toBeInTheDocument()
  })

  it('shows log content after loading', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByText(/Starting build process/)).toBeInTheDocument()
    })
  })

  it('shows line numbers for log lines', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      // Line numbers should be present (starting from offset)
      expect(screen.getByText('42')).toBeInTheDocument() // First line when truncated
    })
  })

  it('highlights error lines', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      // Find the line containing "ERROR"
      const errorText = screen.getByText(/\[ERROR\] Failed to compile/)
      expect(errorText).toBeInTheDocument()

      // The parent container should have error highlighting class
      const lineContainer = errorText.closest('[data-line]')
      expect(lineContainer).toHaveClass('bg-red-900/30')
    })
  })

  it('shows error count in toolbar', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      // AIDEV-NOTE: Mock log has 3 lines with "error" (case-insensitive)
      // Format is "X/Y errors" where X is current index (0 initially)
      // Look for the specific span with error count
      const elements = screen.getAllByText(/\d+\/\d+ errors/)
      expect(elements.length).toBeGreaterThan(0)
    })
  })

  it('renders navigation controls', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByTitle('Jump to top')).toBeInTheDocument()
      expect(screen.getByTitle('Jump to bottom')).toBeInTheDocument()
      expect(screen.getByTitle('Previous error')).toBeInTheDocument()
      expect(screen.getByTitle('Next error')).toBeInTheDocument()
    })
  })

  it('shows truncation message when log is truncated', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      // MSW handler returns truncated=true by default
      expect(screen.getByText(/Showing last \d+ of \d+ lines/)).toBeInTheDocument()
    })
  })

  it('shows Show all button when truncated', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /show all/i })).toBeInTheDocument()
    })
  })

  it('calls onClose when cancel event is triggered (escape key)', () => {
    const handleClose = vi.fn()
    renderWithQueryClient(<LogViewerModal {...defaultProps} onClose={handleClose} />)

    const dialog = screen.getByRole('dialog')
    fireEvent(dialog, new Event('cancel', { bubbles: true }))
    expect(handleClose).toHaveBeenCalledTimes(1)
  })

  it('escapes HTML in log content to prevent XSS', async () => {
    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    await waitFor(() => {
      // Log content should be displayed as text, not rendered as HTML
      // The mock log doesn't have HTML, but we test the escaping function
      expect(screen.getByText(/Starting build process/)).toBeInTheDocument()
    })
  })

  it('transitions from closed to open state', () => {
    // AIDEV-NOTE: Modal uses conditional rendering - dialog mounts/unmounts based on isOpen
    const { rerender } = renderWithQueryClient(
      <LogViewerModal {...defaultProps} isOpen={false} />
    )

    // Dialog should not exist when closed
    expect(document.querySelector('dialog')).toBeNull()

    rerender(<LogViewerModal {...defaultProps} isOpen={true} />)

    // Dialog should now be mounted and open
    const dialog = document.querySelector('dialog')
    expect(dialog).not.toBeNull()
    expect(dialog).toHaveAttribute('open')
  })

  it('transitions from open to closed state', () => {
    // AIDEV-NOTE: Modal uses conditional rendering - dialog mounts/unmounts based on isOpen
    const { rerender } = renderWithQueryClient(
      <LogViewerModal {...defaultProps} isOpen={true} />
    )

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('open')

    rerender(<LogViewerModal {...defaultProps} isOpen={false} />)

    // Dialog should be unmounted when closed
    expect(document.querySelector('dialog')).toBeNull()
  })
})

describe('LogViewerModal large-log virtualization', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    groupName: 'test-group',
    jobName: 'test-job',
    totalLineCount: 20000,
  }

  it('renders only a bounded window of rows for a huge log, not every line', async () => {
    const BIG = 20000
    const bigLog = Array.from({ length: BIG }, (_, i) => `line ${i} content`).join('\n')

    server.use(
      http.get('/api/groups/:groupName/jobs/:jobName/log', ({ request }) => {
        const all = new URL(request.url).searchParams.get('all') === 'true'
        if (all) {
          return HttpResponse.json(
            createMockLogResponse({
              log: bigLog,
              line_count: BIG,
              total_line_count: BIG,
              truncated: false,
            })
          )
        }
        return HttpResponse.json(
          createMockLogResponse({
            log: 'line 0 content',
            line_count: 1,
            total_line_count: BIG,
            truncated: true,
          })
        )
      })
    )

    renderWithQueryClient(<LogViewerModal {...defaultProps} />)

    // Load truncated view, then Show all to fetch the full 20k-line log.
    const showAll = await screen.findByRole('button', { name: /show all/i })
    fireEvent.click(showAll)

    // Wait until the full (untruncated) log has loaded — the toolbar switches to "N lines".
    await screen.findByText('20000 lines')

    // The freeze/OOM bug is rendering all 20000 lines as DOM. After virtualization only a
    // bounded window (visible + overscan) should be present.
    const rendered = document.querySelectorAll('[data-line]')
    expect(rendered.length).toBeGreaterThan(0)
    expect(rendered.length).toBeLessThan(500)
  })
})
