# StatShed Frontend — Robustness & Reliability Plan

**Date:** 2026-06-06
**Source:** Multi-agent robustness review of the `src/` frontend (12 subsystem dimensions, adversarial verification of every finding). 55 raw findings → **51 confirmed** (7 high, 17 medium, 27 low), 4 rejected as false positives.
**Status legend:** ✅ done · ⬜ todo

> **Status (refreshed 2026-06-15): the backlog is complete.** All 7 high, 17 medium, and 27 low findings are implemented and committed to `master`, except a single deferred low item — `dialog-no-fallback-when-showmodal-unavailable` — which is a non-issue for every targeted (evergreen) browser. Suite: 174 unit + 45 e2e passing, lint 0 errors, typecheck clean. Phase 1–3 quick wins landed first in commit `2a2ec19`; the final low-item sweep landed 2026-06-14.

---

## Executive summary

The happy path is solid: clean architecture, strict TypeScript, optimistic mutations, sensible React Query + Socket.IO usage. The weaknesses cluster in exactly the conditions a monitoring dashboard must survive — **backend outages, socket disconnects, large logs, and invalid input** — plus, until this work, a **broken test safety net with no CI**.

Recurring root causes: (1) no defense against backend/socket failure (errors swallowed, no resync on reconnect, retry storms), (2) a couple of cache key/shape correctness bugs in optimistic mutations, and (3) debug/console code shipped to prod with no lint/CI enforcement. Almost every fix is low-to-moderate effort. Recommended sequence: restore a trustworthy test suite + CI first (done in part), then the high-severity correctness/deployment bugs, then resilience hardening, then validation, then a11y/cleanup.

---

## Verified non-issues (do not re-flag)

- **Log viewer XSS** — safe. `LogViewerModal` renders log lines via React text interpolation (`{line}`), no `dangerouslySetInnerHTML` (`LogViewerModal.tsx:421-429`).
- **Cache shape consistency** — `getGroupJobs` returns `Job[]` and `useGroupJobs` caches that; the optimistic-mutation hooks and socket handlers all assume `Job[]` for `groupJobs`. Consistent.
- **URL encoding** — group/job path params are `encodeURIComponent`'d in `src/api/groups.ts`.
- **No install drift** — `node_modules` matches the lockfile; not a cause of the test failures.
- **`cancelQueries` "race"** — not real; `cancelQueries({queryKey: ['groups']})` prefix-matches the `groupJobs` keys.

---

## Root-cause themes

1. **Broken test safety net + no CI gate** — network-backed tests were non-functional; nothing gated merges or the Docker build. *(Resolved: suite restored, CI gates lint/typecheck/unit/build/audit, and the e2e suite is hermetic and wired into CI.)*
2. **No resilience to backend/socket failure** — swallowed query errors, no resync on reconnect, no polling fallback, 4xx retried ~7s, ~2min of skeletons on a hung backend, misleading green favicon during outage.
3. **Cache key/shape correctness in optimistic mutations** — `['jobs']` prefix collides with `jobLog`/`jobsByStatus` caches; optimistic health status ignores in-progress jobs.
4. **Unbounded / unvalidated data at boundaries** — unvirtualized log rendering; `response.json()` cast to `T` with no runtime validation on most endpoints.
5. **Forms surface nothing on invalid input** — validation hidden in `try/catch` that only `console.error`s.
6. **Debug/console code shipped to production** — no console stripping in the Vite build.
7. **Accessibility gaps in shared UI primitives** — dialogs unnamed, helper text not announced, skeletons not announced.
8. **Deployment / security hardening** — vulnerable deps, no nginx security headers, no `engines` pin / Node version drift.

---

## Remediation phases

### Phase 1 — Restore a trustworthy test suite & CI gate

Make automated verification work and enforce it, so later fixes ship with regression protection.

