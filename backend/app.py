"""Flask application factory and routes.

AIDEV-NOTE: This is the main entry point for the StatShed backend.
Run with: python app.py or use Flask's development server.

AIDEV-NOTE: No authentication is implemented by design. This application is
intended for internal/localhost use only. See the design doc section
"Design Decision: No Authentication" for rationale. For external exposure,
use a reverse proxy with authentication.
"""

import os
import re
from collections import defaultdict
from datetime import UTC, datetime, timedelta

from flask import (
    Blueprint,
    Flask,
    Response,
    abort,
    jsonify,
    request,
    send_from_directory,
)
from flask_cors import CORS
from flask_socketio import SocketIO
from sqlalchemy import func
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import contains_eager, defer
from werkzeug.exceptions import HTTPException, NotFound

from background import start_timeout_checker
from config import Config
from models import (
    UNHEALTHY_STATUSES,
    VALID_STATUSES,
    Group,
    Job,
    db_session,
    get_config_value,
    init_db,
    set_config_value,
)

# Initialize Flask app
app = Flask(__name__)
app.config.from_object(Config)
app.config["MAX_CONTENT_LENGTH"] = Config.MAX_CONTENT_LENGTH

# Initialize CORS
CORS(app, origins=Config.CORS_ORIGINS)

# Initialize Socket.IO with gevent for WebSocket support
# AIDEV-NOTE: async_mode='gevent' enables true WebSocket connections (not just long-polling).
# Gevent uses greenlets (cooperative multitasking), which is safe with SQLite.
# The previous 'threading' mode caused 500 errors on WebSocket upgrade attempts.
socketio = SocketIO(
    app,
    async_mode="gevent",
    cors_allowed_origins=Config.CORS_ORIGINS,
    ping_interval=Config.PING_INTERVAL,
    ping_timeout=Config.PING_TIMEOUT,
    max_http_buffer_size=Config.MAX_HTTP_BUFFER_SIZE,
)

# AIDEV-NOTE: The REST API lives under an /api Blueprint so the unified image can
# serve the React SPA at root without route collisions (e.g. the SPA's /jobs page
# vs this API's GET /jobs). nginx used to strip /api; now Flask owns the prefix.
api = Blueprint("api", __name__)

# AIDEV-NOTE: Alembic is the single source of truth for the production schema
# (entrypoint.sh runs `alembic upgrade head` before launching). We deliberately do
# NOT call init_db()/create_all() at import time, so gunicorn workers never create
# tables out-of-band and mask missing migrations. create_all() is used only by the
# dev server (see __main__ below) and by the test suite (see tests/conftest.py).

# Start background timeout checker
# AIDEV-NOTE: This must run at module import time for gunicorn (the worker needs the
# background greenlet running; gunicorn imports `app:app` rather than running __main__).
start_timeout_checker(app, socketio, db_session)

# Name validation regex (alphanumeric, dash, underscore, dot)
NAME_PATTERN = re.compile(r"^[a-z0-9._-]+$")


def is_valid_int(value) -> bool:
    """Check if value is an integer but not a boolean.

    AIDEV-NOTE: In Python, bool is a subclass of int, so isinstance(True, int)
    returns True. This helper rejects booleans for config values.
    """
    return isinstance(value, int) and not isinstance(value, bool)


def validate_name(
    name: str, field_name: str, max_length: int
) -> tuple[str | None, str]:
    """Validate and normalize a name field.

    Args:
        name: The name to validate
        field_name: Name of the field for error messages
        max_length: Maximum allowed length

    Returns:
        Tuple of (error_message or None, normalized_name)
    """
    if not name:
        return f"{field_name} is required", ""

    # AIDEV-NOTE: Reject non-string JSON values (e.g. {"group": 123}) with a clean
    # 400 instead of letting .strip() raise AttributeError -> 500.
    if not isinstance(name, str):
        return f"{field_name} must be a string", ""

    normalized = name.strip().lower()

    if len(normalized) > max_length:
        return f"{field_name} exceeds maximum length of {max_length} characters", ""

    if not NAME_PATTERN.match(normalized):
        return (
            f"{field_name} contains invalid characters. "
            "Only alphanumeric, dash, underscore, and dot are allowed.",
            "",
        )

    return None, normalized


def compute_expires_at(group: Group, updated_at: datetime) -> datetime:
    """Compute the expires_at timestamp for a job.

    AIDEV-NOTE: Expiration is calculated as updated_at + expiration_timeout_hours.
    Uses group-specific override if set, otherwise falls back to global default.

    Args:
        group: The group the job belongs to
        updated_at: The job's updated_at timestamp

    Returns:
        The computed expires_at datetime
    """
    # Get expiration timeout (group override or global default)
    expiration_hours = group.expiration_timeout_hours
    if expiration_hours is None:
        expiration_hours = get_config_value(
            "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
        )
    return updated_at + timedelta(hours=expiration_hours)


def health_from_counts(
    job_count: int, unhealthy_count: int, in_progress_count: int
) -> str:
    """Derive the health enum from aggregate job counts.

    AIDEV-NOTE: Single source of truth for health precedence (highest to lowest):
    1. empty       - no jobs exist
    2. unhealthy   - at least one unacked error/timeout/stale job
    3. in_progress - at least one progress job
    4. healthy     - everything else (including all-unhealthy-but-acked)
    Counts come from SQL aggregates; unhealthy_count already excludes acked jobs.
    """
    if job_count == 0:
        return "empty"
    if unhealthy_count > 0:
        return "unhealthy"
    if in_progress_count > 0:
        return "in_progress"
    return "healthy"


def zero_status_counts() -> dict[str, int]:
    """Return a fresh {status: 0} seed covering every VALID_STATUSES key.

    AIDEV-NOTE: Single source of the zero-seed shared by /health, /groups, and
    /admin/stats so a status added to VALID_STATUSES shows up everywhere at once.
    """
    return {status: 0 for status in VALID_STATUSES}


def process_log_file(log_file) -> tuple[str | None, str | None, int, bool]:
    """Process an uploaded log file.

    AIDEV-NOTE: Reads log file content, validates encoding (UTF-8 with fallback
    to latin-1), and truncates to MAX_LOG_LINES if needed.

    Args:
        log_file: Flask FileStorage object from request.files

    Returns:
        Tuple of (error_message or None, log_content, line_count, truncated)
    """
    if log_file is None:
        return None, None, 0, False

    try:
        # Read log content
        content_bytes = log_file.read()

        # Try UTF-8 first, fall back to latin-1 (accepts any byte sequence)
        try:
            content = content_bytes.decode("utf-8")
        except UnicodeDecodeError:
            content = content_bytes.decode("latin-1")

        # Split into lines and count
        lines = content.splitlines(keepends=True)
        total_line_count = len(lines)

        # Truncate to last N lines if needed
        truncated = False
        if total_line_count > Config.MAX_LOG_LINES:
            lines = lines[-Config.MAX_LOG_LINES :]
            truncated = True
            content = "".join(lines)

        return None, content, len(lines), truncated

    except Exception as e:
        return f"Failed to read log file: {e}", None, 0, False


