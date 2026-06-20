"""Ported from backend/tests/test_api.py — DELETE /api/jobs/<id> behavior.

AIDEV-NOTE: Delete returns 200 with `{deleted_job: {...}, group_id, group_name}` (not a
count -- behavioral-map §2) and removes the job; a second delete of the same id is a
404 not_found. Deleting also flows through to health and group aggregates. The backend
test client auto-prefixed /api, so every `/status`, `/jobs`, `/jobs/<id>`, `/health`,
and `/groups` path here is spelled out in full. Job ids come straight from the POST
response. Bucket 1 (runs as-is over the wire).
"""

import httpx
import pytest


class TestDeleteJobEndpoint:
    """Tests for DELETE /api/jobs/<id> endpoint."""

    @pytest.mark.default
    def test_delete_job_success(self, client: httpx.Client) -> None:
        """Test deleting an existing job returns 200 and removes it."""
        # Create a job; the POST response carries its id.
        created = client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )
        assert created.status_code == 201
        job_id = created.json()["job"]["id"]

        assert client.get("/api/jobs").json()["total"] == 1

        # Delete the job
        response = client.delete(f"/api/jobs/{job_id}")
        assert response.status_code == 200
        data = response.json()
        assert data["deleted_job"]["id"] == job_id
        assert data["deleted_job"]["name"] == "job1"
        assert data["group_name"] == "test"

        # Verify job is gone
        assert client.get("/api/jobs").json()["total"] == 0

    @pytest.mark.default
    def test_delete_job_not_found(self, client: httpx.Client) -> None:
        """Test deleting a non-existent job returns 404."""
        # Create and delete a job to get a known non-existent id.
        created = client.post(
            "/api/status", json={"group": "test", "job": "temp", "status": "success"}
        )
        deleted_id = created.json()["job"]["id"]
        client.delete(f"/api/jobs/{deleted_id}")

        # Now try to delete the already-deleted job
        response = client.delete(f"/api/jobs/{deleted_id}")
        assert response.status_code == 404
        data = response.json()
        assert data["error"] == "not_found"

    @pytest.mark.default
    def test_delete_job_twice_returns_404(self, client: httpx.Client) -> None:
        """Test deleting the same job twice returns 404 on second call."""
        created = client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )
        job_id = created.json()["job"]["id"]

        # First delete succeeds
        response = client.delete(f"/api/jobs/{job_id}")
        assert response.status_code == 200

        # Second delete returns 404
        response = client.delete(f"/api/jobs/{job_id}")
        assert response.status_code == 404

    @pytest.mark.default
    def test_delete_job_updates_health(self, client: httpx.Client) -> None:
        """Test that deleting a job updates health counts."""
        # Create an error job
        created = client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        job_id = created.json()["job"]["id"]

        # Verify unhealthy count
        assert client.get("/api/health").json()["unhealthy"] == 1

        client.delete(f"/api/jobs/{job_id}")

        # Verify health is now empty
        data = client.get("/api/health").json()
        assert data["status"] == "empty"
        assert data["total_jobs"] == 0

    @pytest.mark.default
    def test_delete_job_updates_group_counts(self, client: httpx.Client) -> None:
        """Test that deleting a job updates group job count."""
        # Create two jobs in the same group.
        created = client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )
        job_id = created.json()["job"]["id"]
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Verify group has 2 jobs (find by name to avoid ordering assumptions).
        group = next(
            g for g in client.get("/api/groups").json()["groups"] if g["name"] == "test"
        )
        assert group["job_count"] == 2

        # Delete one job.
        client.delete(f"/api/jobs/{job_id}")

        # Verify group now has 1 job.
        group = next(
            g for g in client.get("/api/groups").json()["groups"] if g["name"] == "test"
        )
        assert group["job_count"] == 1