- ✅ **Fix the jsdom/undici `AbortSignal` realm mismatch** (`jsdom-undici-abortsignal-realm-mismatch`, high). jsdom's `AbortSignal` + Node's native `Request` (used by MSW) reject each other; `client.ts` always attaches a signal, so every network test threw and hung. Fixed with a `src/test/undici-globals.ts` shim importing undici's Fetch globals before MSW. **Suite: 30-fail → 137-pass, ~9s → ~3.5s.**
- ✅ **Fix the `no-control-regex` ESLint error** (`eslint-error-fails-lint`, `ansi-regex-control-char-lint-error`, medium) — scoped disable at `LogViewerModal.tsx:45`; also added `.remember` to ESLint ignores. `npm run lint` now exits 0.
- ✅ **Add a CI workflow** (`no-ci-pipeline`, medium) — **[DONE a40b490]** `.github/workflows/ci.yml` runs on PRs + pushes to main/master: `npm ci` → lint → typecheck → `test:ci` (vitest run) → build → `npm audit --omit=dev --audit-level=high`. Node 20 (Dockerfile parity); added `typecheck`/`test:ci` scripts. e2e excluded (not hermetic). (Decided NOT to wire tests into the Docker build — keeps the image build fast; CI is the gate.)
- ✅ **Type-check config/e2e files** (`tsc-b-single-tsconfig-no-node-project`, low) — **[DONE b4d3d7f]** `tsconfig.json` now references `tsconfig.app.json` (src) + `tsconfig.node.json` (vite/vitest/playwright configs + `e2e/`, `types:["node"]`); added `@types/node@^20`. `tsc -b` now type-checks the whole repo, not just `src/`.
- ✅ **De-couple the e2e suite from internal DNS and a live backend** (`e2e-hardcoded-internal-host`, `e2e-requires-live-seeded-backend`, medium) — **[DONE 432452e]** chose the `page.route` mocking route: `e2e/fixtures/` intercepts the `/api` boundary with deterministic data shaped against `src/types` (stateful for config writes, 501 on unmocked paths, Socket.IO aborted), auto-installed by a custom Playwright `test`. Deleted `diagnose-errors-card.spec.ts`; dropped no-data skip branches. Added a parallel hermetic `e2e` CI job. (`playwright.config` has a `PLAYWRIGHT_CHROMIUM_BIN` env override for the NixOS dev host.)

### Phase 2 — High-severity production correctness & deployment bugs

- ✅ **Guard optimistic jobs updaters against prefix-key collision** (`joblog-prefix-match-crash`, high). `setQueriesData({queryKey: ['jobs']})` also matches `jobLog` caches (`['jobs','log',…]`, a `LogResponse` with no `.jobs`); `old.jobs.map(...)` threw inside `onMutate`, silently aborting Ack/Delete after a user viewed a job log. Added `if (!old || !Array.isArray(old.jobs)) return old` to all four hooks.
- ✅ **Make socket transport same-origin** (`websocket-baked-localhost-breaks-prod`, high) — **[DONE 1f54c35]** config-only: `docker-compose.yml` + `.env.example` now default the build-time `VITE_BACKEND_URL` to EMPTY (same-origin), so the socket goes through nginx's `/socket.io` proxy like `/api`. Code already supported this (`getBackendUrl()` → `io('')`); it was baking `localhost:7828` into the bundle, breaking realtime for all users.
- ✅ **Bound log rendering** (`logviewer-unbounded-render-no-virtualization`, high; `logviewer-baseline-from-prop-mismatch`, low) — **[DONE af262e5]** `LogViewerModal` body virtualized with `@tanstack/react-virtual` (v3.14.2); only the visible window + overscan is in the DOM at any size. Rows keep wrapping (variable height, measureElement); error-nav uses `scrollToIndex`. (baseLine clamp was already present.)
- ✅ **Surface config-read failure on Settings** (`config-fetch-silent-failure`, high) — **[DONE 1f54c35]** `GlobalConfigForm` now reads `isError`/`error`/`refetch` and renders the Dashboard/Jobs-style error card + Try Again, hiding the editable form so a failed read can't be overwritten with blanks.
- ✅ **Read the backend's human-readable error message** (`error-key-assumed-no-contract`, medium) — **[DONE c02448b]** `client.ts` now prefers `message`, then `error`, then a status fallback, with non-string guards. (Backend error contract still worth pinning server-side.)

### Phase 3 — Resilience to backend/socket failure

