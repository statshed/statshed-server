/**
 * AIDEV-NOTE: Dialog component tests
 */

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Dialog from './Dialog'

describe('Dialog', () => {
  it('renders dialog with title and content when open', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <p>Dialog content</p>
      </Dialog>
    )
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Test Dialog')).toBeInTheDocument()
    expect(screen.getByText('Dialog content')).toBeInTheDocument()
  })

  it('has open attribute when isOpen is true', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('open')
  })

  it('does not have open attribute when isOpen is false', () => {
    render(
      <Dialog isOpen={false} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )
    const dialog = document.querySelector('dialog')
    expect(dialog).not.toHaveAttribute('open')
  })

  it('renders close button', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )
    const closeButton = screen.getByRole('button', { name: /close dialog/i })
    expect(closeButton).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const handleClose = vi.fn()
    render(
      <Dialog isOpen={true} onClose={handleClose} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )
    const closeButton = screen.getByRole('button', { name: /close dialog/i })
    fireEvent.click(closeButton)
    expect(handleClose).toHaveBeenCalledTimes(1)
  })

  it('renders children content', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <form>
          <input name="test" placeholder="Test input" />
          <button type="submit">Submit</button>
        </form>
      </Dialog>
    )
    expect(screen.getByPlaceholderText('Test input')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Submit' })).toBeInTheDocument()
  })

  it('applies custom className', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog" className="custom-class">
        <p>Content</p>
      </Dialog>
    )
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveClass('custom-class')
  })

  it('transitions from closed to open state', () => {
    const { rerender } = render(
      <Dialog isOpen={false} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )

    const dialog = document.querySelector('dialog')
    expect(dialog).not.toHaveAttribute('open')

    rerender(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )

    expect(dialog).toHaveAttribute('open')
  })

  it('transitions from open to closed state', () => {
    const { rerender } = render(
      <Dialog isOpen={true} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('open')

    rerender(
      <Dialog isOpen={false} onClose={() => {}} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )

    expect(dialog).not.toHaveAttribute('open')
  })

  it('exposes its title as the dialog accessible name', () => {
    render(
      <Dialog isOpen={true} onClose={() => {}} title="Configure backups">
        <p>Content</p>
      </Dialog>
    )
    // a11y: the native dialog needs aria-labelledby pointing at its title, otherwise
    // screen readers announce it with no name.
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Configure backups')
  })

  it('calls onClose when cancel event is triggered (escape key)', () => {
    const handleClose = vi.fn()
    render(
      <Dialog isOpen={true} onClose={handleClose} title="Test Dialog">
        <p>Content</p>
      </Dialog>
    )

    const dialog = screen.getByRole('dialog')
    fireEvent(dialog, new Event('cancel', { bubbles: true }))
    expect(handleClose).toHaveBeenCalledTimes(1)
  })
})
