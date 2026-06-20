"""Socket.IO oracle tests for jobs_acked, job_deleted, and job_expired.

AIDEV-NOTE: The six real-time events become the SSE oracle when the transport is
switched to Server-Sent Events (spec.md 7/8.4). Three already have pinned Python
payloads: status_update + group_created (TestWebSocketIntegration in
test_integration.py) and health_update (TestHealthUpdateEmit in test_background.py).
This module pins the remaining three so the Go SSE-frame tests have a verified
payload reference for all six. Payloads mirror the socketio.emit(...) call sites:
jobs_acked / job_deleted in app.py (ack + delete handlers), job_expired in
background.py (run_expiration_check).

Two emission mechanisms, two test styles (matching the existing suites):
- jobs_acked / job_deleted are emitted by HTTP handlers on the module-level socketio,
  so they are observed via the connected socketio_client (as TestWebSocketIntegration
  does for status_update / group_created).
- job_expired is emitted by run_expiration_check(db_session, socketio), so it is
  observed by injecting a recording stand-in (as TestHealthUpdateEmit does).

Ids are read from the POST /status response (job.to_dict) to avoid touching detached
ORM instances across request teardown.
"""

import re
from datetime import UTC, datetime, timedelta

# Whole-second UTC timestamp the API/events render: "YYYY-MM-DDTHH:MM:SSZ".
TIMESTAMP_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")


class _RecordingSocketIO:
    """Minimal Socket.IO stand-in that records emit() calls (see test_background)."""

    def __init__(self) -> None:
        self.emitted: list[tuple[str, dict]] = []

    def emit(self, event: str, payload: dict) -> None:
        self.emitted.append((event, payload))


def _events(received: list[dict], name: str) -> list[dict]:
    """The payload dicts of every received event with the given name."""
    return [r["args"][0] for r in received if r["name"] == name]


def _create_job(client, group: str, job: str, status: str) -> dict:
    """POST /status and return the created job dict (job.to_dict)."""
    resp = client.post("/status", json={"group": group, "job": job, "status": status})
    assert resp.status_code == 201
    return resp.get_json()["job"]


class TestJobsAckedEvent:
    def test_single_job_ack(self, app, client, socketio_client):  # noqa: ARG002
        assert socketio_client.is_connected()
        job = _create_job(client, "g", "j", "error")
        socketio_client.get_received()  # clear group_created + status_update

        client.post(f"/jobs/{job['id']}/ack")

        events = _events(socketio_client.get_received(), "jobs_acked")
        assert len(events) == 1
        data = events[0]
        assert set(data) == {
            "schema_version",
            "job_ids",
            "group_id",
            "group_name",
            "acked_count",
            "timestamp",
        }
        assert data["schema_version"] == 1
        assert data["job_ids"] == [job["id"]]
        assert data["group_id"] == job["group_id"]
        assert data["group_name"] == "g"
        assert data["acked_count"] == 1
        assert TIMESTAMP_RE.match(data["timestamp"])

    def test_group_ack(self, app, client, socketio_client):  # noqa: ARG002
        assert socketio_client.is_connected()
        a = _create_job(client, "g", "a", "error")
        b = _create_job(client, "g", "b", "timeout")
        socketio_client.get_received()

        client.post("/groups/g/ack")

        events = _events(socketio_client.get_received(), "jobs_acked")
        assert len(events) == 1
        data = events[0]
        assert data["schema_version"] == 1
        assert set(data["job_ids"]) == {a["id"], b["id"]}  # id arrays compared as sets
        assert data["group_id"] == a["group_id"]
        assert data["group_name"] == "g"
        assert data["acked_count"] == 2
        assert TIMESTAMP_RE.match(data["timestamp"])

    def test_ack_all_has_null_group_scope(self, app, client, socketio_client):  # noqa: ARG002
        assert socketio_client.is_connected()
        a = _create_job(client, "g1", "a", "error")
        b = _create_job(client, "g2", "b", "stale")
        socketio_client.get_received()

        client.post("/ack-all")

        events = _events(socketio_client.get_received(), "jobs_acked")
        assert len(events) == 1
        data = events[0]
        assert data["schema_version"] == 1
        assert set(data["job_ids"]) == {a["id"], b["id"]}
        # ack-all is global scope: group_id/group_name are null (not absent).
        assert data["group_id"] is None
        assert data["group_name"] is None
        assert data["acked_count"] == 2
        assert TIMESTAMP_RE.match(data["timestamp"])


class TestJobDeletedEvent:
    def test_delete_job(self, app, client, socketio_client):  # noqa: ARG002
        assert socketio_client.is_connected()
        job = _create_job(client, "g", "j", "success")
        socketio_client.get_received()

        client.delete(f"/jobs/{job['id']}")

        events = _events(socketio_client.get_received(), "job_deleted")
        assert len(events) == 1
        data = events[0]
        assert set(data) == {
            "schema_version",
            "job_id",
            "job_name",
            "group_id",
            "group_name",
            "timestamp",
        }
        assert data["schema_version"] == 1
        assert data["job_id"] == job["id"]
        assert data["job_name"] == "j"
        assert data["group_id"] == job["group_id"]
        assert data["group_name"] == "g"
        assert TIMESTAMP_RE.match(data["timestamp"])


class TestJobExpiredEvent:
    def test_expiration_emits_job_expired(self, app, client, db_session):  # noqa: ARG002
        from background import run_expiration_check
        from models import Job

        job = _create_job(client, "g", "j", "success")
        # Backdate the expiry so the next pass deletes it.
        row = db_session.query(Job).filter_by(id=job["id"]).first()
        row.expires_at = datetime.now(UTC).replace(tzinfo=None) - timedelta(hours=1)
        db_session.commit()

        sock = _RecordingSocketIO()
        run_expiration_check(db_session, sock)

        expired = [p for (event, p) in sock.emitted if event == "job_expired"]
        assert len(expired) == 1
        data = expired[0]
        assert set(data) == {
            "schema_version",
            "job_id",
            "job_name",
            "group_id",
            "group_name",
            "timestamp",
        }
        assert data["schema_version"] == 1
        assert data["job_id"] == job["id"]
        assert data["job_name"] == "j"
        assert data["group_id"] == job["group_id"]
        assert data["group_name"] == "g"
        assert TIMESTAMP_RE.match(data["timestamp"])
