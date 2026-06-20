"""Ported from backend/tests/test_api.py — admin endpoints over HTTP.

AIDEV-NOTE: Covers GET /api/admin/stats and DELETE /api/admin/cleanup (data
retention). Mostly Bucket 1; the three cleanup tests that need aged rows are Bucket 3 --
they POST jobs normally, then `conftest.backdate()` ages `updated_at` ~60 days into the
past via direct SQL (the server only ever writes 'now'). DELETE-with-a-body goes through
client.request("DELETE", ...) since httpx's delete() shortcut takes no body.
"""

from datetime import UTC, datetime, timedelta

import httpx
import pytest

from conftest import backdate

# ~60 days in the past, in the app's stored text format (matches conftest helpers).
OLD_TS = (datetime.now(UTC) - timedelta(days=60)).strftime("%Y-%m-%d %H:%M:%S.%f")


class TestAdminEndpoints:
    """Tests for admin endpoints (data retention & cleanup)."""

    @pytest.mark.default
    def test_get_admin_stats_empty(self, client: httpx.Client) -> None:
        """Test admin stats when database is empty."""
        response = client.get("/api/admin/stats")
        assert response.status_code == 200
        data = response.json()
        assert data["total_jobs"] == 0
        assert data["total_groups"] == 0
        assert data["jobs_by_status"]["success"] == 0
        assert data["jobs_by_status"]["error"] == 0

    @pytest.mark.default
    def test_get_admin_stats_with_data(self, client: httpx.Client) -> None:
        """Test admin stats with jobs in database."""
        # Create some jobs with different statuses.
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "success"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g2", "job": "job1", "status": "progress"}
        )

        response = client.get("/api/admin/stats")
        assert response.status_code == 200
        data = response.json()
        assert data["total_jobs"] == 3
        assert data["total_groups"] == 2
        assert data["jobs_by_status"]["success"] == 1
        assert data["jobs_by_status"]["error"] == 1
        assert data["jobs_by_status"]["progress"] == 1

    @pytest.mark.default
    def test_admin_cleanup_requires_older_than_days(self, client: httpx.Client) -> None:
        """Test cleanup requires the older_than_days parameter."""
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"statuses": ["stale"]},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["field"] == "older_than_days"

    @pytest.mark.default
    def test_admin_cleanup_invalid_older_than_days(self, client: httpx.Client) -> None:
        """Test cleanup rejects a non-positive older_than_days."""
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 0},
        )
        assert response.status_code == 400
        assert "positive integer" in response.json()["message"]

    @pytest.mark.default
    def test_admin_cleanup_invalid_status(self, client: httpx.Client) -> None:
        """Test cleanup rejects invalid status values."""
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["invalid"]},
        )
        assert response.status_code == 400
        assert "Invalid status" in response.json()["message"]

    @pytest.mark.default
    def test_admin_cleanup_dry_run(self, client: httpx.Client) -> None:
        """Test cleanup dry_run returns counts without deleting.

        AIDEV-NOTE: Bucket 3 -- POST an aged stale job (backdated via SQL), then verify
        dry_run reports it but the job still appears in GET /api/jobs afterward.
        """
        client.post(
            "/api/status", json={"group": "test-group", "job": "old", "status": "stale"}
        )
        backdate("jobs", "name='old'", updated_at=OLD_TS)

        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"], "dry_run": True},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["deleted_jobs"] == 1
        assert data["deleted_groups"] == 1
        assert data["dry_run"] is True

        # Dry run must NOT delete: the job still exists.
        names = {j["name"] for j in client.get("/api/jobs").json()["jobs"]}
        assert "old" in names

    @pytest.mark.default
    def test_admin_cleanup_deletes_jobs(self, client: httpx.Client) -> None:
        """Test cleanup actually deletes old jobs.

        AIDEV-NOTE: Bucket 3 -- same aged stale job, but dry_run=False removes it.
        """
        client.post(
            "/api/status", json={"group": "test-group", "job": "old", "status": "stale"}
        )
        backdate("jobs", "name='old'", updated_at=OLD_TS)

        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"], "dry_run": False},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["deleted_jobs"] == 1
        assert data["deleted_groups"] == 1
        assert data["dry_run"] is False

        # The job is gone.
        names = {j["name"] for j in client.get("/api/jobs").json()["jobs"]}
        assert "old" not in names

    @pytest.mark.default
    def test_admin_cleanup_preserves_recent_jobs(self, client: httpx.Client) -> None:
        """Test cleanup doesn't delete recent jobs."""
        # Create a recent job (not backdated).
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "stale"}
        )

        # Try to cleanup; the job is recent so nothing is deleted.
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"]},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["deleted_jobs"] == 0
        assert data["deleted_groups"] == 0

    @pytest.mark.default
    def test_admin_cleanup_respects_status_filter(self, client: httpx.Client) -> None:
        """Test cleanup only deletes jobs with the specified statuses.

        AIDEV-NOTE: Bucket 3 -- an aged `stale` job and an aged `error` job. Default
        cleanup statuses are ["stale","timeout"], so the error job survives.
        """
        client.post(
            "/api/status",
            json={"group": "test-group", "job": "stale-job", "status": "stale"},
        )
        client.post(
            "/api/status",
            json={"group": "test-group", "job": "error-job", "status": "error"},
        )
        backdate("jobs", "name='stale-job'", updated_at=OLD_TS)
        backdate("jobs", "name='error-job'", updated_at=OLD_TS)

        # Cleanup with default statuses (stale, timeout).
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={"older_than_days": 30},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["deleted_jobs"] == 1  # Only the stale job.

        # The error job still exists.
        names = {j["name"] for j in client.get("/api/jobs").json()["jobs"]}
        assert "error-job" in names

    @pytest.mark.default
    def test_admin_cleanup_requires_json(self, client: httpx.Client) -> None:
        """Test cleanup rejects an empty body (older_than_days is required)."""
        response = client.request(
            "DELETE",
            "/api/admin/cleanup",
            json={},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["field"] == "older_than_days"
