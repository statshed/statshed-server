"""Ported from backend/tests/test_api.py — POST /api/status behavior.

AIDEV-NOTE: Covers job create/update, name normalization, validation, and the
multipart `log` file part (behavioral-map §2). The backend test client auto-prefixed
/api, so every `/status`, `/jobs`, and `/groups/.../log` path here is spelled out in
full. Bucket 1 (runs as-is over the wire), except the create-then-update test, which is
the B2 re-expression of test_integration.py::TestIntegrityErrorRetry::
test_group_and_job_both_exist (ORM setup replaced by an over-the-wire POST-then-POST).
The progress->success workflow is ported from test_integration.py::TestCliIntegration.
"""

import httpx
import pytest


class TestStatusEndpoint:
    """Tests for POST /api/status endpoint."""

    @pytest.mark.default
    def test_submit_status_creates_job(self, client: httpx.Client) -> None:
        """Test submitting a status creates a new job."""
        response = client.post(
            "/api/status",
            json={
                "group": "backups",
                "job": "daily-backup",
                "status": "success",
                "message": "Completed in 45s",
            },
        )

        assert response.status_code == 201
        data = response.json()
        assert data["success"] is True
        assert data["job"]["name"] == "daily-backup"
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "Completed in 45s"
        assert data["job"]["group_name"] == "backups"

    @pytest.mark.default
    def test_submit_status_updates_existing(self, client: httpx.Client) -> None:
        """Test submitting a status updates an existing job."""
        # Create job
        client.post(
            "/api/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Update job
        response = client.post(
            "/api/status",
            json={"group": "test", "job": "job1", "status": "success"},
        )

        assert response.status_code == 201
        data = response.json()
        assert data["job"]["status"] == "success"

    @pytest.mark.default
    def test_submit_status_missing_group(self, client: httpx.Client) -> None:
        """Test submitting without group returns error."""
        response = client.post("/api/status", json={"job": "job1", "status": "success"})
        assert response.status_code == 400
        data = response.json()
        assert data["error"] == "validation_error"
        assert data["field"] == "group"

    @pytest.mark.default
    def test_submit_status_missing_job(self, client: httpx.Client) -> None:
        """Test submitting without job returns error."""
        response = client.post(
            "/api/status", json={"group": "test", "status": "success"}
        )
        assert response.status_code == 400
        data = response.json()
        assert data["field"] == "job"

    @pytest.mark.default
    def test_submit_status_missing_status(self, client: httpx.Client) -> None:
        """Test submitting without status returns error."""
        response = client.post("/api/status", json={"group": "test", "job": "job1"})
        assert response.status_code == 400
        data = response.json()
        assert data["field"] == "status"

    @pytest.mark.default
    def test_submit_status_invalid_status(self, client: httpx.Client) -> None:
        """Test submitting with invalid status returns error."""
        response = client.post(
            "/api/status",
            json={"group": "test", "job": "job1", "status": "invalid"},
        )
        assert response.status_code == 400
        data = response.json()
        assert "status must be one of" in data["message"]

    @pytest.mark.default
    def test_submit_status_normalizes_names(self, client: httpx.Client) -> None:
        """Test that group and job names are normalized to lowercase."""
        response = client.post(
            "/api/status",
            json={"group": "MyGroup", "job": "MyJob", "status": "success"},
        )

        assert response.status_code == 201
        data = response.json()
        assert data["job"]["group_name"] == "mygroup"
        assert data["job"]["name"] == "myjob"

    @pytest.mark.default
    def test_submit_status_invalid_group_name(self, client: httpx.Client) -> None:
        """Test that invalid group names are rejected."""
        response = client.post(
            "/api/status",
            json={"group": "my group!", "job": "job1", "status": "success"},
        )
        assert response.status_code == 400
        assert "invalid characters" in response.json()["message"]

    @pytest.mark.default
    def test_submit_status_message_too_long(self, client: httpx.Client) -> None:
        """Test that messages exceeding max length are rejected."""
        long_message = "x" * 5000
        response = client.post(
            "/api/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "success",
                "message": long_message,
            },
        )
        assert response.status_code == 400
        assert "maximum length" in response.json()["message"]


