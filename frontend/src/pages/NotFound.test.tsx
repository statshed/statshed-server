/**
 * AIDEV-NOTE: NotFound (404) page tests.
 * Unknown URLs previously rendered a blank page (no catch-all route); this page gives a
 * recoverable not-found state with a link back to the dashboard.
 */

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import NotFound from './NotFound'

describe('NotFound', () => {
  it('shows a not-found message and a link back to the dashboard', () => {
    render(
      <MemoryRouter>
        <NotFound />
      </MemoryRouter>
    )

    // AIDEV-NOTE: Copy is intentionally playful ("This trail's gone cold") to match the
    // Lookout theme — assert the recoverable not-found heading + the way back, not exact words.
    expect(screen.getByRole('heading', { name: /trail's gone cold/i })).toBeInTheDocument()
    const link = screen.getByRole('link', { name: /dashboard/i })
    expect(link).toHaveAttribute('href', '/')
  })
})
