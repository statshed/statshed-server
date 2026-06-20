"""nginx's security headers/CSP are preserved now that Flask serves responses."""

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


def test_security_headers_present(raw_client):
    resp = raw_client.get("/api/health")
    assert resp.headers["X-Frame-Options"] == "DENY"
    assert resp.headers["X-Content-Type-Options"] == "nosniff"
    assert resp.headers["Referrer-Policy"] == "strict-origin-when-cross-origin"


def test_csp_matches_nginx(raw_client):
    resp = raw_client.get("/api/health")
    assert resp.headers["Content-Security-Policy"] == EXPECTED_CSP
