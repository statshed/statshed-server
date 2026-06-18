"""SQLAlchemy database models.

AIDEV-NOTE: All timestamps are stored in UTC. The database uses SQLite with WAL mode
for better concurrent read/write performance. Group and job names are case-insensitive
and stored in lowercase.
"""

import json
import os
from datetime import UTC, datetime
from typing import overload

import sqlalchemy as sa
from sqlalchemy import (
    Boolean,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    String,
    Text,
    UniqueConstraint,
    create_engine,
    event,
)
from sqlalchemy.orm import (
    DeclarativeBase,
    Mapped,
    column_property,
    mapped_column,
    relationship,
    scoped_session,
    sessionmaker,
)


class Base(DeclarativeBase):
    """Base class for all models."""

    pass


class Group(Base):
    """Group model for organizing jobs.

    AIDEV-NOTE: Group names are case-insensitive and stored as lowercase.
    Timeout overrides (progress_timeout_minutes, staleness_timeout_hours,
    expiration_timeout_hours) take precedence over global config when set.
    Staleness is opt-in via staleness_enabled (disabled by default).
    Expiration is when jobs auto-delete; staleness is just a warning state.
    """

    __tablename__ = "groups"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    progress_timeout_minutes: Mapped[int | None] = mapped_column(Integer, nullable=True)
    staleness_timeout_hours: Mapped[int | None] = mapped_column(Integer, nullable=True)
    # AIDEV-NOTE: staleness_enabled controls whether staleness transitions occur.
    # When False (default), jobs skip the stale state and just expire.
    staleness_enabled: Mapped[bool] = mapped_column(
        Boolean, nullable=False, default=False, server_default=sa.false()
    )
    # AIDEV-NOTE: expiration_timeout_hours controls when jobs auto-delete.
    # NULL means use global default; explicit value overrides.
    expiration_timeout_hours: Mapped[int | None] = mapped_column(Integer, nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime, nullable=False, default=lambda: datetime.now(UTC)
    )

    # Relationship to jobs
    jobs: Mapped[list["Job"]] = relationship(
        "Job", back_populates="group", cascade="all, delete-orphan"
    )

    def to_dict(self) -> dict:
        """Serialize group to dictionary."""
        return {
            "id": self.id,
            "name": self.name,
            "progress_timeout_minutes": self.progress_timeout_minutes,
            "staleness_timeout_hours": self.staleness_timeout_hours,
            "staleness_enabled": self.staleness_enabled,
            "expiration_timeout_hours": self.expiration_timeout_hours,
            "created_at": self.created_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
        }


# Valid job status values
VALID_STATUSES = {"success", "error", "progress", "timeout", "stale"}
# Statuses that indicate unhealthy jobs
UNHEALTHY_STATUSES = {"error", "timeout", "stale"}


