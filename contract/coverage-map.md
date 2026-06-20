# Contract coverage map

Accounts for **every** test in `backend/tests/` (≈192 methods) so the port from the
Python suite to the shared HTTP contract suite is auditable — a green gate cannot hide a
silently-dropped test. Each backend test maps to one of:

- a **`contract/` test** (ported, runs over HTTP against both servers), or
- a **Go-side test** (replaces a Python-internal test — schema/perf/SSE), or
- a **per-language test** (each server keeps its own — e.g. a forced 500), or
- an explicit **drop** with a reason.

Buckets (spec.md §8.3): **B1** run-as-is over HTTP · **B2** setup/asserts re-expressed via
the API · **B3** direct-SQLite backdating of aged rows · **B4** reconfigured-server profile ·
**B5** Python-only → Go-side/per-language · **B6** concurrency/integrity invariants.

## Resolved porting decisions

1. **URL space.** Contract tests address the real URL space with explicit `/api/...` paths
   via the root `client` fixture (the Python suite's `/api` auto-prefix wrapper becomes a
   plain base-URL join — spec.md §8.1). `raw_client` tests (routing/security) also use the
   root client.
2. **`expires_at` is in the job JSON** (`job.to_dict`), so the expiration-cascade (B3) and
   expiry tests read it via `GET /api/jobs` rather than the ORM.
3. **Empty group (B2).** A bare group with no jobs cannot be created via the API (a group is
   created implicitly by a job POST). `conftest.insert_group()` inserts one via direct SQL
   (the harness owns `STATSHED_DB_FILE`).
4. **Backdating (B3).** `conftest.backdate()` sets `updated_at`/`created_at`/`expires_at` into
   the past via SQL after a normal POST.
5. **Config profiles (`runner.py` PROFILE_ENV; one harness pass each):**
   - `log_disabled` → `LOG_UPLOAD_ENABLED=false`
   - `max_log_lines` → `MAX_LOG_LINES=1500` (one value serves all three log tests: a
     >1500-line log truncates to 1500; a <1500-line log is untruncated; a 1500-line log
     stored then retrieved with the default tail caps at 1000 < 1500). Input line counts are
     re-expressed relative to 1500 (intent preserved).
   - `max_page_size` → `MAX_JOBS_PAGE_SIZE=2` (a `limit=100` request clamps to 2)
   - `with_spa` → `STATIC_DIR` points at a synthetic dist the runner writes (`index.html`
     containing `StatShed`, `assets/app.js` containing `console.log`)
   - `no_spa` → Python: `STATIC_DIR` set to a non-existent dir (no SPA registered); Go:
     `STATIC_DISABLED=1`
6. **Background transitions (B5 here, cross-language in the suite).** `test_background.py` and
   the Socket.IO event tests call the checker / assert emits in-process — Python-only. Their
   cross-language coverage is the tick hook `POST /api/admin/run-checks` driven from
   `contract/test_background.py` (backdate rows → run-checks → assert per-type id split +
   resulting state via GET). The Python socket payload oracle is already pinned
   (`test_socket_events.py`, Task 1.2); the Go SSE frames are Task 5.3.
7. **Performance (B5) → Go-side** (Task 4.3, SQL-shape introspection has no HTTP analog). The
   two HTTP-observable slices (`limit=2` returns 2 rows while `total==3`) ARE ported, into
   `test_jobs.py` / `test_groups.py`.
8. **Migrations (B5) → Go schema tests** (Task 2.2): the 3 tables, 7 named job indexes, and
   the `(group_id,name)` unique constraint on a fresh DB.
9. **Forced 500 (B5) → per-language.** A 500 cannot be forced over HTTP; Python keeps its
   `monkeypatch` test, Go gets an injected-store-error test (Task 4.3). Not in the suite.
10. **IntegrityError retry (B6).** The Python tests force the race with `patch.object(...,
    "flush")` (Python-only). The suite asserts the OUTCOME via concurrent HTTP POSTs (no 5xx,
    exactly one group/one job) in `test_concurrency.py`.

## Contract test files

`test_health.py` · `test_status.py` · `test_jobs.py` · `test_groups.py` · `test_config.py` ·
`test_admin.py` · `test_ack.py` · `test_delete.py` · `test_logs.py` · `test_errors.py` ·
`test_routing.py` (no_spa) · `test_spa.py` (with_spa) · `test_security.py` ·
`test_concurrency.py` · `test_background.py` (tick hook) · `test_smoke.py`.

---

## backend/tests/test_api.py (130) — 116 B1, 1 B2, 7 B3, 6 B4

