"""Tests that all error responses honor the API's JSON error contract.

AIDEV-NOTE: Every documented endpoint advertises a JSON error envelope
({"error": ..., "message": ...}). Werkzeug's default HTML pages for
404/405/413/415/500 and for malformed JSON broke that contract; these tests
lock in JSON responses across the whole API surface.
"""

import json


def _assert_json_error(resp, status):
    assert resp.status_code == status
    assert resp.content_type.startswith("application/json"), (
        f"expected JSON, got {resp.content_type}"
    )
    body = resp.get_json()
    assert isinstance(body, dict)
    assert "error" in body and "message" in body


class TestHttpErrorEnvelopes:
    def test_unknown_route_returns_json_404(self, client):
        _assert_json_error(client.get("/no-such-route"), 404)

    def test_wrong_method_returns_json_405(self, client):
        # /health is GET-only
        _assert_json_error(client.delete("/health"), 405)

    def test_oversized_body_returns_json_413(self, client):
        # MAX_CONTENT_LENGTH is 1 MB; send more than that.
        big = b"x" * (1024 * 1024 + 1024)
        resp = client.post(
            "/status",
            data=big,
            content_type="application/json",
        )
        _assert_json_error(resp, 413)


class TestMalformedJson:
    def test_malformed_json_status_returns_json_400(self, client):
        resp = client.post(
            "/status",
            data="{not valid json",
            content_type="application/json",
        )
        _assert_json_error(resp, 400)

    def test_malformed_json_config_returns_json_400(self, client):
        resp = client.put(
            "/config",
            data="garbage",
            content_type="application/json",
        )
        _assert_json_error(resp, 400)

    def test_wrong_content_type_status_returns_json_400(self, client):
        # No JSON / wrong content-type should yield the JSON "object required" 400,
        # not a Werkzeug HTML 415.
        resp = client.post("/status", data="group=a&job=b&status=success")
        _assert_json_error(resp, 400)


class TestNonStringFieldsRejectedAs400:
    def test_non_string_group_returns_json_400(self, client):
        resp = client.post(
            "/status",
            data=json.dumps({"group": 123, "job": "j", "status": "success"}),
            content_type="application/json",
        )
        _assert_json_error(resp, 400)
        assert resp.get_json()["field"] == "group"

    def test_non_string_job_returns_json_400(self, client):
        resp = client.post(
            "/status",
            data=json.dumps({"group": "g", "job": 123, "status": "success"}),
            content_type="application/json",
        )
        _assert_json_error(resp, 400)
        assert resp.get_json()["field"] == "job"

    def test_non_string_status_returns_json_400(self, client):
        resp = client.post(
            "/status",
            data=json.dumps({"group": "g", "job": "j", "status": 123}),
            content_type="application/json",
        )
        _assert_json_error(resp, 400)
        assert resp.get_json()["field"] == "status"


class TestInternalErrorEnvelope:
    def test_unexpected_exception_returns_json_500(self, client, monkeypatch):
        # Force an unexpected error inside a handler and confirm the catch-all
        # returns the JSON envelope (not an HTML 500) instead of propagating.
        client.application.config["PROPAGATE_EXCEPTIONS"] = False

        import app as app_module

        def boom(*args, **kwargs):
            raise RuntimeError("kaboom")

        monkeypatch.setattr(app_module.db_session, "query", boom)
        _assert_json_error(client.get("/health"), 500)
