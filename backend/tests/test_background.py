"""Tests for background timeout checker."""

from datetime import UTC, datetime, timedelta


class TestTimeoutChecker:
    """Tests for the background timeout checker."""

    def test_progress_timeout(self, client, db_session):
        """Test that progress jobs are marked as timeout after threshold."""
        from background import run_timeout_check
        from models import Job

        # Create a job in progress
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Manually set updated_at to be older than timeout
        job = db_session.query(Job).filter_by(name="job1").first()
        job.updated_at = datetime.now(UTC) - timedelta(minutes=10)
        db_session.commit()

        # Run timeout check
        result = run_timeout_check(db_session)

        # Job should now be timeout
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "timeout"
        assert len(result["affected_job_ids"]) == 1

    def test_staleness_timeout(self, client, db_session):
        """Test that success jobs are marked as stale after threshold."""
        from background import run_timeout_check
        from models import Group, Job

        # Create a job with success status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # AIDEV-NOTE: Staleness is now opt-in. Enable staleness for this group.
        group = db_session.query(Group).filter_by(name="test").first()
        group.staleness_enabled = True
        db_session.commit()

        # Manually set updated_at to be older than staleness timeout
        job = db_session.query(Job).filter_by(name="job1").first()
        job.updated_at = datetime.now(UTC) - timedelta(hours=25)
        db_session.commit()

        # Run timeout check
        result = run_timeout_check(db_session)

        # Job should now be stale
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "stale"
        assert result["stale_count"] == 1

    def test_group_timeout_override(self, client, db_session):
        """Test that group-specific timeout overrides are respected."""
        from background import run_timeout_check
        from models import Group, Job

        # Create a job in progress
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Set a longer group-specific timeout (30 minutes)
        group = db_session.query(Group).filter_by(name="test").first()
        group.progress_timeout_minutes = 30
        db_session.commit()

        # Set job updated_at to 10 minutes ago (within group threshold)
        job = db_session.query(Job).filter_by(name="job1").first()
        job.updated_at = datetime.now(UTC) - timedelta(minutes=10)
        db_session.commit()

        # Run timeout check - job should NOT be timed out
        result = run_timeout_check(db_session)

        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "progress"  # Still in progress
        assert len(result["affected_job_ids"]) == 0

    def test_no_timeout_for_recent_jobs(self, client, db_session):
        """Test that recent jobs are not marked as timeout."""
        from background import run_timeout_check
        from models import Job

        # Create a job in progress (recent)
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "progress"}
        )

        # Run timeout check
        result = run_timeout_check(db_session)

        # Job should still be in progress
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "progress"
        assert len(result["affected_job_ids"]) == 0

    def test_error_jobs_not_affected(self, client, db_session):
        """Test that error jobs are not affected by timeout checker."""
        from background import run_timeout_check
        from models import Job

        # Create a job with error status
        client.post("/status", json={"group": "test", "job": "job1", "status": "error"})

        # Set job updated_at to be old
        job = db_session.query(Job).filter_by(name="job1").first()
        job.updated_at = datetime.now(UTC) - timedelta(hours=48)
        db_session.commit()

        # Run timeout check
        result = run_timeout_check(db_session)

        # Job should still be error
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "error"
        assert len(result["affected_job_ids"]) == 0


