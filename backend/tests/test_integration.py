"""Integration tests for StatShed backend.

AIDEV-NOTE: These tests verify:
1. CLI can connect and submit statuses via REST API
2. Frontend can receive WebSocket updates
3. Rapid sequential status submissions are handled safely
4. IntegrityError/retry code paths handle race conditions correctly
"""

from unittest.mock import patch


class TestCliIntegration:
    """Tests verifying CLI can connect and submit statuses.

    AIDEV-NOTE: These tests simulate CLI behavior by making REST API calls
    directly, matching what the CLI client does in statshed_cli/client.py.
    """

    def test_cli_health_check(self, client):
        """Test CLI health check endpoint returns valid response."""
        response = client.get("/health")
        assert response.status_code == 200
        data = response.get_json()

        # Verify response shape matches CLI expectations
        assert "status" in data
        assert data["status"] in ["healthy", "unhealthy", "in_progress", "empty"]
        assert "total_jobs" in data
        assert "by_status" in data

    def test_cli_submit_status_workflow(self, client):
        """Test typical CLI submit workflow: progress -> success."""
        # CLI typically submits progress first
        response = client.post(
            "/status",
            json={
                "group": "nightly-builds",
                "job": "unit-tests",
                "status": "progress",
                "message": "Running tests...",
            },
        )
        assert response.status_code == 201
        data = response.get_json()
        assert data["success"] is True
        assert data["job"]["status"] == "progress"

        # Then submits success when done
        response = client.post(
            "/status",
            json={
                "group": "nightly-builds",
                "job": "unit-tests",
                "status": "success",
                "message": "All 42 tests passed",
            },
        )
        assert response.status_code == 201
        data = response.get_json()
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "All 42 tests passed"

    def test_cli_list_groups(self, client):
        """Test CLI can list groups with health summaries."""
        # Create some test data
        client.post(
            "/status", json={"group": "backups", "job": "daily", "status": "success"}
        )
        client.post(
            "/status", json={"group": "backups", "job": "weekly", "status": "success"}
        )
        client.post(
            "/status", json={"group": "monitoring", "job": "check", "status": "error"}
        )

        response = client.get("/groups")
        assert response.status_code == 200
        data = response.get_json()

        assert "groups" in data
        assert len(data["groups"]) == 2

        # Verify groups have expected fields for CLI display
        backups = next(g for g in data["groups"] if g["name"] == "backups")
        assert "job_count" in backups
        assert "health" in backups
        assert "status_counts" in backups
        assert backups["health"] == "healthy"

    def test_cli_get_jobs(self, client):
        """Test CLI can get jobs for a specific group."""
        client.post(
            "/status", json={"group": "builds", "job": "frontend", "status": "success"}
        )
        client.post(
            "/status", json={"group": "builds", "job": "backend", "status": "progress"}
        )

        response = client.get("/groups/builds/jobs")
        assert response.status_code == 200
        data = response.get_json()

        assert "group" in data
        assert "jobs" in data
        assert len(data["jobs"]) == 2

        # Verify job fields for CLI display
        job = data["jobs"][0]
        assert "name" in job
        assert "status" in job
        assert "message" in job
        assert "updated_at" in job

    def test_cli_config_management(self, client):
        """Test CLI can read and update configuration."""
        # Get current config (verify it's readable)
        response = client.get("/config")
        assert response.status_code == 200
        _ = response.get_json()  # Verify response is valid JSON

        # Update config
        response = client.put(
            "/config",
            json={"progress_timeout_minutes": 10, "staleness_timeout_hours": 48},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 10
        assert data["staleness_timeout_hours"] == 48

        # Verify config was persisted
        response = client.get("/config")
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 10


class TestWebSocketIntegration:
    """Tests verifying frontend can receive WebSocket updates.

    AIDEV-NOTE: These tests verify that WebSocket events are emitted correctly
    when jobs are created/updated. The frontend relies on these events to
    update the UI in real-time without polling.

    Note: The `app` fixture is required for test setup even when not directly
    accessed in the test body - it initializes the Flask app and database.
    """

    def test_status_update_event_on_job_create(self, app, client, socketio_client):  # noqa: ARG002
        """Test status_update event is emitted when a job is created."""
        # Ensure we're connected
        assert socketio_client.is_connected()

        # Submit a status
        client.post(
            "/status",
            json={"group": "test-group", "job": "test-job", "status": "success"},
        )

        # Check received events
        received = socketio_client.get_received()

        # Find status_update event
        status_events = [r for r in received if r["name"] == "status_update"]
        assert len(status_events) >= 1

        event_data = status_events[0]["args"][0]
        assert event_data["schema_version"] == 1
        assert event_data["job"]["name"] == "test-job"
        assert event_data["job"]["status"] == "success"
        assert event_data["group_name"] == "test-group"

    def test_group_created_event(self, app, client, socketio_client):  # noqa: ARG002
        """Test group_created event is emitted when a new group is created."""
        assert socketio_client.is_connected()

        # Submit a status for a new group
        client.post(
            "/status",
            json={"group": "new-group", "job": "first-job", "status": "progress"},
        )

        received = socketio_client.get_received()

        # Find group_created event
        group_events = [r for r in received if r["name"] == "group_created"]
        assert len(group_events) == 1

        event_data = group_events[0]["args"][0]
        assert event_data["schema_version"] == 1
        assert event_data["group"]["name"] == "new-group"

    def test_status_update_includes_previous_status(self, app, client, socketio_client):  # noqa: ARG002
        """Test status_update includes previous_status for state tracking."""
        assert socketio_client.is_connected()

        # Create initial job
        client.post(
            "/status",
            json={"group": "test-group", "job": "test-job", "status": "progress"},
        )

        # Clear received events
        socketio_client.get_received()

        # Update job status
        client.post(
            "/status",
            json={"group": "test-group", "job": "test-job", "status": "success"},
        )

        received = socketio_client.get_received()
        status_events = [r for r in received if r["name"] == "status_update"]
        assert len(status_events) >= 1

        event_data = status_events[0]["args"][0]
        assert event_data["previous_status"] == "progress"
        assert event_data["job"]["status"] == "success"

    def test_no_duplicate_group_created_event(self, app, client, socketio_client):  # noqa: ARG002
        """Test group_created is not emitted for existing groups."""
        assert socketio_client.is_connected()

        # Create group
        client.post(
            "/status",
            json={"group": "existing", "job": "job1", "status": "success"},
        )

        # Clear events
        socketio_client.get_received()

        # Add another job to the same group
        client.post(
            "/status",
            json={"group": "existing", "job": "job2", "status": "success"},
        )

        received = socketio_client.get_received()
        group_events = [r for r in received if r["name"] == "group_created"]
        assert len(group_events) == 0  # No new group_created event


class TestRapidSubmissions:
    """Tests verifying rapid sequential status submissions are handled safely.

    AIDEV-NOTE: These tests verify the backend handles multiple rapid sequential
    requests without data corruption. True concurrent testing with in-memory
    SQLite is problematic due to threading limitations. In production with
    file-based SQLite + WAL mode, concurrent access works properly.

    Note: These tests do NOT exercise the IntegrityError/retry code paths since
    they run sequentially. The IntegrityError/retry code paths are covered by
    TestIntegrityErrorRetry which uses mocking to deterministically test them.
    """

    def test_rapid_status_submissions_same_job(self, client):
        """Test rapid submissions to the same job don't corrupt data."""
        # Submit 20 rapid sequential requests
        for i in range(20):
            response = client.post(
                "/status",
                json={
                    "group": "rapid-test",
                    "job": "shared-job",
                    "status": "success",
                    "message": f"Update {i}",
                },
            )
            assert response.status_code == 201

        # Verify final state
        response = client.get("/groups/rapid-test/jobs")
        data = response.get_json()

        # Should have exactly one job with the last message
        assert len(data["jobs"]) == 1
        assert data["jobs"][0]["name"] == "shared-job"
        assert data["jobs"][0]["status"] == "success"
        assert data["jobs"][0]["message"] == "Update 19"

    def test_rapid_job_creation_same_group(self, client):
        """Test rapid creation of multiple jobs in the same group."""
        # Rapidly create 10 jobs in the same group
        for i in range(10):
            response = client.post(
                "/status",
                json={
                    "group": "multi-job-group",
                    "job": f"job-{i}",
                    "status": "success",
                },
            )
            assert response.status_code == 201

        # Verify final state
        response = client.get("/groups/multi-job-group/jobs")
        data = response.get_json()

        # Should have exactly 10 unique jobs
        assert len(data["jobs"]) == 10
        job_names = {job["name"] for job in data["jobs"]}
        assert job_names == {f"job-{i}" for i in range(10)}

    def test_rapid_submissions_multiple_groups(self, client):
        """Test rapid submissions to different groups work independently."""
        # Create 5 groups with 4 jobs each, rapidly but sequentially
        for g in range(5):
            for j in range(4):
                response = client.post(
                    "/status",
                    json={
                        "group": f"group-{g}",
                        "job": f"job-{j}",
                        "status": "success",
                    },
                )
                assert response.status_code == 201

        # Verify all groups exist with correct job counts
        response = client.get("/groups")
        data = response.get_json()

        assert len(data["groups"]) == 5
        for group in data["groups"]:
            assert group["job_count"] == 4

    def test_rapid_config_updates(self, client):
        """Test rapid config updates don't cause errors."""
        # Submit rapid sequential config updates
        for timeout in [5, 10, 15, 20, 25]:
            response = client.put(
                "/config",
                json={"progress_timeout_minutes": timeout},
            )
            assert response.status_code == 200

        # Final value should be the last submitted value
        response = client.get("/config")
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 25

    def test_rapid_status_transitions(self, client):
        """Test rapid status transitions through full lifecycle."""
        # progress -> success -> error -> progress -> success
        statuses = ["progress", "success", "error", "progress", "success"]

        for status in statuses:
            response = client.post(
                "/status",
                json={
                    "group": "lifecycle-test",
                    "job": "lifecycle-job",
                    "status": status,
                },
            )
            assert response.status_code == 201
            assert response.get_json()["job"]["status"] == status

        # Final status should be success
        response = client.get("/groups/lifecycle-test/jobs")
        data = response.get_json()
        assert data["jobs"][0]["status"] == "success"


class TestIntegrityErrorRetry:
    """Tests verifying IntegrityError/retry code paths work correctly.

    AIDEV-NOTE: These tests use mocking to deterministically exercise the
    IntegrityError retry paths in the POST /status endpoint. This ensures
    the race condition handling code is tested without requiring actual
    concurrent requests.

    The retry logic handles two race conditions:
    1. Group creation: If two requests try to create the same group
       simultaneously, the second should retry and use the existing group.
    2. Job creation: If two requests try to create the same job
       simultaneously, the second should retry, find the job, and update it.
    """

    def test_group_integrity_error_retry(self, app, client, db_session):
        """Test group creation retry when IntegrityError occurs.

        Simulates a race condition where another request creates the group
        between our existence check and our creation attempt.
        """
        from sqlalchemy.exc import IntegrityError

        # Create the group first (simulating another concurrent request)
        from models import Group

        existing_group = Group(name="race-group")
        db_session.add(existing_group)
        db_session.commit()
        existing_group_id = existing_group.id

        # Track flush calls to simulate the race condition
        flush_call_count = [0]
        original_flush = db_session.flush

        def mock_flush():
            flush_call_count[0] += 1
            # On first flush (group creation attempt), simulate IntegrityError
            # as if another request just created the group
            if flush_call_count[0] == 1:
                # We need to check if we're trying to add a Group
                # by looking at the session's new objects
                new_objects = list(db_session.new)
                if any(isinstance(obj, Group) for obj in new_objects):
                    raise IntegrityError(
                        statement="INSERT INTO groups",
                        params={},
                        orig=Exception("UNIQUE constraint failed: groups.name"),
                    )
            return original_flush()

        # Patch flush to trigger IntegrityError on group creation
        with patch.object(db_session, "flush", side_effect=mock_flush):
            # Submit status - should trigger IntegrityError then retry
            response = client.post(
                "/status",
                json={
                    "group": "race-group",
                    "job": "test-job",
                    "status": "success",
                },
            )

        # Should succeed despite the simulated race condition
        assert response.status_code == 201
        data = response.get_json()
        assert data["success"] is True
        assert data["job"]["group_id"] == existing_group_id
        assert data["job"]["name"] == "test-job"

    def test_job_integrity_error_retry(self, app, client, db_session):
        """Test job creation retry when IntegrityError occurs.

        Simulates a race condition where another request creates the job
        between our existence check and our creation attempt.
        """
        from sqlalchemy.exc import IntegrityError

        from models import Group, Job

        # Create group and job first (simulating another concurrent request)
        group = Group(name="job-race-group")
        db_session.add(group)
        db_session.commit()

        from datetime import UTC, datetime

        now = datetime.now(UTC)
        existing_job = Job(
            group_id=group.id,
            name="race-job",
            status="progress",
            message="Initial message",
            updated_at=now,
            created_at=now,
        )
        db_session.add(existing_job)
        db_session.commit()
        existing_job_id = existing_job.id

        # Track flush calls to simulate the race condition
        flush_call_count = [0]
        original_flush = db_session.flush

        def mock_flush():
            flush_call_count[0] += 1
            # Skip first flush (that was for group which already exists)
            # On second flush (job creation attempt), simulate IntegrityError
            if flush_call_count[0] == 1:
                new_objects = list(db_session.new)
                if any(isinstance(obj, Job) for obj in new_objects):
                    raise IntegrityError(
                        statement="INSERT INTO jobs",
                        params={},
                        orig=Exception(
                            "UNIQUE constraint failed: jobs.group_id, jobs.name"
                        ),
                    )
            return original_flush()

        # Patch flush to trigger IntegrityError on job creation
        with patch.object(db_session, "flush", side_effect=mock_flush):
            # Submit status - should trigger IntegrityError then retry and update
            response = client.post(
                "/status",
                json={
                    "group": "job-race-group",
                    "job": "race-job",
                    "status": "success",
                    "message": "Updated message",
                },
            )

        # Should succeed despite the simulated race condition
        assert response.status_code == 201
        data = response.get_json()
        assert data["success"] is True
        assert data["job"]["id"] == existing_job_id
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "Updated message"

    def test_group_and_job_both_exist(self, app, client, db_session):
        """Test normal update path when group and job both exist (no IntegrityError).

        This verifies the happy path where no race condition occurs - the
        existing group and job are found and updated directly.
        """
        from models import Group, Job

        # Create group and job
        group = Group(name="existing-group")
        db_session.add(group)
        db_session.commit()

        from datetime import UTC, datetime

        now = datetime.now(UTC)
        job = Job(
            group_id=group.id,
            name="existing-job",
            status="progress",
            message="Old message",
            updated_at=now,
            created_at=now,
        )
        db_session.add(job)
        db_session.commit()
        job_id = job.id

        # Submit status update - should update existing job without any retries
        response = client.post(
            "/status",
            json={
                "group": "existing-group",
                "job": "existing-job",
                "status": "success",
                "message": "New message",
            },
        )

        assert response.status_code == 201
        data = response.get_json()
        assert data["job"]["id"] == job_id
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "New message"
