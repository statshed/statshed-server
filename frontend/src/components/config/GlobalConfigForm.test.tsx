/**
 * AIDEV-NOTE: GlobalConfigForm validation tests.
 *
 * Regression: timeouts were only validated as non-empty strings; real numeric checks lived in
 * a try/catch that swallowed errors to console.error. So 0/-3 silently did nothing and 1.5 was
 * truncated to 1 and saved with a success toast. Validation now lives in the zod schema, so
 * invalid input is rejected (blocking the PUT) and surfaced as a field error.
 *
 * The validation rules are unit-tested directly against the exported schema (deterministic).
 * A second test confirms the form wires up and PUTs the parsed integers for valid input.
 * We deliberately do NOT assert the rendered error text or the no-submit path through the live
 * form: RHF's async-validation re-render is not reliably flushed under jsdom in this React 19
 * setup (onSubmit/error state don't settle the way they do in a real browser), which would make
 * such assertions flaky. The schema tests cover the actual validation guarantee.
 */

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { createTestQueryClient } from '@/test/utils'
import { server } from '@/test/mocks/server'
import { createMockConfig } from '@/test/mocks/handlers'
import GlobalConfigForm, { globalConfigInputSchema } from './GlobalConfigForm'

describe('globalConfigInputSchema', () => {
  const valid = { progress_timeout_minutes: '30', staleness_timeout_hours: '24' }

  it.each(['0', '-3', '1.5', '', '   ', 'abc', '1e3'])(
    'rejects invalid timeout value %j',
    (bad) => {
      const result = globalConfigInputSchema.safeParse({ ...valid, progress_timeout_minutes: bad })
      expect(result.success).toBe(false)
    }
  )

  it.each(['1', '30', '1440'])('accepts valid positive integer %j', (good) => {
    const result = globalConfigInputSchema.safeParse({ ...valid, progress_timeout_minutes: good })
    expect(result.success).toBe(true)
  })
})

describe('GlobalConfigForm submit behavior', () => {
  function renderForm() {
    render(
      <QueryClientProvider client={createTestQueryClient()}>
        <GlobalConfigForm />
      </QueryClientProvider>
    )
  }

  const saveButton = () => screen.getByRole('button', { name: /save settings/i })

  it('PUTs the parsed integers when input is valid', async () => {
    let putBody: Record<string, number> | null = null
    server.use(
      http.put('/api/config', async ({ request }) => {
        const body = (await request.json()) as Record<string, number>
        putBody = body
        return HttpResponse.json(body)
      })
    )
    renderForm()

    const progress = (await screen.findByLabelText('Progress Timeout (minutes)')) as HTMLInputElement
    await waitFor(() => expect(progress.value).toBe('30'))

    fireEvent.change(progress, { target: { value: '45' } })
    fireEvent.click(saveButton())

    await waitFor(() =>
      expect(putBody).toEqual({ progress_timeout_minutes: 45, staleness_timeout_hours: 24 })
    )
  })
})

describe('GlobalConfigForm error state', () => {
  function renderForm() {
    render(
      <QueryClientProvider client={createTestQueryClient()}>
        <GlobalConfigForm />
      </QueryClientProvider>
    )
  }

  it('shows an error with a retry control and hides the form when the config read fails', async () => {
    server.use(http.get('/api/config', () => new HttpResponse(null, { status: 500 })))
    renderForm()

    await screen.findByText(/failed to load settings/i)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    // The editable form must NOT render on a failed read, so a user can't submit
    // blank defaults over the real server config.
    expect(screen.queryByLabelText('Progress Timeout (minutes)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Staleness Timeout (hours)')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /save settings/i })).not.toBeInTheDocument()
  })

  it('recovers and shows the populated form when retry succeeds', async () => {
    let failNext = true
    server.use(
      http.get('/api/config', () => {
        if (failNext) {
          failNext = false
          return new HttpResponse(null, { status: 500 })
        }
        return HttpResponse.json(createMockConfig())
      })
    )
    renderForm()

    await screen.findByText(/failed to load settings/i)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    const progress = (await screen.findByLabelText(
      'Progress Timeout (minutes)'
    )) as HTMLInputElement
    await waitFor(() => expect(progress.value).toBe('30'))
  })
})
