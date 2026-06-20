"""Concurrency / integrity invariants over HTTP (spec.md §8.3 buckets 1 & 6).

AIDEV-NOTE: The Python suite forced the create-or-update race with monkeypatched
`flush` (Python-only). The wire suite instead asserts the OBSERVABLE invariants: rapid
sequential writes don't corrupt state, and concurrent POSTs to the same new group/job
yield no 5xx and exactly one group/one job — whether or not the IntegrityError-retry path
actually fires. Ported from test_integration.py TestRapidSubmissions + TestIntegrityErrorRetry.
"""

import concurrent.futures

import httpx
import pytest

# --- Rapid sequential writes (bucket 1) -----------------------------------------


@pytest.mark.default
def test_rapid_status_submissions_same_job(client: httpx.Client) -> None:
    for i in range(20):
        resp = client.post(
            "/api/status",
            json={
                "group": "rapid",
                "job": "shared-job",
                "status": "success",
                "message": f"Update {i}",
            },
        )
        assert resp.status_code == 201
    jobs = client.get("/api/groups/rapid/jobs").json()["jobs"]
    assert len(jobs) == 1
    assert jobs[0]["status"] == "success"
    assert jobs[0]["message"] == "Update 19"  # last write wins


@pytest.mark.default
def test_rapid_job_creation_same_group(client: httpx.Client) -> None:
    for i in range(10):
        resp = client.post(
            "/api/status",
            json={"group": "rapid", "job": f"job-{i}", "status": "success"},
        )
        assert resp.status_code == 201
    jobs = client.get("/api/groups/rapid/jobs").json()["jobs"]
    assert len(jobs) == 10  # exactly 10 rows (a set comparison alone would hide a dup)
    assert {j["name"] for j in jobs} == {f"job-{i}" for i in range(10)}


@pytest.mark.default
def test_rapid_submissions_multiple_groups(client: httpx.Client) -> None:
    for g in range(5):
        for j in range(4):
            resp = client.post(
                "/api/status",
                json={"group": f"group-{g}", "job": f"job-{j}", "status": "success"},
            )
            assert resp.status_code == 201
    groups = client.get("/api/groups").json()["groups"]
    assert len(groups) == 5
    assert all(g["job_count"] == 4 for g in groups)


@pytest.mark.default
def test_rapid_config_updates(client: httpx.Client) -> None:
    for value in (5, 10, 15, 20, 25):
        resp = client.put("/api/config", json={"progress_timeout_minutes": value})
        assert resp.status_code == 200
    assert client.get("/api/config").json()["progress_timeout_minutes"] == 25


@pytest.mark.default
def test_rapid_status_transitions(client: httpx.Client) -> None:
    for status in ("progress", "success", "error", "progress", "success"):
        resp = client.post(
            "/api/status", json={"group": "t", "job": "j", "status": status}
        )
        assert resp.status_code == 201
        assert resp.json()["job"]["status"] == status
    assert client.get("/api/jobs").json()["jobs"][0]["status"] == "success"


# --- Concurrent invariants (bucket 6) -------------------------------------------


def _concurrent_posts(
    client: httpx.Client, payloads: list[dict]
) -> list[httpx.Response]:
    # httpx.Client is safe to use concurrently across threads (shared connection pool).
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(payloads)) as pool:
        futures = [pool.submit(client.post, "/api/status", json=p) for p in payloads]
        return [f.result() for f in futures]


@pytest.mark.default
def test_concurrent_same_group_different_jobs(client: httpx.Client) -> None:
    # Concurrent POSTs to the same NEW group -> exactly one group row, all jobs, no 5xx.
    payloads = [
        {"group": "race", "job": f"job-{i}", "status": "success"} for i in range(10)
    ]
    responses = _concurrent_posts(client, payloads)
    assert all(r.status_code == 201 for r in responses)

    groups = [
        g for g in client.get("/api/groups").json()["groups"] if g["name"] == "race"
    ]
    assert len(groups) == 1
    assert groups[0]["job_count"] == 10


@pytest.mark.default
def test_concurrent_same_group_same_job(client: httpx.Client) -> None:
    # Concurrent POSTs to the same NEW group AND job -> one group, one job, no 5xx.
    payloads = [
        {"group": "race2", "job": "shared", "status": "success"} for _ in range(10)
    ]
    responses = _concurrent_posts(client, payloads)
    assert all(r.status_code == 201 for r in responses)
    groups = [g for g in client.get("/api/groups").json()["groups"] if g["name"] == "race2"]
    assert len(groups) == 1  # exactly one group row...
    assert len(client.get("/api/groups/race2/jobs").json()["jobs"]) == 1  # ...and one job