@app.teardown_appcontext
def shutdown_session(exception=None) -> None:
    """Remove database session at the end of each request."""
    db_session.remove()


# =============================================================================
# Error Handlers
# =============================================================================
# AIDEV-NOTE: Without these, Werkzeug returns HTML for 404/405/413/415/500 and for
# malformed JSON, breaking the API's JSON error contract (every endpoint documents
# an {"error", "message"} envelope). These handlers keep all error responses JSON.

ERROR_SLUGS: dict[int, str] = {
    400: "bad_request",
    404: "not_found",
    405: "method_not_allowed",
    413: "payload_too_large",
    415: "unsupported_media_type",
    500: "internal_server_error",
}


@app.errorhandler(HTTPException)
def handle_http_exception(error: HTTPException):
    """Return HTTP errors (404/405/413/415/...) as the JSON error envelope."""
    code = error.code or 500
    return (
        jsonify(
            {
                "error": ERROR_SLUGS.get(code, "http_error"),
                "message": error.description,
            }
        ),
        code,
    )


@app.errorhandler(Exception)
def handle_unexpected_exception(error: Exception):
    """Catch-all: roll back and return a JSON 500 instead of an HTML page.

    AIDEV-NOTE: Rolls back so a failed transaction does not leave the (per-greenlet)
    session in a pending-rollback state for any reuse before teardown.
    """
    db_session.rollback()
    app.logger.exception("Unhandled exception")
    return (
        jsonify(
            {
                "error": "internal_server_error",
                "message": "An internal server error occurred",
            }
        ),
        500,
    )


# =============================================================================
# Health Endpoint
# =============================================================================


@api.route("/health", methods=["GET"])
def get_health():
    """Get overall health summary across all jobs.

    AIDEV-NOTE: unhealthy count excludes acked jobs. acked count is new field.
    by_status contains raw counts (including acked jobs).

    Returns:
        JSON with status, total_jobs, healthy/unhealthy/acked/in_progress counts,
        and detailed by_status breakdown.
    """
    # AIDEV-NOTE: Count in SQL (GROUP BY status + filtered counts) instead of loading
    # every Job row into Python. by_status holds raw counts (including acked);
    # unhealthy excludes acked. The health enum is derived from these aggregate counts
    # via health_from_counts().
    status_counts = zero_status_counts()
    for status, count in (
        db_session.query(Job.status, func.count(Job.id)).group_by(Job.status).all()
    ):
        if status in status_counts:
            status_counts[status] = count

    total_jobs = db_session.query(func.count(Job.id)).scalar() or 0
    # Unhealthy count excludes acked jobs
    unhealthy_count = (
        db_session.query(func.count(Job.id))
        .filter(Job.status.in_(UNHEALTHY_STATUSES), Job.acked.is_(False))
        .scalar()
        or 0
    )
    # Count acked jobs separately
    acked_count = (
        db_session.query(func.count(Job.id)).filter(Job.acked.is_(True)).scalar() or 0
    )
    healthy_count = status_counts.get("success", 0)
    in_progress_count = status_counts.get("progress", 0)
    health = health_from_counts(total_jobs, unhealthy_count, in_progress_count)

    return jsonify(
        {
            "status": health,
            "total_jobs": total_jobs,
            "healthy": healthy_count,
            "unhealthy": unhealthy_count,
            "acked": acked_count,
            "in_progress": in_progress_count,
            "by_status": status_counts,
        }
    )


# =============================================================================
# Jobs Listing Endpoint
# =============================================================================


def _pagination_error(message: str, field: str):
    """Build a 400 validation_error response for a bad pagination param."""
    return (
        jsonify({"error": "validation_error", "message": message, "field": field}),
        400,
    )


class PaginationError(ValueError):
    """Raised when a limit/offset query param is invalid.

    Carries the offending field so the caller can build the JSON error envelope.
    """

    def __init__(self, message: str, field: str) -> None:
        super().__init__(message)
        self.message = message
        self.field = field


def parse_pagination(args) -> tuple[int | None, int]:
    """Parse opt-in limit/offset query params shared by the list endpoints.

    AIDEV-NOTE: Pagination is opt-in. Returns (limit, offset): limit is None when no
    limit was requested (return all); a requested limit is validated (positive int)
    and clamped to Config.MAX_JOBS_PAGE_SIZE. offset defaults to 0 and must be
    non-negative. Raises PaginationError on invalid input.
    """
    limit: int | None = None
    offset = 0
    limit_param = args.get("limit", "").strip()
    offset_param = args.get("offset", "").strip()

    if limit_param:
        try:
            limit = int(limit_param)
        except ValueError:
            raise PaginationError("limit must be an integer", "limit") from None
        if limit < 1:
            raise PaginationError("limit must be a positive integer", "limit")
        limit = min(limit, Config.MAX_JOBS_PAGE_SIZE)

    if offset_param:
        try:
            offset = int(offset_param)
        except ValueError:
            raise PaginationError("offset must be an integer", "offset") from None
        if offset < 0:
            raise PaginationError("offset must be a non-negative integer", "offset")

    return limit, offset


@api.route("/jobs", methods=["GET"])
def get_jobs():
    """Get jobs filtered by status, with optional pagination.

    AIDEV-NOTE: This endpoint supports filtering by one or more statuses.
    Used by the health card click-through feature to show jobs with specific statuses.

    AIDEV-NOTE: Pagination is opt-in and backward-compatible: with no limit/offset
    params the full matching result set is returned (total == number of jobs returned).
    When limit/offset are supplied, only that window is returned but `total` remains the
    full matching count. A requested limit is clamped to Config.MAX_JOBS_PAGE_SIZE.

    Query Parameters:
        status: Comma-separated list of statuses to filter by (optional)
                e.g., ?status=success or ?status=error,timeout
        limit:  Max number of jobs to return (optional, positive int, clamped to
                Config.MAX_JOBS_PAGE_SIZE). Omit to return all matching jobs.
        offset: Number of jobs to skip (optional, non-negative int, default 0).

    Returns:
        JSON with list of jobs matching the filter and the full matching total.
        Jobs are ordered by updated_at DESC (most recently updated first).
        400 if invalid status, limit, or offset provided.
    """
    status_param = request.args.get("status", "")

    # Parse comma-separated statuses
    if status_param:
        statuses = [s.strip().lower() for s in status_param.split(",") if s.strip()]
    else:
        statuses = []

    # Validate status values
    for status in statuses:
        if status not in VALID_STATUSES:
            valid_list = ", ".join(sorted(VALID_STATUSES))
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"Invalid status '{status}'. Must be one of: {valid_list}",
                        "field": "status",
                    }
                ),
                400,
            )

    # AIDEV-NOTE: Opt-in limit/offset pagination (see parse_pagination). With no params
    # the full result set is returned (backward compatible); `total` below is always
    # the full matching count.
    try:
        limit, offset = parse_pagination(request.args)
    except PaginationError as exc:
        return _pagination_error(exc.message, exc.field)

    # Build the row query with eager loading of group for group_name.
    # AIDEV-NOTE: Use contains_eager() to populate the relationship from the join,
    # avoiding N+1 queries when accessing job.group in to_dict(). defer(log_content)
    # keeps the big log blob out of the list query -- to_dict() reports has_log via
    # the SQL column_property instead, so no per-row lazy load occurs.
    query = (
        db_session.query(Job)
        .join(Job.group)
        .options(contains_eager(Job.group), defer(Job.log_content))
    )

    # Apply status filter if provided
    if statuses:
        query = query.filter(Job.status.in_(statuses))

    # Order by most recently updated, then apply the optional pagination window
    query = query.order_by(Job.updated_at.desc())
    if offset:
        query = query.offset(offset)
    if limit is not None:
        query = query.limit(limit)

    jobs = query.all()

    # AIDEV-NOTE: total is the full matching count, independent of the limit/offset
    # window. On the default (no-pagination) path total == len(jobs), so we skip the
    # extra COUNT aggregate; only a requested window needs the separate count.
    if limit is None and not offset:
        total = len(jobs)
    else:
        count_query = db_session.query(func.count(Job.id))
        if statuses:
            count_query = count_query.filter(Job.status.in_(statuses))
        total = count_query.scalar() or 0

    return jsonify(
        {
            "jobs": [job.to_dict() for job in jobs],
            "total": total,
        }
    )


