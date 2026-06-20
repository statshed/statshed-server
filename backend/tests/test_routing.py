"""The REST API is served under /api, not at root.

AIDEV-NOTE: Uses raw_client (no /api auto-prefix) to assert the real URL space.
"""


def test_health_is_under_api(raw_client):
    assert raw_client.get("/api/health").status_code == 200


def test_health_not_at_root(raw_client):
    # Root /health no longer exists (no SPA static dir in tests -> plain 404).
    assert raw_client.get("/health").status_code == 404


def test_status_is_under_api(raw_client):
    # /status moved too; bare POST /status is gone.
    assert raw_client.post("/status", json={}).status_code == 404