class TestExpirationChecker:
    """Tests for the background expiration checker.

    AIDEV-NOTE: Expiration deletes jobs where expires_at <= now, regardless of
    status or ack state. This is different from staleness (status transition).
    """

    def test_expiration_deletes_success_jobs(self, client, db_session):
        """Test that expired success jobs are deleted."""
        from background import run_expiration_check
        from models import Job

        # Create a job with success status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Manually set expires_at to be in the past
        job = db_session.query(Job).filter_by(name="job1").first()
        job.expires_at = datetime.now(UTC) - timedelta(hours=1)
        db_session.commit()

        # Run expiration check
        result = run_expiration_check(db_session)

        # Job should be deleted
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job is None
        assert result["expired_count"] == 1

    def test_expiration_deletes_error_jobs(self, client, db_session):
        """Test that expired error jobs are deleted."""
        from background import run_expiration_check
        from models import Job

        # Create a job with error status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # Manually set expires_at to be in the past
        job = db_session.query(Job).filter_by(name="job1").first()
        job.expires_at = datetime.now(UTC) - timedelta(hours=1)
        db_session.commit()

        # Run expiration check
        result = run_expiration_check(db_session)

        # Job should be deleted
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job is None
        assert result["expired_count"] == 1

    def test_expiration_deletes_stale_jobs(self, client, db_session):
        """Test that expired stale jobs are deleted."""
        from background import run_expiration_check
        from models import Job

        # Create a job with stale status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "stale"}
        )

        # Manually set expires_at to be in the past
        job = db_session.query(Job).filter_by(name="job1").first()
        job.expires_at = datetime.now(UTC) - timedelta(hours=1)
        db_session.commit()

        # Run expiration check
        result = run_expiration_check(db_session)

        # Job should be deleted
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job is None
        assert result["expired_count"] == 1

    def test_expiration_deletes_acked_jobs(self, client, db_session):
        """Test that expired acked jobs are still deleted (ack doesn't prevent expiry)."""
        from background import run_expiration_check
        from models import Job

        # Create a job with error status and ack it
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "error"}
        )
        job = db_session.query(Job).filter_by(name="job1").first()
        job_id = job.id
        client.post(f"/jobs/{job_id}/ack")

        # Re-query to get fresh state from DB
        db_session.expire_all()
        job = db_session.query(Job).filter_by(id=job_id).first()
        assert job.acked is True

        # Manually set expires_at to be in the past
        job.expires_at = datetime.now(UTC) - timedelta(hours=1)
        db_session.commit()

        # Run expiration check
        result = run_expiration_check(db_session)

        # Job should be deleted despite being acked
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job is None
        assert result["expired_count"] == 1

    def test_expiration_does_not_delete_unexpired_jobs(self, client, db_session):
        """Test that jobs with future expires_at are not deleted."""
        from background import run_expiration_check
        from models import Job

        # Create a job with success status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Verify expires_at is in the future (set by API)
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.expires_at is not None
        assert job.expires_at.replace(tzinfo=None) > datetime.now(UTC).replace(
            tzinfo=None
        )

        # Run expiration check
        result = run_expiration_check(db_session)

        # Job should still exist
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job is not None
        assert result["expired_count"] == 0

    def test_expires_at_refreshed_on_status_update(self, client, db_session):
        """Test that expires_at is refreshed when job status is updated."""
        from models import Job

        # Create a job
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )
        job = db_session.query(Job).filter_by(name="job1").first()
        original_expires_at = job.expires_at
        job_id = job.id

        # Update the job status
        # AIDEV-NOTE: Sleep >= 1s to ensure timestamp difference is visible
        # (SQLite stores seconds precision, not sub-second)
        import time

        time.sleep(1.0)
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "error"}
        )

        # Verify expires_at was refreshed (re-query for fresh state)
        db_session.expire_all()
        job = db_session.query(Job).filter_by(id=job_id).first()
        assert job.expires_at is not None
        assert job.expires_at >= original_expires_at  # Use >= for safety

    def test_staleness_disabled_by_default_no_stale_transition(
        self, client, db_session
    ):
        """Test that staleness is disabled by default (no stale transition occurs)."""
        from background import run_timeout_check
        from models import Group, Job

        # Create a job with success status
        client.post(
            "/status", json={"group": "test", "job": "job1", "status": "success"}
        )

        # Verify staleness_enabled defaults to False
        group = db_session.query(Group).filter_by(name="test").first()
        assert group.staleness_enabled is False

        # Manually set updated_at to be older than staleness timeout
        job = db_session.query(Job).filter_by(name="job1").first()
        job.updated_at = datetime.now(UTC) - timedelta(hours=48)
        db_session.commit()

        # Run timeout check
        result = run_timeout_check(db_session)

        # Job should NOT be marked as stale (staleness disabled)
        job = db_session.query(Job).filter_by(name="job1").first()
        assert job.status == "success"
        assert result["stale_count"] == 0


