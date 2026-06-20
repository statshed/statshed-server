"""Ported from backend/tests/test_api.py — GET /api/jobs listing and pagination.

AIDEV-NOTE: The backend test client auto-prefixed /api, so every `/jobs` and `/status`
path here is spelled out in full against the real URL space. Mostly bucket 1 (runs
as-is over the wire). Two ordering tests (`…ordered_by_updated_at_desc`,
`…offset_pages_through_in_order`) are bucket 3: the server always writes 'now', so the
ORM-built rows with distinct `updated_at` are re-expressed by POSTing then backdating via
the conftest helper. The `…limit_is_clamped_to_max` test is bucket 4 (`max_page_size`
profile, MAX_JOBS_PAGE_SIZE=2). The final perf slice is ported from
backend/tests/test_performance.py (the one HTTP-observable assertion: a `limit` page
returns the page size while `total` is the full count).
"""

from datetime import UTC, datetime, timedelta

import httpx
import pytest

from conftest import backdate


def _make(
    client: httpx.Client, count: int, group: str = "g1", status: str = "success"
) -> None:
    """POST `count` jobs (job0..jobN-1) into `group`, asserting each 201."""
    for i in range(count):
        resp = client.post(
            "/api/status",
            json={"group": group, "job": f"job{i}", "status": status},
        )
        assert resp.status_code == 201


