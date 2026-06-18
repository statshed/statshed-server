"""Background task for timeout checking and job expiration.

AIDEV-NOTE: This module handles automatic status transitions and cleanup:
- progress -> timeout (after progress_timeout_minutes)
- success -> stale (after staleness_timeout_hours, only if staleness_enabled)
- Job expiration: delete jobs where expires_at <= now() (all statuses)

The timeout checker runs every 60 seconds using Flask-SocketIO's
background task mechanism. Group-specific overrides take precedence
over global settings.

AIDEV-NOTE: Staleness is now opt-in (staleness_enabled must be True).
Expiration is the new default cleanup mechanism - jobs auto-delete after
expiration_timeout_hours regardless of status or ack state.
"""

from datetime import UTC, datetime, timedelta

from flask import Flask
from flask_socketio import SocketIO
from sqlalchemy.orm import scoped_session

from config import Config
from models import Group, Job, get_config_value


def run_timeout_check(
    db_session: scoped_session, socketio: SocketIO | None = None
) -> dict:
    """Check for and update timed-out jobs.

    AIDEV-NOTE: This function queries for jobs that have exceeded their
    timeout thresholds and updates their status. It uses COALESCE to
    apply group-specific overrides when available.

    Args:
        db_session: Database session for queries
        socketio: Optional Socket.IO instance for emitting events

    Returns:
        Dict with affected_job_ids, affected_group_ids, and counts
    """
    now = datetime.now(UTC)
    # AIDEV-NOTE: Track timeout and stale transitions separately so each gets its
    # own correctly-typed health_update event -- a single 60s pass can produce both.
    timeout_job_ids: list[int] = []
    timeout_group_ids: set[int] = set()
    stale_job_ids: list[int] = []
    stale_group_ids: set[int] = set()

    # Get global timeout values
    global_progress_timeout = get_config_value(
        "progress_timeout_minutes", Config.DEFAULT_PROGRESS_TIMEOUT_MINUTES
    )
    global_staleness_timeout = get_config_value(
        "staleness_timeout_hours", Config.DEFAULT_STALENESS_TIMEOUT_HOURS
    )

    # Check for progress timeouts
    # AIDEV-NOTE: Query joins with Group to access per-group overrides
    progress_jobs = (
        db_session.query(Job).join(Group).filter(Job.status == "progress").all()
    )

    for job in progress_jobs:
        # Use group override if available, otherwise global
        timeout_minutes = (
            job.group.progress_timeout_minutes
            if job.group.progress_timeout_minutes is not None
            else global_progress_timeout
        )
        cutoff = now - timedelta(minutes=timeout_minutes)

        # AIDEV-NOTE: SQLite returns naive datetimes. We assume UTC and add tzinfo.
        job_updated = job.updated_at.replace(tzinfo=UTC)
        if job_updated < cutoff:
            # AIDEV-NOTE: progress -> timeout is a new error, clear any ack state
            # (though progress jobs shouldn't be acked, this ensures consistency)
            job.acked = False
            job.acked_at = None
            job.status = "timeout"
            job.updated_at = now
            timeout_job_ids.append(job.id)
            timeout_group_ids.add(job.group_id)

    # Check for staleness timeouts (only for groups with staleness_enabled)
    # AIDEV-NOTE: Staleness is now opt-in. Only groups with staleness_enabled=True.
    success_jobs = (
        db_session.query(Job)
        .join(Group)
        .filter(Job.status == "success")
        .filter(Group.staleness_enabled == True)  # noqa: E712
        .all()
    )

    for job in success_jobs:
        # Use group override if available, otherwise global
        timeout_hours = (
            job.group.staleness_timeout_hours
            if job.group.staleness_timeout_hours is not None
            else global_staleness_timeout
        )
        cutoff = now - timedelta(hours=timeout_hours)

        # AIDEV-NOTE: SQLite returns naive datetimes. We assume UTC and add tzinfo.
        job_updated = job.updated_at.replace(tzinfo=UTC)
        if job_updated < cutoff:
            # AIDEV-NOTE: success -> stale is a new error, clear any ack state
            # (though success jobs shouldn't be acked, this ensures consistency)
            job.acked = False
            job.acked_at = None
            job.status = "stale"
            job.updated_at = now
            stale_job_ids.append(job.id)
            stale_group_ids.add(job.group_id)

    affected_jobs = timeout_job_ids + stale_job_ids
    affected_groups = timeout_group_ids | stale_group_ids

    # Commit changes if any jobs were affected
    if affected_jobs:
        db_session.commit()

        # AIDEV-NOTE: Emit one health_update per transition type that occurred, so
        # each carries the correct transition_type and only its own ids. The
        # frontend ignores the payload and just refetches, but the documented
        # contract and other consumers (CLI, raw socket clients) rely on accurate
        # labeling -- a stale job must never be reported as a "timeout".
        if socketio:
            timestamp = now.strftime("%Y-%m-%dT%H:%M:%SZ")
            if timeout_job_ids:
                socketio.emit(
                    "health_update",
                    {
                        "schema_version": 1,
                        "affected_job_ids": timeout_job_ids,
                        "affected_group_ids": list(timeout_group_ids),
                        "transition_type": "timeout",
                        "timestamp": timestamp,
                    },
                )
            if stale_job_ids:
                socketio.emit(
                    "health_update",
                    {
                        "schema_version": 1,
                        "affected_job_ids": stale_job_ids,
                        "affected_group_ids": list(stale_group_ids),
                        "transition_type": "stale",
                        "timestamp": timestamp,
                    },
                )

    return {
        "affected_job_ids": affected_jobs,
        "affected_group_ids": list(affected_groups),
        "timeout_count": len(timeout_job_ids),
        "stale_count": len(stale_job_ids),
    }


