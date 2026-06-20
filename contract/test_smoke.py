"""Smoke test: the harness reaches a live server and it starts on a pristine DB.

AIDEV-NOTE: This is the first contract test (impl-guide Task 1.3). It proves the
end-to-end loop -- runner.py boots a server, exports STATSHED_BASE_URL/STATSHED_DB_FILE,
the autouse reset_db fixture truncates, and httpx reaches it -- before any real
assertions are ported (Task 1.4).
"""

import httpx
import pytest


@pytest.mark.default
def test_health_is_empty_on_fresh_db(client: httpx.Client) -> None:
    resp = client.get("/api/health")
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "empty"
    assert body["total_jobs"] == 0
