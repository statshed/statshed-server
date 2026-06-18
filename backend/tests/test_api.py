"""Tests for REST API endpoints."""


class TestHealthEndpoint:
    """Tests for GET /health endpoint."""

    def test_health_empty(self, client):
        """Test health returns empty when no jobs exist."""
        response = client.get("/health")
        assert response.status_code == 200
        data = response.get_json()
        assert data["status"] == "empty"
        assert data["total_jobs"] == 0
        assert data["healthy"] == 0
        assert data["unhealthy"] == 0
        assert data["in_progress"] == 0

    def test_health_healthy(self, client):
        """Test health returns healthy when all jobs are success."""
        # Create a job with success status
        response = client.post(
            "/status",
            json={"group": "test", "job": "job1", "status": "success"},
        )
        assert response.status_code == 201

        response = client.get("/health")
        assert response.status_code == 200
        data = response.get_json()
        assert data["status"] == "healthy"
        assert data["total_jobs"] == 1
        assert data["healthy"] == 1

    def test_health_unhealthy_with_error(self, client):
        """Test health returns unhealthy when any job has error status."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1

    def test_health_in_progress(self, client):
        """Test health returns in_progress when jobs are running."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "in_progress"
        assert data["in_progress"] == 1

    def test_health_unhealthy_takes_precedence(self, client):
        """Test that unhealthy status takes precedence over in_progress."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )
        client.post("/status", json={"group": "test", "job": "job2", "status": "error"})

        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "unhealthy"


class TestStatusEndpoint:
    """Tests for POST /status endpoint."""

    def test_submit_status_creates_job(self, client):
        """Test submitting a status creates a new job."""
        response = client.post(
            "/status",
            json={
                "group": "backups",
                "job": "daily-backup",
                "status": "success",
                "message": "Completed in 45s",
            },
        )

        assert response.status_code == 201
        data = response.get_json()
        assert data["success"] is True
        assert data["job"]["name"] == "daily-backup"
        assert data["job"]["status"] == "success"
        assert data["job"]["message"] == "Completed in 45s"
        assert data["job"]["group_name"] == "backups"

    def test_submit_status_updates_existing(self, client):
        """Test submitting a status updates an existing job."""
        # Create job
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Update job
        response = client.post(
            "/status",
            json={"group": "test", "job": "job1", "status": "success"},
        )

        assert response.status_code == 201
        data = response.get_json()
        assert data["job"]["status"] == "success"

    def test_submit_status_missing_group(self, client):
        """Test submitting without group returns error."""
        response = client.post("/status", json={"job": "job1", "status": "success"})
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "validation_error"
        assert data["field"] == "group"

    def test_submit_status_missing_job(self, client):
        """Test submitting without job returns error."""
        response = client.post("/status", json={"group": "test", "status": "success"})
        assert response.status_code == 400
        data = response.get_json()
        assert data["field"] == "job"

    def test_submit_status_missing_status(self, client):
        """Test submitting without status returns error."""
        response = client.post("/status", json={"group": "test", "job": "job1"})
        assert response.status_code == 400
        data = response.get_json()
        assert data["field"] == "status"

    def test_submit_status_invalid_status(self, client):
        """Test submitting with invalid status returns error."""
        response = client.post(
            "/status",
            json={"group": "test", "job": "job1", "status": "invalid"},
        )
        assert response.status_code == 400
        data = response.get_json()
        assert "status must be one of" in data["message"]

    def test_submit_status_normalizes_names(self, client):
        """Test that group and job names are normalized to lowercase."""
        response = client.post(
            "/status",
            json={"group": "MyGroup", "job": "MyJob", "status": "success"},
        )

        assert response.status_code == 201
        data = response.get_json()
        assert data["job"]["group_name"] == "mygroup"
        assert data["job"]["name"] == "myjob"

    def test_submit_status_invalid_group_name(self, client):
        """Test that invalid group names are rejected."""
        response = client.post(
            "/status",
            json={"group": "my group!", "job": "job1", "status": "success"},
        )
        assert response.status_code == 400
        assert "invalid characters" in response.get_json()["message"]

    def test_submit_status_message_too_long(self, client):
        """Test that messages exceeding max length are rejected."""
        long_message = "x" * 5000
        response = client.post(
            "/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "success",
                "message": long_message,
            },
        )
        assert response.status_code == 400
        assert "maximum length" in response.get_json()["message"]


class TestJobsEndpoint:
    """Tests for GET /jobs endpoint.

    AIDEV-NOTE: Tests for the jobs listing endpoint used by health card click-through.
    """

    def test_get_jobs_empty(self, client):
        """Test getting jobs when none exist."""
        response = client.get("/jobs")
        assert response.status_code == 200
        data = response.get_json()
        assert data["jobs"] == []
        assert data["total"] == 0

    def test_get_jobs_all(self, client):
        """Test getting all jobs without filter."""
        # Create some jobs
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})
        client.post(
            "/status", json={"group": "g2", "job": "job1", "status": "progress"}
        )

        response = client.get("/jobs")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total"] == 3
        assert len(data["jobs"]) == 3

    def test_get_jobs_filter_single_status(self, client):
        """Test filtering jobs by single status."""
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})
        client.post("/status", json={"group": "g1", "job": "job3", "status": "success"})

        response = client.get("/jobs?status=success")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total"] == 2
        assert all(job["status"] == "success" for job in data["jobs"])

    def test_get_jobs_filter_multiple_statuses(self, client):
        """Test filtering jobs by multiple comma-separated statuses."""
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})
        client.post("/status", json={"group": "g1", "job": "job3", "status": "timeout"})
        client.post(
            "/status", json={"group": "g1", "job": "job4", "status": "progress"}
        )

        # This matches the "Errors" card behavior (error + timeout)
        response = client.get("/jobs?status=error,timeout")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total"] == 2
        statuses = {job["status"] for job in data["jobs"]}
        assert statuses == {"error", "timeout"}

    def test_get_jobs_invalid_status(self, client):
        """Test filtering with invalid status returns 400."""
        response = client.get("/jobs?status=invalid")
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "validation_error"
        assert "Invalid status" in data["message"]
        assert data["field"] == "status"

    def test_get_jobs_one_valid_one_invalid_status(self, client):
        """Test filtering with one valid and one invalid status returns 400."""
        response = client.get("/jobs?status=success,badstatus")
        assert response.status_code == 400
        data = response.get_json()
        assert "Invalid status 'badstatus'" in data["message"]

    def test_get_jobs_empty_status_returns_all(self, client):
        """Test that empty status parameter returns all jobs."""
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})

        response = client.get("/jobs?status=")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total"] == 2

    def test_get_jobs_includes_group_name(self, client):
        """Test that jobs include group_name in response."""
        client.post(
            "/status", json={"group": "my-group", "job": "job1", "status": "success"}
        )

        response = client.get("/jobs")
        data = response.get_json()
        assert data["jobs"][0]["group_name"] == "my-group"

    def test_get_jobs_ordered_by_updated_at_desc(self, client, db_session):
        """Test that jobs are ordered by updated_at descending (newest first)."""
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        # Create a group
        group = Group(name="test-group")
        db_session.add(group)
        db_session.flush()

        # Create jobs with different timestamps
        now = datetime.now(UTC).replace(tzinfo=None)
        job1 = Job(
            group_id=group.id,
            name="older-job",
            status="success",
            updated_at=now - timedelta(hours=2),
            created_at=now - timedelta(hours=2),
        )
        job2 = Job(
            group_id=group.id,
            name="newer-job",
            status="success",
            updated_at=now - timedelta(hours=1),
            created_at=now - timedelta(hours=1),
        )
        job3 = Job(
            group_id=group.id,
            name="newest-job",
            status="success",
            updated_at=now,
            created_at=now,
        )
        db_session.add_all([job1, job2, job3])
        db_session.commit()

        response = client.get("/jobs")
        data = response.get_json()

        # Verify order: newest first
        assert data["jobs"][0]["name"] == "newest-job"
        assert data["jobs"][1]["name"] == "newer-job"
        assert data["jobs"][2]["name"] == "older-job"

    def test_get_jobs_response_structure(self, client):
        """Test that response structure matches the expected format."""
        client.post(
            "/status",
            json={
                "group": "backups",
                "job": "daily-backup",
                "status": "error",
                "message": "Connection failed",
            },
        )

        response = client.get("/jobs")
        data = response.get_json()

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

    def test_get_jobs_filter_whitespace_handling(self, client):
        """Test that whitespace in status parameter is handled."""
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})

        # Status with extra whitespace
        response = client.get("/jobs?status= success , error ")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total"] == 2


class TestJobsPagination:
    """Tests for opt-in limit/offset pagination on GET /jobs.

    AIDEV-NOTE: Pagination is opt-in and backward-compatible: with no params the
    response is unchanged (all jobs, total == len(jobs)). When limit/offset are
    given, `total` is the full matching count, not the page size.
    """

    def _make(self, client, count, group="g1", status="success"):
        for i in range(count):
            resp = client.post(
                "/status",
                json={"group": group, "job": f"job{i}", "status": status},
            )
            assert resp.status_code == 201

    def test_no_params_returns_all(self, client):
        self._make(client, 5)
        data = client.get("/jobs").get_json()
        assert len(data["jobs"]) == 5
        assert data["total"] == 5

    def test_limit_returns_slice_with_full_total(self, client):
        self._make(client, 5)
        data = client.get("/jobs?limit=2").get_json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    def test_offset_pages_through_in_order(self, client, db_session):
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        group = Group(name="g1")
        db_session.add(group)
        db_session.flush()
        now = datetime.now(UTC).replace(tzinfo=None)
        # newest first: j0 (newest) ... j4 (oldest)
        for i in range(5):
            db_session.add(
                Job(
                    group_id=group.id,
                    name=f"j{i}",
                    status="success",
                    updated_at=now - timedelta(hours=i),
                    created_at=now - timedelta(hours=i),
                )
            )
        db_session.commit()

        page1 = client.get("/jobs?limit=2&offset=0").get_json()
        page2 = client.get("/jobs?limit=2&offset=2").get_json()
        page3 = client.get("/jobs?limit=2&offset=4").get_json()

        assert [j["name"] for j in page1["jobs"]] == ["j0", "j1"]
        assert [j["name"] for j in page2["jobs"]] == ["j2", "j3"]
        assert [j["name"] for j in page3["jobs"]] == ["j4"]
        for page in (page1, page2, page3):
            assert page["total"] == 5

    def test_offset_beyond_end_returns_empty(self, client):
        self._make(client, 3)
        data = client.get("/jobs?limit=10&offset=100").get_json()
        assert data["jobs"] == []
        assert data["total"] == 3

    def test_limit_is_clamped_to_max(self, client, monkeypatch):
        monkeypatch.setattr("config.Config.MAX_JOBS_PAGE_SIZE", 2)
        self._make(client, 5)
        data = client.get("/jobs?limit=100").get_json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    def test_pagination_respects_status_filter(self, client):
        self._make(client, 3, status="success")
        self._make(client, 2, group="g1", status="error")
        data = client.get("/jobs?status=error&limit=1").get_json()
        assert len(data["jobs"]) == 1
        assert data["jobs"][0]["status"] == "error"
        # total reflects the filtered set, not all jobs
        assert data["total"] == 2

    def test_invalid_limit_returns_400(self, client):
        resp = client.get("/jobs?limit=abc")
        assert resp.status_code == 400
        data = resp.get_json()
        assert data["error"] == "validation_error"
        assert data["field"] == "limit"

    def test_zero_limit_returns_400(self, client):
        resp = client.get("/jobs?limit=0")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "limit"

    def test_negative_offset_returns_400(self, client):
        resp = client.get("/jobs?offset=-1")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "offset"

    def test_invalid_offset_returns_400(self, client):
        resp = client.get("/jobs?offset=xyz")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "offset"


class TestGroupsEndpoint:
    """Tests for groups endpoints."""

    def test_get_groups_empty(self, client):
        """Test getting groups when none exist."""
        response = client.get("/groups")
        assert response.status_code == 200
        data = response.get_json()
        assert data["groups"] == []

    def test_get_groups_with_data(self, client):
        """Test getting groups with jobs."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )
        client.post(
            "/status", json={"group": "backups", "job": "job2", "status": "success"}
        )
        client.post(
            "/status", json={"group": "monitoring", "job": "job1", "status": "error"}
        )

        response = client.get("/groups")
        data = response.get_json()

        assert len(data["groups"]) == 2

        backups = next(g for g in data["groups"] if g["name"] == "backups")
        assert backups["job_count"] == 2
        assert backups["health"] == "healthy"
        assert backups["status_counts"]["success"] == 2

        monitoring = next(g for g in data["groups"] if g["name"] == "monitoring")
        assert monitoring["job_count"] == 1
        assert monitoring["health"] == "unhealthy"

    def test_get_groups_includes_zero_job_group(self, client, db_session):
        """A group with no jobs must still appear, with all-zero counts.

        AIDEV-NOTE: Guards the aggregate /groups implementation against using an
        inner join (which would silently drop empty groups).
        """
        from models import Group

        db_session.add(Group(name="empty-group"))
        db_session.commit()
        client.post(
            "/status", json={"group": "busy", "job": "job1", "status": "success"}
        )

        response = client.get("/groups")
        data = response.get_json()

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

    def test_get_group_jobs(self, client):
        """Test getting jobs for a specific group."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )
        client.post(
            "/status", json={"group": "backups", "job": "job2", "status": "progress"}
        )

        response = client.get("/groups/backups/jobs")
        assert response.status_code == 200
        data = response.get_json()

        assert data["group"]["name"] == "backups"
        assert len(data["jobs"]) == 2

    def test_get_group_jobs_not_found(self, client):
        """Test getting jobs for non-existent group returns 404."""
        response = client.get("/groups/nonexistent/jobs")
        assert response.status_code == 404
        data = response.get_json()
        assert data["error"] == "not_found"

    def test_get_group_jobs_case_insensitive(self, client):
        """Test that group name lookup is case-insensitive."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.get("/groups/BACKUPS/jobs")
        assert response.status_code == 200