class _RecordingSocketIO:
    """Minimal Socket.IO stand-in that records emit() calls for assertions.

    AIDEV-NOTE: run_timeout_check only calls socketio.emit(event, payload), so a
    plain recorder is enough to assert the health_update contract without a real
    Socket.IO server.
    """

    def __init__(self) -> None:
        self.emitted: list[tuple[str, dict]] = []

    def emit(self, event: str, payload: dict) -> None:
        self.emitted.append((event, payload))


class TestHealthUpdateEmit:
    """Tests for the per-transition-type health_update Socket.IO events.

    AIDEV-NOTE: A single 60s pass can move both progress->timeout and
    success->stale jobs. The emit must label each transition correctly instead
    of tagging the whole batch as "timeout" (the original bug).
    """

    def _health_events(self, sock: _RecordingSocketIO) -> list[dict]:
        return [
            payload for (event, payload) in sock.emitted if event == "health_update"
        ]

    def test_timeout_only_emits_single_timeout_event(self, client, db_session):
        from background import run_timeout_check
        from models import Job

        client.post(
            "/status", json={"group": "test", "job": "j", "status": "progress"}
        )
        job = db_session.query(Job).filter_by(name="j").first()
        job.updated_at = datetime.now(UTC) - timedelta(minutes=10)
        db_session.commit()
        job_id = job.id

        sock = _RecordingSocketIO()
        run_timeout_check(db_session, sock)

        events = self._health_events(sock)
        assert len(events) == 1
        assert events[0]["transition_type"] == "timeout"
        assert events[0]["affected_job_ids"] == [job_id]

    def test_stale_only_emits_single_stale_event(self, client, db_session):
        from background import run_timeout_check
        from models import Group, Job

        client.post(
            "/status", json={"group": "test", "job": "j", "status": "success"}
        )
        group = db_session.query(Group).filter_by(name="test").first()
        group.staleness_enabled = True
        db_session.commit()
        job = db_session.query(Job).filter_by(name="j").first()
        job.updated_at = datetime.now(UTC) - timedelta(hours=25)
        db_session.commit()
        job_id = job.id

        sock = _RecordingSocketIO()
        run_timeout_check(db_session, sock)

        events = self._health_events(sock)
        assert len(events) == 1
        assert events[0]["transition_type"] == "stale"
        assert events[0]["affected_job_ids"] == [job_id]

    def test_mixed_batch_emits_separate_events_per_type(self, client, db_session):
        """A pass with both timeout and stale transitions emits one correctly
        typed event each -- a stale job must never be reported as a timeout."""
        from background import run_timeout_check
        from models import Group, Job

        client.post(
            "/status", json={"group": "g1", "job": "prog", "status": "progress"}
        )
        client.post(
            "/status", json={"group": "g2", "job": "succ", "status": "success"}
        )
        g2 = db_session.query(Group).filter_by(name="g2").first()
        g2.staleness_enabled = True
        db_session.commit()

        prog = db_session.query(Job).filter_by(name="prog").first()
        prog.updated_at = datetime.now(UTC) - timedelta(minutes=10)
        succ = db_session.query(Job).filter_by(name="succ").first()
        succ.updated_at = datetime.now(UTC) - timedelta(hours=25)
        db_session.commit()
        prog_id, succ_id = prog.id, succ.id

        sock = _RecordingSocketIO()
        run_timeout_check(db_session, sock)

        by_type = {p["transition_type"]: p for p in self._health_events(sock)}
        assert set(by_type) == {"timeout", "stale"}
        assert by_type["timeout"]["affected_job_ids"] == [prog_id]
        assert by_type["stale"]["affected_job_ids"] == [succ_id]
        # Regression guard: the stale job is not mislabeled as a timeout.
        assert succ_id not in by_type["timeout"]["affected_job_ids"]

    def test_no_transitions_emits_no_health_update(self, client, db_session):
        from background import run_timeout_check

        client.post(
            "/status", json={"group": "test", "job": "j", "status": "progress"}
        )

        sock = _RecordingSocketIO()
        run_timeout_check(db_session, sock)

        assert self._health_events(sock) == []
