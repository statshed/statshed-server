"""Performance-oriented tests for the read endpoints.

AIDEV-NOTE: These tests guard the *query shape*, not just the JSON output:
- list endpoints must not SELECT the heavy log_content blob, and
- /groups must not issue one query per group (N+1).
They use a SQL-capture context manager that listens on the per-test engine.
"""

from collections.abc import Iterator
from contextlib import contextmanager

from sqlalchemy import event
from sqlalchemy.engine import Engine


@contextmanager
def capture_sql(engine: Engine) -> Iterator[list[str]]:
    """Capture every SQL statement executed on ``engine`` while in the block."""
    statements: list[str] = []

    def _before(_conn, _cursor, statement, _parameters, _context, _executemany):
        statements.append(statement)

    event.listen(engine, "before_cursor_execute", _before)
    try:
        yield statements
    finally:
        event.remove(engine, "before_cursor_execute", _before)


def _selects_against(statements, table):
    """Return SELECT statements that read from ``table``."""
    return [
        s
        for s in statements
        if s.lstrip().upper().startswith("SELECT") and f"FROM {table}" in s
    ]


class TestLogBlobNotLoaded:
    """The log_content blob must never be fetched by list endpoints."""

    def test_get_jobs_does_not_select_log_content_blob(self, client, db_session):
        import models
        from models import Group, Job

        group = Group(name="g1")
        db_session.add(group)
        db_session.flush()
        db_session.add(
            Job(
                group_id=group.id,
                name="j1",
                status="success",
                log_content="x" * 10000,
                log_line_count=1,
            )
        )
        db_session.commit()

        with capture_sql(models.engine) as statements:
            resp = client.get("/jobs")

        assert resp.status_code == 200
        # has_log correctness is preserved even though the blob isn't loaded.
        assert resp.get_json()["jobs"][0]["has_log"] is True

        # The only permitted appearance of log_content in the jobs query is the
        # `log_content IS NOT NULL` expression that backs has_log -- never the
        # bare blob column in the SELECT projection.
        jobs_selects = _selects_against(statements, "jobs")
        assert jobs_selects, "expected a SELECT against the jobs table"
        for stmt in jobs_selects:
            residual = stmt.replace("log_content IS NOT NULL", "")
            assert "log_content" not in residual, (
                f"log_content blob was selected by /jobs:\n{stmt}"
            )

    def test_group_jobs_does_not_select_log_content_blob(self, client, db_session):
        import models
        from models import Group, Job

        group = Group(name="g1")
        db_session.add(group)
        db_session.flush()
        db_session.add(
            Job(
                group_id=group.id,
                name="j1",
                status="success",
                log_content="x" * 10000,
                log_line_count=1,
            )
        )
        db_session.commit()

        with capture_sql(models.engine) as statements:
            resp = client.get("/groups/g1/jobs")

        assert resp.status_code == 200
        assert resp.get_json()["jobs"][0]["has_log"] is True

        jobs_selects = _selects_against(statements, "jobs")
        assert jobs_selects, "expected a SELECT against the jobs table"
        for stmt in jobs_selects:
            residual = stmt.replace("log_content IS NOT NULL", "")
            assert "log_content" not in residual, (
                f"log_content blob was selected by /groups/<name>/jobs:\n{stmt}"
            )


def _make_jobs(client, group, count, status="success"):
    """Create ``count`` jobs in ``group`` via the API."""
    for i in range(count):
        resp = client.post(
            "/status",
            json={"group": group, "job": f"job{i}", "status": status},
        )
        assert resp.status_code == 201


class TestHealthUsesAggregates:
    """GET /health must count in SQL, not load every Job row into Python."""

    def test_health_does_not_load_job_rows(self, client):
        import models

        _make_jobs(client, "g1", 5)

        with capture_sql(models.engine) as statements:
            resp = client.get("/health")

        assert resp.status_code == 200
        job_selects = _selects_against(statements, "jobs")
        assert job_selects, "expected at least one aggregate SELECT against jobs"
        for stmt in job_selects:
            assert "count(" in stmt.lower(), (
                f"/health loaded job rows instead of aggregating in SQL:\n{stmt}"
            )


class TestGroupsAvoidsNPlusOne:
    """GET /groups must aggregate in SQL with a bounded query count (no per-group
    query, no full Job row loads)."""

    def test_groups_query_count_is_bounded(self, client):
        import models

        # Eight groups, each with a couple of jobs. An N+1 implementation would
        # issue ~8 per-group job SELECTs; the aggregate version issues a small
        # constant number regardless of group count.
        for g in range(8):
            _make_jobs(client, f"g{g}", 2)

        with capture_sql(models.engine) as statements:
            resp = client.get("/groups")

        assert resp.status_code == 200
        job_selects = _selects_against(statements, "jobs")
        assert len(job_selects) <= 4, (
            f"N+1 detected: {len(job_selects)} job SELECTs for 8 groups"
        )
        for stmt in job_selects:
            assert "count(" in stmt.lower(), (
                f"/groups loaded job rows instead of aggregating in SQL:\n{stmt}"
            )


def _count_queries(statements):
    """Return statements that issue a SQL COUNT aggregate."""
    return [s for s in statements if "count(" in s.lower()]


class TestNoRedundantCountOnDefaultPath:
    """On the backward-compatible no-pagination path, total == len(jobs), so the
    list endpoints must NOT issue a separate COUNT aggregate. When a window is
    requested (limit/offset), the COUNT is required to report the full total."""

    def test_get_jobs_without_pagination_issues_no_count(self, client):
        import models

        _make_jobs(client, "g1", 3)

        with capture_sql(models.engine) as statements:
            resp = client.get("/jobs")

        assert resp.status_code == 200
        assert resp.get_json()["total"] == 3
        assert _count_queries(statements) == [], (
            "GET /jobs with no pagination issued a redundant COUNT:\n"
            + "\n".join(_count_queries(statements))
        )

    def test_get_jobs_with_limit_issues_count_for_full_total(self, client):
        import models

        _make_jobs(client, "g1", 3)

        with capture_sql(models.engine) as statements:
            resp = client.get("/jobs?limit=2")

        data = resp.get_json()
        assert resp.status_code == 200
        assert len(data["jobs"]) == 2
        assert data["total"] == 3  # full count, not the page size
        assert _count_queries(statements), (
            "GET /jobs?limit= must COUNT to report the full total"
        )

    def test_group_jobs_without_pagination_issues_no_count(self, client):
        import models

        _make_jobs(client, "g1", 3)

        with capture_sql(models.engine) as statements:
            resp = client.get("/groups/g1/jobs")

        assert resp.status_code == 200
        assert resp.get_json()["total"] == 3
        assert _count_queries(statements) == [], (
            "GET /groups/<name>/jobs with no pagination issued a redundant COUNT:\n"
            + "\n".join(_count_queries(statements))
        )

    def test_group_jobs_with_limit_issues_count_for_full_total(self, client):
        import models

        _make_jobs(client, "g1", 3)

        with capture_sql(models.engine) as statements:
            resp = client.get("/groups/g1/jobs?limit=2")

        data = resp.get_json()
        assert resp.status_code == 200
        assert len(data["jobs"]) == 2
        assert data["total"] == 3
        assert _count_queries(statements), (
            "GET /groups/<name>/jobs?limit= must COUNT to report the full total"
        )
