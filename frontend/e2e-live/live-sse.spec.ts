/**
 * AIDEV-NOTE: Executable, NON-MOCKED live SSE gate (Task 5.5, spec §7.4/§14.2).
 *
 * Unlike the hermetic e2e/ suite (which aborts /api/events), this drives a REAL Go server
 * through the Vite dev proxy (:7827 -> :7828) and proves the two failure modes the mocked
 * suites cannot:
 *   1. Unbuffered live delivery — a POST /api/status must reflect in the DOM within a short
 *      timeout (a buffering proxy would batch the SSE event and miss it).
 *   2. Reconnect-driven refetch — a DB change made WHILE THE SERVER IS DOWN emits no SSE
 *      event, so the only way the UI can learn of it is by refetching when its EventSource
 *      reconnects. We stop Go, insert a group straight into SQLite, restart, and assert the
 *      UI shows it.
 *
 * The Go server is spawned here (not a Playwright webServer) so the reconnect test can
 * restart it. `make live-e2e` builds the binary and passes GO_BIN.
 */

import { test, expect } from '@playwright/test'
import { spawn, type ChildProcess } from 'node:child_process'
import { DatabaseSync } from 'node:sqlite'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const GO_BIN = process.env.GO_BIN || join(__dirname, '..', '..', 'statshed-server')
const PORT = 7828
const HEALTH = `http://127.0.0.1:${PORT}/api/health`

const dbDir = mkdtempSync(join(tmpdir(), 'statshed-live-'))
const DB_PATH = join(dbDir, 'live.db')

let go: ChildProcess | null = null

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function startGo(): Promise<void> {
  go = spawn(GO_BIN, [], {
    env: {
      ...process.env,
      // 4-slash sqlite URL = absolute path (the temp DB).
      DATABASE_URL: `sqlite:///${DB_PATH}`,
      PORT: String(PORT),
      HOST: '::', // dual-stack so the proxy's localhost (which resolves to ::1) reaches it
      STATSHED_TEST_HOOKS: '1', // disable the 60s worker -> no stray background events
      STATIC_DISABLED: '1', // the SPA is served by Vite here, not the embedded build
    },
    stdio: 'ignore',
  })
  for (let i = 0; i < 150; i++) {
    try {
      const r = await fetch(HEALTH)
      if (r.ok) return
    } catch {
      // not up yet
    }
    await sleep(100)
  }
  throw new Error('Go server did not become healthy')
}

async function stopGo(): Promise<void> {
  const p = go
  go = null
  if (!p) return
  await new Promise<void>((resolve) => {
    p.on('exit', () => resolve())
    p.kill('SIGTERM')
  })
  // Give the OS a beat to release the port before any restart.
  await sleep(200)
}

test.beforeAll(startGo)
test.afterAll(stopGo)

test('live status update reflects in the DOM promptly (unbuffered SSE through the proxy)', async ({
  page,
}) => {
  await page.goto('/')
  await expect(page.getByText('Connected')).toBeVisible({ timeout: 20_000 })

  // POST a status for a brand-new group, through the Vite proxy.
  const res = await page.request.post('/api/status', {
    data: { group: 'livegroup', job: 'liveprobe', status: 'error' },
  })
  expect(res.status()).toBe(201)

  // The new group must appear quickly; a buffering proxy would batch the event and fail this.
  await expect(page.getByText('livegroup')).toBeVisible({ timeout: 8_000 })
})

test('reconnect refetches a DB change made while the server was down', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Connected')).toBeVisible({ timeout: 20_000 })

  // Take the server down; the badge flips to Disconnected.
  await stopGo()
  await expect(page.getByText('Disconnected')).toBeVisible({ timeout: 20_000 })

  // Insert a group straight into SQLite while Go is down — this emits NO SSE event.
  const db = new DatabaseSync(DB_PATH)
  db.exec("PRAGMA journal_mode=WAL")
  db.exec(
    "INSERT INTO groups (name, created_at) VALUES ('offlinegroup', '2026-06-20 12:00:00.000000')",
  )
  db.exec(
    "INSERT INTO jobs (group_id, name, status, updated_at, created_at) VALUES " +
      "((SELECT id FROM groups WHERE name='offlinegroup'), 'offlinejob', 'success', " +
      "'2026-06-20 12:00:00.000000', '2026-06-20 12:00:00.000000')",
  )
  db.exec("PRAGMA wal_checkpoint(TRUNCATE)")
  db.close()

  // Bring Go back. The EventSource reconnects, the provider resyncs, and the dashboard
  // refetches — surfacing the offline insert that no live event ever announced.
  await startGo()
  await expect(page.getByText('Connected')).toBeVisible({ timeout: 20_000 })
  await expect(page.getByText('offlinegroup')).toBeVisible({ timeout: 20_000 })
})
