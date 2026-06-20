"""Flask serves the built SPA at root, without shadowing the /api API."""


def _make_dist(tmp_path):
    (tmp_path / "index.html").write_text(
        "<!doctype html><title>StatShed</title><div id='root'></div>"
    )
    assets = tmp_path / "assets"
    assets.mkdir()
    (assets / "app.js").write_text("console.log('hi')")
    return str(tmp_path)


def test_spa_served_at_root(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/")
    assert resp.status_code == 200
    assert b"StatShed" in resp.data


def test_spa_fallback_does_not_shadow_jobs_api(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    client = app.test_client()
    # /jobs is an SPA route -> must return the SPA shell, NOT the GET /jobs JSON.
    assert b"StatShed" in client.get("/jobs").data
    # The API is still reachable under /api.
    assert client.get("/api/health").status_code == 200


def test_real_asset_is_served(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/assets/app.js")
    assert resp.status_code == 200
    assert b"console.log" in resp.data


def test_unknown_api_path_is_404_not_spa(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/api/does-not-exist")
    assert resp.status_code == 404
    assert b"StatShed" not in resp.data  # JSON error, not the SPA shell