class TestGroupJobsPagination:
    """Tests for opt-in limit/offset pagination on GET /groups/<name>/jobs.

    AIDEV-NOTE: Mirrors GET /jobs -- opt-in and backward-compatible. With no params
    the full result set is returned; the response gains a `total` (full matching
    count for the group) alongside the existing `group` and `jobs` keys.
    """

    def _make(self, client, count, group="g1", status="success"):
        for i in range(count):
            resp = client.post(
                "/status",
                json={"group": group, "job": f"job{i}", "status": status},
            )
            assert resp.status_code == 201

    def test_no_params_returns_all_with_total(self, client):
        self._make(client, 5)
        data = client.get("/groups/g1/jobs").get_json()
        assert len(data["jobs"]) == 5
        assert data["total"] == 5

    def test_limit_returns_slice_with_full_total(self, client):
        self._make(client, 5)
        data = client.get("/groups/g1/jobs?limit=2").get_json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    def test_offset_pages_through_in_order(self, client, db_session):
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        group = Group(name="g1")
        db_session.add(group)
        db_session.flush()
        now = datetime.now(UTC).replace(tzinfo=None)
        for i in range(5):
            db_session.add(
                Job(
                    group_id=group.id,
                    name=f"j{i}",
                    status="success",
                    updated_at=now - timedelta(hours=i),
                    created_at=now - timedelta(hours=i),
                )
            )
        db_session.commit()

        page1 = client.get("/groups/g1/jobs?limit=2&offset=0").get_json()
        page2 = client.get("/groups/g1/jobs?limit=2&offset=2").get_json()

        assert [j["name"] for j in page1["jobs"]] == ["j0", "j1"]
        assert [j["name"] for j in page2["jobs"]] == ["j2", "j3"]
        assert page1["total"] == 5
        assert page2["total"] == 5

    def test_offset_beyond_end_returns_empty(self, client):
        self._make(client, 3)
        data = client.get("/groups/g1/jobs?limit=10&offset=100").get_json()
        assert data["jobs"] == []
        assert data["total"] == 3

    def test_limit_is_clamped_to_max(self, client, monkeypatch):
        monkeypatch.setattr("config.Config.MAX_JOBS_PAGE_SIZE", 2)
        self._make(client, 5)
        data = client.get("/groups/g1/jobs?limit=100").get_json()
        assert len(data["jobs"]) == 2
        assert data["total"] == 5

    def test_total_is_scoped_to_group(self, client):
        self._make(client, 3, group="g1")
        self._make(client, 4, group="g2")
        data = client.get("/groups/g1/jobs?limit=1").get_json()
        assert len(data["jobs"]) == 1
        assert data["total"] == 3

    def test_invalid_limit_returns_400(self, client):
        self._make(client, 1)
        resp = client.get("/groups/g1/jobs?limit=abc")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "limit"

    def test_zero_limit_returns_400(self, client):
        self._make(client, 1)
        resp = client.get("/groups/g1/jobs?limit=0")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "limit"

    def test_negative_offset_returns_400(self, client):
        self._make(client, 1)
        resp = client.get("/groups/g1/jobs?offset=-1")
        assert resp.status_code == 400
        assert resp.get_json()["field"] == "offset"