# =============================================================================
# Job Acknowledgement Endpoint
# =============================================================================


@api.route("/jobs/<int:job_id>/ack", methods=["POST"])
def ack_job(job_id: int):
    """Acknowledge an error/timeout/stale job.

    AIDEV-NOTE: This endpoint is unauthenticated by design, following the same
    security model as the rest of the API (intended for localhost/intranet use).
    For external deployments, protect via reverse proxy with authentication.

    AIDEV-NOTE: Only jobs with unhealthy status (error, timeout, stale) can be acked.
    Acking an already-acked job is a no-op (returns success, does not update acked_at).

    Args:
        job_id: Job ID to acknowledge

    Returns:
        200: Job successfully acked (or already acked)
        404: Job not found
        400: Job is not in an acknowledgeable state (not unhealthy)
    """
    from sqlalchemy.orm import contains_eager

    job = (
        db_session.query(Job)
        .join(Job.group)
        .options(contains_eager(Job.group))
        .filter(Job.id == job_id)
        .first()
    )

    if not job:
        return (
            jsonify(
                {"error": "not_found", "message": f"Job with id {job_id} not found"}
            ),
            404,
        )

    # Only unhealthy jobs can be acked
    if job.status not in UNHEALTHY_STATUSES:
        return (
            jsonify(
                {
                    "error": "invalid_state",
                    "message": f"Cannot ack job with status '{job.status}'. "
                    f"Only error, timeout, or stale jobs can be acknowledged.",
                }
            ),
            400,
        )

    # Acking an already-acked job is a no-op
    if not job.acked:
        job.acked = True
        job.acked_at = datetime.now(UTC)
        db_session.commit()

        # Emit WebSocket event
        socketio.emit(
            "jobs_acked",
            {
                "schema_version": 1,
                "job_ids": [job.id],
                "group_id": job.group_id,
                "group_name": job.group.name,
                "acked_count": 1,
                "timestamp": job.acked_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        )

    return jsonify({"job": job.to_dict()})


# =============================================================================
# Job Deletion Endpoint
# =============================================================================


@api.route("/jobs/<int:job_id>", methods=["DELETE"])
def delete_job(job_id: int):
    """Delete a single job.

    AIDEV-NOTE: This endpoint is unauthenticated by design, following the same
    security model as the rest of the API (intended for localhost/intranet use).
    For external deployments, protect via reverse proxy with authentication.

    Args:
        job_id: Job ID to delete

    Returns:
        200: Job deleted with job data
        404: Job not found
    """
    job = (
        db_session.query(Job)
        .join(Job.group)
        .options(contains_eager(Job.group))
        .filter(Job.id == job_id)
        .first()
    )

    if not job:
        return (
            jsonify(
                {"error": "not_found", "message": f"Job with id {job_id} not found"}
            ),
            404,
        )

    # Store job data before deletion for response/event
    deleted_job_data = job.to_dict()
    group_id = job.group_id
    group_name = job.group.name

    # Delete the job
    db_session.delete(job)
    db_session.commit()

    # Emit WebSocket event
    socketio.emit(
        "job_deleted",
        {
            "schema_version": 1,
            "job_id": job_id,
            "job_name": deleted_job_data["name"],
            "group_id": group_id,
            "group_name": group_name,
            "timestamp": datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        },
    )

    return jsonify(
        {
            "deleted_job": deleted_job_data,
            "group_id": group_id,
            "group_name": group_name,
        }
    )


@api.route("/groups/<name>/ack", methods=["POST"])
def ack_group(name: str):
    """Acknowledge all errored/timed-out/stale jobs in a group.

    AIDEV-NOTE: This endpoint acks all jobs in the specified group that have
    unhealthy status (error, timeout, stale) and are not already acked.
    Follows the same security model as other endpoints (no auth, localhost/intranet use).

    Args:
        name: Group name (URL-encoded)

    Returns:
        200: Jobs successfully acked with count
        404: Group not found
    """
    # Normalize the group name
    normalized_name = name.strip().lower()

    group = db_session.query(Group).filter_by(name=normalized_name).first()
    if not group:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Group '{name}' not found",
                }
            ),
            404,
        )

    # Find all unacked unhealthy jobs in this group
    jobs_to_ack = (
        db_session.query(Job)
        .filter(
            Job.group_id == group.id,
            Job.status.in_(UNHEALTHY_STATUSES),
            Job.acked == False,  # noqa: E712 - SQLAlchemy requires == for comparison
        )
        .all()
    )

    acked_count = len(jobs_to_ack)
    now = datetime.now(UTC)
    job_ids = []

    for job in jobs_to_ack:
        job.acked = True
        job.acked_at = now
        job_ids.append(job.id)

    if acked_count > 0:
        db_session.commit()

        # Emit WebSocket event
        socketio.emit(
            "jobs_acked",
            {
                "schema_version": 1,
                "job_ids": job_ids,
                "group_id": group.id,
                "group_name": group.name,
                "acked_count": acked_count,
                "timestamp": now.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        )

    return jsonify(
        {
            "acked_count": acked_count,
            "group": group.name,
        }
    )


@api.route("/ack-all", methods=["POST"])
def ack_all():
    """Acknowledge all errored/timed-out/stale jobs globally.

    AIDEV-NOTE: This endpoint acks all jobs across all groups that have
    unhealthy status (error, timeout, stale) and are not already acked.
    Follows the same security model as other endpoints (no auth, localhost/intranet use).

    Returns:
        200: Jobs successfully acked with count
    """
    # Find all unacked unhealthy jobs
    jobs_to_ack = (
        db_session.query(Job)
        .filter(
            Job.status.in_(UNHEALTHY_STATUSES),
            Job.acked == False,  # noqa: E712 - SQLAlchemy requires == for comparison
        )
        .all()
    )

    acked_count = len(jobs_to_ack)
    now = datetime.now(UTC)
    job_ids = []

    for job in jobs_to_ack:
        job.acked = True
        job.acked_at = now
        job_ids.append(job.id)

    if acked_count > 0:
        db_session.commit()

        # Emit WebSocket event
        socketio.emit(
            "jobs_acked",
            {
                "schema_version": 1,
                "job_ids": job_ids,
                "group_id": None,
                "group_name": None,
                "acked_count": acked_count,
                "timestamp": now.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        )

    return jsonify({"acked_count": acked_count})


# =============================================================================
# Status Submission Endpoint
# =============================================================================


@api.route("/status", methods=["POST"])
def post_status():
    """Submit or update a job status.

    Creates the group and/or job if they don't exist.
    Accepts JSON or multipart/form-data (when including a log file).

    AIDEV-NOTE: When Content-Type is multipart/form-data, fields are extracted
    from request.form and optional log from request.files['log'].
    When Content-Type is application/json, fields are extracted from JSON body.

    Request Fields:
        group: Group name (required, max 255 chars)
        job: Job name (required, max 255 chars)
        status: One of success, error, progress, timeout, stale (required)
        message: Optional message (max 4096 chars)
        log: Optional log file (multipart only, text/plain)

    Returns:
        201: Job created/updated successfully
        400: Validation error
        413: Log file too large
    """
    # Parse request based on content type
    # AIDEV-NOTE: Support both JSON and multipart/form-data for CLI flexibility
    content_type = request.content_type or ""
    log_file = None
    log_ignored_warning = None

    if "multipart/form-data" in content_type:
        # Multipart request with potential log file
        data = {
            "group": request.form.get("group", ""),
            "job": request.form.get("job", ""),
            "status": request.form.get("status", ""),
            "message": request.form.get("message"),
        }
        log_file = request.files.get("log")

        # Check if log uploads are disabled
        if log_file and not Config.LOG_UPLOAD_ENABLED:
            log_ignored_warning = "Log uploads are disabled; log file was ignored"
            log_file = None
    else:
        # JSON request (no log file possible)
        # AIDEV-NOTE: silent=True returns None on malformed JSON or wrong content-type
        # so we emit the JSON envelope below instead of Werkzeug's HTML 400/415.
        data = request.get_json(silent=True)
        if not data or not isinstance(data, dict):
            return (
                jsonify({"error": "bad_request", "message": "JSON object required"}),
                400,
            )

    # Validate group name
    error, group_name = validate_name(
        data.get("group", ""), "group", Config.MAX_GROUP_NAME_LENGTH
    )
    if error:
        return (
            jsonify({"error": "validation_error", "message": error, "field": "group"}),
            400,
        )

    # Validate job name
    error, job_name = validate_name(
        data.get("job", ""), "job", Config.MAX_JOB_NAME_LENGTH
    )
    if error:
        return (
            jsonify({"error": "validation_error", "message": error, "field": "job"}),
            400,
        )

    # Validate status
    # AIDEV-NOTE: Guard non-string status (e.g. {"status": 123}) before .strip().
    status_raw = data.get("status")
    if status_raw is not None and not isinstance(status_raw, str):
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "status must be a string",
                    "field": "status",
                }
            ),
            400,
        )
    status = (status_raw or "").strip().lower()
    if not status:
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "status is required",
                    "field": "status",
                }
            ),
            400,
        )

    if status not in VALID_STATUSES:
        valid_list = ", ".join(sorted(VALID_STATUSES))
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": f"status must be one of: {valid_list}",
                    "field": "status",
                }
            ),
            400,
        )

    # Validate message (optional)
    message = data.get("message")
    if message is not None:
        if not isinstance(message, str):
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": "message must be a string",
                        "field": "message",
                    }
                ),
                400,
            )
        if len(message) > Config.MAX_MESSAGE_LENGTH:
            max_len = Config.MAX_MESSAGE_LENGTH
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"message exceeds maximum length of {max_len}",
                        "field": "message",
                    }
                ),
                400,
            )

    # Process log file if present
    log_content = None
    log_line_count = 0
    log_truncated = False

    if log_file:
        error, log_content, log_line_count, log_truncated = process_log_file(log_file)
        if error:
            return (
                jsonify(
                    {"error": "validation_error", "message": error, "field": "log"}
                ),
                400,
            )

    # Get or create group with race condition handling
    # AIDEV-NOTE: Handle IntegrityError for concurrent group creation
    group = db_session.query(Group).filter_by(name=group_name).first()
    group_created = False

    if not group:
        try:
            group = Group(name=group_name)
            db_session.add(group)
            db_session.flush()  # Get the group ID
            group_created = True
        except IntegrityError:
            # Another request created the group concurrently, retry query
            db_session.rollback()
            group = db_session.query(Group).filter_by(name=group_name).first()

    # Get or create job with race condition handling
    job = db_session.query(Job).filter_by(group_id=group.id, name=job_name).first()
    previous_status = None

    now = datetime.now(UTC)
    expires_at = compute_expires_at(group, now)

    if job:
        previous_status = job.status
        job.status = status
        job.message = message
        job.updated_at = now
        # AIDEV-NOTE: Refresh expires_at on each status update (extends expiration)
        job.expires_at = expires_at
        # AIDEV-NOTE: Clear ack on recovery (success/progress)
        # When a job transitions to a healthy state, clear the ack so that
        # future errors require a new acknowledgement
        if status in ("success", "progress"):
            job.acked = False
            job.acked_at = None
        # AIDEV-NOTE: Update log data (replaces previous log)
        # Log is cleared if no new log provided (log_content=None clears old log)
        if log_file is not None:
            job.log_content = log_content
            job.log_line_count = log_line_count
            job.log_truncated = log_truncated
            job.log_updated_at = now
    else:
        try:
            job = Job(
                group_id=group.id,
                name=job_name,
                status=status,
                message=message,
                updated_at=now,
                created_at=now,
                expires_at=expires_at,
                log_content=log_content,
                log_line_count=log_line_count if log_content else None,
                log_truncated=log_truncated,
                log_updated_at=now if log_content else None,
            )
            db_session.add(job)
            db_session.flush()
        except IntegrityError:
            # Another request created the job concurrently, retry query and update
            db_session.rollback()
            job = (
                db_session.query(Job)
                .filter_by(group_id=group.id, name=job_name)
                .first()
            )
            if job:
                previous_status = job.status
                job.status = status
                job.message = message
                job.updated_at = now
                # AIDEV-NOTE: Refresh expires_at on retry path too
                job.expires_at = expires_at
                # AIDEV-NOTE: Clear ack on recovery (same logic as above)
                if status in ("success", "progress"):
                    job.acked = False
                    job.acked_at = None
                # AIDEV-NOTE: Update log on retry path too
                if log_file is not None:
                    job.log_content = log_content
                    job.log_line_count = log_line_count
                    job.log_truncated = log_truncated
                    job.log_updated_at = now

    db_session.commit()

    # Emit WebSocket events
    if group_created:
        socketio.emit("group_created", {"schema_version": 1, "group": group.to_dict()})

    socketio.emit(
        "status_update",
        {
            "schema_version": 1,
            "job": job.to_dict(),
            "group_id": group.id,
            "group_name": group.name,
            "previous_status": previous_status,
        },
    )

    # Build response with optional warning
    response_data = {"success": True, "job": job.to_dict()}
    if log_ignored_warning:
        response_data["warning"] = log_ignored_warning

    return jsonify(response_data), 201