- ✅ **Invalidate the jobs cache on `status_update`** (`status-update-skips-jobs-cache`, high) — the most common realtime event refreshed groups/health/groupJobs but not `['jobs']`, so the Jobs page (byStatus queries) stayed stale up to ~60s. Now invalidates `queryKeys.jobs`.
- ✅ **Stop retrying deterministic 4xx errors** (`retry-3-on-all-status-codes`, `retry-on-404-group-not-found`, medium) — `DEFAULT_QUERY_OPTIONS.retry` is now a predicate that skips 4xx (`ApiError.status` 400–499), so 404/400 surface immediately instead of ~7s of skeletons, while network/5xx still retry.
- ✅ **Resync queries on socket reconnect** (`no-resync-on-reconnect`, high; `stale-data-on-socket-disconnect`, low) — **[DONE 1f54c35]** the `connect` handler now invalidates health/groups/jobs on reconnect, gated behind a `hasConnected` closure flag so the initial connect (mount queries already fetch) doesn't trigger a spurious refetch.
- ✅ **Add a polling fallback for groups/groupJobs** (`groups-groupjobs-no-polling-socket-only`, medium) — **[DONE c02448b]** both hooks now use `refetchInterval: 60000`, matching health/jobs.
- ✅ **Make the favicon reflect an "unknown" state when health is unavailable** (`favicon-green-when-backend-down`, medium) — **[DONE c02448b]** `useFavicon` is now a 3-state `'healthy' | 'error' | 'unknown'` (grey); Dashboard/GroupDetail/Jobs pass `'unknown'` when health/jobs are unavailable so an outage never shows green.
- ✅ **Surface group-config read failure & prevent default-overwrite** (`group-config-silent-failure`, medium) — **[DONE c02448b]** `GroupConfigForm` guards `isError`: shows an error + Try Again and hides the form/Save so a failed read can't be overwritten with the form's hardcoded defaults.
- ✅ **Add a slow-backend indicator** (`long-retry-blocks-error-ui`, medium) — **[DONE 3a0bd16]** `useSlowLoading` hook + a "taking longer than usual" hint on the Dashboard after a few seconds of loading. (Optionally still: lower retry/total time for read queries — deferred.)
- ✅ **Add a global query-error fallback** (`no-global-query-error-fallback`, low) — **[DONE 037f5d0]** `src/lib/queryClient.ts` `createQueryClient` installs a `QueryCache` `onError` that toasts ONLY background-refetch failures (cached data already present); initial-load failures stay owned by the page error cards. Unit-tested in `queryClient.test.ts`.

### Phase 4 — Input validation & data-boundary hardening

- ✅ **Validate GlobalConfigForm timeouts as positive integers** (`globalconfig-silent-validation-failure`, `globalconfig-no-staleness-vs-progress-or-coercion-of-floats`, medium) — validation moved into the zod schema (positive whole numbers), so `0`/`-3`/`1.5` are rejected with field errors and blocked from submit instead of silently doing nothing or truncating `1.5`→`1` and saving.
- ✅ **Add runtime validation at the HTTP boundary** (`no-runtime-validation-most-endpoints`, `success-json-parse-misclassified-network`, low) — **[DONE 7827e29, refined 0574ec8]** structural guards (groups.ts-style) added to `getHealth`/`getConfig`/`getJobsByStatus`/`getJobLog`; `client.ts` now reports a malformed 2xx body as an `ApiError` carrying the real status (SyntaxError only — a body stream that drops mid-flight stays a status-0 network error). The `by_status` guard rejects `null` specifically, since HealthStats's `by_status = {}` default only covers `undefined`.
- ✅ **Re-validate the dependent staleness field in GroupConfigForm** (`groupconfig-stale-cross-field-error-not-revalidated`, low) — **[DONE 4f66849]** the form watches `expiration` and, once submitted, calls `trigger('staleness_timeout_hours')` so the "must be < expiration" error clears the moment expiration is raised to resolve it; e2e asserts the clearing.

### Phase 5 — Accessibility, deployment hardening & cleanup

