/**
 * AIDEV-NOTE: Test-environment Fetch API polyfill — MUST be imported before MSW.
 *
 * Under vitest's jsdom environment there is a cross-realm mismatch: globalThis.AbortSignal
 * is jsdom's implementation, while globalThis.Request is Node's built-in (native undici),
 * whose Request constructor brand-checks the signal and REJECTS a jsdom AbortSignal.
 * src/api/client.ts always attaches an AbortController signal to fetch, so when MSW's
 * interceptor reconstructs the request via `new Request(input, init)` it throws
 * synchronously ("Expected signal to be an instance of AbortSignal"). That surfaces as an
 * immediate query error, making every network-backed test hang until its waitFor timeout.
 *
 * Overriding the Fetch globals with the `undici` package's (realm-tolerant) implementations
 * resolves the mismatch. This is a test-only shim; production runs in a real browser where
 * all Fetch globals share one realm. See robustness review finding
 * "jsdom-undici-abortsignal-realm-mismatch".
 */
import { fetch, Request, Response, Headers } from 'undici'

Object.assign(globalThis, { fetch, Request, Response, Headers })
