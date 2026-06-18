"""add_expiration_and_staleness_enabled

Revision ID: c7e8f9a1b2d3
Revises: bbc0d80cc9f1
Create Date: 2026-01-25 10:00:00.000000

AIDEV-NOTE: This migration adds support for the expiring status entries feature:
- Groups: staleness_enabled (opt-in staleness), expiration_timeout_hours (auto-delete)
- Jobs: expires_at (computed expiration timestamp for efficient queries)

The migration also sets expires_at for existing jobs based on the default expiration
timeout (24 hours from their updated_at). This is intentional - existing jobs should
follow the new expiration behavior; expiration logic (added in Phase 2) will handle
the actual deletion.

AIDEV-NOTE: This migration assumes SQLite database. PostgreSQL interval syntax is
included as fallback but has not been tested. MySQL is NOT supported.
"""

from collections.abc import Sequence

import sqlalchemy as sa

from alembic import op

# revision identifiers, used by Alembic.
revision: str = "c7e8f9a1b2d3"
down_revision: str | Sequence[str] | None = "bbc0d80cc9f1"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# Default expiration timeout in hours (matches config.py)
DEFAULT_EXPIRATION_TIMEOUT_HOURS = 24


def upgrade() -> None:
    """Upgrade schema."""
    # Add new columns to groups table
    with op.batch_alter_table("groups", schema=None) as batch_op:
        # AIDEV-NOTE: staleness_enabled defaults to False (opt-in staleness)
        batch_op.add_column(
            sa.Column(
                "staleness_enabled",
                sa.Boolean(),
                server_default=sa.false(),
                nullable=False,
            )
        )
        # AIDEV-NOTE: expiration_timeout_hours is nullable;
        # NULL means use global default
        batch_op.add_column(
            sa.Column("expiration_timeout_hours", sa.Integer(), nullable=True)
        )

    # Add expires_at column to jobs table with index
    with op.batch_alter_table("jobs", schema=None) as batch_op:
        batch_op.add_column(sa.Column("expires_at", sa.DateTime(), nullable=True))
        batch_op.create_index("ix_jobs_expires_at", ["expires_at"], unique=False)

    # Set expires_at for existing jobs based on default expiration timeout
    # AIDEV-NOTE: This uses raw SQL for efficiency with potentially large datasets.
    # The f-string is safe here because `hours` is a compile-time constant (24).
    bind = op.get_bind()

    # SQLite datetime arithmetic: use datetime() function with modifiers
    hours = DEFAULT_EXPIRATION_TIMEOUT_HOURS
    if "sqlite" in bind.dialect.name:
        # SQLite: datetime(updated_at, '+24 hours')
        sql = f"UPDATE jobs SET expires_at = datetime(updated_at, '+{hours} hours')"
        bind.execute(sa.text(sql))
    else:
        # PostgreSQL (untested): updated_at + interval
        sql = f"UPDATE jobs SET expires_at = updated_at + interval '{hours} hours'"
        bind.execute(sa.text(sql))


def downgrade() -> None:
    """Downgrade schema."""
    with op.batch_alter_table("jobs", schema=None) as batch_op:
        batch_op.drop_index("ix_jobs_expires_at")
        batch_op.drop_column("expires_at")

    with op.batch_alter_table("groups", schema=None) as batch_op:
        batch_op.drop_column("expiration_timeout_hours")
        batch_op.drop_column("staleness_enabled")