- ✅ **Remove debug console.logs in GroupDetail** (`groupdetail-console-log-job-data`, `debug-console-log-render`, `console-log-job-data`, `debug-console-logs-in-prod`, low) — removed the two `filteredJobs` debug blocks that ran on every render.
- ✅ **Gate/strip remaining console logging** (`console-logging-in-prod`, low) — **[DONE 4de0f91]** `vite.config` `esbuild.drop: ['console','debugger']` gated to production (dev/e2e keep logs); added a `no-console` ESLint rule (error in src, allow `warn`/`error`, off for tests/e2e/config); removed a redundant disconnect log. Verified the built bundle has zero `console.*`.
- ✅ **Give dialogs accessible names & link helper text** (`dialog-no-accessible-name`, `input-select-helpertext-not-announced`, `input-select-label-unassociated-without-name`, `skeleton-no-aria-busy-live`, medium/low) — **[DONE 3a0bd16 + 1c335a1]** `useId` + `aria-labelledby` on both `Dialog` and `LogViewerModal`; Input/Select link helper text via `aria-describedby` and fall back to a generated id (label always associated); Skeleton exposes `role=status`/`aria-busy`/`aria-label` with composites announcing once.
- ✅ **Upgrade vulnerable runtime deps** (`runtime-deps-known-cves`, medium) — **[DONE 9647147]** react-router(-dom) 7.12.0→7.17.0 (RCE/XSS/redirect/DoS), socket.io-parser 4.2.5→4.2.6, engine.io-client 6.6.4→6.6.5, ws→8.20.1 (all non-breaking); pinned react-router-dom floor to `^7.17.0`. `npm audit --omit=dev` is now clean. The `npm audit --omit=dev` gate was added to CI **[a40b490]**, and the dev-only `vitest` critical was cleared by bumping vitest 3→4 **[de95b24]** — `npm audit` is now clean across the entire tree.
- ✅ **Add nginx security headers** (`missing-security-headers`, low) — **[DONE 6ffa8f6]** `X-Frame-Options DENY`, `X-Content-Type-Options nosniff`, `Referrer-Policy`, and a SPA-tuned CSP, all with `always`; repeated in the static-asset location because nginx `add_header` inheritance is all-or-nothing per block. The pre-hydration theme script is allowlisted in the CSP by sha256 hash.
- ✅ **Add a catch-all 404 route** (`no-404-catch-all-route`, medium) — **[DONE 3a0bd16]** `<Route path="*" element={<NotFound/>} />` + a NotFound page with a link back to the dashboard.
- ✅ **Eliminate the theme flash** (`theme-flash-no-preload`, low) — **[DONE e9b2ed2]** an inline pre-hydration bootstrap script in `index.html` reads `localStorage['statshed-theme']` and sets the dark class before paint; allowlisted in the CSP by sha256 hash (recompute steps documented in `nginx.conf.template`). Verified in-browser with zero CSP violations.
- ✅ **Centralize favicon source-of-truth & tidy SocketContext value** (`favicon-cross-page-conflict`, `favicon-no-cleanup`, `socket-value-null-not-memoized`, low) — **[DONE 2be5ac4]** a single `useFavicon` in `<Header>` driven by overall `/health` covers every route (incl. Settings/404); removed the per-page calls (Dashboard/GroupDetail/Jobs) and the dead `linkRef`; SocketContext drops the unused `socket` field and memoizes its provider value.
- ✅ **Add `engines` pin & align Docker Node version** (`no-engines-node-version-drift`, low) — **[DONE 2e81643]** added `engines.node: ">=20"`, standardizing on the Docker `node:20-alpine` builder and CI (node 20) rather than bumping to 22.
- ✅ **Optimistic health status ignores in-progress jobs** (`health-status-flash-ignores-in-progress`, low) — **[DONE cb6e3e5]** useAckJob/useAckGroup/useAckAll now fall to `in_progress` (not `healthy`) when `unhealthy` hits 0 while in-progress jobs remain; deterministic tests via `delay('infinite')`.
- ⬜ **Dialog has no fallback when `showModal` is unavailable** (`dialog-no-fallback-when-showmodal-unavailable`, low) — **DEFERRED (non-issue).** `Dialog.tsx:45` wraps `showModal()` in a `try/catch`, but the catch only swallows the already-open DOMException; there is no fallback render path if the native `<dialog>`/`showModal` is genuinely unsupported. Every targeted (evergreen) browser supports it and the app calls it unconditionally, so this is left as a known theoretical gap rather than fixed.

---

## Appendix — confirmed findings reference

### High (7)

| id | file | one-liner |
|----|------|-----------|
| jsdom-undici-abortsignal-realm-mismatch ✅ | `src/api/client.ts:48` | jsdom/undici AbortSignal mismatch broke all 30 network tests |
| joblog-prefix-match-crash ✅ | `src/hooks/useAckJob.ts:70` (+3) | `['jobs']` prefix matches jobLog cache → onMutate throws, Ack/Delete silently fails |
| status-update-skips-jobs-cache ✅ | `src/contexts/SocketContext.tsx:78` | status_update didn't invalidate the Jobs page cache |
| websocket-baked-localhost-breaks-prod ✅ | `docker-compose.yml:15` | bundled localhost backend → realtime fails for all users |
| logviewer-unbounded-render-no-virtualization ✅ | `src/components/jobs/LogViewerModal.tsx:395` | unvirtualized "Show all" can freeze/OOM the tab |
| no-resync-on-reconnect ✅ | `src/contexts/SocketContext.tsx:55` | socket reconnect doesn't refetch; stale dashboard |
| config-fetch-silent-failure ✅ | `src/components/config/GlobalConfigForm.tsx:40` | Settings shows a blank form on /config failure |

