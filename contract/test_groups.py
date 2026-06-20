"""Ported from backend/tests/test_api.py — GET /api/groups and group-jobs pagination.

AIDEV-NOTE: The backend test client auto-prefixed /api, so every `/groups` and `/status`
path here is spelled out in full. Mostly bucket 1 (runs as-is over the wire).
`…includes_zero_job_group` is bucket 2: a no-job group cannot be created via the API (a
group is created implicitly by the first job POST), so it is inserted via the conftest
`insert_group` helper. `…offset_pages_through_in_order` is bucket 3: the rows are POSTed
then backdated to distinct descending `updated_at`. `…limit_is_clamped_to_max` is bucket 4
(`max_page_size` profile, MAX_JOBS_PAGE_SIZE=2). The two CLI tests are ported from
backend/tests/test_integration.py::TestCliIntegration, and the final perf slice from
backend/tests/test_performance.py (the one HTTP-observable assertion).
"""

from datetime import UTC, datetime, timedelta

import httpx
import pytest

from conftest import backdate, insert_group


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


class TestGroupsEndpoint:
    """Tests for groups endpoints."""

    @pytest.mark.default
    def test_get_groups_empty(self, client: httpx.Client) -> None:
        """Test getting groups when none exist."""
        response = client.get("/api/groups")
        assert response.status_code == 200
        data = response.json()
        assert data["groups"] == []

    @pytest.mark.default
    def test_get_groups_with_data(self, client: httpx.Client) -> None:
        """Test getting groups with jobs."""
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "backups", "job": "job2", "status": "success"}
        )
        client.post(
            "/api/status",
            json={"group": "monitoring", "job": "job1", "status": "error"},
        )

        response = client.get("/api/groups")
        data = response.json()

        assert len(data["groups"]) == 2

        backups = next(g for g in data["groups"] if g["name"] == "backups")
        assert backups["job_count"] == 2
        assert backups["health"] == "healthy"
        assert backups["status_counts"]["success"] == 2

        monitoring = next(g for g in data["groups"] if g["name"] == "monitoring")
        assert monitoring["job_count"] == 1
        assert monitoring["health"] == "unhealthy"

    @pytest.mark.default
    def test_get_groups_includes_zero_job_group(self, client: httpx.Client) -> None:
        """A group with no jobs must still appear, with all-zero counts.

        AIDEV-NOTE: Bucket 2. Guards the aggregate /groups implementation against using
        an inner join (which would silently drop empty groups). A bare group cannot be
        created via the API, so it is inserted directly via conftest.insert_group.
        """
        insert_group("empty-group")
        client.post(
            "/api/status", json={"group": "busy", "job": "job1", "status": "success"}
        )

        response = client.get("/api/groups")
        data = response.json()

        names = {g["name"] for g in data["groups"]}
        assert names == {"empty-group", "busy"}

        empty = next(g for g in data["groups"] if g["name"] == "empty-group")
        assert empty["job_count"] == 0
        assert empty["health"] == "empty"
        assert empty["unhealthy_count"] == 0
        assert empty["acked_count"] == 0
        assert empty["status_counts"] == {
            "success": 0,
            "error": 0,
            "progress": 0,
            "timeout": 0,
            "stale": 0,
        }

    @pytest.mark.default
    def test_get_group_jobs(self, client: httpx.Client) -> None:
        """Test getting jobs for a specific group."""
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status",
            json={"group": "backups", "job": "job2", "status": "progress"},
        )

        response = client.get("/api/groups/backups/jobs")
        assert response.status_code == 200
        data = response.json()

        assert data["group"]["name"] == "backups"
        assert len(data["jobs"]) == 2

    @pytest.mark.default
    def test_get_group_jobs_not_found(self, client: httpx.Client) -> None:
        """Test getting jobs for non-existent group returns 404."""
        response = client.get("/api/groups/nonexistent/jobs")
        assert response.status_code == 404
        data = response.json()
        assert data["error"] == "not_found"

    @pytest.mark.default
    def test_get_group_jobs_case_insensitive(self, client: httpx.Client) -> None:
        """Test that group name lookup is case-insensitive."""
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.get("/api/groups/BACKUPS/jobs")
        assert response.status_code == 200


