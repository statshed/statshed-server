"""Ported from backend/tests/test_api.py — job log upload retrieval + log config flags.

AIDEV-NOTE: Retrieval (GET .../log) tail/all behavior is default-profile (bucket 1). The
store-side limit + default-tail tests run under the `max_log_lines` profile
(MAX_LOG_LINES=1500), and the ignored-log warning runs under `log_disabled`
(LOG_UPLOAD_ENABLED=false). Line counts are re-expressed relative to 1500 — see
coverage-map.md.
"""

import httpx
import pytest


def _submit_log(client: httpx.Client, group: str, job: str, content: str) -> dict:
    """POST a status with a multipart log file part; return the job dict."""
    resp = client.post(
        "/api/status",
        data={"group": group, "job": job, "status": "success"},
        files={"log": ("log.txt", content, "text/plain")},
    )
    assert resp.status_code == 201
    return resp.json()


def _lines(n: int) -> str:
    return "\n".join(f"line {i}" for i in range(n))


# --- Retrieval (default profile) ------------------------------------------------


@pytest.mark.default
def test_get_log_full_content(client: httpx.Client) -> None:
    content = "line one\nline two\nline three"
    _submit_log(client, "builds", "test", content)
    resp = client.get("/api/groups/builds/jobs/test/log")
    assert resp.status_code == 200
    body = resp.json()
    assert body["log"] == content
    assert body["line_count"] == 3
    assert body["truncated"] is False


@pytest.mark.default
def test_get_log_with_tail_param(client: httpx.Client) -> None:
    _submit_log(client, "builds", "test", _lines(100))
    resp = client.get("/api/groups/builds/jobs/test/log?tail=5")
    assert resp.status_code == 200
    body = resp.json()
    assert body["line_count"] == 5
    assert body["truncated"] is True
    assert body["total_line_count"] == 100


@pytest.mark.default
def test_get_log_with_all_param(client: httpx.Client) -> None:
    _submit_log(client, "builds", "test", _lines(100))
    resp = client.get("/api/groups/builds/jobs/test/log?all=true")
    assert resp.status_code == 200
    body = resp.json()
    assert body["line_count"] == 100
    assert body["truncated"] is False


@pytest.mark.default
def test_get_log_not_found_group(client: httpx.Client) -> None:
    resp = client.get("/api/groups/nonexistent/jobs/test/log")
    assert resp.status_code == 404
    assert "not found" in resp.json()["message"].lower()


@pytest.mark.default
def test_get_log_not_found_job(client: httpx.Client) -> None:
    _submit_log(client, "builds", "exists", "x")
    resp = client.get("/api/groups/builds/jobs/nonexistent/log")
    assert resp.status_code == 404
    assert "not found" in resp.json()["message"].lower()


@pytest.mark.default
def test_get_log_no_log_available(client: httpx.Client) -> None:
    client.post("/api/status", json={"group": "g", "job": "nolog", "status": "success"})
    resp = client.get("/api/groups/g/jobs/nolog/log")
    assert resp.status_code == 404
    assert "no log" in resp.json()["message"].lower()


# --- Store-side limits (max_log_lines profile, MAX_LOG_LINES=1500) ---------------


@pytest.mark.max_log_lines
def test_get_log_default_tail_1000(client: httpx.Client) -> None:
    # Store 1500 lines (at the cap, not store-truncated); the retrieval default tail
    # caps at 1000 < 1500, so the returned log is a truncated tail of the full 1500.
    _submit_log(client, "builds", "big", _lines(1500))
    resp = client.get("/api/groups/builds/jobs/big/log")
    assert resp.status_code == 200
    body = resp.json()
    assert body["line_count"] == 1000
    assert body["truncated"] is True
    assert body["total_line_count"] == 1500


@pytest.mark.max_log_lines
def test_log_truncation_to_max_lines(client: httpx.Client) -> None:
    # A log longer than MAX_LOG_LINES is truncated to the last 1500 at store time.
    job = _submit_log(client, "builds", "huge", _lines(1600))["job"]
    assert job["has_log"] is True
    assert job["log_line_count"] == 1500
    assert job["log_truncated"] is True


@pytest.mark.max_log_lines
def test_log_within_max_lines_not_truncated(client: httpx.Client) -> None:
    job = _submit_log(client, "builds", "small", _lines(50))["job"]
    assert job["log_line_count"] == 50
    assert job["log_truncated"] is False


# --- Log upload disabled (log_disabled profile) ----------------------------------


@pytest.mark.log_disabled
def test_log_upload_disabled(client: httpx.Client) -> None:
    resp = client.post(
        "/api/status",
        data={"group": "g", "job": "j", "status": "success"},
        files={"log": ("log.txt", "a\nb\nc", "text/plain")},
    )
    assert resp.status_code == 201
    body = resp.json()
    assert body["job"]["has_log"] is False
    # The conditional `warning` key is present ONLY because a log was ignored.
    assert "warning" in body
    assert "disabled" in body["warning"].lower()