### Medium (17)

| id | file | one-liner |
|----|------|-----------|
| error-key-assumed-no-contract ✅ | `src/api/client.ts:54` | reads `errorData.error`, loses human `message` |
| retry-3-on-all-status-codes ✅ | `src/lib/constants.ts:116` | retried non-retryable 4xx |
| no-ci-pipeline ✅ | `package.json:6` | lint/typecheck/tests/build never enforced |
| eslint-error-fails-lint ✅ | `src/components/jobs/LogViewerModal.tsx:45` | no-control-regex made lint exit 1 |
| groups-groupjobs-no-polling-socket-only ✅ | `src/hooks/useGroups.ts:10` | no refetchInterval; freshness depends on socket |
| globalconfig-silent-validation-failure ✅ | `src/components/config/GlobalConfigForm.tsx:14` | invalid timeouts silently ignored |
| globalconfig-no-staleness-vs-progress-or-coercion-of-floats ✅ | `src/components/config/GlobalConfigForm.tsx:23` | `1.5` truncated to `1` silently |
| no-404-catch-all-route ✅ | `src/App.tsx:42` | unknown URLs render a blank page |
| retry-on-404-group-not-found ✅ | `src/lib/constants.ts:113` | 404 pages retried ~7s before erroring |
| group-config-silent-failure ✅ | `src/pages/GroupDetail.tsx:54` | group-config error hidden; defaults can overwrite real settings |
| favicon-green-when-backend-down ✅ | `src/pages/Dashboard.tsx:55` | favicon green during a total outage |
| long-retry-blocks-error-ui ✅ | `src/lib/constants.ts:113` | ~2min of skeletons on a hung backend |
| runtime-deps-known-cves ✅ | `package.json:23` | react-router / socket.io stack CVEs |
| e2e-hardcoded-internal-host ✅ | `e2e/diagnose-errors-card.spec.ts:61` | hardcoded internal DNS, no real assertions |
| e2e-requires-live-seeded-backend ✅ | `e2e/group-detail.spec.ts:70` | e2e assumes a live seeded backend |
| dialog-no-accessible-name ✅ | `src/components/ui/Dialog.tsx:82` | dialog has no accessible name |
| input-select-helpertext-not-announced ✅ | `src/components/ui/Input.tsx:46` | helper text not linked via aria-describedby |

### Low (27)

`no-runtime-validation-most-endpoints` ✅, `success-json-parse-misclassified-network` ✅, `missing-security-headers` ✅, `tsc-b-single-tsconfig-no-node-project` ✅, `no-engines-node-version-drift` ✅, `ansi-regex-control-char-lint-error` ✅, `logviewer-xss-safe-confirmed` (non-issue), `groupdetail-console-log-job-data` ✅, `logviewer-baseline-from-prop-mismatch` ✅ (clamp already present), `health-status-flash-ignores-in-progress` ✅, `favicon-no-cleanup` ✅, `jobsbystatus-suspected-shape-not-a-bug` (non-issue), `groupconfig-stale-cross-field-error-not-revalidated` ✅, `debug-console-log-render` ✅, `favicon-cross-page-conflict` ✅, `theme-flash-no-preload` ✅, `console-logging-in-prod` ✅, `socket-value-null-not-memoized` ✅, `stale-data-on-socket-disconnect` ✅, `debug-console-logs-in-prod` ✅, `no-global-query-error-fallback` ✅, `console-log-job-data` ✅, `no-install-drift` (non-issue), `onunhandledrequest-error-good-but-masked` ✅, `input-select-label-unassociated-without-name` ✅, `dialog-no-fallback-when-showmodal-unavailable` ⬜ (deferred non-issue), `skeleton-no-aria-busy-live` ✅.

### Rejected (false positives)

- `ackall-no-onmutate-cancel-groupjobs` / `ackjob-deletejob-no-cancel-groupjobs` — `cancelQueries(['groups'])` prefix-matches the groupJobs keys.
- `logviewer-array-index-key` — index keys are fine here (no reordering/insertion within the rendered window).
- `jobstatusbadge-acked-visual-only-strikethrough` — the acked/expiring state is conveyed with text, not color/strikethrough alone, when viewed in the rendered card.