# =============================================================================
# Groups Endpoints
# =============================================================================


@api.route("/groups", methods=["GET"])
def get_groups():
    """List all groups with health summary.

    AIDEV-NOTE: unhealthy_count excludes acked jobs for consistency with /health.
    acked_count shows number of acked jobs in the group.

    Returns:
        JSON with list of groups including job_count, health, unhealthy_count,
        acked_count, and status_counts.
    """
    groups = db_session.query(Group).all()

    # AIDEV-NOTE: Aggregate per-group counts in SQL with three grouped queries instead
    # of the previous 1+N pattern (a lazy load of group.jobs per group). Empty groups
    # are preserved because we start from all groups and default missing aggregates to
    # zero -- never inner-join jobs, which would drop zero-job groups.
    status_rows = (
        db_session.query(Job.group_id, Job.status, func.count(Job.id))
        .group_by(Job.group_id, Job.status)
        .all()
    )
    unhealthy_rows = (
        db_session.query(Job.group_id, func.count(Job.id))
        .filter(Job.status.in_(UNHEALTHY_STATUSES), Job.acked.is_(False))
        .group_by(Job.group_id)
        .all()
    )
    acked_rows = (
        db_session.query(Job.group_id, func.count(Job.id))
        .filter(Job.acked.is_(True))
        .group_by(Job.group_id)
        .all()
    )

    # status_counts_by_group maps group_id -> {status: count}.
    # AIDEV-NOTE: defaultdict so the zero-seed is built once per group on first
    # access -- setdefault would allocate a throwaway seed dict on every status row.
    status_counts_by_group = defaultdict(zero_status_counts)
    for group_id, status, count in status_rows:
        if status in VALID_STATUSES:
            status_counts_by_group[group_id][status] = count
    unhealthy_by_group = {group_id: count for group_id, count in unhealthy_rows}
    acked_by_group = {group_id: count for group_id, count in acked_rows}

    result = []
    for group in groups:
        status_counts = status_counts_by_group.get(group.id) or zero_status_counts()
        job_count = sum(status_counts.values())
        unhealthy_count = unhealthy_by_group.get(group.id, 0)
        in_progress_count = status_counts.get("progress", 0)
        health = health_from_counts(job_count, unhealthy_count, in_progress_count)

        group_data = group.to_dict()
        group_data["job_count"] = job_count
        group_data["health"] = health
        group_data["unhealthy_count"] = unhealthy_count
        group_data["acked_count"] = acked_by_group.get(group.id, 0)
        group_data["status_counts"] = status_counts
        result.append(group_data)

    return jsonify({"groups": result})


