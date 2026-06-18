/**
 * AIDEV-NOTE: GroupConfigForm error-state tests.
 *
 * Regression: the form called useGroupConfig itself but ignored isError. On a failed
 * GET /groups/:name/config it fell through to the editable form with hardcoded defaults
 * (24h expiration), and Save was enabled — so a user could overwrite the real server
 * config with defaults. It now shows an error + Try Again and hides the form/Save.
 *
 * Per the file-wide convention (see GlobalConfigForm.test.tsx), we assert presence/absence
 * of error-card elements and recovery, not RHF async-validation text (flaky under React 19).
 */

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { createTestQueryClient } from '@/test/utils'
import { server } from '@/test/mocks/server'
import { createMockGroupConfig } from '@/test/mocks/handlers'
import GroupConfigForm from './GroupConfigForm'

function renderForm() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <GroupConfigForm groupName="backups" isOpen onClose={vi.fn()} />
    </QueryClientProvider>
  )
}

describe('GroupConfigForm error state', () => {
  it('shows an error + retry and hides the form/Save when the config read fails', async () => {
    server.use(
      http.get('/api/groups/:name/config', () => new HttpResponse(null, { status: 500 }))
    )
    renderForm()

    await screen.findByText(/failed to load group configuration/i)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    // Critical: no Save on a failed read — prevents overwriting real config with defaults.
    expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/expiration timeout/i)).not.toBeInTheDocument()
  })

  it('recovers and shows the form when retry succeeds', async () => {
    let failNext = true
    server.use(
      http.get('/api/groups/:name/config', () => {
        if (failNext) {
          failNext = false
          return new HttpResponse(null, { status: 500 })
        }
        return HttpResponse.json(createMockGroupConfig())
      })
    )
    renderForm()

    await screen.findByText(/failed to load group configuration/i)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    expect(
      await screen.findByRole('button', { name: /save changes/i })
    ).toBeInTheDocument()
  })
})
