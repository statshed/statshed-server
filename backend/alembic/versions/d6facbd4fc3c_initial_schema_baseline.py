"""initial_schema_baseline

Revision ID: d6facbd4fc3c
Revises:
Create Date: 2026-01-21 22:42:16.401607

AIDEV-NOTE: This baseline migration creates the v1 schema (groups, jobs, config)
as it existed before the acked/expiration/log feature migrations. Alembic is the
single source of truth for the schema; `Base.metadata.create_all()` is now used
only for tests/dev (see app.py). Subsequent migrations (bbc0d80cc9f1,
c7e8f9a1b2d3, 192b2938d2b9) layer the later columns/indexes on top of this.

AIDEV-NOTE: An idempotency guard skips creation when the tables already exist, so
this revision can be safely applied to (or stamped onto) databases that predate
this rewrite and were originally built by create_all().
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy import inspect

from alembic import op

# revision identifiers, used by Alembic.
revision: str = "d6facbd4fc3c"
down_revision: str | Sequence[str] | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    """Upgrade schema."""
    bind = op.get_bind()
    # AIDEV-NOTE: Legacy databases were built by create_all() before this baseline
    # had real operations; skip if the schema is already present.
    if "jobs" in inspect(bind).get_table_names():
        return

    op.create_table(
        "groups",
        sa.Column("id", sa.Integer(), nullable=False),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column("progress_timeout_minutes", sa.Integer(), nullable=True),
        sa.Column("staleness_timeout_hours", sa.Integer(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("name"),
    )

    op.create_table(
        "jobs",
        sa.Column("id", sa.Integer(), nullable=False),
        sa.Column("group_id", sa.Integer(), nullable=False),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column("status", sa.String(length=50), nullable=False),
        sa.Column("message", sa.Text(), nullable=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(["group_id"], ["groups.id"]),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("group_id", "name", name="uq_job_group_name"),
    )
    op.create_index("ix_jobs_group_id", "jobs", ["group_id"], unique=False)
    op.create_index("ix_jobs_status", "jobs", ["status"], unique=False)
    op.create_index(
        "ix_jobs_status_updated", "jobs", ["status", "updated_at"], unique=False
    )
    op.create_index("ix_jobs_updated_at", "jobs", ["updated_at"], unique=False)

    op.create_table(
        "config",
        sa.Column("key", sa.String(length=255), nullable=False),
        sa.Column("value", sa.Text(), nullable=False),
        sa.PrimaryKeyConstraint("key"),
    )


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_table("config")
    op.drop_index("ix_jobs_updated_at", table_name="jobs")
    op.drop_index("ix_jobs_status_updated", table_name="jobs")
    op.drop_index("ix_jobs_status", table_name="jobs")
    op.drop_index("ix_jobs_group_id", table_name="jobs")
    op.drop_table("jobs")
    op.drop_table("groups")