class TestConfigEndpoints:
    """Tests for configuration endpoints."""

    def test_get_config_defaults(self, client):
        """Test getting config returns default values."""
        response = client.get("/config")
        assert response.status_code == 200
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 5
        assert data["staleness_timeout_hours"] == 24

    def test_update_config(self, client):
        """Test updating global config."""
        response = client.put(
            "/config",
            json={"progress_timeout_minutes": 10, "staleness_timeout_hours": 48},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 10
        assert data["staleness_timeout_hours"] == 48

    def test_update_config_invalid_value(self, client):
        """Test updating config with invalid value returns error."""
        response = client.put(
            "/config",
            json={"progress_timeout_minutes": 0},
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "validation_error"

    def test_update_config_partial(self, client):
        """Test updating only one config value."""
        response = client.put(
            "/config",
            json={"progress_timeout_minutes": 15},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["progress_timeout_minutes"] == 15
        assert data["staleness_timeout_hours"] == 24

    def test_get_group_config(self, client):
        """Test getting group-specific config."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.get("/groups/backups/config")
        assert response.status_code == 200
        data = response.get_json()
        assert data["group_name"] == "backups"
        assert data["progress_timeout_minutes"] is None
        assert data["staleness_enabled"] is False
        assert data["staleness_timeout_hours"] is None
        assert data["expiration_timeout_hours"] is None
        assert data["effective_progress_timeout_minutes"] == 5
        assert data["effective_staleness_timeout_hours"] == 24
        assert data["effective_expiration_timeout_hours"] == 24

    def test_update_group_config(self, client):
        """Test updating group-specific config."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 72},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["staleness_timeout_hours"] == 72
        assert data["effective_staleness_timeout_hours"] == 72

    def test_update_group_config_null_reverts(self, client):
        """Test setting group config to null reverts to global."""
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # Set override
        client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 72},
        )

        # Revert to null
        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": None},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["staleness_timeout_hours"] is None

    def test_update_group_config_staleness_validation(self, client):
        """Test that staleness_timeout_hours must be less than expiration_timeout_hours.

        AIDEV-NOTE: This validation only applies when staleness_enabled is True.
        """
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # First enable staleness and set expiration to 24 hours
        response = client.put(
            "/groups/backups/config",
            json={
                "staleness_enabled": True,
                "expiration_timeout_hours": 24,
                "staleness_timeout_hours": 12,
            },
        )
        assert response.status_code == 200  # Ensure setup succeeded

        # Try to set staleness >= expiration - should fail
        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 24},  # Equal to expiration
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "validation_error"
        assert "staleness_timeout_hours" in data.get("fields", {})

        # Try to set staleness > expiration - should also fail
        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 48},  # Greater than expiration
        )
        assert response.status_code == 400

        # Valid: staleness < expiration should work
        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 12},  # Less than 24
        )
        assert response.status_code == 200

    def test_update_group_config_staleness_validation_skipped_when_disabled(
        self, client
    ):
        """Test that staleness validation is skipped when staleness_enabled is False.

        AIDEV-NOTE: Users can set staleness_timeout_hours >= expiration_timeout_hours
        when staleness is disabled, since the staleness feature won't be used.
        """
        client.post(
            "/status", json={"group": "backups", "job": "job1", "status": "success"}
        )

        # Set expiration to 24 hours, staleness disabled (default)
        response = client.put(
            "/groups/backups/config",
            json={"expiration_timeout_hours": 24, "staleness_enabled": False},
        )
        assert response.status_code == 200  # Ensure setup succeeded

        # Setting staleness >= expiration should work when staleness is disabled
        response = client.put(
            "/groups/backups/config",
            json={"staleness_timeout_hours": 48},
        )
        assert response.status_code == 200

    def test_update_config_boolean_rejected(self, client):
        """Test that boolean values are rejected for config."""
        response = client.put(
            "/config",
            json={"progress_timeout_minutes": True},
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "validation_error"

    def test_update_config_json_array_rejected(self, client):
        """Test that JSON arrays are rejected."""
        response = client.put(
            "/config",
            content_type="application/json",
            data="[10, 20]",
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "bad_request"

    def test_status_json_array_rejected(self, client):
        """Test that JSON arrays are rejected for status endpoint."""
        response = client.post(
            "/status",
            content_type="application/json",
            data='["invalid"]',
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "bad_request"

    def test_get_group_config_not_found(self, client):
        """Test getting config for non-existent group returns 404."""
        response = client.get("/groups/nonexistent/config")
        assert response.status_code == 404

    def test_update_config_expiration_cascades_to_non_override_groups(
        self, client, db_session
    ):
        """Changing global expiration_timeout_hours must refresh expires_at for
        jobs in groups WITHOUT an override, and leave override-group jobs alone.

        AIDEV-NOTE: Mirrors the group-level refresh. SQLite's datetime() truncates
        to whole seconds, so deltas are compared with a small tolerance.
        """
        from datetime import timedelta

        from models import Job

        # Group with no expiration override -> follows the global default.
        client.post(
            "/status", json={"group": "noverride", "job": "a", "status": "success"}
        )
        # Group with an explicit 10-hour expiration override.
        client.post(
            "/status", json={"group": "override", "job": "b", "status": "success"}
        )
        resp = client.put(
            "/groups/override/config", json={"expiration_timeout_hours": 10}
        )
        assert resp.status_code == 200

        # Change the global expiration from the 24h default to 48h.
        resp = client.put("/config", json={"expiration_timeout_hours": 48})
        assert resp.status_code == 200

        db_session.expire_all()
        job_a = db_session.query(Job).filter_by(name="a").first()
        job_b = db_session.query(Job).filter_by(name="b").first()

        # Non-override group's job: expires_at refreshed to updated_at + 48h.
        expected_a = job_a.updated_at + timedelta(hours=48)
        assert abs((job_a.expires_at - expected_a).total_seconds()) < 2

        # Override group's job: untouched, still updated_at + 10h.
        expected_b = job_b.updated_at + timedelta(hours=10)
        assert abs((job_b.expires_at - expected_b).total_seconds()) < 2


class TestAdminEndpoints:
    """Tests for admin endpoints (data retention & cleanup)."""

    def test_get_admin_stats_empty(self, client):
        """Test admin stats when database is empty."""
        response = client.get("/admin/stats")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total_jobs"] == 0
        assert data["total_groups"] == 0
        assert data["jobs_by_status"]["success"] == 0
        assert data["jobs_by_status"]["error"] == 0

    def test_get_admin_stats_with_data(self, client):
        """Test admin stats with jobs in database."""
        # Create some jobs with different statuses
        client.post("/status", json={"group": "g1", "job": "job1", "status": "success"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})
        client.post(
            "/status", json={"group": "g2", "job": "job1", "status": "progress"}
        )

        response = client.get("/admin/stats")
        assert response.status_code == 200
        data = response.get_json()
        assert data["total_jobs"] == 3
        assert data["total_groups"] == 2
        assert data["jobs_by_status"]["success"] == 1
        assert data["jobs_by_status"]["error"] == 1
        assert data["jobs_by_status"]["progress"] == 1

    def test_admin_cleanup_requires_older_than_days(self, client):
        """Test cleanup requires older_than_days parameter."""
        response = client.delete(
            "/admin/cleanup",
            json={"statuses": ["stale"]},
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["field"] == "older_than_days"

    def test_admin_cleanup_invalid_older_than_days(self, client):
        """Test cleanup rejects invalid older_than_days."""
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 0},
        )
        assert response.status_code == 400
        assert "positive integer" in response.get_json()["message"]

    def test_admin_cleanup_invalid_status(self, client):
        """Test cleanup rejects invalid status values."""
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["invalid"]},
        )
        assert response.status_code == 400
        assert "Invalid status" in response.get_json()["message"]

    def test_admin_cleanup_dry_run(self, client, db_session):
        """Test cleanup dry_run returns counts without deleting."""
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        # Create a group with an old stale job
        group = Group(name="test-group")
        db_session.add(group)
        db_session.flush()

        old_date = datetime.now(UTC) - timedelta(days=60)
        job = Job(
            group_id=group.id,
            name="old-job",
            status="stale",
            updated_at=old_date.replace(tzinfo=None),
            created_at=old_date.replace(tzinfo=None),
        )
        db_session.add(job)
        db_session.commit()

        # Save ID before API call (session may be invalidated)
        job_id = job.id

        # Run dry_run cleanup
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"], "dry_run": True},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["deleted_jobs"] == 1
        assert data["deleted_groups"] == 1
        assert data["dry_run"] is True

        # Verify job still exists (use saved ID, not the potentially detached object)
        db_session.expire_all()
        job_check = db_session.get(Job, job_id)
        assert job_check is not None

    def test_admin_cleanup_deletes_jobs(self, client, db_session):
        """Test cleanup actually deletes old jobs."""
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        # Create a group with an old stale job
        group = Group(name="test-group")
        db_session.add(group)
        db_session.flush()

        old_date = datetime.now(UTC) - timedelta(days=60)
        job = Job(
            group_id=group.id,
            name="old-job",
            status="stale",
            updated_at=old_date.replace(tzinfo=None),
            created_at=old_date.replace(tzinfo=None),
        )
        db_session.add(job)
        db_session.commit()
        job_id = job.id

        # Run actual cleanup
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"], "dry_run": False},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["deleted_jobs"] == 1
        assert data["deleted_groups"] == 1
        assert data["dry_run"] is False

        # Verify job is deleted
        db_session.expire_all()
        job_check = db_session.get(Job, job_id)
        assert job_check is None

    def test_admin_cleanup_preserves_recent_jobs(self, client):
        """Test cleanup doesn't delete recent jobs."""
        # Create a recent job
        client.post("/status", json={"group": "test", "job": "job1", "status": "stale"})

        # Try to cleanup (should not delete because job is recent)
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 30, "statuses": ["stale"]},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["deleted_jobs"] == 0
        assert data["deleted_groups"] == 0

    def test_admin_cleanup_respects_status_filter(self, client, db_session):
        """Test cleanup only deletes jobs with specified statuses."""
        from datetime import UTC, datetime, timedelta

        from models import Group, Job

        # Create a group with old jobs of different statuses
        group = Group(name="test-group")
        db_session.add(group)
        db_session.flush()

        old_date = datetime.now(UTC) - timedelta(days=60)

        # Create stale job (should be deleted)
        stale_job = Job(
            group_id=group.id,
            name="stale-job",
            status="stale",
            updated_at=old_date.replace(tzinfo=None),
            created_at=old_date.replace(tzinfo=None),
        )
        db_session.add(stale_job)

        # Create error job (should NOT be deleted with default statuses)
        error_job = Job(
            group_id=group.id,
            name="error-job",
            status="error",
            updated_at=old_date.replace(tzinfo=None),
            created_at=old_date.replace(tzinfo=None),
        )
        db_session.add(error_job)
        db_session.commit()

        # Save ID before cleanup
        error_job_id = error_job.id

        # Run cleanup with default statuses (stale, timeout)
        response = client.delete(
            "/admin/cleanup",
            json={"older_than_days": 30},
        )
        assert response.status_code == 200
        data = response.get_json()
        assert data["deleted_jobs"] == 1  # Only stale job

        # Verify error job still exists
        db_session.expire_all()
        error_check = db_session.get(Job, error_job_id)
        assert error_check is not None

    def test_admin_cleanup_requires_json(self, client):
        """Test cleanup requires JSON body with older_than_days."""
        # Empty JSON object should fail validation for older_than_days
        response = client.delete(
            "/admin/cleanup",
            json={},
        )
        assert response.status_code == 400
        data = response.get_json()
        assert data["field"] == "older_than_days"


