"""Cross-language background transitions via the guarded tick hook (spec.md §8.4).

AIDEV-NOTE: The Python suite drove run_timeout_check/run_expiration_check in-process
(Python-only). The shared suite drives them over HTTP through POST /api/admin/run-checks
(registered only under STATSHED_TEST_HOOKS, which the harness always sets) after backdating
rows, then asserts the structured result — including the per-type id split (a stale job
never appears under timeout_job_ids) — and the resulting state via GET. This is the
language-neutral background coverage for both the Python and Go servers.
"""

from datetime import UTC, datetime, timedelta

import httpx
import pytest

from conftest import backdate


def _ago(**kwargs: float) -> str:
    """A stored-format ('YYYY-MM-DD HH:MM:SS.ffffff') timestamp `kwargs` in the past."""
    return (datetime.now(UTC) - timedelta(**kwargs)).strftime("%Y-%m-%d %H:%M:%S.%f")


def _submit(client: httpx.Client, group: str, job: str, status: str) -> int:
    resp = client.post(
        "/api/status", json={"group": group, "job": job, "status": status}
    )
    assert resp.status_code == 201
    return resp.json()["job"]["id"]


def _run_checks(client: httpx.Client) -> dict:
    resp = client.post("/api/admin/run-checks")
    assert resp.status_code == 200
    return resp.json()


def _job_by_name(client: httpx.Client, name: str) -> dict | None:
    for job in client.get("/api/jobs").json()["jobs"]:
        if job["name"] == name:
            return job
    return None


def _require_job(client: httpx.Client, name: str) -> dict:
    job = _job_by_name(client, name)
    assert job is not None, f"job {name!r} not found"
    return job


@pytest.mark.default
def test_progress_times_out(client: httpx.Client) -> None:
    job_id = _submit(client, "g", "slow", "progress")
    backdate("jobs", "name='slow'", updated_at=_ago(minutes=10))  # past 5-min default
    result = _run_checks(client)
    assert result["timeout_result"]["timeout_count"] == 1
    assert job_id in result["timeout_result"]["timeout_job_ids"]
    assert _require_job(client, "slow")["status"] == "timeout"


@pytest.mark.default
def test_success_goes_stale_when_enabled(client: httpx.Client) -> None:
    _submit(client, "g", "ok", "success")
    # Enable staleness with a 1h window (must be < the 24h expiration to pass validation).
    resp = client.put(
        "/api/groups/g/config",
        json={"staleness_enabled": True, "staleness_timeout_hours": 1},
    )
    assert resp.status_code == 200
    backdate("jobs", "name='ok'", updated_at=_ago(hours=2))
    result = _run_checks(client)
    assert result["timeout_result"]["stale_count"] == 1
    assert _require_job(client, "ok")["status"] == "stale"


@pytest.mark.default
def test_error_never_transitions(client: httpx.Client) -> None:
    _submit(client, "g", "bad", "error")
    backdate("jobs", "name='bad'", updated_at=_ago(hours=48))
    result = _run_checks(client)
    assert result["timeout_result"]["timeout_count"] == 0
    assert result["timeout_result"]["stale_count"] == 0
    assert _require_job(client, "bad")["status"] == "error"


@pytest.mark.default
def test_staleness_off_by_default(client: httpx.Client) -> None:
    _submit(client, "g", "ok", "success")
    backdate("jobs", "name='ok'", updated_at=_ago(hours=48))
    result = _run_checks(client)
    assert result["timeout_result"]["stale_count"] == 0
    assert _require_job(client, "ok")["status"] == "success"


@pytest.mark.default
def test_group_override_suppresses_timeout(client: httpx.Client) -> None:
    _submit(client, "g", "slow", "progress")
    resp = client.put("/api/groups/g/config", json={"progress_timeout_minutes": 30})
    assert resp.status_code == 200
    backdate(
        "jobs", "name='slow'", updated_at=_ago(minutes=10)
    )  # < the 30-min override
    result = _run_checks(client)
    assert result["timeout_result"]["timeout_count"] == 0
    assert _require_job(client, "slow")["status"] == "progress"


@pytest.mark.default
def test_expiry_deletes_job(client: httpx.Client) -> None:
    job_id = _submit(client, "g", "gone", "success")
    backdate("jobs", "name='gone'", expires_at=_ago(hours=1))
    result = _run_checks(client)
    assert job_id in result["expiration_result"]["expired_job_ids"]
    assert _job_by_name(client, "gone") is None


@pytest.mark.default
def test_expiry_deletes_acked_job(client: httpx.Client) -> None:
    # Acking does not shield a job from expiry.
    job_id = _submit(client, "g", "acked", "error")
    assert client.post(f"/api/jobs/{job_id}/ack").status_code == 200
    backdate("jobs", "name='acked'", expires_at=_ago(hours=1))
    result = _run_checks(client)
    assert job_id in result["expiration_result"]["expired_job_ids"]
    assert _job_by_name(client, "acked") is None


@pytest.mark.default
def test_expiry_preserves_unexpired(client: httpx.Client) -> None:
    _submit(client, "g", "fresh", "success")  # default expires_at is ~24h in the future
    result = _run_checks(client)
    assert result["expiration_result"]["expired_count"] == 0
    assert _job_by_name(client, "fresh") is not None


@pytest.mark.default
def test_timeout_and_stale_split_by_type(client: httpx.Client) -> None:
    timeout_id = _submit(client, "g1", "prog", "progress")
    stale_id = _submit(client, "g2", "succ", "success")
    resp = client.put(
        "/api/groups/g2/config",
        json={"staleness_enabled": True, "staleness_timeout_hours": 1},
    )
    assert resp.status_code == 200
    backdate("jobs", "name='prog'", updated_at=_ago(minutes=10))
    backdate("jobs", "name='succ'", updated_at=_ago(hours=2))

    timeout_result = _run_checks(client)["timeout_result"]
    assert timeout_result["timeout_job_ids"] == [timeout_id]
    assert timeout_result["stale_job_ids"] == [stale_id]
    # The split must be clean: a stale job is never reported as a timeout.
    assert stale_id not in timeout_result["timeout_job_ids"]
    assert timeout_id not in timeout_result["stale_job_ids"]


@pytest.mark.default
def test_expires_at_refreshed_on_update(client: httpx.Client) -> None:
    _submit(client, "g", "j", "success")
    # Age both timestamps into the past, then update: the update must recompute expires_at
    # to the future (insert path: updated_at + expiration), proving it was refreshed.
    backdate("jobs", "name='j'", updated_at=_ago(hours=1), expires_at=_ago(hours=1))
    _submit(client, "g", "j", "success")
    expires_at = _require_job(client, "j")["expires_at"]
    parsed = datetime.strptime(expires_at, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=UTC)
    assert parsed > datetime.now(UTC)