class TestStatusCreateThenUpdate:
    """Create-then-update path on POST /api/status.

    AIDEV-NOTE: Re-expresses test_integration.py::TestIntegrityErrorRetry::
    test_group_and_job_both_exist as an over-the-wire workflow (B2): the first POST
    creates the group+job, the second POST (same group/job, new status+message) takes
    the plain update path -- same job id, updated fields -- NOT a forced IntegrityError.
    The progress->success CLI workflow is folded in from TestCliIntegration.
    """

    @pytest.mark.default
    def test_group_and_job_both_exist(self, client: httpx.Client) -> None:
        """Second POST for an existing group+job updates it in place."""
        # First POST creates the group and job.
        created = client.post(
            "/api/status",
            json={
                "group": "existing-group",
                "job": "existing-job",
                "status": "progress",
                "message": "Old message",
            },
        )
        assert created.status_code == 201
        job_id = created.json()["job"]["id"]

        # Second POST (same group/job) updates the existing job in place.
        response = client.post(
            "/api/status",
            json={
                "group": "existing-group",
                "job": "existing-job",
                "status": "success",
                "message": "New message",
            },
        )

        assert response.status_code == 201
        data = response.json()
        assert data["job"]["id"] == job_id
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "New message"

    @pytest.mark.default
    def test_cli_submit_status_workflow(self, client: httpx.Client) -> None:
        """Test typical CLI submit workflow: progress -> success."""
        # CLI typically submits progress first
        response = client.post(
            "/api/status",
            json={
                "group": "nightly-builds",
                "job": "unit-tests",
                "status": "progress",
                "message": "Running tests...",
            },
        )
        assert response.status_code == 201
        data = response.json()
        assert data["success"] is True
        assert data["job"]["status"] == "progress"

        # Then submits success when done
        response = client.post(
            "/api/status",
            json={
                "group": "nightly-builds",
                "job": "unit-tests",
                "status": "success",
                "message": "All 42 tests passed",
            },
        )
        assert response.status_code == 201
        data = response.json()
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "All 42 tests passed"


