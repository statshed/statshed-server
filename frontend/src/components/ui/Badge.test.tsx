/**
 * AIDEV-NOTE: Badge component tests
 */

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Badge, JobStatusBadge } from './Badge'

describe('Badge', () => {
  it('renders children correctly', () => {
    render(<Badge>Test Badge</Badge>)
    expect(screen.getByText('Test Badge')).toBeInTheDocument()
  })

  it('applies variant classes correctly', () => {
    const { rerender } = render(<Badge variant="success">Success</Badge>)
    expect(screen.getByText('Success')).toHaveClass('bg-green-100')

    rerender(<Badge variant="error">Error</Badge>)
    expect(screen.getByText('Error')).toHaveClass('bg-red-100')

    rerender(<Badge variant="warning">Warning</Badge>)
    expect(screen.getByText('Warning')).toHaveClass('bg-orange-100')

    rerender(<Badge variant="progress">Progress</Badge>)
    expect(screen.getByText('Progress')).toHaveClass('bg-blue-100')
  })

  it('applies neutral variant by default', () => {
    render(<Badge>Neutral</Badge>)
    expect(screen.getByText('Neutral')).toHaveClass('bg-gray-100')
  })
})

describe('JobStatusBadge', () => {
  it('renders status label correctly', () => {
    render(<JobStatusBadge status="success" />)
    expect(screen.getByText('Success')).toBeInTheDocument()
  })

  it('renders all status types', () => {
    const { rerender } = render(<JobStatusBadge status="error" />)
    expect(screen.getByText('Error')).toBeInTheDocument()

    rerender(<JobStatusBadge status="progress" />)
    expect(screen.getByText('In Progress')).toBeInTheDocument()

    rerender(<JobStatusBadge status="timeout" />)
    expect(screen.getByText('Timeout')).toBeInTheDocument()

    rerender(<JobStatusBadge status="stale" />)
    expect(screen.getByText('Stale')).toBeInTheDocument()
  })
})
