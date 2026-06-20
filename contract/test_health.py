"""Ported from backend/tests/test_api.py — GET /api/health behavior.

AIDEV-NOTE: Health precedence is empty > unhealthy > in_progress > healthy; the
response carries status, total_jobs, healthy, unhealthy, acked, in_progress and a
by_status map (behavioral-map §2). The backend test client auto-prefixed /api, so
every `/health` and `/status` path here is spelled out in full. Bucket 1 (runs as-is
over the wire). The last test is ported from test_integration.py::TestCliIntegration.
"""

import httpx
import pytest


class TestHealthEndpoint:
    """Tests for GET /api/health endpoint."""

    @pytest.mark.default
    def test_health_empty(self, client: httpx.Client) -> None:
        """Test health returns empty when no jobs exist."""
        response = client.get("/api/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "empty"
        assert data["total_jobs"] == 0
        assert data["healthy"] == 0
        assert data["unhealthy"] == 0
        assert data["in_progress"] == 0

    @pytest.mark.default
    def test_health_healthy(self, client: httpx.Client) -> None:
        """Test health returns healthy when all jobs are success."""
        # Create a job with success status
        response = client.post(
            "/api/status",
            json={"group": "test", "job": "job1", "status": "success"},
        )
        assert response.status_code == 201

        response = client.get("/api/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert data["total_jobs"] == 1
        assert data["healthy"] == 1

    @pytest.mark.default
    def test_health_unhealthy_with_error(self, client: httpx.Client) -> None:
        """Test health returns unhealthy when any job has error status."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1

    @pytest.mark.default
    def test_health_in_progress(self, client: httpx.Client) -> None:
        """Test health returns in_progress when jobs are running."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "in_progress"
        assert data["in_progress"] == 1

    @pytest.mark.default
    def test_health_unhealthy_takes_precedence(self, client: httpx.Client) -> None:
        """Test that unhealthy status takes precedence over in_progress."""
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "progress"}
        )
        client.post(
            "/api/status", json={"group": "test", "job": "job2", "status": "error"}
        )

        response = client.get("/api/health")
        data = response.json()
        assert data["status"] == "unhealthy"


class TestCliIntegration:
    """Tests verifying CLI can connect and submit statuses.

    AIDEV-NOTE: Ported from test_integration.py -- the health-check shape the CLI
    client (statshed_cli/client.py) depends on.
    """

    @pytest.mark.default
    def test_cli_health_check(self, client: httpx.Client) -> None:
        """Test CLI health check endpoint returns valid response."""
        response = client.get("/api/health")
        assert response.status_code == 200
        data = response.json()

        # Verify response shape matches CLI expectations
        assert "status" in data
        assert data["status"] in ["healthy", "unhealthy", "in_progress", "empty"]
        assert "total_jobs" in data
        assert "by_status" in data