class TestLogUpload:
    """Tests for log upload functionality on POST /api/status.

    AIDEV-NOTE: The log is a `log` file part in a multipart/form-data body (the other
    fields ride along as form data). Over httpx that is `data={...}, files={"log":
    (...)}`. A JSON body carries no log, so `has_log` is False and `log_line_count` is
    None. Bucket 1.
    """

    @pytest.mark.default
    def test_submit_status_with_log_multipart(self, client: httpx.Client) -> None:
        """Test submitting status with log file via multipart/form-data."""
        response = client.post(
            "/api/status",
            data={
                "group": "builds",
                "job": "compile",
                "status": "success",
                "message": "Build completed",
            },
            files={"log": ("build.log", "line1\nline2\nline3\n", "text/plain")},
        )

        assert response.status_code == 201
        result = response.json()
        assert result["success"] is True
        assert result["job"]["has_log"] is True
        assert result["job"]["log_line_count"] == 3
        assert result["job"]["log_truncated"] is False
        assert result["job"]["log_updated_at"] is not None

    @pytest.mark.default
    def test_submit_status_without_log_json(self, client: httpx.Client) -> None:
        """Test submitting status without log via JSON."""
        response = client.post(
            "/api/status",
            json={"group": "builds", "job": "test", "status": "success"},
        )

        assert response.status_code == 201
        result = response.json()
        assert result["job"]["has_log"] is False
        assert result["job"]["log_line_count"] is None

    @pytest.mark.default
    def test_submit_status_multipart_without_log(self, client: httpx.Client) -> None:
        """Test submitting status via multipart without log file."""
        # AIDEV-NOTE: To force multipart/form-data with form fields but NO file, pass the
        # fields as (None, value) tuples in `files=`. An empty `files={}` makes httpx send
        # form-urlencoded instead, which the server (correctly) 400s as a non-JSON body.
        response = client.post(
            "/api/status",
            files={
                "group": (None, "builds"),
                "job": (None, "deploy"),
                "status": (None, "success"),
            },
        )

        assert response.status_code == 201
        result = response.json()
        assert result["job"]["has_log"] is False

    @pytest.mark.default
    def test_log_replaces_previous_on_update(self, client: httpx.Client) -> None:
        """Test that new log replaces previous log on status update."""
        # Submit initial log
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "progress"},
            files={"log": ("log.txt", "initial log\n", "text/plain")},
        )
        assert response.status_code == 201

        # Verify initial log content
        response = client.get("/api/groups/builds/jobs/test/log")
        assert "initial log" in response.json()["log"]

        # Submit new log
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "success"},
            files={
                "log": ("log.txt", "new log line 1\nnew log line 2\n", "text/plain")
            },
        )
        assert response.status_code == 201
        result = response.json()
        assert result["job"]["log_line_count"] == 2

        # Verify the log was replaced (not appended)
        response = client.get("/api/groups/builds/jobs/test/log")
        log = response.json()["log"]
        assert "initial log" not in log
        assert "new log line 1" in log

    @pytest.mark.default
    def test_log_not_cleared_without_new_log(self, client: httpx.Client) -> None:
        """Test that existing log is preserved when updating without new log."""
        # Submit with log
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "progress"},
            files={"log": ("log.txt", "my log content\n", "text/plain")},
        )
        assert response.status_code == 201

        # Update without log (JSON)
        response = client.post(
            "/api/status",
            json={"group": "builds", "job": "test", "status": "success"},
        )
        assert response.status_code == 201
        result = response.json()
        # Log should still be present
        assert result["job"]["has_log"] is True
        assert result["job"]["log_line_count"] == 1

    @pytest.mark.default
    def test_log_metadata_in_job_response(self, client: httpx.Client) -> None:
        """Test that log metadata is included in job responses."""
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "success"},
            files={"log": ("log.txt", "line\n" * 10, "text/plain")},
        )

        result = response.json()
        job = result["job"]
        assert "has_log" in job
        assert "log_line_count" in job
        assert "log_truncated" in job
        assert "log_updated_at" in job
        assert job["log_line_count"] == 10


class TestLogEncodingHandling:
    """Tests for log file encoding handling.

    AIDEV-NOTE: UTF-8 content round-trips verbatim; non-UTF-8 bytes (valid latin-1)
    must still be accepted (server falls back to latin-1) rather than 400. The latin-1
    case sends raw bytes as the file part body. Bucket 1.
    """

    @pytest.mark.default
    def test_utf8_log_content(self, client: httpx.Client) -> None:
        """Test handling of UTF-8 encoded log content."""
        log_content = "Hello 世界\nUnicode: émojis 🎉\n"
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "success"},
            files={"log": ("log.txt", log_content.encode("utf-8"), "text/plain")},
        )

        assert response.status_code == 201
        result = response.json()
        assert result["job"]["has_log"] is True

        # Verify content is preserved
        response = client.get("/api/groups/builds/jobs/test/log")
        assert log_content in response.json()["log"]

    @pytest.mark.default
    def test_latin1_fallback_encoding(self, client: httpx.Client) -> None:
        """Test fallback to latin-1 for non-UTF-8 content."""
        # Bytes that are valid latin-1 but not valid UTF-8.
        log_content = b"Line with latin-1: caf\xe9\n"
        response = client.post(
            "/api/status",
            data={"group": "builds", "job": "test", "status": "success"},
            files={"log": ("log.txt", log_content, "text/plain")},
        )

        assert response.status_code == 201
        result = response.json()
        assert result["job"]["has_log"] is True
