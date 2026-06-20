"""Ported from backend/tests/test_api.py — job acknowledgement behavior.

AIDEV-NOTE: Only error/timeout/stale jobs are ack-eligible (success/progress -> 400
invalid_state); ack is idempotent (acked_at unchanged on re-ack); a recovery to
success/progress clears acked. Health/group `unhealthy` counts EXCLUDE acked jobs,
while by_status/status_counts count everything raw. Single ack returns `{"job": ...}`;
group-ack and ack-all return `{"acked_count": N, ...}` (behavioral-map §2). The backend
test client auto-prefixed /api, so every path here is spelled out in full. Bucket 1.
"""

import httpx
import pytest


class TestAckEndpoint:
    """Tests for job acknowledgement endpoints.

    AIDEV-NOTE: Tests for the ack feature that allows acknowledging
    errors/timeouts/stale jobs.
    """

    @pytest.mark.default
    def test_ack_job_success(self, client: httpx.Client) -> None:
        """Test acknowledging a job in error state."""
        # Create an error job
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # Get job ID
        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        # Ack the job
        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 200
        data = response.json()
        assert data["job"]["acked"] is True
        assert data["job"]["acked_at"] is not None
        assert data["job"]["status"] == "error"

    @pytest.mark.default
    def test_ack_job_timeout_status(self, client: httpx.Client) -> None:
        """Test acknowledging a job in timeout state."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "timeout"}
        )

        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 200
        assert response.json()["job"]["acked"] is True

    @pytest.mark.default
    def test_ack_job_stale_status(self, client: httpx.Client) -> None:
        """Test acknowledging a job in stale state."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "stale"}
        )

        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 200
        assert response.json()["job"]["acked"] is True

    @pytest.mark.default
    def test_ack_job_not_found(self, client: httpx.Client) -> None:
        """Test acknowledging a non-existent job returns 404."""
        response = client.post("/api/jobs/99999/ack")
        assert response.status_code == 404
        data = response.json()
        assert data["error"] == "not_found"

    @pytest.mark.default
    def test_ack_job_invalid_state_success(self, client: httpx.Client) -> None:
        """Test that acking a success job returns 400."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "invalid_state"
        assert "Cannot ack job with status 'success'" in data["message"]

    @pytest.mark.default
    def test_ack_job_invalid_state_progress(self, client: httpx.Client) -> None:
        """Test that acking a progress job returns 400."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 400
        assert "Cannot ack job with status 'progress'" in response.json()["message"]

    @pytest.mark.default
    def test_ack_job_idempotent(self, client: httpx.Client) -> None:
        """Test that acking an already-acked job is a no-op."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        response = client.get("/api/jobs")
        job_id = response.json()["jobs"][0]["id"]

        # First ack
        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 200
        first_acked_at = response.json()["job"]["acked_at"]

        # Second ack (should be no-op, acked_at should not change)
        response = client.post(f"/api/jobs/{job_id}/ack")
        assert response.status_code == 200
        second_acked_at = response.json()["job"]["acked_at"]

        # acked_at should be the same (no update)
        assert first_acked_at == second_acked_at


class TestAckHealthCalculation:
    """Tests for health calculation with acked jobs.

    AIDEV-NOTE: Verifies that acked jobs are excluded from unhealthy count.
    """

    @pytest.mark.default
    def test_health_excludes_acked_from_unhealthy(self, client: httpx.Client) -> None:
        """Test that acked jobs are excluded from unhealthy count."""
        # Create an error job
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # Before ack: unhealthy = 1
        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1
        assert data["acked"] == 0

        # Ack the job
        jobs_response = client.get("/api/jobs")
        job_id = jobs_response.json()["jobs"][0]["id"]
        client.post(f"/api/jobs/{job_id}/ack")

        # After ack: unhealthy = 0, acked = 1
        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 1

    @pytest.mark.default
    def test_health_status_healthy_when_all_errors_acked(
        self, client: httpx.Client
    ) -> None:
        """Test that dashboard shows healthy when all errors are acked."""
        # Create multiple error jobs
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "timeout"}
        )

        response = client.get("/api/health")
        assert response.json()["status"] == "unhealthy"

        # Ack all jobs
        jobs = client.get("/api/jobs").json()["jobs"]
        for job in jobs:
            client.post(f"/api/jobs/{job['id']}/ack")

        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 2

    @pytest.mark.default
    def test_health_mixed_acked_and_unacked(self, client: httpx.Client) -> None:
        """Test health with a mix of acked and unacked errors."""
        # Create two error jobs
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "error"}
        )

        # Ack only one job
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1
        assert data["acked"] == 1

    @pytest.mark.default
    def test_health_by_status_includes_acked(self, client: httpx.Client) -> None:
        """Test that by_status counts include acked jobs (raw counts)."""
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "error"}
        )

        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        response = client.get("/api/health")
        data = response.json()
        # by_status should still show raw count
        assert data["by_status"]["error"] == 1


class TestAckGroupSummary:
    """Tests for group summary with acked jobs."""

    @pytest.mark.default
    def test_group_unhealthy_count_excludes_acked(self, client: httpx.Client) -> None:
        """Test that group unhealthy_count excludes acked jobs."""
        # Create error jobs in a group
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Before ack
        response = client.get("/api/groups")
        group = response.json()["groups"][0]
        assert group["unhealthy_count"] == 1
        assert group["acked_count"] == 0
        assert group["health"] == "unhealthy"

        # Ack the error job
        jobs = client.get("/api/jobs?status=error").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # After ack
        response = client.get("/api/groups")
        group = response.json()["groups"][0]
        assert group["unhealthy_count"] == 0
        assert group["acked_count"] == 1
        assert group["health"] == "healthy"

    @pytest.mark.default
    def test_group_status_counts_include_acked(self, client: httpx.Client) -> None:
        """Test that status_counts include acked jobs (raw counts)."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        response = client.get("/api/groups")
        group = response.json()["groups"][0]
        # status_counts should still show raw count
        assert group["status_counts"]["error"] == 1


