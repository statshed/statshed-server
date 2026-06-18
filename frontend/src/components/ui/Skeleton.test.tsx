/**
 * AIDEV-NOTE: Skeleton a11y tests.
 * Skeletons were purely visual (no busy/status semantics), so screen-reader users got no
 * indication content was loading. Base Skeleton now exposes a loading status; it can be
 * marked decorative (aria-hidden), and composites expose a single status, not one per line.
 */

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Skeleton, SkeletonCard } from './Skeleton'

describe('Skeleton', () => {
  it('exposes a loading status to assistive tech', () => {
    render(<Skeleton />)
    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('aria-busy', 'true')
    expect(status).toHaveAccessibleName(/loading/i)
  })

  it('can be marked decorative with aria-hidden (no status exposed)', () => {
    render(<Skeleton aria-hidden />)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('SkeletonCard exposes a single loading status, not one per line', () => {
    render(<SkeletonCard />)
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })
})
