"""add_log_columns_to_jobs

Revision ID: 192b2938d2b9
Revises: c7e8f9a1b2d3
Create Date: 2026-01-26 10:00:00.000000

AIDEV-NOTE: This migration adds log storage columns to the jobs table:
- log_content: Text field for storing log file content (nullable)
- log_line_count: Integer count of lines in the log (nullable)
- log_truncated: Boolean flag indicating if log was truncated (NOT NULL, default False)
- log_updated_at: Timestamp when log was last updated (nullable)

These columns support optional log file attachments for status updates. Logs are
replaced on each status update and deleted when jobs expire.

AIDEV-NOTE: This is a safe migration - all columns are nullable except log_truncated
which has a server_default, so no data loss risk. Uses direct op.add_column() instead
of batch mode to avoid SQLite circular dependency issues (following pattern from
bbc0d80cc9f1_add_acked_columns_to_jobs.py).
"""

from collections.abc import Sequence

import sqlalchemy as sa

from alembic import op

# revision identifiers, used by Alembic.
revision: str = "192b2938d2b9"
down_revision: str | Sequence[str] | None = "c7e8f9a1b2d3"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    """Upgrade schema."""
    # AIDEV-NOTE: Using direct ALTER TABLE instead of batch mode to avoid
    # SQLite circular dependency error with column reordering.
    # See: https://alembic.sqlalchemy.org/en/latest/batch.html#working-with-sqlite
    op.add_column("jobs", sa.Column("log_content", sa.Text(), nullable=True))
    op.add_column("jobs", sa.Column("log_line_count", sa.Integer(), nullable=True))
    op.add_column(
        "jobs",
        sa.Column(
            "log_truncated",
            sa.Boolean(),
            server_default=sa.false(),
            nullable=False,
        ),
    )
    op.add_column("jobs", sa.Column("log_updated_at", sa.DateTime(), nullable=True))


def downgrade() -> None:
    """Downgrade schema."""
    # AIDEV-NOTE: Using batch mode for DROP COLUMN operations.
    # SQLite doesn't support DROP COLUMN directly, but batch mode works for drops.
    with op.batch_alter_table("jobs", schema=None) as batch_op:
        batch_op.drop_column("log_updated_at")
        batch_op.drop_column("log_truncated")
        batch_op.drop_column("log_line_count")
        batch_op.drop_column("log_content")