class TestAckClearOnRecovery:
    """Tests for ack clearing when job recovers."""

    @pytest.mark.default
    def test_ack_cleared_on_success(self, client: httpx.Client) -> None:
        """Test that ack is cleared when job transitions to success."""
        # Create error job and ack it
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Verify acked
        jobs = client.get("/api/jobs").json()["jobs"]
        assert jobs[0]["acked"] is True

        # Job recovers to success
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Ack should be cleared
        jobs = client.get("/api/jobs").json()["jobs"]
        assert jobs[0]["acked"] is False
        assert jobs[0]["acked_at"] is None

    @pytest.mark.default
    def test_ack_cleared_on_progress(self, client: httpx.Client) -> None:
        """Test that ack is cleared when job transitions to progress."""
        # Create error job and ack it
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Job transitions to progress
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Ack should be cleared
        jobs = client.get("/api/jobs").json()["jobs"]
        assert jobs[0]["acked"] is False

    @pytest.mark.default
    def test_ack_preserved_on_new_error(self, client: httpx.Client) -> None:
        """Test that ack is preserved when an acked job gets another error submission."""
        # Create error job and ack it
        client.post(
            "/api/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "error",
                "message": "fail1",
            },
        )
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Submit new error (same job, new error message)
        client.post(
            "/api/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "error",
                "message": "fail2",
            },
        )

        # Ack should be preserved
        jobs = client.get("/api/jobs").json()["jobs"]
        assert jobs[0]["acked"] is True
        assert jobs[0]["message"] == "fail2"

    @pytest.mark.default
    def test_error_after_recovery_requires_new_ack(self, client: httpx.Client) -> None:
        """Test that error after recovery requires a new ack."""
        # Create error job and ack it
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Job recovers
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Job errors again
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # Should require new ack (acked should be False)
        jobs = client.get("/api/jobs").json()["jobs"]
        assert jobs[0]["acked"] is False

        response = client.get("/api/health")
        assert response.json()["status"] == "unhealthy"


class TestJobsResponseIncludesAckedFields:
    """Tests that job responses include acked fields."""

    @pytest.mark.default
    def test_jobs_list_includes_acked_fields(self, client: httpx.Client) -> None:
        """Test that /api/jobs response includes acked and acked_at fields."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        response = client.get("/api/jobs")
        job = response.json()["jobs"][0]
        assert "acked" in job
        assert "acked_at" in job
        assert job["acked"] is False
        assert job["acked_at"] is None

    @pytest.mark.default
    def test_group_jobs_includes_acked_fields(self, client: httpx.Client) -> None:
        """Test that /api/groups/<name>/jobs response includes acked fields."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        response = client.get("/api/groups/test/jobs")
        job = response.json()["jobs"][0]
        assert "acked" in job
        assert "acked_at" in job

    @pytest.mark.default
    def test_status_response_includes_acked_fields(self, client: httpx.Client) -> None:
        """Test that POST /api/status response includes acked fields."""
        response = client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        job = response.json()["job"]
        assert "acked" in job
        assert "acked_at" in job
        assert job["acked"] is False


