"""Ported from backend/tests/test_security_headers.py — exact security headers + CSP.

AIDEV-NOTE: The CSP string is byte-for-byte load-bearing; the script-src sha256 pins the
inline theme-bootstrap script in index.html (behavioral-map §6). Asserted on /api/health.
Bucket 1.
"""

import httpx
import pytest

EXPECTED_CSP = (
    "default-src 'self'; "
    "script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; "
    "style-src 'self' 'unsafe-inline'; "
    "img-src 'self' data:; "
    "font-src 'self'; "
    "connect-src 'self'; "
    "object-src 'none'; "
    "base-uri 'self'; "
    "frame-ancestors 'none'; "
    "form-action 'self'"
)


@pytest.mark.default
def test_security_headers_present(client: httpx.Client) -> None:
    headers = client.get("/api/health").headers
    assert headers["X-Frame-Options"] == "DENY"
    assert headers["X-Content-Type-Options"] == "nosniff"
    assert headers["Referrer-Policy"] == "strict-origin-when-cross-origin"


@pytest.mark.default
def test_csp_matches(client: httpx.Client) -> None:
    assert client.get("/api/health").headers["Content-Security-Policy"] == EXPECTED_CSP
