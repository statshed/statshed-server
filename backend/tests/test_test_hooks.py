"""Tests for the guarded, test-only hooks (STATSHED_TEST_HOOKS).

AIDEV-NOTE: These cover the contract-suite scaffolding added in impl-guide Task 1.1:
- POST /api/admin/run-checks is registered ONLY when STATSHED_TEST_HOOKS is set.
- run_timeout_check returns sorted per-type id arrays (timeout_job_ids/stale_job_ids).
- The 60s background scheduler is skipped when the flag is set, started otherwise.

The flag is read live from the env (config.is_test_hooks_enabled), so the route /
scheduler decisions depend on the env at the moment app.py is (re)imported. The fixtures
below set the env BEFORE forcing a fresh import, mirroring tests/conftest.py's `app`
fixture (which deletes `app`/`background` from sys.modules so module-level code re-runs).
"""

import sys
from datetime import UTC, datetime, timedelta
from unittest.mock import MagicMock

import pytest


def _build_app():
    """Create a fresh in-memory app, re-importing app.py so its env-gated module-level
    code (the run-checks route, the scheduler start) re-evaluates under the current env.

    Returns (flask_app, engine, session, saved) where `saved` restores models globals.
    """
    from models import Base, create_db_engine, create_db_session

    test_engine = create_db_engine("sqlite:///:memory:")
    test_session = create_db_session(test_engine)
    Base.metadata.create_all(test_engine)

    import models

    saved = (models.db_session, models.engine)
    models.db_session = test_session
    models.engine = test_engine

    sys.modules.pop("app", None)
    sys.modules.pop("background", None)

    from app import app as flask_app

    flask_app.config["TESTING"] = True
    return flask_app, test_engine, test_session, saved


def _teardown_app(test_engine, test_session, saved):
    import models
    from models import Base

    test_session.remove()
    Base.metadata.drop_all(test_engine)
    models.db_session, models.engine = saved
    # Drop so the next test's `app` fixture re-imports cleanly under its own env.
    sys.modules.pop("app", None)
    sys.modules.pop("background", None)


@pytest.fixture
def hooks_app(monkeypatch):
    """A fresh Flask app imported with STATSHED_TEST_HOOKS=1."""
    monkeypatch.setenv("STATSHED_TEST_HOOKS", "1")
    flask_app, engine, session, saved = _build_app()
    try:
        yield flask_app
    finally:
        _teardown_app(engine, session, saved)


@pytest.fixture
def noflag_app(monkeypatch):
    """A fresh Flask app imported with STATSHED_TEST_HOOKS explicitly unset.

    Robust against a STATSHED_TEST_HOOKS that happens to be set in the shell when the
    backend suite is run.
    """
    monkeypatch.delenv("STATSHED_TEST_HOOKS", raising=False)
    flask_app, engine, session, saved = _build_app()
    try:
        yield flask_app
    finally:
        _teardown_app(engine, session, saved)


class TestRunChecksHookRegistration:
    def test_route_absent_when_hooks_disabled(self, noflag_app):
        # With the flag unset the route is not registered, so the path is an ordinary
        # JSON 404 -- indistinguishable from any other unknown /api path.
        resp = noflag_app.test_client().post("/api/admin/run-checks")
        assert resp.status_code == 404
        assert resp.json["error"] == "not_found"

    def test_route_present_when_hooks_enabled(self, hooks_app):
        resp = hooks_app.test_client().post("/api/admin/run-checks")
        assert resp.status_code == 200
        body = resp.json
        assert set(body) == {"timeout_result", "expiration_result"}


class TestRunChecksHookExecution:
    def test_drives_timeout_and_expiration(self, hooks_app):
        import models
        from models import Job

        raw = hooks_app.test_client()
        raw.post(
            "/api/status", json={"group": "g", "job": "slow", "status": "progress"}
        )
        raw.post("/api/status", json={"group": "g", "job": "gone", "status": "success"})

        # Fetch AFTER the posts (avoid detached instances), backdate, commit.
        past = datetime.now(UTC).replace(tzinfo=None) - timedelta(minutes=10)
        slow = models.db_session.query(Job).filter_by(name="slow").first()
        gone = models.db_session.query(Job).filter_by(name="gone").first()
        slow_id, gone_id = slow.id, gone.id
        slow.updated_at = past  # past the 5-minute default progress timeout
        gone.expires_at = past  # already expired
        models.db_session.commit()

        resp = raw.post("/api/admin/run-checks")
        assert resp.status_code == 200
        body = resp.json
        assert body["timeout_result"]["timeout_job_ids"] == [slow_id]
        assert body["timeout_result"]["stale_job_ids"] == []
        assert body["timeout_result"]["timeout_count"] == 1
        assert gone_id in body["expiration_result"]["expired_job_ids"]

        # State reflects the synchronous pass.
        assert (
            models.db_session.query(Job).filter_by(name="slow").first().status
            == "timeout"
        )
        assert models.db_session.query(Job).filter_by(name="gone").first() is None


class TestTimeoutCheckEnrichment:
    def test_sorted_per_type_split(self, client, db_session):
        # run_timeout_check is enriched with sorted per-type id arrays so the
        # cross-language tick-hook contract can assert the timeout-vs-stale split.
        from background import run_timeout_check
        from models import Group, Job

        client.post("/status", json={"group": "g", "job": "p2", "status": "progress"})
        client.post("/status", json={"group": "g", "job": "p1", "status": "progress"})
        client.post("/status", json={"group": "s", "job": "s1", "status": "success"})

        # Enable staleness on group s so its success job can go stale.
        db_session.query(Group).filter_by(name="s").first().staleness_enabled = True
        db_session.commit()

        old = datetime.now(UTC).replace(tzinfo=None) - timedelta(days=30)
        for name in ("p1", "p2", "s1"):
            db_session.query(Job).filter_by(name=name).first().updated_at = old
        db_session.commit()

        result = run_timeout_check(db_session)

        p1 = db_session.query(Job).filter_by(name="p1").first()
        p2 = db_session.query(Job).filter_by(name="p2").first()
        s1 = db_session.query(Job).filter_by(name="s1").first()

        assert result["timeout_job_ids"] == sorted([p1.id, p2.id])
        assert result["stale_job_ids"] == [s1.id]
        assert result["timeout_count"] == 2
        assert result["stale_count"] == 1
        # The split must be clean: a stale job never appears under timeout, and v.v.
        assert s1.id not in result["timeout_job_ids"]
        assert p1.id not in result["stale_job_ids"]
        # Every id array is sorted ascending so the Python and Go hook results compare
        # deterministically (spec.md 8.4) -- not just the per-type arrays above.
        assert result["affected_job_ids"] == sorted([p1.id, p2.id, s1.id])
        assert result["affected_group_ids"] == sorted(result["affected_group_ids"])


class TestSchedulerGuard:
    def test_scheduler_skipped_when_hooks_enabled(self, monkeypatch):
        monkeypatch.setenv("STATSHED_TEST_HOOKS", "1")
        from background import start_timeout_checker

        sio = MagicMock()
        start_timeout_checker(MagicMock(), sio, MagicMock())
        sio.start_background_task.assert_not_called()

    def test_scheduler_started_when_hooks_disabled(self, monkeypatch):
        monkeypatch.delenv("STATSHED_TEST_HOOKS", raising=False)
        from background import start_timeout_checker

        sio = MagicMock()
        start_timeout_checker(MagicMock(), sio, MagicMock())
        sio.start_background_task.assert_called_once()
