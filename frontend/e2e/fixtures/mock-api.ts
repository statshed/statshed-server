/**
 * AIDEV-NOTE: Hermetic API mocking for the e2e suite.
 *
 * Installs Playwright request interception so every `/api/*` call the app makes
 * is answered from the deterministic fixtures in mock-data.ts — no backend, no
 * DB seeding, no internal DNS. Socket.IO has no server in this environment, so we
 * abort its handshake; the app already tolerates a disconnected socket (it just
 * shows the "disconnected" indicator and relies on its polling/mount fetches).
 *
 * Unmocked endpoints return 501 with a descriptive body so a forgotten mock fails
 * loudly in the test instead of silently hanging on the (absent) dev proxy target.
 */

import type { Page, Route, Request } from '@playwright/test'
import type { Config, GroupConfig } from '../../src/types'
import * as data from './mock-data'

/**
 * Per-page mutable state so writes persist within a single test (e.g. saving the
 * settings form, then the form's refetch reads the saved value back). Scoped per
 * mockApi() call — each test gets a fresh page, hence fresh state, with no leakage.
 */
interface MockState {
  config: Config
  groupConfigs: Map<string, GroupConfig>
}

function json(route: Route, body: unknown, status = 200): Promise<void> {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}

function postBody(request: Request): Record<string, unknown> {
  try {
    return (request.postDataJSON() as Record<string, unknown>) ?? {}
  } catch {
    return {}
  }
}

async function handle(route: Route, request: Request, state: MockState): Promise<void> {
  const method = request.method()
  const url = new URL(request.url())
  // The client prepends `/api`; strip it and split the rest into decoded segments.
  const path = url.pathname.replace(/^\/api/, '')
  const seg = path.split('/').filter(Boolean).map(decodeURIComponent)

  // --- Reads --------------------------------------------------------------
  if (method === 'GET') {
    if (path === '/health') return json(route, data.healthSummary)
    if (path === '/groups') return json(route, { groups: data.groups })
    if (path === '/config') return json(route, state.config)
    if (seg[0] === 'groups' && seg[2] === 'jobs' && seg.length === 3) {
      return json(route, { group: data.groupStub(seg[1]), jobs: data.jobsForGroup(seg[1]) })
    }
    if (seg[0] === 'groups' && seg[2] === 'config') {
      return json(route, state.groupConfigs.get(seg[1]) ?? data.groupConfig())
    }
    if (seg[0] === 'groups' && seg[2] === 'jobs' && seg[4] === 'log') {
      return json(route, data.logResponse())
    }
    if (seg[0] === 'jobs' && seg.length === 1) {
      const statusParam = url.searchParams.get('status')
      const statuses = statusParam ? statusParam.split(',') : []
      return json(route, data.jobsByStatus(statuses))
    }
  }

  // --- Mutations (echo success) ------------------------------------------
  if (method === 'POST' && path === '/status') {
    return json(route, jobFromStatusPayload(postBody(request)))
  }
  if (method === 'POST' && seg[0] === 'jobs' && seg[2] === 'ack') {
    return json(route, { job: { ...data.jobsForGroup('backups')[1], acked: true, acked_at: '2026-06-14T12:00:00.000Z' } })
  }
  if (method === 'POST' && seg[0] === 'groups' && seg[2] === 'ack') {
    return json(route, { acked_count: 1, group: seg[1] })
  }
  if (method === 'POST' && path === '/ack-all') {
    return json(route, { acked_count: 1 })
  }
  if (method === 'DELETE' && seg[0] === 'jobs') {
    const deleted = data.jobsForGroup('backups')[1]
    return json(route, {
      deleted_job: deleted,
      group_id: deleted.group_id,
      group_name: deleted.group_name,
    })
  }
  if (method === 'PUT' && path === '/config') {
    state.config = { ...state.config, ...postBody(request) } as Config
    return json(route, state.config)
  }
  if (method === 'PUT' && seg[0] === 'groups' && seg[2] === 'config') {
    const current = state.groupConfigs.get(seg[1]) ?? data.groupConfig()
    const updated = { ...current, ...postBody(request) } as GroupConfig
    state.groupConfigs.set(seg[1], updated)
    return json(route, updated)
  }

  return json(route, { error: `Unmocked endpoint: ${method} ${path}` }, 501)
}

function jobFromStatusPayload(body: Record<string, unknown>) {
  const base = data.jobsForGroup(String(body.group ?? 'backups'))[0]
  return {
    ...base,
    name: String(body.job ?? base.name),
    status: (body.status as string) ?? base.status,
    message: (body.message as string | undefined) ?? null,
  }
}

export async function mockApi(page: Page): Promise<void> {
  const state: MockState = {
    config: { ...data.config },
    groupConfigs: new Map(),
  }

  // AIDEV-NOTE: Match by pathname PREFIX, not a '**/api/**' glob. Under the Vite
  // dev server the app's own ES modules are served from paths like
  // `/src/api/client.ts`, which a loose `**/api/**` glob would wrongly intercept
  // (returning 501 and preventing the app from mounting). The client only ever
  // fetches `/api/...` and Socket.IO only hits `/socket.io/...`.
  await page.route(
    (url) => url.pathname.startsWith('/socket.io'),
    (route) => route.abort()
  )
  await page.route(
    (url) => url.pathname.startsWith('/api/'),
    (route, request) => handle(route, request, state)
  )
}