class Job(Base):
    """Job model for tracking individual job statuses.

    AIDEV-NOTE: Job names are unique within a group (composite constraint).
    Status values are validated at the API layer before insertion.
    The updated_at field is used for timeout calculations.
    The acked field tracks whether errors have been acknowledged.
    The expires_at field is computed on insert/update for efficient expiration queries.
    Log fields (log_content, log_line_count, log_truncated, log_updated_at) store
    optional log file data attached to status updates.
    """

    __tablename__ = "jobs"
    __table_args__ = (
        UniqueConstraint("group_id", "name", name="uq_job_group_name"),
        Index("ix_jobs_status", "status"),
        Index("ix_jobs_updated_at", "updated_at"),
        Index("ix_jobs_group_id", "group_id"),
        Index("ix_jobs_status_updated", "status", "updated_at"),
        Index("ix_jobs_acked", "acked"),
        Index("ix_jobs_status_acked", "status", "acked"),
        # AIDEV-NOTE: Index on expires_at enables efficient expiration queries
        Index("ix_jobs_expires_at", "expires_at"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    group_id: Mapped[int] = mapped_column(
        Integer, ForeignKey("groups.id"), nullable=False
    )
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    status: Mapped[str] = mapped_column(String(50), nullable=False)
    message: Mapped[str | None] = mapped_column(Text, nullable=True)
    # AIDEV-NOTE: server_default=sa.false() ensures portable boolean default
    # across databases
    acked: Mapped[bool] = mapped_column(
        Boolean, nullable=False, default=False, server_default=sa.false()
    )
    acked_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    # AIDEV-NOTE: expires_at is set by app.py on insert/update as:
    # updated_at + expiration_timeout. See POST /status endpoint (Phase 2).
    # NULL means no expiration (allows flexibility).
    expires_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    # AIDEV-NOTE: Log fields store optional log file content attached to status updates.
    # log_content is the full text; log_line_count and log_truncated are metadata.
    # Logs are replaced on each status update and deleted when job expires.
    log_content: Mapped[str | None] = mapped_column(Text, nullable=True)
    log_line_count: Mapped[int | None] = mapped_column(Integer, nullable=True)
    log_truncated: Mapped[bool] = mapped_column(
        Boolean, nullable=False, default=False, server_default=sa.false()
    )
    log_updated_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    # AIDEV-NOTE: has_log is computed in SQL as `log_content IS NOT NULL` via
    # column_property, NOT in Python. This lets list endpoints defer(log_content) --
    # the big blob -- while to_dict() still reports whether a log exists, with no
    # per-row lazy load. Byte-identical to the old `self.log_content is not None`
    # (empty-string logs included: "" IS NOT NULL is true, just like "" is not None).
    has_log: Mapped[bool] = column_property(log_content.isnot(None))
    updated_at: Mapped[datetime] = mapped_column(
        DateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime, nullable=False, default=lambda: datetime.now(UTC)
    )

    # Relationship to group
    group: Mapped["Group"] = relationship("Group", back_populates="jobs")

    def to_dict(self) -> dict:
        """Serialize job to dictionary.

        AIDEV-NOTE: has_log is a SQL-computed column_property (log_content IS NOT
        NULL), so reading it here never loads the deferred log_content blob.
        Log content itself is NOT included - use the dedicated log endpoint.
        """
        return {
            "id": self.id,
            "group_id": self.group_id,
            "group_name": self.group.name,
            "name": self.name,
            "status": self.status,
            "message": self.message,
            "acked": self.acked,
            "acked_at": (
                self.acked_at.strftime("%Y-%m-%dT%H:%M:%SZ") if self.acked_at else None
            ),
            "expires_at": (
                self.expires_at.strftime("%Y-%m-%dT%H:%M:%SZ")
                if self.expires_at
                else None
            ),
            "has_log": self.has_log,
            "log_line_count": self.log_line_count,
            "log_truncated": self.log_truncated,
            "log_updated_at": (
                self.log_updated_at.strftime("%Y-%m-%dT%H:%M:%SZ")
                if self.log_updated_at
                else None
            ),
            "updated_at": self.updated_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "created_at": self.created_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
        }


class ConfigEntry(Base):
    """Key-value configuration storage.

    AIDEV-NOTE: Values are stored as JSON strings to support various types.
    """

    __tablename__ = "config"

    key: Mapped[str] = mapped_column(String(255), primary_key=True)
    value: Mapped[str] = mapped_column(Text, nullable=False)

    def get_value(self) -> int | str | bool | list | dict | None:
        """Deserialize the JSON value."""
        return json.loads(self.value)

    def set_value(self, val: int | str | bool | list | dict | None) -> None:
        """Serialize value to JSON."""
        self.value = json.dumps(val)


def create_db_engine(database_url: str | None = None):
    """Create a database engine with proper SQLite configuration.

    Args:
        database_url: Database URL (defaults to DATABASE_URL env var or SQLite)

    Returns:
        SQLAlchemy engine
    """
    if database_url is None:
        database_url = os.environ.get("DATABASE_URL", "sqlite:///statshed.db")

    connect_args = {}
    if "sqlite" in database_url:
        connect_args["check_same_thread"] = False

    eng = create_engine(
        database_url,
        connect_args=connect_args,
        pool_pre_ping=True,
    )

    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_connection, _connection_record) -> None:
        """Configure SQLite pragmas for better performance and concurrency.

        AIDEV-NOTE: WAL mode allows concurrent reads during writes.
        busy_timeout prevents immediate failure on lock contention.
        """
        if "sqlite" in database_url:
            cursor = dbapi_connection.cursor()
            cursor.execute("PRAGMA journal_mode=WAL")
            cursor.execute("PRAGMA busy_timeout=5000")
            cursor.execute("PRAGMA synchronous=NORMAL")
            cursor.close()

    return eng


def create_db_session(eng):
    """Create a scoped database session.

    Args:
        eng: SQLAlchemy engine

    Returns:
        Scoped session
    """
    session_factory = sessionmaker(bind=eng)
    return scoped_session(session_factory)


# Create default engine and session for production use
engine = create_db_engine()
db_session = create_db_session(engine)


def init_db(eng=None) -> None:
    """Initialize database tables.

    Args:
        eng: Optional engine to use (defaults to global engine)
    """
    if eng is None:
        eng = engine
    Base.metadata.create_all(eng)


type ConfigValue = int | str | bool | list | dict | None


# AIDEV-NOTE: Overloads let callers that pass a typed default (e.g. an int timeout
# from Config.DEFAULT_*) get that concrete type back instead of the wide ConfigValue
# union, so `timedelta(hours=get_config_value(...))` type-checks. bool precedes int
# because bool is a subtype of int and must match first.
@overload
def get_config_value(key: str, default: bool, session=...) -> bool: ...
@overload
def get_config_value(key: str, default: int, session=...) -> int: ...
@overload
def get_config_value(key: str, default: str, session=...) -> str: ...
@overload
def get_config_value(
    key: str, default: ConfigValue = ..., session=...
) -> ConfigValue: ...
def get_config_value(
    key: str, default: ConfigValue = None, session=None
) -> ConfigValue:
    """Get a configuration value from the database.

    Args:
        key: Configuration key to retrieve
        default: Default value if key doesn't exist
        session: Optional database session (defaults to global db_session)

    Returns:
        The configuration value or default
    """
    if session is None:
        session = db_session
    entry = session.query(ConfigEntry).filter_by(key=key).first()
    if entry:
        return entry.get_value()
    return default


def set_config_value(
    key: str, value: int | str | bool | list | dict | None, session=None
) -> None:
    """Set a configuration value in the database.

    Args:
        key: Configuration key to set
        value: Value to store (will be JSON serialized)
        session: Optional database session (defaults to global db_session)
    """
    if session is None:
        session = db_session
    entry = session.query(ConfigEntry).filter_by(key=key).first()
    if entry:
        entry.set_value(value)
    else:
        entry = ConfigEntry(key=key, value=json.dumps(value))
        session.add(entry)
    session.commit()