class TestAckEndpoint:
    """Tests for job acknowledgement endpoints.

    AIDEV-NOTE: Tests for the ack feature that allows acknowledging errors/timeouts/stale jobs.
    """

    def test_ack_job_success(self, client):
        """Test acknowledging a job in error state."""
        # Create an error job
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # Get job ID
        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        # Ack the job
        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 200
        data = response.get_json()
        assert data["job"]["acked"] is True
        assert data["job"]["acked_at"] is not None
        assert data["job"]["status"] == "error"

    def test_ack_job_timeout_status(self, client):
        """Test acknowledging a job in timeout state."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "timeout"}
        )

        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 200
        assert response.get_json()["job"]["acked"] is True

    def test_ack_job_stale_status(self, client):
        """Test acknowledging a job in stale state."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "stale"})

        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 200
        assert response.get_json()["job"]["acked"] is True

    def test_ack_job_not_found(self, client):
        """Test acknowledging a non-existent job returns 404."""
        response = client.post("/jobs/99999/ack")
        assert response.status_code == 404
        data = response.get_json()
        assert data["error"] == "not_found"

    def test_ack_job_invalid_state_success(self, client):
        """Test that acking a success job returns 400."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 400
        data = response.get_json()
        assert data["error"] == "invalid_state"
        assert "Cannot ack job with status 'success'" in data["message"]

    def test_ack_job_invalid_state_progress(self, client):
        """Test that acking a progress job returns 400."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 400
        assert "Cannot ack job with status 'progress'" in response.get_json()["message"]

    def test_ack_job_idempotent(self, client):
        """Test that acking an already-acked job is a no-op."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        response = client.get("/jobs")
        job_id = response.get_json()["jobs"][0]["id"]

        # First ack
        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 200
        first_acked_at = response.get_json()["job"]["acked_at"]

        # Second ack (should be no-op, acked_at should not change)
        response = client.post(f"/jobs/{job_id}/ack")
        assert response.status_code == 200
        second_acked_at = response.get_json()["job"]["acked_at"]

        # acked_at should be the same (no update)
        assert first_acked_at == second_acked_at


class TestAckHealthCalculation:
    """Tests for health calculation with acked jobs.

    AIDEV-NOTE: Verifies that acked jobs are excluded from unhealthy count.
    """

    def test_health_excludes_acked_from_unhealthy(self, client):
        """Test that acked jobs are excluded from unhealthy count."""
        # Create an error job
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # Before ack: unhealthy = 1
        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1
        assert data["acked"] == 0

        # Ack the job
        jobs_response = client.get("/jobs")
        job_id = jobs_response.get_json()["jobs"][0]["id"]
        client.post(f"/jobs/{job_id}/ack")

        # After ack: unhealthy = 0, acked = 1
        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 1

    def test_health_status_healthy_when_all_errors_acked(self, client):
        """Test that dashboard shows healthy when all errors are acked."""
        # Create multiple error jobs
        client.post("/status", json={"group": "g1", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "timeout"})

        response = client.get("/health")
        assert response.get_json()["status"] == "unhealthy"

        # Ack all jobs
        jobs = client.get("/jobs").get_json()["jobs"]
        for job in jobs:
            client.post(f"/jobs/{job['id']}/ack")

        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 2

    def test_health_mixed_acked_and_unacked(self, client):
        """Test health with a mix of acked and unacked errors."""
        # Create two error jobs
        client.post("/status", json={"group": "g1", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "error"})

        # Ack only one job
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "unhealthy"
        assert data["unhealthy"] == 1
        assert data["acked"] == 1

    def test_health_by_status_includes_acked(self, client):
        """Test that by_status counts include acked jobs (raw counts)."""
        client.post("/status", json={"group": "g1", "job": "job1", "status": "error"})

        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        response = client.get("/health")
        data = response.get_json()
        # by_status should still show raw count
        assert data["by_status"]["error"] == 1


class TestAckGroupSummary:
    """Tests for group summary with acked jobs."""

    def test_group_unhealthy_count_excludes_acked(self, client):
        """Test that group unhealthy_count excludes acked jobs."""
        # Create error jobs in a group
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        client.post(
            "/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Before ack
        response = client.get("/groups")
        group = response.get_json()["groups"][0]
        assert group["unhealthy_count"] == 1
        assert group["acked_count"] == 0
        assert group["health"] == "unhealthy"

        # Ack the error job
        jobs = client.get("/jobs?status=error").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # After ack
        response = client.get("/groups")
        group = response.get_json()["groups"][0]
        assert group["unhealthy_count"] == 0
        assert group["acked_count"] == 1
        assert group["health"] == "healthy"

    def test_group_status_counts_include_acked(self, client):
        """Test that status_counts include acked jobs (raw counts)."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        response = client.get("/groups")
        group = response.get_json()["groups"][0]
        # status_counts should still show raw count
        assert group["status_counts"]["error"] == 1