class TestJobsEndpoint:
    """Tests for GET /api/jobs endpoint.

    AIDEV-NOTE: Tests for the jobs listing endpoint used by health card click-through.
    """

    @pytest.mark.default
    def test_get_jobs_empty(self, client: httpx.Client) -> None:
        """Test getting jobs when none exist."""
        response = client.get("/api/jobs")
        assert response.status_code == 200
        data = response.json()
        assert data["jobs"] == []
        assert data["total"] == 0

    @pytest.mark.default
    def test_get_jobs_all(self, client: httpx.Client) -> None:
        """Test getting all jobs without filter."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g2", "job": "job1", "status": "progress"}
        )

        response = client.get("/api/jobs")
        assert response.status_code == 200
        data = response.json()
        assert data["total"] == 3
        assert len(data["jobs"]) == 3

    @pytest.mark.default
    def test_get_jobs_filter_single_status(self, client: httpx.Client) -> None:
        """Test filtering jobs by single status."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job3", "status": "success"}
        )

        response = client.get("/api/jobs?status=success")
        assert response.status_code == 200
        data = response.json()
        assert data["total"] == 2
        assert all(job["status"] == "success" for job in data["jobs"])

    @pytest.mark.default
    def test_get_jobs_filter_multiple_statuses(self, client: httpx.Client) -> None:
        """Test filtering jobs by multiple comma-separated statuses."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job3", "status": "timeout"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job4", "status": "progress"}
        )

        # This matches the "Errors" card behavior (error + timeout)
        response = client.get("/api/jobs?status=error,timeout")
        assert response.status_code == 200
        data = response.json()
        assert data["total"] == 2
        statuses = {job["status"] for job in data["jobs"]}
        assert statuses == {"error", "timeout"}

    @pytest.mark.default
    def test_get_jobs_invalid_status(self, client: httpx.Client) -> None:
        """Test filtering with invalid status returns 400."""
        response = client.get("/api/jobs?status=invalid")
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "validation_error"
        assert "Invalid status" in data["message"]
        assert data["field"] == "status"

    @pytest.mark.default
    def test_get_jobs_one_valid_one_invalid_status(self, client: httpx.Client) -> None:
        """Test filtering with one valid and one invalid status returns 400."""
        response = client.get("/api/jobs?status=success,badstatus")
        assert response.status_code == 400
        data = response.json()
        assert "Invalid status 'badstatus'" in data["message"]

    @pytest.mark.default
    def test_get_jobs_empty_status_returns_all(self, client: httpx.Client) -> None:
        """Test that empty status parameter returns all jobs."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )

        response = client.get("/api/jobs?status=")
        assert response.status_code == 200
        data = response.json()
        assert data["total"] == 2

    @pytest.mark.default
    def test_get_jobs_includes_group_name(self, client: httpx.Client) -> None:
        """Test that jobs include group_name in response."""
        client.post(
            "/api/status",
            json={"group": "my-group", "job": "job1", "status": "success"},
        )

        response = client.get("/api/jobs")
        data = response.json()
        assert data["jobs"][0]["group_name"] == "my-group"

    @pytest.mark.default
    def test_get_jobs_ordered_by_updated_at_desc(self, client: httpx.Client) -> None:
        """Test that jobs are ordered by updated_at descending (newest first).

        AIDEV-NOTE: Bucket 3. The original built ORM rows with distinct updated_at;
        here the rows are POSTed and then backdated to distinct descending timestamps
        so newest-first ordering is deterministic.
        """
        for name in ("older-job", "newer-job", "newest-job"):
            resp = client.post(
                "/api/status",
                json={"group": "test-group", "job": name, "status": "success"},
            )
            assert resp.status_code == 201

        now = datetime.now(UTC)
        offsets = {"newest-job": 0, "newer-job": 1, "older-job": 2}
        for name, hours in offsets.items():
            ts = (now - timedelta(hours=hours)).strftime("%Y-%m-%d %H:%M:%S.%f")
            backdate("jobs", f"name='{name}'", updated_at=ts)

        response = client.get("/api/jobs")
        data = response.json()

        # Verify order: newest first
        assert data["jobs"][0]["name"] == "newest-job"
        assert data["jobs"][1]["name"] == "newer-job"
        assert data["jobs"][2]["name"] == "older-job"

    @pytest.mark.default
    def test_get_jobs_response_structure(self, client: httpx.Client) -> None:
        """Test that response structure matches the expected format."""
        client.post(
            "/api/status",
            json={
                "group": "backups",
                "job": "daily-backup",
                "status": "error",
                "message": "Connection failed",
            },
        )

        response = client.get("/api/jobs")
        data = response.json()

        assert "jobs" in data
        assert "total" in data

        job = data["jobs"][0]
        assert "id" in job
        assert "name" in job
        assert "group_id" in job
        assert "group_name" in job
        assert "status" in job
        assert "message" in job
        assert "updated_at" in job
        assert "created_at" in job

        # Verify specific values
        assert job["name"] == "daily-backup"
        assert job["group_name"] == "backups"
        assert job["status"] == "error"
        assert job["message"] == "Connection failed"

    @pytest.mark.default
    def test_get_jobs_filter_whitespace_handling(self, client: httpx.Client) -> None:
        """Test that whitespace in status parameter is handled."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )

        # Status with extra whitespace
        response = client.get("/api/jobs?status= success , error ")
        assert response.status_code == 200
        data = response.json()
        assert data["total"] == 2


class TestJobsPagination:
    """Tests for opt-in limit/offset pagination on GET /api/jobs.

    AIDEV-NOTE: Pagination is opt-in and backward-compatible: with no params the
    response is unchanged (all jobs, total == len(jobs)). When limit/offset are
    given, `total` is the full matching count, not the page size.
    """

    @pytest.mark.default
    def test_no_params_returns_all(self, client: httpx.Client) -> None:
        _make(client, 5)
        data = client.get("/api/jobs").json()
        assert len(data["jobs"]) == 5
        assert data["total"] == 5

    @pytest.mark.default
    def test_limit_returns_slice_with_full_total(self, client: httpx.Client) -> None:
        _make(client, 5)
        data = client.get("/api/jobs?limit=2").json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    @pytest.mark.default
    def test_offset_pages_through_in_order(self, client: httpx.Client) -> None:
        """AIDEV-NOTE: Bucket 3. POST then backdate to distinct descending updated_at
        so the page slices are deterministic (newest first: j0 .. j4 oldest).
        """
        _make(client, 5)
        now = datetime.now(UTC)
        for i in range(5):
            ts = (now - timedelta(hours=i)).strftime("%Y-%m-%d %H:%M:%S.%f")
            backdate("jobs", f"name='job{i}'", updated_at=ts)

        page1 = client.get("/api/jobs?limit=2&offset=0").json()
        page2 = client.get("/api/jobs?limit=2&offset=2").json()
        page3 = client.get("/api/jobs?limit=2&offset=4").json()

        assert [j["name"] for j in page1["jobs"]] == ["job0", "job1"]
        assert [j["name"] for j in page2["jobs"]] == ["job2", "job3"]
        assert [j["name"] for j in page3["jobs"]] == ["job4"]
        for page in (page1, page2, page3):
            assert page["total"] == 5

    @pytest.mark.default
    def test_offset_beyond_end_returns_empty(self, client: httpx.Client) -> None:
        _make(client, 3)
        data = client.get("/api/jobs?limit=10&offset=100").json()
        assert data["jobs"] == []
        assert data["total"] == 3

    @pytest.mark.max_page_size
    def test_limit_is_clamped_to_max(self, client: httpx.Client) -> None:
        """AIDEV-NOTE: Bucket 4. Under the `max_page_size` profile MAX_JOBS_PAGE_SIZE=2,
        so a limit=100 request clamps to 2 while total stays the full count.
        """
        _make(client, 5)
        data = client.get("/api/jobs?limit=100").json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    @pytest.mark.default
    def test_pagination_respects_status_filter(self, client: httpx.Client) -> None:
        _make(client, 3, status="success")
        _make(client, 2, group="g1", status="error")
        data = client.get("/api/jobs?status=error&limit=1").json()
        assert len(data["jobs"]) == 1
        assert data["jobs"][0]["status"] == "error"
        # total reflects the filtered set, not all jobs
        assert data["total"] == 2

    @pytest.mark.default
    def test_invalid_limit_returns_400(self, client: httpx.Client) -> None:
        resp = client.get("/api/jobs?limit=abc")
        assert resp.status_code == 400
        data = resp.json()
        assert data["error"] == "validation_error"
        assert data["field"] == "limit"

    @pytest.mark.default
    def test_zero_limit_returns_400(self, client: httpx.Client) -> None:
        resp = client.get("/api/jobs?limit=0")
        assert resp.status_code == 400
        assert resp.json()["field"] == "limit"

    @pytest.mark.default
    def test_negative_offset_returns_400(self, client: httpx.Client) -> None:
        resp = client.get("/api/jobs?offset=-1")
        assert resp.status_code == 400
        assert resp.json()["field"] == "offset"

    @pytest.mark.default
    def test_invalid_offset_returns_400(self, client: httpx.Client) -> None:
        resp = client.get("/api/jobs?offset=xyz")
        assert resp.status_code == 400
        assert resp.json()["field"] == "offset"


class TestJobsPaginationPerf:
    """Ported HTTP-observable slice of backend/tests/test_performance.py.

    AIDEV-NOTE: The SQL-shape assertions have no HTTP analog (Go-side, coverage-map §7);
    the one observable invariant — a `limit` page returns the page size while `total`
    stays the full match count — is asserted here over the wire.
    """

    @pytest.mark.default
    def test_limit_page_returns_page_size_with_full_total(
        self, client: httpx.Client
    ) -> None:
        _make(client, 3)
        data = client.get("/api/jobs?limit=2").json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 3
