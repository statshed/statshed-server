"""Tests that the Alembic migration chain builds the full schema from an empty DB.

AIDEV-NOTE: These tests guard against the "empty baseline migration" class of bug,
where `alembic upgrade head` fails on a brand-new database because the baseline
migration assumed the schema was pre-created by Base.metadata.create_all().
Alembic is the single source of truth for the schema; create_all() is dev/test only.
"""

import os
from pathlib import Path

import pytest
from sqlalchemy import create_engine, inspect

REPO_ROOT = Path(__file__).resolve().parent.parent

# AIDEV-NOTE: Indexes and the unique constraint declared on the Job model that a
# migration-built database MUST contain (regression guard for model-vs-migration drift).
EXPECTED_JOB_INDEXES = {
    "ix_jobs_status",
    "ix_jobs_updated_at",
    "ix_jobs_group_id",
    "ix_jobs_status_updated",
    "ix_jobs_acked",
    "ix_jobs_status_acked",
    "ix_jobs_expires_at",
}


def _run_upgrade_head(db_url: str) -> None:
    """Run `alembic upgrade head` against the given database URL."""
    from alembic.config import Config

    from alembic import command

    cfg = Config(str(REPO_ROOT / "alembic.ini"))
    cfg.set_main_option("script_location", str(REPO_ROOT / "alembic"))
    cfg.set_main_option("sqlalchemy.url", db_url)
    # env.py reads DATABASE_URL from the environment and overrides sqlalchemy.url,
    # so it must point at the same throwaway DB for this run.
    old = os.environ.get("DATABASE_URL")
    os.environ["DATABASE_URL"] = db_url
    try:
        command.upgrade(cfg, "head")
    finally:
        if old is None:
            os.environ.pop("DATABASE_URL", None)
        else:
            os.environ["DATABASE_URL"] = old


@pytest.fixture
def empty_db_url(tmp_path) -> str:
    """A file-based SQLite URL pointing at a brand-new, empty database."""
    return f"sqlite:///{tmp_path / 'fresh.db'}"


def test_upgrade_head_on_empty_db_succeeds(empty_db_url):
    """`alembic upgrade head` must run cleanly against a brand-new empty database."""
    # Must not raise (previously raised NoSuchTableError: jobs from the empty baseline)
    _run_upgrade_head(empty_db_url)

    engine = create_engine(empty_db_url)
    tables = set(inspect(engine).get_table_names())
    assert {"groups", "jobs", "config"}.issubset(tables)


def test_migrated_schema_matches_models(empty_db_url):
    """The schema built purely by migrations must match the SQLAlchemy models."""
    from models import Base

    _run_upgrade_head(empty_db_url)
    engine = create_engine(empty_db_url)
    insp = inspect(engine)

    # Every model table exists with exactly the model's columns.
    for table_name, table in Base.metadata.tables.items():
        assert table_name in insp.get_table_names(), f"missing table {table_name}"
        db_cols = {c["name"] for c in insp.get_columns(table_name)}
        model_cols = {c.name for c in table.columns}
        assert db_cols == model_cols, (
            f"column mismatch on {table_name}: "
            f"missing={model_cols - db_cols} extra={db_cols - model_cols}"
        )

    # All declared job indexes were created by the migration chain (drift guard).
    job_indexes = {ix["name"] for ix in insp.get_indexes("jobs")}
    assert EXPECTED_JOB_INDEXES.issubset(job_indexes), (
        f"missing job indexes: {EXPECTED_JOB_INDEXES - job_indexes}"
    )

    # The (group_id, name) uniqueness is enforced.
    uniques = insp.get_unique_constraints("jobs")
    assert any(set(uc["column_names"]) == {"group_id", "name"} for uc in uniques), (
        f"missing unique constraint on (group_id, name); got {uniques}"
    )