@api.route("/groups/<name>/jobs", methods=["GET"])
def get_group_jobs(name: str):
    """Get jobs in a specific group, with optional pagination.

    AIDEV-NOTE: Pagination is opt-in and backward-compatible (see parse_pagination):
    with no limit/offset the full result set is returned. `total` is always the full
    job count for the group, independent of the returned page.

    Args:
        name: Group name (URL-encoded)

    Query Parameters:
        limit:  Max number of jobs to return (positive int, clamped to
                Config.MAX_JOBS_PAGE_SIZE). Omit to return all.
        offset: Number of jobs to skip (non-negative int, default 0).

    Returns:
        200: Group details, list of jobs, and the full matching total
        400: Invalid limit or offset
        404: Group not found
    """
    # Normalize the group name
    normalized_name = name.strip().lower()

    group = db_session.query(Group).filter_by(name=normalized_name).first()
    if not group:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Group '{name}' not found",
                }
            ),
            404,
        )

    try:
        limit, offset = parse_pagination(request.args)
    except PaginationError as exc:
        return _pagination_error(exc.message, exc.field)

    # AIDEV-NOTE: Query the jobs with defer(log_content) so the big log blob stays out
    # of the list response; contains_eager keeps group_name available in to_dict()
    # without a per-job lazy load. Ordered by updated_at desc for stable pagination.
    query = (
        db_session.query(Job)
        .join(Job.group)
        .options(contains_eager(Job.group), defer(Job.log_content))
        .filter(Job.group_id == group.id)
        .order_by(Job.updated_at.desc())
    )
    if offset:
        query = query.offset(offset)
    if limit is not None:
        query = query.limit(limit)

    rows = query.all()
    jobs = [job.to_dict() for job in rows]

    # AIDEV-NOTE: total is the group's full job count, independent of the page window.
    # On the default (no-pagination) path total == len(rows), so we skip the extra
    # COUNT aggregate; only a requested window needs the separate count.
    if limit is None and not offset:
        total = len(rows)
    else:
        total = (
            db_session.query(func.count(Job.id))
            .filter(Job.group_id == group.id)
            .scalar()
            or 0
        )

    return jsonify({"group": group.to_dict(), "jobs": jobs, "total": total})


@api.route("/groups/<group_name>/jobs/<job_name>/log", methods=["GET"])
def get_job_log(group_name: str, job_name: str):
    """Retrieve log content for a specific job.

    AIDEV-NOTE: Returns the last N lines by default (tail parameter).
    Use all=true to retrieve the full log. Log content is stored in the
    database and expires with the job.

    Args:
        group_name: Group name (URL-encoded)
        job_name: Job name (URL-encoded)

    Query Parameters:
        tail: Number of lines from the end (default: 1000)
        all: If "true", return the full log (ignores tail)

    Returns:
        200: Log content with metadata
        404: Job or log not found
    """
    # Normalize names
    normalized_group = group_name.strip().lower()
    normalized_job = job_name.strip().lower()

    # Find the group
    group = db_session.query(Group).filter_by(name=normalized_group).first()
    if not group:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Group '{group_name}' not found",
                }
            ),
            404,
        )

    # Find the job
    job = (
        db_session.query(Job).filter_by(group_id=group.id, name=normalized_job).first()
    )
    if not job:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Job '{job_name}' not found in group '{group_name}'",
                }
            ),
            404,
        )

    # Check if log exists
    if job.log_content is None:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"No log available for job '{job_name}'",
                }
            ),
            404,
        )

    # Parse query parameters
    return_all = request.args.get("all", "false").lower() == "true"
    tail = request.args.get("tail", "1000")

    try:
        tail_lines = int(tail)
        if tail_lines < 1:
            tail_lines = 1000
    except ValueError:
        tail_lines = 1000

    # Get log content
    log_content = job.log_content
    lines = log_content.splitlines(keepends=True)
    total_lines = len(lines)

    # Apply tail truncation unless all=true
    truncated = False
    if not return_all and total_lines > tail_lines:
        lines = lines[-tail_lines:]
        log_content = "".join(lines)
        truncated = True

    return jsonify(
        {
            "log": log_content,
            "line_count": len(lines),
            "truncated": truncated,
            "total_line_count": total_lines,
        }
    )


