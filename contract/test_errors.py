"""Ported from backend/tests/test_error_handling.py — HTTP error envelopes.

AIDEV-NOTE: Every error path returns a JSON {error, message, field?} envelope, never
HTML. The forced-500 test is per-language (a 500 can't be forced over HTTP) and is not
here — see coverage-map.md. Bucket 1 (runs as-is over the wire).
"""

import httpx
import pytest

JSON = "application/json"


def _assert_json_error(resp: httpx.Response, status: int) -> dict:
    assert resp.status_code == status
    assert resp.headers["content-type"].startswith(JSON)
    body = resp.json()
    assert isinstance(body, dict)
    assert "error" in body
    assert "message" in body
    return body


@pytest.mark.default
def test_unknown_route_returns_json_404(client: httpx.Client) -> None:
    _assert_json_error(client.get("/api/no-such-route"), 404)


@pytest.mark.default
def test_wrong_method_returns_json_405(client: httpx.Client) -> None:
    # /api/health is GET-only.
    _assert_json_error(client.delete("/api/health"), 405)


@pytest.mark.default
def test_oversized_body_returns_json_413(client: httpx.Client) -> None:
    # MAX_CONTENT_LENGTH is 1 MB; exceed it.
    oversized = "x" * (1024 * 1024 + 1024)
    resp = client.post("/api/status", content=oversized, headers={"content-type": JSON})
    _assert_json_error(resp, 413)


@pytest.mark.default
def test_malformed_json_status_returns_json_400(client: httpx.Client) -> None:
    resp = client.post(
        "/api/status", content="{not valid json", headers={"content-type": JSON}
    )
    _assert_json_error(resp, 400)


@pytest.mark.default
def test_malformed_json_config_returns_json_400(client: httpx.Client) -> None:
    resp = client.put("/api/config", content="garbage", headers={"content-type": JSON})
    _assert_json_error(resp, 400)


@pytest.mark.default
def test_wrong_content_type_status_returns_json_400(client: httpx.Client) -> None:
    # Form-encoded body (not JSON, not multipart-with-log) must surface as a 400
    # "JSON object required" — NOT a Werkzeug 415. (behavioral-map §2 endpoint note.)
    resp = client.post(
        "/api/status", data={"group": "a", "job": "b", "status": "success"}
    )
    _assert_json_error(resp, 400)


@pytest.mark.default
@pytest.mark.parametrize("field", ["group", "job", "status"])
def test_non_string_field_returns_json_400(client: httpx.Client, field: str) -> None:
    payload: dict[str, object] = {"group": "g", "job": "j", "status": "success"}
    payload[field] = 123  # non-string
    body = _assert_json_error(client.post("/api/status", json=payload), 400)
    assert body["field"] == field
