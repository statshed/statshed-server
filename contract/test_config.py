"""Ported from backend/tests/test_api.py — config endpoints over HTTP.

AIDEV-NOTE: Covers GET/PUT /api/config (global) and GET/PUT
/api/groups/<name>/config (per-group overrides + effective_* values). Mostly Bucket 1
(runs as-is over the wire); the expiration-cascade test is Bucket 3 -- it backdates
nothing but reads `expires_at`/`updated_at` from the job JSON via GET /api/jobs rather
than the ORM (coverage-map.md decision 2). The final CLI test is ported from
test_integration.py::TestCliIntegration::test_cli_config_management.

IMPORTANT isolation: PUT /api/config mutates GLOBAL config, which persists on the live
server. The autouse reset_db fixture (conftest.py) truncates the `config` table before
each test, so every test starts from defaults again -- a PUT here never leaks into the
next test.
"""

from datetime import datetime

import httpx
import pytest

JSON = "application/json"


class TestConfigEndpoints:
    """Tests for configuration endpoints."""

    @pytest.mark.default
    def test_get_config_defaults(self, client: httpx.Client) -> None:
        """Test getting config returns default values."""
        response = client.get("/api/config")
        assert response.status_code == 200
        data = response.json()
        assert data["progress_timeout_minutes"] == 5
        assert data["staleness_timeout_hours"] == 24
        assert data["expiration_timeout_hours"] == 24

    @pytest.mark.default
    def test_update_config(self, client: httpx.Client) -> None:
        """Test updating global config."""
        response = client.put(
            "/api/config",
            json={"progress_timeout_minutes": 10, "staleness_timeout_hours": 48},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["progress_timeout_minutes"] == 10
        assert data["staleness_timeout_hours"] == 48

    @pytest.mark.default
    def test_update_config_invalid_value(self, client: httpx.Client) -> None:
        """Test updating config with an out-of-range value returns 400."""
        # progress_timeout_minutes valid range is 1-10080; 0 is below it.
        response = client.put(
            "/api/config",
            json={"progress_timeout_minutes": 0},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "validation_error"

    @pytest.mark.default
    def test_update_config_partial(self, client: httpx.Client) -> None:
        """Test updating only one config value leaves the others unchanged."""
        response = client.put(
            "/api/config",
            json={"progress_timeout_minutes": 15},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["progress_timeout_minutes"] == 15
        assert data["staleness_timeout_hours"] == 24

    @pytest.mark.default
    def test_get_group_config(self, client: httpx.Client) -> None:
        """Test getting group-specific config.

        Overrides are null until set; effective_* fall back to the global defaults; a
        legacy `group` field mirrors `group_name` (behavioral-map §2).
        """
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.get("/api/groups/backups/config")
        assert response.status_code == 200
        data = response.json()
        assert data["group_name"] == "backups"
        assert data["group"] == "backups"  # legacy alias
        assert data["progress_timeout_minutes"] is None
        assert data["staleness_enabled"] is False
        assert data["staleness_timeout_hours"] is None
        assert data["expiration_timeout_hours"] is None
        assert data["effective_progress_timeout_minutes"] == 5
        assert data["effective_staleness_timeout_hours"] == 24
        assert data["effective_expiration_timeout_hours"] == 24

    @pytest.mark.default
    def test_update_group_config(self, client: httpx.Client) -> None:
        """Test updating group-specific config sets the override + effective value."""
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 72},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["staleness_timeout_hours"] == 72
        assert data["effective_staleness_timeout_hours"] == 72

    @pytest.mark.default
    def test_update_group_config_null_reverts(self, client: httpx.Client) -> None:
        """Test setting a group override to null reverts it to the global value."""
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # Set override.
        client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 72},
        )

        # Revert to null.
        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": None},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["staleness_timeout_hours"] is None

    @pytest.mark.default
    def test_update_group_config_staleness_validation(
        self, client: httpx.Client
    ) -> None:
        """staleness_timeout_hours must be < expiration_timeout_hours when enabled.

        AIDEV-NOTE: Cross-field rule applies only when staleness_enabled is True. The
        error carries a `fields` map (plural), not a single `field`.
        """
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # First enable staleness and set expiration to 24 hours, staleness to 12.
        response = client.put(
            "/api/groups/backups/config",
            json={
                "staleness_enabled": True,
                "expiration_timeout_hours": 24,
                "staleness_timeout_hours": 12,
            },
        )
        assert response.status_code == 200  # Ensure setup succeeded.

        # staleness == expiration -> fail.
        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 24},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "validation_error"
        assert "staleness_timeout_hours" in data.get("fields", {})

        # staleness > expiration -> also fail.
        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 48},
        )
        assert response.status_code == 400

        # Valid: staleness < expiration.
        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 12},
        )
        assert response.status_code == 200

    @pytest.mark.default
    def test_update_group_config_staleness_validation_skipped_when_disabled(
        self, client: httpx.Client
    ) -> None:
        """The cross-field rule is skipped when staleness_enabled is False.

        AIDEV-NOTE: With staleness disabled, staleness_timeout_hours >=
        expiration_timeout_hours is allowed (the feature won't be used).
        """
        client.post(
            "/api/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # Set expiration to 24 hours, staleness disabled.
        response = client.put(
            "/api/groups/backups/config",
            json={"expiration_timeout_hours": 24, "staleness_enabled": False},
        )
        assert response.status_code == 200  # Ensure setup succeeded.

        # staleness >= expiration is fine while staleness is disabled.
        response = client.put(
            "/api/groups/backups/config",
            json={"staleness_timeout_hours": 48},
        )
        assert response.status_code == 200

    @pytest.mark.default
    def test_update_config_boolean_rejected(self, client: httpx.Client) -> None:
        """Test that boolean values are rejected for config."""
        response = client.put(
            "/api/config",
            json={"progress_timeout_minutes": True},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "validation_error"

    @pytest.mark.default
    def test_update_config_json_array_rejected(self, client: httpx.Client) -> None:
        """Test that a top-level JSON array body is rejected with bad_request."""
        response = client.put(
            "/api/config",
            content="[10, 20]",
            headers={"content-type": JSON},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "bad_request"

    @pytest.mark.default
    def test_status_json_array_rejected(self, client: httpx.Client) -> None:
        """Test that a top-level JSON array body is rejected for the status endpoint."""
        response = client.post(
            "/api/status",
            content='["invalid"]',
            headers={"content-type": JSON},
        )
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "bad_request"

    @pytest.mark.default
    def test_get_group_config_not_found(self, client: httpx.Client) -> None:
        """Test getting config for a non-existent group returns 404."""
        response = client.get("/api/groups/nonexistent/config")
        assert response.status_code == 404

    @pytest.mark.default
    def test_update_config_expiration_cascades_to_non_override_groups(
        self, client: httpx.Client
    ) -> None:
        """Changing global expiration_timeout_hours refreshes expires_at for jobs in
        groups WITHOUT an override, and leaves override-group jobs alone.

        AIDEV-NOTE: Bucket 3 -- reads `expires_at` and `updated_at` from the job JSON
        (both rendered YYYY-MM-DDTHH:MM:SSZ) via GET /api/jobs rather than the ORM
        (coverage-map.md decision 2). SQLite's datetime() truncates to whole seconds, so
        deltas are compared with a small tolerance.
        """
        # Group with no expiration override -> follows the global default.
        client.post(
            "/api/status", json={"group": "noverride", "job": "a", "status": "success"}
        )
        # Group with an explicit 10-hour expiration override.
        client.post(
            "/api/status", json={"group": "override", "job": "b", "status": "success"}
        )
        resp = client.put(
            "/api/groups/override/config", json={"expiration_timeout_hours": 10}
        )
        assert resp.status_code == 200

        # Change the global expiration from the 24h default to 48h.
        resp = client.put("/api/config", json={"expiration_timeout_hours": 48})
        assert resp.status_code == 200

        jobs = {j["name"]: j for j in client.get("/api/jobs").json()["jobs"]}
        job_a = jobs["a"]
        job_b = jobs["b"]

        fmt = "%Y-%m-%dT%H:%M:%SZ"
        a_updated = datetime.strptime(job_a["updated_at"], fmt)
        a_expires = datetime.strptime(job_a["expires_at"], fmt)
        b_updated = datetime.strptime(job_b["updated_at"], fmt)
        b_expires = datetime.strptime(job_b["expires_at"], fmt)

        # Non-override group's job: expires_at refreshed to updated_at + 48h.
        assert abs((a_expires - a_updated).total_seconds() - 48 * 3600) < 2

        # Override group's job: untouched, still updated_at + 10h.
        assert abs((b_expires - b_updated).total_seconds() - 10 * 3600) < 2


class TestCliConfigManagement:
    """Tests verifying CLI can read and update configuration.

    AIDEV-NOTE: Ported from test_integration.py::TestCliIntegration.
    """

    @pytest.mark.default
    def test_cli_config_management(self, client: httpx.Client) -> None:
        """Test CLI can read and update configuration."""
        # Get current config (verify it's readable).
        response = client.get("/api/config")
        assert response.status_code == 200
        _ = response.json()  # Verify response is valid JSON.

        # Update config.
        response = client.put(
            "/api/config",
            json={"progress_timeout_minutes": 10, "staleness_timeout_hours": 48},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["progress_timeout_minutes"] == 10
        assert data["staleness_timeout_hours"] == 48

        # Verify config was persisted.
        response = client.get("/api/config")
        data = response.json()
        assert data["progress_timeout_minutes"] == 10