| Backend class | n | Bucket | Contract target |
|---|---|---|---|
| TestHealthEndpoint | 5 | B1 | `test_health.py` |
| TestStatusEndpoint | 9 | B1 | `test_status.py` |
| TestJobsEndpoint | 11 | B1 (10) + B3 (`…ordered_by_updated_at_desc`) | `test_jobs.py` |
| TestJobsPagination | 10 | B1 (8) + B3 (`…offset_pages_through_in_order`) + B4 max_page_size (`…limit_is_clamped_to_max`) | `test_jobs.py` |
| TestGroupsEndpoint | 6 | B1 (5) + B2 (`…includes_zero_job_group`, via `insert_group`) | `test_groups.py` |
| TestGroupJobsPagination | 9 | B1 (7) + B3 (`…offset_pages_through_in_order`) + B4 max_page_size (`…limit_is_clamped_to_max`) | `test_groups.py` |
| TestConfigEndpoints | 14 | B1 (13) + B3 (`…expiration_cascades_to_non_override_groups`, read `expires_at` via API) | `test_config.py` |
| TestAdminEndpoints | 10 | B1 (7) + B3 (`…cleanup_dry_run`, `…cleanup_deletes_jobs`, `…cleanup_respects_status_filter`) | `test_admin.py` |
| TestAckEndpoint | 7 | B1 | `test_ack.py` |
| TestAckHealthCalculation | 4 | B1 | `test_ack.py` |
| TestAckGroupSummary | 2 | B1 | `test_ack.py` |
| TestAckClearOnRecovery | 4 | B1 | `test_ack.py` |
| TestJobsResponseIncludesAckedFields | 3 | B1 | `test_ack.py` |
| TestAckGroupEndpoint | 7 | B1 | `test_ack.py` |
| TestAckAllEndpoint | 6 | B1 | `test_ack.py` |
| TestDeleteJobEndpoint | 5 | B1 | `test_delete.py` |
| TestLogUpload | 6 | B1 | `test_status.py` (multipart) |
| TestLogRetrieval | 7 | B1 (6) + B4 max_log_lines (`…default_tail_1000`) | `test_logs.py` |
| TestLogConfigFlags | 3 | B4: log_disabled (`…upload_disabled`), max_log_lines (`…truncation`, `…within_max`) | `test_logs.py` |
| TestLogEncodingHandling | 2 | B1 | `test_status.py` |

## backend/tests/test_error_handling.py (10) — 9 B1, 1 B5

| Backend test | Bucket | Target |
|---|---|---|
| TestHttpErrorEnvelopes::* (404, 405, 413) | B1 | `test_errors.py` |
| TestMalformedJson::* (400 ×3, incl. wrong-content-type→400) | B1 | `test_errors.py` |
| TestNonStringFieldsRejectedAs400::* (`field`==group/job/status) | B1 | `test_errors.py` |
| TestInternalErrorEnvelope::test_unexpected_exception_returns_json_500 | B5 | per-language (Python keeps it; Go = injected store error, Task 4.3). Not in suite. |

## backend/tests/test_routing.py (3) — B4 no_spa (re-authored over the wire)
All three → `test_routing.py` under `no_spa`: `GET /api/health`→200, bare `GET /health`→404,
bare `POST /status`→404.

## backend/tests/test_security_headers.py (2) — B1
Both → `test_security.py`: the three security headers + the exact CSP string on `/api/health`.

## backend/tests/test_static_serving.py (4) — B4 with_spa (re-authored over the wire)
All four → `test_spa.py` under `with_spa` (synthetic dist): `/`→shell(`StatShed`),
`/jobs`→shell (fallback), `/assets/app.js`→asset(`console.log`), `/api/does-not-exist`→404 JSON.

## backend/tests/test_integration.py (17) — 10 B1, 1 B2, 2 B6, 4 B5

| Backend class | Bucket | Target |
|---|---|---|
| TestCliIntegration (5) | B1 | `test_health.py`/`test_status.py`/`test_groups.py`/`test_config.py` (folded into area files) |
| TestWebSocketIntegration (4) | B5 | Python socket oracle (`test_socket_events.py` / existing, Task 1.2) + Go SSE (Task 5.3). Not in suite. |
| TestRapidSubmissions (5) | B1 | `test_concurrency.py` (sequential rapid writes) |
| TestIntegrityErrorRetry::test_group_… / test_job_… (2) | B6 | `test_concurrency.py` (outcome via concurrent POSTs) |
| TestIntegrityErrorRetry::test_group_and_job_both_exist (1) | B2 | `test_status.py` (create-then-update over HTTP) |

## backend/tests/test_background.py (16) — B5 → cross-language tick hook
All 16 call `run_timeout_check`/`run_expiration_check` in-process or assert emits (Python-only
oracle/unit). Cross-language coverage → `contract/test_background.py` via
`POST /api/admin/run-checks`: progress→timeout, success+staleness→stale, group override
suppresses timeout, error never transitions, staleness off by default, expiry deletes
success/error/stale/acked, expiry preserves unexpired, `expires_at` refreshed on update, and
the per-type id split (a stale job never under `timeout_job_ids`). Health_update payloads stay
Python-side (oracle) + Go SSE (Task 5.3).

## backend/tests/test_performance.py (8) — B5 → Go-side (Task 4.3)
SQL-shape introspection (no `log_content` blob in list SELECTs; aggregate `/health`; N+1-free
`/groups`; no redundant COUNT on the default path) → Go statement-counter tests. The two
HTTP-observable pagination slices (`/jobs?limit=2`→2 rows, `total==3`; same for group jobs)
ARE ported into `test_jobs.py` / `test_groups.py`.

## backend/tests/test_migrations.py (2) — B5 → Go schema tests (Task 2.2)
`upgrade head` on empty DB + schema-matches-models → Go: assert the 3 tables, 7 named job
indexes, and `(group_id,name)` unique constraint on a fresh DB.

---

## Totals

| | test_api | others | total |
|---|---|---|---|
| B1 (→ contract, as-is) | 116 | 21 | 137 |
| B2 (→ contract, re-expressed) | 1 | 2 | 3 |
| B3 (→ contract, backdated) | 7 | 0 | 7 |
| B4 (→ contract, profile) | 6 | 7 | 13 |
| B5 (→ Go-side / per-language / oracle) | 0 | 30 | 30 |
| B6 (→ contract, outcome) | 0 | 2 | 2 |
| **total** | **130** | **62** | **192** |

**Ported into the contract suite:** B1+B2+B3+B4+B6 = 137+3+7+13+2 = **162** HTTP tests.
**Not in the suite (accounted):** B5 = 30 → Go schema (2) + Go perf (8) + per-language 500 (1) +
Socket.IO/background oracle & SSE (19). Every backend method is mapped above.