class TestAckGroupEndpoint:
    """Tests for POST /api/groups/<name>/ack endpoint.

    AIDEV-NOTE: Tests for bulk acking all errored jobs in a group.
    """

    @pytest.mark.default
    def test_ack_group_success(self, client: httpx.Client) -> None:
        """Test acknowledging all errors in a group."""
        # Create jobs with different statuses
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "timeout"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job3", "status": "success"}
        )

        # Ack all errors in the group
        response = client.post("/api/groups/test/ack")
        assert response.status_code == 200
        data = response.json()
        assert data["acked_count"] == 2
        assert data["group"] == "test"

        # Verify jobs are acked
        response = client.get("/api/jobs?status=error,timeout")
        jobs = response.json()["jobs"]
        assert all(job["acked"] is True for job in jobs)

    @pytest.mark.default
    def test_ack_group_not_found(self, client: httpx.Client) -> None:
        """Test acking non-existent group returns 404."""
        response = client.post("/api/groups/nonexistent/ack")
        assert response.status_code == 404
        data = response.json()
        assert data["error"] == "not_found"

    @pytest.mark.default
    def test_ack_group_case_insensitive(self, client: httpx.Client) -> None:
        """Test that group name lookup is case-insensitive."""
        client.post(
            "/api/status", json={"group": "mygroup", "job": "job1", "status": "error"}
        )

        response = client.post("/api/groups/MyGroup/ack")
        assert response.status_code == 200
        assert response.json()["acked_count"] == 1

    @pytest.mark.default
    def test_ack_group_no_errors(self, client: httpx.Client) -> None:
        """Test acking a group with no errors returns 0."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.post("/api/groups/test/ack")
        assert response.status_code == 200
        data = response.json()
        assert data["acked_count"] == 0

    @pytest.mark.default
    def test_ack_group_skips_already_acked(self, client: httpx.Client) -> None:
        """Test that already-acked jobs are not counted."""
        # Create error jobs
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "error"}
        )

        # Ack one job individually
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Ack the group (should only ack the remaining one)
        response = client.post("/api/groups/test/ack")
        assert response.status_code == 200
        assert response.json()["acked_count"] == 1

    @pytest.mark.default
    def test_ack_group_only_affects_target_group(self, client: httpx.Client) -> None:
        """Test that acking a group doesn't affect other groups."""
        # Create errors in two groups
        client.post(
            "/api/status", json={"group": "group1", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "group2", "job": "job1", "status": "error"}
        )

        # Ack only group1
        response = client.post("/api/groups/group1/ack")
        assert response.json()["acked_count"] == 1

        # Check group2 is not affected
        response = client.get("/api/groups")
        groups = response.json()["groups"]
        group2 = next(g for g in groups if g["name"] == "group2")
        assert group2["unhealthy_count"] == 1
        assert group2["acked_count"] == 0

    @pytest.mark.default
    def test_ack_group_health_becomes_healthy(self, client: httpx.Client) -> None:
        """Test that group health becomes healthy after acking all errors."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Before ack
        response = client.get("/api/groups")
        group = response.json()["groups"][0]
        assert group["health"] == "unhealthy"

        # Ack the group
        client.post("/api/groups/test/ack")

        # After ack
        response = client.get("/api/groups")
        group = response.json()["groups"][0]
        assert group["health"] == "healthy"
        assert group["acked_count"] == 1


class TestAckAllEndpoint:
    """Tests for POST /api/ack-all endpoint.

    AIDEV-NOTE: Tests for bulk acking all errored jobs globally.
    """

    @pytest.mark.default
    def test_ack_all_success(self, client: httpx.Client) -> None:
        """Test acknowledging all errors globally."""
        # Create errors in multiple groups
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g1", "job": "job2", "status": "timeout"}
        )
        client.post(
            "/api/status", json={"group": "g2", "job": "job1", "status": "stale"}
        )
        client.post(
            "/api/status", json={"group": "g2", "job": "job2", "status": "success"}
        )

        # Ack all
        response = client.post("/api/ack-all")
        assert response.status_code == 200
        data = response.json()
        assert data["acked_count"] == 3

        # Verify health
        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 3

    @pytest.mark.default
    def test_ack_all_no_errors(self, client: httpx.Client) -> None:
        """Test ack-all when no errors exist."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.post("/api/ack-all")
        assert response.status_code == 200
        assert response.json()["acked_count"] == 0

    @pytest.mark.default
    def test_ack_all_empty_database(self, client: httpx.Client) -> None:
        """Test ack-all when database is empty."""
        response = client.post("/api/ack-all")
        assert response.status_code == 200
        assert response.json()["acked_count"] == 0

    @pytest.mark.default
    def test_ack_all_skips_already_acked(self, client: httpx.Client) -> None:
        """Test that already-acked jobs are not counted."""
        # Create error jobs
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "error"}
        )

        # Ack one job individually
        jobs = client.get("/api/jobs").json()["jobs"]
        client.post(f"/api/jobs/{jobs[0]['id']}/ack")

        # Ack all (should only ack the remaining one)
        response = client.post("/api/ack-all")
        assert response.status_code == 200
        assert response.json()["acked_count"] == 1

    @pytest.mark.default
    def test_ack_all_affects_all_groups(self, client: httpx.Client) -> None:
        """Test that ack-all affects all groups."""
        # Create errors in multiple groups
        client.post(
            "/api/status", json={"group": "g1", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g2", "job": "job1", "status": "error"}
        )
        client.post(
            "/api/status", json={"group": "g3", "job": "job1", "status": "error"}
        )

        # Ack all
        response = client.post("/api/ack-all")
        assert response.json()["acked_count"] == 3

        # Check all groups
        response = client.get("/api/groups")
        groups = response.json()["groups"]
        for group in groups:
            assert group["unhealthy_count"] == 0
            assert group["acked_count"] == 1

    @pytest.mark.default
    def test_ack_all_idempotent(self, client: httpx.Client) -> None:
        """Test that calling ack-all twice is idempotent."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # First call
        response = client.post("/api/ack-all")
        assert response.json()["acked_count"] == 1

        # Second call (should be 0 since already acked)
        response = client.post("/api/ack-all")
        assert response.json()["acked_count"] == 0