class TestAckClearOnRecovery:
    """Tests for ack clearing when job recovers."""

    def test_ack_cleared_on_success(self, client):
        """Test that ack is cleared when job transitions to success."""
        # Create error job and ack it
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Verify acked
        jobs = client.get("/jobs").get_json()["jobs"]
        assert jobs[0]["acked"] is True

        # Job recovers to success
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Ack should be cleared
        jobs = client.get("/jobs").get_json()["jobs"]
        assert jobs[0]["acked"] is False
        assert jobs[0]["acked_at"] is None

    def test_ack_cleared_on_progress(self, client):
        """Test that ack is cleared when job transitions to progress."""
        # Create error job and ack it
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Job transitions to progress
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Ack should be cleared
        jobs = client.get("/jobs").get_json()["jobs"]
        assert jobs[0]["acked"] is False

    def test_ack_preserved_on_new_error(self, client):
        """Test that ack is preserved when an acked job gets another error submission."""
        # Create error job and ack it
        client.post(
            "/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "error",
                "message": "fail1",
            },
        )
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Submit new error (same job, new error message)
        client.post(
            "/status",
            json={
                "group": "test",
                "job": "job1",
                "status": "error",
                "message": "fail2",
            },
        )

        # Ack should be preserved
        jobs = client.get("/jobs").get_json()["jobs"]
        assert jobs[0]["acked"] is True
        assert jobs[0]["message"] == "fail2"

    def test_error_after_recovery_requires_new_ack(self, client):
        """Test that error after recovery requires a new ack."""
        # Create error job and ack it
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Job recovers
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Job errors again
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # Should require new ack (acked should be False)
        jobs = client.get("/jobs").get_json()["jobs"]
        assert jobs[0]["acked"] is False

        response = client.get("/health")
        assert response.get_json()["status"] == "unhealthy"


