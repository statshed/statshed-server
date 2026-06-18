/**
 * AIDEV-NOTE: apiRequest error-handling tests.
 *
 * Regression: the client read only `errorData.error` (often a machine code like "not_found"),
 * so user-facing toasts/error screens showed codes instead of the backend's human `message`.
 * The client now prefers `message`, then `error`, then a status fallback.
 */

import { describe, it, expect } from 'vitest'
import { http, HttpResponse } from 'msw'
import { server } from '@/test/mocks/server'
import { api, ApiError } from './client'

describe('apiRequest error message extraction', () => {
  it('prefers the human-readable `message` over the machine `error` code', async () => {
    server.use(
      http.get('/api/test-error', () =>
        HttpResponse.json({ message: 'Group already exists', error: 'conflict' }, { status: 409 })
      )
    )

    await expect(api.get('/test-error')).rejects.toMatchObject({
      message: 'Group already exists',
      status: 409,
    })
  })

  it('falls back to `error` when there is no `message`', async () => {
    server.use(
      http.get('/api/test-error', () =>
        HttpResponse.json({ error: 'Group not found' }, { status: 404 })
      )
    )

    await expect(api.get('/test-error')).rejects.toMatchObject({
      message: 'Group not found',
      status: 404,
    })
  })

  it('falls back to a status message when the body has neither key', async () => {
    server.use(http.get('/api/test-error', () => new HttpResponse(null, { status: 500 })))

    await expect(api.get('/test-error')).rejects.toMatchObject({
      message: 'Request failed with status 500',
      status: 500,
    })
  })

  it('ignores non-string message/error values and uses the status fallback', async () => {
    server.use(
      http.get('/api/test-error', () =>
        HttpResponse.json({ message: { nested: true }, error: 42 }, { status: 400 })
      )
    )

    await expect(api.get('/test-error')).rejects.toMatchObject({
      message: 'Request failed with status 400',
      status: 400,
    })
  })

  it('throws an ApiError instance carrying the raw body as data', async () => {
    server.use(
      http.get('/api/test-error', () =>
        HttpResponse.json({ error: 'boom', detail: 'x' }, { status: 400 })
      )
    )

    await expect(api.get('/test-error')).rejects.toBeInstanceOf(ApiError)
    await expect(api.get('/test-error')).rejects.toMatchObject({
      data: { error: 'boom', detail: 'x' },
    })
  })
})

describe('apiRequest success-body parsing', () => {
  // Regression: a 2xx whose body fails to parse (HTML error page, truncated JSON)
  // was caught and reported as the status-0 "network error", hiding that the request
  // actually reached the server. It must surface with the real HTTP status instead.
  it('reports a malformed 2xx body as a server error carrying the real status', async () => {
    server.use(
      http.get(
        '/api/bad-json',
        () =>
          new HttpResponse('<html>not json</html>', {
            status: 200,
            headers: { 'Content-Type': 'text/html' },
          })
      )
    )

    await expect(api.get('/bad-json')).rejects.toMatchObject({
      message: 'Received a malformed response from the server',
      status: 200,
    })
  })

  it('still reports a fetch-level failure as a status-0 network error', async () => {
    server.use(http.get('/api/network-down', () => HttpResponse.error()))

    await expect(api.get('/network-down')).rejects.toMatchObject({ status: 0 })
  })

  it('classifies a body stream that fails after headers as a network error, not malformed 2xx', async () => {
    // Headers arrive (200), then the body stream errors mid-flight — a transport failure
    // (TypeError), not invalid JSON (SyntaxError). It must NOT be reported as a malformed
    // 2xx with status 200.
    server.use(
      http.get('/api/stream-fail', () => {
        const stream = new ReadableStream({
          start(controller) {
            controller.enqueue(new TextEncoder().encode('{"partial":'))
            controller.error(new Error('connection reset'))
          },
        })
        return new HttpResponse(stream, {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      })
    )

    const err = await api.get('/stream-fail').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).not.toBe(200)
  })
})