class TestGroupJobsPagination:
    """Tests for opt-in limit/offset pagination on GET /api/groups/<name>/jobs.

    AIDEV-NOTE: Mirrors GET /api/jobs -- opt-in and backward-compatible. With no params
    the full result set is returned; the response gains a `total` (full matching
    count for the group) alongside the existing `group` and `jobs` keys.
    """

    @pytest.mark.default
    def test_no_params_returns_all_with_total(self, client: httpx.Client) -> None:
        _make(client, 5)
        data = client.get("/api/groups/g1/jobs").json()
        assert len(data["jobs"]) == 5
        assert data["total"] == 5

    @pytest.mark.default
    def test_limit_returns_slice_with_full_total(self, client: httpx.Client) -> None:
        _make(client, 5)
        data = client.get("/api/groups/g1/jobs?limit=2").json()
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

        page1 = client.get("/api/groups/g1/jobs?limit=2&offset=0").json()
        page2 = client.get("/api/groups/g1/jobs?limit=2&offset=2").json()

        assert [j["name"] for j in page1["jobs"]] == ["job0", "job1"]
        assert [j["name"] for j in page2["jobs"]] == ["job2", "job3"]
        assert page1["total"] == 5
        assert page2["total"] == 5

    @pytest.mark.default
    def test_offset_beyond_end_returns_empty(self, client: httpx.Client) -> None:
        _make(client, 3)
        data = client.get("/api/groups/g1/jobs?limit=10&offset=100").json()
        assert data["jobs"] == []
        assert data["total"] == 3

    @pytest.mark.max_page_size
    def test_limit_is_clamped_to_max(self, client: httpx.Client) -> None:
        """AIDEV-NOTE: Bucket 4. Under the `max_page_size` profile MAX_JOBS_PAGE_SIZE=2,
        so a limit=100 request clamps to 2 while total stays the full count.
        """
        _make(client, 5)
        data = client.get("/api/groups/g1/jobs?limit=100").json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    @pytest.mark.default
    def test_total_is_scoped_to_group(self, client: httpx.Client) -> None:
        _make(client, 3, group="g1")
        _make(client, 4, group="g2")
        data = client.get("/api/groups/g1/jobs?limit=1").json()
        assert len(data["jobs"]) == 1
        assert data["total"] == 3

    @pytest.mark.default
    def test_invalid_limit_returns_400(self, client: httpx.Client) -> None:
        _make(client, 1)
        resp = client.get("/api/groups/g1/jobs?limit=abc")
        assert resp.status_code == 400
        assert resp.json()["field"] == "limit"

    @pytest.mark.default
    def test_zero_limit_returns_400(self, client: httpx.Client) -> None:
        _make(client, 1)
        resp = client.get("/api/groups/g1/jobs?limit=0")
        assert resp.status_code == 400
        assert resp.json()["field"] == "limit"

    @pytest.mark.default
    def test_negative_offset_returns_400(self, client: httpx.Client) -> None:
        _make(client, 1)
        resp = client.get("/api/groups/g1/jobs?offset=-1")
        assert resp.status_code == 400
        assert resp.json()["field"] == "offset"


class TestCliIntegration:
    """Tests verifying the CLI can list groups and fetch group jobs.

    AIDEV-NOTE: Ported from test_integration.py -- the group/jobs shapes the CLI client
    (statshed_cli/client.py) depends on for its display.
    """

    @pytest.mark.default
    def test_cli_list_groups(self, client: httpx.Client) -> None:
        """Test CLI can list groups with health summaries."""
        client.post(
            "/api/status",
            json={"group": "backups", "job": "daily", "status": "success"},
        )
        client.post(
            "/api/status",
            json={"group": "backups", "job": "weekly", "status": "success"},
        )
        client.post(
            "/api/status",
            json={"group": "monitoring", "job": "check", "status": "error"},
        )

        response = client.get("/api/groups")
        assert response.status_code == 200
        data = response.json()

        assert "groups" in data
        assert len(data["groups"]) == 2

        # Verify groups have expected fields for CLI display
        backups = next(g for g in data["groups"] if g["name"] == "backups")
        assert "job_count" in backups
        assert "health" in backups
        assert "status_counts" in backups
        assert backups["health"] == "healthy"

    @pytest.mark.default
    def test_cli_get_jobs(self, client: httpx.Client) -> None:
        """Test CLI can get jobs for a specific group."""
        client.post(
            "/api/status",
            json={"group": "builds", "job": "frontend", "status": "success"},
        )
        client.post(
            "/api/status",
            json={"group": "builds", "job": "backend", "status": "progress"},
        )

        response = client.get("/api/groups/builds/jobs")
        assert response.status_code == 200
        data = response.json()

        assert "group" in data
        assert "jobs" in data
        assert len(data["jobs"]) == 2

        # Verify job fields for CLI display
        job = data["jobs"][0]
        assert "name" in job
        assert "status" in job
        assert "message" in job
        assert "updated_at" in job


class TestGroupJobsPaginationPerf:
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
        data = client.get("/api/groups/g1/jobs?limit=2").json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 3