class TestJobsResponseIncludesAckedFields:
    """Tests that job responses include acked fields."""

    def test_jobs_list_includes_acked_fields(self, client):
        """Test that /jobs response includes acked and acked_at fields."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        response = client.get("/jobs")
        job = response.get_json()["jobs"][0]
        assert "acked" in job
        assert "acked_at" in job
        assert job["acked"] is False
        assert job["acked_at"] is None

    def test_group_jobs_includes_acked_fields(self, client):
        """Test that /groups/<name>/jobs response includes acked fields."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        response = client.get("/groups/test/jobs")
        job = response.get_json()["jobs"][0]
        assert "acked" in job
        assert "acked_at" in job

    def test_status_response_includes_acked_fields(self, client):
        """Test that POST /status response includes acked fields."""
        response = client.post(
            "/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        job = response.get_json()["job"]
        assert "acked" in job
        assert "acked_at" in job
        assert job["acked"] is False


class TestAckGroupEndpoint:
    """Tests for POST /groups/<name>/ack endpoint.

    AIDEV-NOTE: Tests for bulk acking all errored jobs in a group.
    """

    def test_ack_group_success(self, client):
        """Test acknowledging all errors in a group."""
        # Create jobs with different statuses
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        client.post(
            "/status", json={"group": "test", "job": "job2", "status": "timeout"}
        )
        client.post(
            "/status", json={"group": "test", "job": "job3", "status": "success"}
        )

        # Ack all errors in the group
        response = client.post("/groups/test/ack")
        assert response.status_code == 200
        data = response.get_json()
        assert data["acked_count"] == 2
        assert data["group"] == "test"

        # Verify jobs are acked
        response = client.get("/jobs?status=error,timeout")
        jobs = response.get_json()["jobs"]
        assert all(job["acked"] is True for job in jobs)

    def test_ack_group_not_found(self, client):
        """Test acking non-existent group returns 404."""
        response = client.post("/groups/nonexistent/ack")
        assert response.status_code == 404
        data = response.get_json()
        assert data["error"] == "not_found"

    def test_ack_group_case_insensitive(self, client):
        """Test that group name lookup is case-insensitive."""
        client.post(
            "/status", json={"group": "mygroup", "job": "job1", "status": "error"}
        )

        response = client.post("/groups/MyGroup/ack")
        assert response.status_code == 200
        assert response.get_json()["acked_count"] == 1

    def test_ack_group_no_errors(self, client):
        """Test acking a group with no errors returns 0."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.post("/groups/test/ack")
        assert response.status_code == 200
        data = response.get_json()
        assert data["acked_count"] == 0

    def test_ack_group_skips_already_acked(self, client):
        """Test that already-acked jobs are not counted."""
        # Create error jobs
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "test", "job": "job2", "status": "error"})

        # Ack one job individually
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Ack the group (should only ack the remaining one)
        response = client.post("/groups/test/ack")
        assert response.status_code == 200
        assert response.get_json()["acked_count"] == 1

    def test_ack_group_only_affects_target_group(self, client):
        """Test that acking a group doesn't affect other groups."""
        # Create errors in two groups
        client.post(
            "/status", json={"group": "group1", "job": "job1", "status": "error"}
        )
        client.post(
            "/status", json={"group": "group2", "job": "job1", "status": "error"}
        )

        # Ack only group1
        response = client.post("/groups/group1/ack")
        assert response.get_json()["acked_count"] == 1

        # Check group2 is not affected
        response = client.get("/groups")
        groups = response.get_json()["groups"]
        group2 = next(g for g in groups if g["name"] == "group2")
        assert group2["unhealthy_count"] == 1
        assert group2["acked_count"] == 0

    def test_ack_group_health_becomes_healthy(self, client):
        """Test that group health becomes healthy after acking all errors."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        client.post(
            "/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Before ack
        response = client.get("/groups")
        group = response.get_json()["groups"][0]
        assert group["health"] == "unhealthy"

        # Ack the group
        client.post("/groups/test/ack")

        # After ack
        response = client.get("/groups")
        group = response.get_json()["groups"][0]
        assert group["health"] == "healthy"
        assert group["acked_count"] == 1


class TestAckAllEndpoint:
    """Tests for POST /ack-all endpoint.

    AIDEV-NOTE: Tests for bulk acking all errored jobs globally.
    """

    def test_ack_all_success(self, client):
        """Test acknowledging all errors globally."""
        # Create errors in multiple groups
        client.post("/status", json={"group": "g1", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "g1", "job": "job2", "status": "timeout"})
        client.post("/status", json={"group": "g2", "job": "job1", "status": "stale"})
        client.post("/status", json={"group": "g2", "job": "job2", "status": "success"})

        # Ack all
        response = client.post("/ack-all")
        assert response.status_code == 200
        data = response.get_json()
        assert data["acked_count"] == 3

        # Verify health
        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "healthy"
        assert data["unhealthy"] == 0
        assert data["acked"] == 3

    def test_ack_all_no_errors(self, client):
        """Test ack-all when no errors exist."""
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        response = client.post("/ack-all")
        assert response.status_code == 200
        assert response.get_json()["acked_count"] == 0

    def test_ack_all_empty_database(self, client):
        """Test ack-all when database is empty."""
        response = client.post("/ack-all")
        assert response.status_code == 200
        assert response.get_json()["acked_count"] == 0

    def test_ack_all_skips_already_acked(self, client):
        """Test that already-acked jobs are not counted."""
        # Create error jobs
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "test", "job": "job2", "status": "error"})

        # Ack one job individually
        jobs = client.get("/jobs").get_json()["jobs"]
        client.post(f"/jobs/{jobs[0]['id']}/ack")

        # Ack all (should only ack the remaining one)
        response = client.post("/ack-all")
        assert response.status_code == 200
        assert response.get_json()["acked_count"] == 1

    def test_ack_all_affects_all_groups(self, client):
        """Test that ack-all affects all groups."""
        # Create errors in multiple groups
        client.post("/status", json={"group": "g1", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "g2", "job": "job1", "status": "error"})
        client.post("/status", json={"group": "g3", "job": "job1", "status": "error"})

        # Ack all
        response = client.post("/ack-all")
        assert response.get_json()["acked_count"] == 3

        # Check all groups
        response = client.get("/groups")
        groups = response.get_json()["groups"]
        for group in groups:
            assert group["unhealthy_count"] == 0
            assert group["acked_count"] == 1

    def test_ack_all_idempotent(self, client):
        """Test that calling ack-all twice is idempotent."""
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # First call
        response = client.post("/ack-all")
        assert response.get_json()["acked_count"] == 1

        # Second call (should be 0 since already acked)
        response = client.post("/ack-all")
        assert response.get_json()["acked_count"] == 0


class TestDeleteJobEndpoint:
    """Tests for DELETE /jobs/<id> endpoint.

    AIDEV-NOTE: Tests for the job deletion feature.
    """

    def test_delete_job_success(self, client):
        """Test deleting an existing job returns 200 and removes it."""
        # Create a job
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Get job ID by name to avoid flaky ordering assumptions
        response = client.get("/jobs")
        data = response.get_json()
        job = next(j for j in data["jobs"] if j["name"] == "job1")
        job_id = job["id"]
        assert data["total"] == 1

        # Delete the job
        response = client.delete(f"/jobs/{job_id}")
        assert response.status_code == 200
        data = response.get_json()
        assert data["deleted_job"]["id"] == job_id
        assert data["deleted_job"]["name"] == "job1"
        assert data["group_name"] == "test"

        # Verify job is gone
        response = client.get("/jobs")
        assert response.get_json()["total"] == 0

    def test_delete_job_not_found(self, client):
        """Test deleting a non-existent job returns 404."""
        # Create and delete a job to get a known non-existent ID
        client.post(
            "/status", json={"group": "test", "job": "temp", "status": "success"}
        )
        response = client.get("/jobs")
        job = next(j for j in response.get_json()["jobs"] if j["name"] == "temp")
        deleted_id = job["id"]
        client.delete(f"/jobs/{deleted_id}")

        # Now try to delete the already-deleted job
        response = client.delete(f"/jobs/{deleted_id}")
        assert response.status_code == 404
        data = response.get_json()
        assert data["error"] == "not_found"

    def test_delete_job_twice_returns_404(self, client):
        """Test deleting the same job twice returns 404 on second call."""
        # Create a job
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Get job ID by name to avoid flaky ordering assumptions
        response = client.get("/jobs")
        job = next(j for j in response.get_json()["jobs"] if j["name"] == "job1")
        job_id = job["id"]

        # First delete succeeds
        response = client.delete(f"/jobs/{job_id}")
        assert response.status_code == 200

        # Second delete returns 404
        response = client.delete(f"/jobs/{job_id}")
        assert response.status_code == 404

    def test_delete_job_updates_health(self, client):
        """Test that deleting a job updates health counts."""
        # Create an error job
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # Verify unhealthy count
        response = client.get("/health")
        assert response.get_json()["unhealthy"] == 1

        # Get job ID by name and delete
        response = client.get("/jobs")
        job = next(j for j in response.get_json()["jobs"] if j["name"] == "job1")
        client.delete(f"/jobs/{job['id']}")

        # Verify health is now empty
        response = client.get("/health")
        data = response.get_json()
        assert data["status"] == "empty"
        assert data["total_jobs"] == 0

    def test_delete_job_updates_group_counts(self, client):
        """Test that deleting a job updates group job count."""
        # Create two jobs in same group
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )
        client.post(
            "/status", json={"group": "test", "job": "job2", "status": "success"}
        )

        # Verify group has 2 jobs (find by name to avoid ordering assumptions)
        response = client.get("/groups")
        group = next(g for g in response.get_json()["groups"] if g["name"] == "test")
        assert group["job_count"] == 2

        # Get job ID by name and delete one
        response = client.get("/jobs")
        job = next(j for j in response.get_json()["jobs"] if j["name"] == "job1")
        client.delete(f"/jobs/{job['id']}")

        # Verify group now has 1 job
        response = client.get("/groups")
        group = next(g for g in response.get_json()["groups"] if g["name"] == "test")
        assert group["job_count"] == 1


class TestLogUpload:
    """Tests for log upload functionality on POST /status."""

    def test_submit_status_with_log_multipart(self, client):
        """Test submitting status with log file via multipart/form-data."""
        from io import BytesIO

        log_content = "line1\nline2\nline3\n"
        data = {
            "group": "builds",
            "job": "compile",
            "status": "success",
            "message": "Build completed",
            "log": (BytesIO(log_content.encode()), "build.log"),
        }

        response = client.post(
            "/status",
            data=data,
            content_type="multipart/form-data",
        )

        assert response.status_code == 201
        result = response.get_json()
        assert result["success"] is True
        assert result["job"]["has_log"] is True
        assert result["job"]["log_line_count"] == 3
        assert result["job"]["log_truncated"] is False
        assert result["job"]["log_updated_at"] is not None

    def test_submit_status_without_log_json(self, client):
        """Test submitting status without log via JSON."""
        response = client.post(
            "/status",
            json={"group": "builds", "job": "test", "status": "success"},
        )

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is False
        assert result["job"]["log_line_count"] is None

    def test_submit_status_multipart_without_log(self, client):
        """Test submitting status via multipart without log file."""
        data = {
            "group": "builds",
            "job": "deploy",
            "status": "success",
        }

        response = client.post(
            "/status",
            data=data,
            content_type="multipart/form-data",
        )

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is False

    def test_log_replaces_previous_on_update(self, client):
        """Test that new log replaces previous log on status update."""
        from io import BytesIO

        # Submit initial log
        data = {
            "group": "builds",
            "job": "test",
            "status": "progress",
            "log": (BytesIO(b"initial log\n"), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")
        assert response.status_code == 201

        # Verify initial log content
        response = client.get("/groups/builds/jobs/test/log")
        assert "initial log" in response.get_json()["log"]

        # Submit new log
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(b"new log line 1\nnew log line 2\n"), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")
        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["log_line_count"] == 2

        # Verify the log was replaced (not appended)
        response = client.get("/groups/builds/jobs/test/log")
        log = response.get_json()["log"]
        assert "initial log" not in log
        assert "new log line 1" in log

    def test_log_not_cleared_without_new_log(self, client):
        """Test that existing log is preserved when updating without new log."""
        from io import BytesIO

        # Submit with log
        data = {
            "group": "builds",
            "job": "test",
            "status": "progress",
            "log": (BytesIO(b"my log content\n"), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")
        assert response.status_code == 201

        # Update without log (JSON)
        response = client.post(
            "/status",
            json={"group": "builds", "job": "test", "status": "success"},
        )
        assert response.status_code == 201
        result = response.get_json()
        # Log should still be present
        assert result["job"]["has_log"] is True
        assert result["job"]["log_line_count"] == 1

    def test_log_metadata_in_job_response(self, client):
        """Test that log metadata is included in job responses."""
        from io import BytesIO

        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(b"line\n" * 10), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        result = response.get_json()
        job = result["job"]
        assert "has_log" in job
        assert "log_line_count" in job
        assert "log_truncated" in job
        assert "log_updated_at" in job
        assert job["log_line_count"] == 10


class TestLogRetrieval:
    """Tests for GET /groups/{name}/jobs/{job}/log endpoint."""

    def test_get_log_full_content(self, client):
        """Test retrieving full log content."""
        from io import BytesIO

        log_content = "line1\nline2\nline3\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        client.post("/status", data=data, content_type="multipart/form-data")

        response = client.get("/groups/builds/jobs/test/log")
        assert response.status_code == 200
        result = response.get_json()
        assert result["log"] == log_content
        assert result["line_count"] == 3
        assert result["truncated"] is False

    def test_get_log_with_tail_param(self, client):
        """Test retrieving log with tail parameter."""
        from io import BytesIO

        log_content = "\n".join([f"line{i}" for i in range(100)]) + "\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        client.post("/status", data=data, content_type="multipart/form-data")

        response = client.get("/groups/builds/jobs/test/log?tail=5")
        assert response.status_code == 200
        result = response.get_json()
        assert result["line_count"] == 5
        assert result["truncated"] is True
        assert result["total_line_count"] == 100

    def test_get_log_with_all_param(self, client):
        """Test retrieving full log with all=true."""
        from io import BytesIO

        log_content = "\n".join([f"line{i}" for i in range(100)]) + "\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        client.post("/status", data=data, content_type="multipart/form-data")

        response = client.get("/groups/builds/jobs/test/log?all=true")
        assert response.status_code == 200
        result = response.get_json()
        assert result["line_count"] == 100
        assert result["truncated"] is False

    def test_get_log_not_found_group(self, client):
        """Test getting log for non-existent group returns 404."""
        response = client.get("/groups/nonexistent/jobs/test/log")
        assert response.status_code == 404
        assert "not found" in response.get_json()["message"].lower()

    def test_get_log_not_found_job(self, client):
        """Test getting log for non-existent job returns 404."""
        # Create group
        client.post(
            "/status",
            json={"group": "builds", "job": "other", "status": "success"},
        )

        response = client.get("/groups/builds/jobs/nonexistent/log")
        assert response.status_code == 404
        assert "not found" in response.get_json()["message"].lower()

    def test_get_log_no_log_available(self, client):
        """Test getting log when job has no log returns 404."""
        client.post(
            "/status",
            json={"group": "builds", "job": "test", "status": "success"},
        )

        response = client.get("/groups/builds/jobs/test/log")
        assert response.status_code == 404
        assert "no log" in response.get_json()["message"].lower()

    def test_get_log_default_tail_1000(self, client, monkeypatch):
        """Test that default tail is 1000 lines on retrieval."""
        from io import BytesIO

        import config

        # Set MAX_LOG_LINES high enough to store all 1500 lines during upload
        monkeypatch.setattr(config.Config, "MAX_LOG_LINES", 2000)

        # Create log with 1500 lines
        log_content = "\n".join([f"line{i}" for i in range(1500)]) + "\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        client.post("/status", data=data, content_type="multipart/form-data")

        # Default retrieval should return last 1000 lines
        response = client.get("/groups/builds/jobs/test/log")
        assert response.status_code == 200
        result = response.get_json()
        assert result["line_count"] == 1000
        assert result["truncated"] is True
        assert result["total_line_count"] == 1500


class TestLogConfigFlags:
    """Tests for log-related configuration flags."""

    def test_log_upload_disabled(self, client, monkeypatch):
        """Test that logs are ignored when LOG_UPLOAD_ENABLED=false."""
        from io import BytesIO

        import config

        monkeypatch.setattr(config.Config, "LOG_UPLOAD_ENABLED", False)

        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(b"ignored log\n"), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is False
        assert "warning" in result
        assert "disabled" in result["warning"].lower()

    def test_log_truncation_to_max_lines(self, client, monkeypatch):
        """Test that logs exceeding MAX_LOG_LINES are truncated."""
        from io import BytesIO

        import config

        monkeypatch.setattr(config.Config, "MAX_LOG_LINES", 10)

        # Create log with 25 lines
        log_content = "\n".join([f"line{i}" for i in range(25)]) + "\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is True
        assert result["job"]["log_line_count"] == 10
        assert result["job"]["log_truncated"] is True

    def test_log_within_max_lines_not_truncated(self, client, monkeypatch):
        """Test that logs within MAX_LOG_LINES are not truncated."""
        from io import BytesIO

        import config

        monkeypatch.setattr(config.Config, "MAX_LOG_LINES", 100)

        log_content = "\n".join([f"line{i}" for i in range(50)]) + "\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode()), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["log_line_count"] == 50
        assert result["job"]["log_truncated"] is False


class TestLogEncodingHandling:
    """Tests for log file encoding handling."""

    def test_utf8_log_content(self, client):
        """Test handling of UTF-8 encoded log content."""
        from io import BytesIO

        log_content = "Hello 世界\nUnicode: émojis 🎉\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content.encode("utf-8")), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is True

        # Verify content is preserved
        response = client.get("/groups/builds/jobs/test/log")
        assert log_content in response.get_json()["log"]

    def test_latin1_fallback_encoding(self, client):
        """Test fallback to latin-1 for non-UTF-8 content."""
        from io import BytesIO

        # Create bytes that are valid latin-1 but not valid UTF-8
        log_content = b"Line with latin-1: caf\xe9\n"
        data = {
            "group": "builds",
            "job": "test",
            "status": "success",
            "log": (BytesIO(log_content), "log.txt"),
        }
        response = client.post("/status", data=data, content_type="multipart/form-data")

        assert response.status_code == 201
        result = response.get_json()
        assert result["job"]["has_log"] is True
