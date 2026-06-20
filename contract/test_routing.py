"""Re-authored from backend/tests/test_routing.py — API under /api, no SPA fallback.

AIDEV-NOTE: Runs under the `no_spa` profile (no SPA registered), so un-prefixed paths
return a JSON 404 instead of an SPA shell. The originals were in-process; these address
the real URL space over the wire (spec.md 8.3 bucket 4; the no_spa assertions only hold
with no SPA fallback registered).
"""

import httpx
import pytest


@pytest.mark.no_spa
def test_health_is_under_api(client: httpx.Client) -> None:
    assert client.get("/api/health").status_code == 200


@pytest.mark.no_spa
def test_health_not_at_root(client: httpx.Client) -> None:
    # Bare /health is not the API and there is no SPA fallback -> 404.
    assert client.get("/health").status_code == 404


@pytest.mark.no_spa
def test_status_is_under_api(client: httpx.Client) -> None:
    assert client.post("/status", json={}).status_code == 404