# AIDEV-NOTE: Batch size for expiration deletions to avoid long locks
EXPIRATION_BATCH_SIZE = 100


def run_expiration_check(
    db_session: scoped_session, socketio: SocketIO | None = None
) -> dict:
    """Delete expired jobs based on expires_at timestamp.

    AIDEV-NOTE: This function deletes jobs where expires_at <= now(), regardless
    of status or ack state. Jobs are deleted in batches to avoid long locks.
    Emits WebSocket events for each deleted job.

    Args:
        db_session: Database session for queries
        socketio: Optional Socket.IO instance for emitting events

    Returns:
        Dict with expired_job_ids, affected_group_ids, and expired_count
    """
    now = datetime.now(UTC)
    # AIDEV-NOTE: SQLite stores naive datetimes, so compare without timezone.
    # This code assumes SQLite; PostgreSQL support would need dialect-aware handling.
    now_naive = now.replace(tzinfo=None)

    expired_jobs: list[dict] = []
    affected_groups: set[int] = set()

    # Query expired jobs in batches
    # AIDEV-NOTE: Use contains_eager to avoid N+1 when accessing job.group
    from sqlalchemy.orm import contains_eager

    while True:
        jobs_to_expire = (
            db_session.query(Job)
            .join(Job.group)
            .options(contains_eager(Job.group))
            .filter(Job.expires_at <= now_naive)
            .filter(Job.expires_at.isnot(None))
            .limit(EXPIRATION_BATCH_SIZE)
            .all()
        )

        if not jobs_to_expire:
            break

        for job in jobs_to_expire:
            # Store job info for WebSocket event before deletion
            expired_jobs.append(
                {
                    "job_id": job.id,
                    "job_name": job.name,
                    "group_id": job.group_id,
                    "group_name": job.group.name,
                }
            )
            affected_groups.add(job.group_id)
            db_session.delete(job)

        db_session.commit()

    # Emit WebSocket events for expired jobs
    if socketio and expired_jobs:
        for job_info in expired_jobs:
            socketio.emit(
                "job_expired",
                {
                    "schema_version": 1,
                    "job_id": job_info["job_id"],
                    "job_name": job_info["job_name"],
                    "group_id": job_info["group_id"],
                    "group_name": job_info["group_name"],
                    "timestamp": now.strftime("%Y-%m-%dT%H:%M:%SZ"),
                },
            )

    return {
        "expired_job_ids": [j["job_id"] for j in expired_jobs],
        "affected_group_ids": list(affected_groups),
        "expired_count": len(expired_jobs),
    }


def start_timeout_checker(app: Flask, socketio: SocketIO, db_session: scoped_session):
    """Start the background timeout and expiration checker task.

    AIDEV-NOTE: Uses Flask-SocketIO's start_background_task for compatibility
    with the threading async mode. Runs every 60 seconds. Handles both:
    - Timeout transitions (progress -> timeout, success -> stale)
    - Expiration deletions (jobs where expires_at <= now)

    Args:
        app: Flask application instance
        socketio: Socket.IO instance
        db_session: Database session factory
    """

    def check_timeouts_and_expirations():
        """Background task that runs timeout and expiration checks."""
        while True:
            socketio.sleep(60)  # Wait 60 seconds between checks
            with app.app_context():
                try:
                    run_timeout_check(db_session, socketio)
                except Exception:
                    # Log with full traceback but don't crash the background task
                    app.logger.exception("Timeout checker error")

                try:
                    result = run_expiration_check(db_session, socketio)
                    if result["expired_count"] > 0:
                        app.logger.info(f"Expired {result['expired_count']} jobs")
                except Exception:
                    # Log with full traceback but don't crash the background task
                    app.logger.exception("Expiration checker error")

    socketio.start_background_task(check_timeouts_and_expirations)