# =============================================================================
# Configuration Endpoints
# =============================================================================


@api.route("/config", methods=["GET"])
def get_config():
    """Get global configuration settings.

    Returns:
        JSON with progress_timeout_minutes, staleness_timeout_hours,
        and expiration_timeout_hours.
    """
    progress_timeout = get_config_value(
        "progress_timeout_minutes", Config.DEFAULT_PROGRESS_TIMEOUT_MINUTES
    )
    staleness_timeout = get_config_value(
        "staleness_timeout_hours", Config.DEFAULT_STALENESS_TIMEOUT_HOURS
    )
    expiration_timeout = get_config_value(
        "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
    )

    return jsonify(
        {
            "progress_timeout_minutes": progress_timeout,
            "staleness_timeout_hours": staleness_timeout,
            "expiration_timeout_hours": expiration_timeout,
        }
    )


@api.route("/config", methods=["PUT"])
def update_config():
    """Update global configuration settings.

    Request Body:
        progress_timeout_minutes: int (optional)
        staleness_timeout_hours: int (optional)
        expiration_timeout_hours: int (optional)

    Returns:
        200: Updated config values
        400: Validation error (out of range)
    """
    data = request.get_json(silent=True)
    if not data or not isinstance(data, dict):
        return jsonify({"error": "bad_request", "message": "JSON object required"}), 400

    # Validate and update progress_timeout_minutes
    if "progress_timeout_minutes" in data:
        value = data["progress_timeout_minutes"]
        min_val = Config.MIN_PROGRESS_TIMEOUT_MINUTES
        max_val = Config.MAX_PROGRESS_TIMEOUT_MINUTES
        if not is_valid_int(value) or value < min_val or value > max_val:
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"progress_timeout_minutes must be "
                        f"between {min_val} and {max_val}",
                        "field": "progress_timeout_minutes",
                    }
                ),
                400,
            )
        set_config_value("progress_timeout_minutes", value)

    # Validate and update staleness_timeout_hours
    if "staleness_timeout_hours" in data:
        value = data["staleness_timeout_hours"]
        min_val = Config.MIN_STALENESS_TIMEOUT_HOURS
        max_val = Config.MAX_STALENESS_TIMEOUT_HOURS
        if not is_valid_int(value) or value < min_val or value > max_val:
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"staleness_timeout_hours must be "
                        f"between {min_val} and {max_val}",
                        "field": "staleness_timeout_hours",
                    }
                ),
                400,
            )
        set_config_value("staleness_timeout_hours", value)

    # AIDEV-NOTE: Track a global expiration change so it can cascade to existing
    # jobs in groups that have no override (mirrors update_group_config's refresh).
    expiration_changed = False
    new_expiration_hours = None

    # Validate and update expiration_timeout_hours
    if "expiration_timeout_hours" in data:
        value = data["expiration_timeout_hours"]
        min_val = Config.MIN_EXPIRATION_TIMEOUT_HOURS
        max_val = Config.MAX_EXPIRATION_TIMEOUT_HOURS
        if not is_valid_int(value) or value < min_val or value > max_val:
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"expiration_timeout_hours must be "
                        f"between {min_val} and {max_val}",
                        "field": "expiration_timeout_hours",
                    }
                ),
                400,
            )
        old_value = get_config_value(
            "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
        )
        if value != old_value:
            expiration_changed = True
            new_expiration_hours = value
        set_config_value("expiration_timeout_hours", value)

    # Cascade a changed global expiration to jobs in groups without an override.
    if expiration_changed:
        # AIDEV-NOTE: Bulk UPDATE mirrors update_group_config's refresh. Only groups
        # with expiration_timeout_hours IS NULL follow the global default, so override
        # groups keep their own expires_at. SQLite-only (datetime()); the value is
        # int-validated above, so the interpolation is safe.
        from sqlalchemy import text

        sql = text(
            f"UPDATE jobs SET expires_at = "
            f"datetime(updated_at, '+{new_expiration_hours} hours') "
            f"WHERE group_id IN "
            f"(SELECT id FROM groups WHERE expiration_timeout_hours IS NULL)"
        )
        db_session.execute(sql)
        db_session.commit()

    # Return current config
    return get_config()


@api.route("/groups/<name>/config", methods=["GET"])
def get_group_config(name: str):
    """Get group-specific configuration overrides.

    Args:
        name: Group name (URL-encoded)

    Returns:
        200: Group config with effective values
        404: Group not found
    """
    normalized_name = name.strip().lower()

    group = db_session.query(Group).filter_by(name=normalized_name).first()
    if not group:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Group '{name}' not found",
                }
            ),
            404,
        )

    # Get global defaults
    global_progress = get_config_value(
        "progress_timeout_minutes", Config.DEFAULT_PROGRESS_TIMEOUT_MINUTES
    )
    global_staleness = get_config_value(
        "staleness_timeout_hours", Config.DEFAULT_STALENESS_TIMEOUT_HOURS
    )
    global_expiration = get_config_value(
        "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
    )

    # Calculate effective values
    effective_progress = (
        group.progress_timeout_minutes
        if group.progress_timeout_minutes is not None
        else global_progress
    )
    effective_staleness = (
        group.staleness_timeout_hours
        if group.staleness_timeout_hours is not None
        else global_staleness
    )
    effective_expiration = (
        group.expiration_timeout_hours
        if group.expiration_timeout_hours is not None
        else global_expiration
    )

    return jsonify(
        {
            # AIDEV-NOTE: "group" kept for backwards compatibility, prefer "group_name"
            "group": group.name,
            "group_name": group.name,
            "progress_timeout_minutes": group.progress_timeout_minutes,
            "staleness_enabled": group.staleness_enabled,
            "staleness_timeout_hours": group.staleness_timeout_hours,
            "expiration_timeout_hours": group.expiration_timeout_hours,
            "effective_progress_timeout_minutes": effective_progress,
            "effective_staleness_timeout_hours": effective_staleness,
            "effective_expiration_timeout_hours": effective_expiration,
        }
    )


