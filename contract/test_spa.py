"""Re-authored from backend/tests/test_static_serving.py — SPA serving + precedence.

AIDEV-NOTE: Runs under the `with_spa` profile; runner.py points STATIC_DIR at a synthetic
dist (index.html containing "StatShed", assets/app.js containing "console.log"). The
originals were in-process (register_spa + app.test_client); these go over the wire
(spec.md 8.3 bucket 4). The /api namespace must never be shadowed by the SPA fallback.
"""

import httpx
import pytest


@pytest.mark.with_spa
def test_spa_served_at_root(client: httpx.Client) -> None:
    resp = client.get("/")
    assert resp.status_code == 200
    assert "StatShed" in resp.text


@pytest.mark.with_spa
def test_spa_fallback_does_not_shadow_jobs_api(client: httpx.Client) -> None:
    # An un-prefixed client route falls back to the SPA shell, not the API JSON.
    spa = client.get("/jobs")
    assert spa.status_code == 200
    assert "StatShed" in spa.text
    # The real API still answers under /api.
    assert client.get("/api/health").status_code == 200


@pytest.mark.with_spa
def test_real_asset_is_served(client: httpx.Client) -> None:
    resp = client.get("/assets/app.js")
    assert resp.status_code == 200
    assert "console.log" in resp.text


@pytest.mark.with_spa
def test_unknown_api_path_is_404_not_spa(client: httpx.Client) -> None:
    resp = client.get("/api/does-not-exist")
    assert resp.status_code == 404
    assert "StatShed" not in resp.text  # JSON error envelope, not the SPA HTML