@api.route("/groups/<name>/config", methods=["PUT"])
def update_group_config(name: str):
    """Update group-specific configuration overrides.

    AIDEV-NOTE: When expiration_timeout_hours changes, all existing jobs in the
    group have their expires_at refreshed to (updated_at + new_expiration_hours).

    Args:
        name: Group name (URL-encoded)

    Request Body:
        progress_timeout_minutes: int or null (optional)
        staleness_enabled: bool (optional)
        staleness_timeout_hours: int or null (optional, only valid if enabled)
        expiration_timeout_hours: int or null (optional)

    Returns:
        200: Updated group config with effective values
        404: Group not found
        400: Validation error
    """
    normalized_name = name.strip().lower()

    group = db_session.query(Group).filter_by(name=normalized_name).first()
    if not group:
        return (
            jsonify(
                {
                    "error": "not_found",
                    "message": f"Group '{name}' not found",
                }
            ),
            404,
        )

    data = request.get_json(silent=True)
    if not data or not isinstance(data, dict):
        return jsonify({"error": "bad_request", "message": "JSON object required"}), 400

    # Track if expiration changed (need to refresh job expires_at)
    expiration_changed = False

    # Validate and update progress_timeout_minutes
    if "progress_timeout_minutes" in data:
        value = data["progress_timeout_minutes"]
        if value is not None:
            min_val = Config.MIN_PROGRESS_TIMEOUT_MINUTES
            max_val = Config.MAX_PROGRESS_TIMEOUT_MINUTES
            if not is_valid_int(value) or value < min_val or value > max_val:
                return (
                    jsonify(
                        {
                            "error": "validation_error",
                            "message": f"progress_timeout_minutes must be "
                            f"between {min_val} and {max_val}",
                            "field": "progress_timeout_minutes",
                        }
                    ),
                    400,
                )
        group.progress_timeout_minutes = value

    # Validate and update staleness_enabled
    if "staleness_enabled" in data:
        value = data["staleness_enabled"]
        if not isinstance(value, bool):
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": "staleness_enabled must be a boolean",
                        "field": "staleness_enabled",
                    }
                ),
                400,
            )
        group.staleness_enabled = value

    # Validate and update staleness_timeout_hours
    if "staleness_timeout_hours" in data:
        value = data["staleness_timeout_hours"]
        if value is not None:
            min_val = Config.MIN_STALENESS_TIMEOUT_HOURS
            max_val = Config.MAX_STALENESS_TIMEOUT_HOURS
            if not is_valid_int(value) or value < min_val or value > max_val:
                return (
                    jsonify(
                        {
                            "error": "validation_error",
                            "message": f"staleness_timeout_hours must be "
                            f"between {min_val} and {max_val}",
                            "field": "staleness_timeout_hours",
                        }
                    ),
                    400,
                )
        group.staleness_timeout_hours = value

    # Validate and update expiration_timeout_hours
    if "expiration_timeout_hours" in data:
        value = data["expiration_timeout_hours"]
        if value is not None:
            min_val = Config.MIN_EXPIRATION_TIMEOUT_HOURS
            max_val = Config.MAX_EXPIRATION_TIMEOUT_HOURS
            if not is_valid_int(value) or value < min_val or value > max_val:
                return (
                    jsonify(
                        {
                            "error": "validation_error",
                            "message": f"expiration_timeout_hours must be "
                            f"between {min_val} and {max_val}",
                            "field": "expiration_timeout_hours",
                        }
                    ),
                    400,
                )
        if group.expiration_timeout_hours != value:
            expiration_changed = True
        group.expiration_timeout_hours = value

    # Validate: staleness_timeout must be < expiration_timeout when both set
    # AIDEV-NOTE: Get effective values for comparison
    effective_staleness = group.staleness_timeout_hours
    if effective_staleness is None:
        effective_staleness = get_config_value(
            "staleness_timeout_hours", Config.DEFAULT_STALENESS_TIMEOUT_HOURS
        )
    effective_expiration = group.expiration_timeout_hours
    if effective_expiration is None:
        effective_expiration = get_config_value(
            "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
        )

    if group.staleness_enabled and effective_staleness >= effective_expiration:
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "staleness_timeout_hours must be less than "
                    "expiration_timeout_hours",
                    "fields": {
                        "staleness_timeout_hours": "must be less than "
                        "expiration_timeout_hours"
                    },
                }
            ),
            400,
        )

    db_session.commit()

    # Refresh expires_at for all jobs in this group if expiration changed
    if expiration_changed:
        # AIDEV-NOTE: Use raw SQL bulk UPDATE for efficiency with large datasets.
        # SQLite-only: uses datetime() function. PostgreSQL not tested.
        from sqlalchemy import text

        expiration_hours = group.expiration_timeout_hours
        if expiration_hours is None:
            expiration_hours = get_config_value(
                "expiration_timeout_hours", Config.DEFAULT_EXPIRATION_TIMEOUT_HOURS
            )
        sql = text(
            f"UPDATE jobs SET expires_at = datetime(updated_at, '+{expiration_hours} hours') "
            f"WHERE group_id = :group_id"
        )
        db_session.execute(sql, {"group_id": group.id})
        db_session.commit()

    return get_group_config(name)


# =============================================================================
# Admin Endpoints (Data Retention & Cleanup)
# =============================================================================


@api.route("/admin/stats", methods=["GET"])
def get_admin_stats():
    """Get database statistics.

    AIDEV-NOTE: This endpoint is unauthenticated by design, following the
    same security model as the rest of the API (intended for localhost/intranet
    use). For external deployments, protect via reverse proxy.

    Returns:
        JSON with job counts by status, group count, and database info.
    """
    from sqlalchemy import text

    # Use SQL aggregate queries to avoid loading all records into memory
    # AIDEV-NOTE: This prevents DoS on large datasets by counting in the database
    total_jobs = db_session.query(func.count(Job.id)).scalar() or 0
    total_groups = db_session.query(func.count(Group.id)).scalar() or 0

    # Get job counts by status using GROUP BY
    status_query = (
        db_session.query(Job.status, func.count(Job.id)).group_by(Job.status).all()
    )
    status_counts = zero_status_counts()
    for status, count in status_query:
        if status in status_counts:
            status_counts[status] = count

    # Get database size for SQLite
    db_size = None
    if "sqlite" in Config.DATABASE_URL:
        try:
            size_result = db_session.execute(
                text(
                    "SELECT page_count * page_size as size "
                    "FROM pragma_page_count(), pragma_page_size()"
                )
            ).fetchone()
            if size_result:
                db_size = size_result[0]
        except Exception:
            # If we can't get the size, just skip it
            pass

    return jsonify(
        {
            "total_jobs": total_jobs,
            "total_groups": total_groups,
            "jobs_by_status": status_counts,
            "database_size_bytes": db_size,
        }
    )


@api.route("/admin/cleanup", methods=["DELETE"])
def admin_cleanup():
    """Trigger manual cleanup of old jobs.

    AIDEV-NOTE: This endpoint is unauthenticated by design, following the
    same security model as the rest of the API (intended for localhost/intranet
    use). For external deployments, protect via reverse proxy.

    AIDEV-NOTE: This endpoint deletes jobs matching the criteria and
    removes any groups that become empty as a result.

    Request Body:
        older_than_days: int (required) - Delete jobs older than this many days
        statuses: list[str] (optional) - Only delete jobs with these statuses
            (defaults to ["stale", "timeout"])
        dry_run: bool (optional) - If true, return counts without deleting

    Returns:
        200: Counts of deleted jobs and groups
        400: Validation error
    """
    from sqlalchemy import func, select

    data = request.get_json(silent=True)
    if data is None or not isinstance(data, dict):
        return jsonify({"error": "bad_request", "message": "JSON object required"}), 400

    # Validate older_than_days (required)
    older_than_days = data.get("older_than_days")
    if older_than_days is None:
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "older_than_days is required",
                    "field": "older_than_days",
                }
            ),
            400,
        )
    if not is_valid_int(older_than_days) or older_than_days < 1:
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "older_than_days must be a positive integer",
                    "field": "older_than_days",
                }
            ),
            400,
        )

    # Validate statuses (optional, defaults to stale and timeout)
    statuses = data.get("statuses", ["stale", "timeout"])
    if not isinstance(statuses, list):
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "statuses must be an array",
                    "field": "statuses",
                }
            ),
            400,
        )
    for status in statuses:
        if status not in VALID_STATUSES:
            valid_list = ", ".join(sorted(VALID_STATUSES))
            return (
                jsonify(
                    {
                        "error": "validation_error",
                        "message": f"Invalid status '{status}'. "
                        f"Must be one of: {valid_list}",
                        "field": "statuses",
                    }
                ),
                400,
            )

    # Validate dry_run (optional, defaults to false)
    dry_run = data.get("dry_run", False)
    if not isinstance(dry_run, bool):
        return (
            jsonify(
                {
                    "error": "validation_error",
                    "message": "dry_run must be a boolean",
                    "field": "dry_run",
                }
            ),
            400,
        )

    # Calculate cutoff date
    # AIDEV-NOTE: SQLite stores naive datetimes, so we compare without timezone
    cutoff = datetime.now(UTC) - timedelta(days=older_than_days)
    cutoff_naive = cutoff.replace(tzinfo=None)

    # Build the query for jobs to delete (used for both counting and deletion)
    jobs_query = (
        db_session.query(Job)
        .filter(Job.status.in_(statuses))
        .filter(Job.updated_at < cutoff_naive)
    )

    # Count jobs to delete using SQL aggregate
    deleted_job_count = jobs_query.count()

    # Get count of jobs to delete per group using SQL aggregation
    # AIDEV-NOTE: This avoids O(n*m) complexity by doing the counting in SQL
    jobs_to_delete_per_group = (
        db_session.query(Job.group_id, func.count(Job.id).label("delete_count"))
        .filter(Job.status.in_(statuses))
        .filter(Job.updated_at < cutoff_naive)
        .group_by(Job.group_id)
        .all()
    )

    # Get total job counts per affected group
    affected_group_ids = [row[0] for row in jobs_to_delete_per_group]
    delete_counts = {row[0]: row[1] for row in jobs_to_delete_per_group}

    # Count groups that would become empty
    deleted_group_count = 0
    if affected_group_ids:
        total_jobs_per_group = (
            db_session.query(Job.group_id, func.count(Job.id).label("total"))
            .filter(Job.group_id.in_(affected_group_ids))
            .group_by(Job.group_id)
            .all()
        )
        for group_id, total in total_jobs_per_group:
            remaining = total - delete_counts.get(group_id, 0)
            if remaining == 0:
                deleted_group_count += 1

    if not dry_run:
        # Delete the jobs using bulk delete
        # AIDEV-NOTE: synchronize_session="fetch" ensures the session is updated
        jobs_query.delete(synchronize_session="fetch")
        db_session.flush()

        # Delete empty groups using a safe subquery pattern
        # AIDEV-NOTE: This avoids race conditions by checking emptiness at delete time
        # rather than relying on pre-computed counts
        if affected_group_ids:
            # Find groups that have no jobs (safe re-check after job deletion)
            empty_groups_subquery = (
                select(Group.id)
                .where(Group.id.in_(affected_group_ids))
                .where(~Group.id.in_(select(Job.group_id).distinct()))
            )
            db_session.query(Group).filter(Group.id.in_(empty_groups_subquery)).delete(
                synchronize_session="fetch"
            )

        db_session.commit()

    return jsonify(
        {
            "deleted_jobs": deleted_job_count,
            "deleted_groups": deleted_group_count,
            "dry_run": dry_run,
        }
    )


# =============================================================================
# Main Entry Point
# =============================================================================


# AIDEV-NOTE: In the unified image Flask serves the built React SPA from
# STATIC_DIR (the Vite dist/, copied to ./static by the Dockerfile). Registered
# only when the dir exists, so local dev (`python app.py`, Vite serves the SPA on
# its own port) is unaffected. /api/* and /socket.io are never served as the SPA.
def register_spa(app: Flask, static_dir: str) -> None:
    """Serve the SPA from static_dir with history-API (index.html) fallback."""

    @app.route("/", defaults={"path": ""})
    @app.route("/<path:path>")
    def serve_spa(path: str) -> Response:
        if path.startswith("api/"):
            abort(404)  # unknown API path -> JSON 404 via errorhandler, not SPA
        if path:
            try:
                resp = send_from_directory(static_dir, path)
                resp.headers["Cache-Control"] = "public, immutable, max-age=31536000"
                return resp
            except NotFound:
                pass  # client-side route -> fall through to index.html
        resp = send_from_directory(static_dir, "index.html")
        resp.headers["Cache-Control"] = "no-cache"
        return resp


_STATIC_DIR: str = os.environ.get("STATIC_DIR") or os.path.join(
    os.path.dirname(__file__), "static"
)
if os.path.isdir(_STATIC_DIR):
    register_spa(app, _STATIC_DIR)

# AIDEV-NOTE: Register the API under /api. Must come after all @api.route defs.
app.register_blueprint(api, url_prefix="/api")

if __name__ == "__main__":
    # AIDEV-NOTE: Dev-server convenience only — create tables from the models so
    # `python app.py` works against a fresh DB without running migrations first.
    # Production (gunicorn) relies solely on `alembic upgrade head` (entrypoint.sh).
    init_db()

    # Run the server (start_timeout_checker already called at module import)
    # AIDEV-NOTE: With async_mode='gevent', socketio.run() uses gevent's pywsgi server
    # which has native WebSocket support. No need for allow_unsafe_werkzeug.
    socketio.run(
        app,
        host=Config.HOST,
        port=Config.PORT,
        debug=Config.DEBUG,
    )
