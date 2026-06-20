#!/usr/bin/env python3
"""Driver for the shared HTTP contract suite (spec.md 8.1/8.2).

Starts a StatShed server (Python or Go) on a fresh SQLite DB under a config profile,
waits for it to be healthy, runs the contract suite against it over HTTP, then tears it
down. One server process per (target, profile) run; pytest's autouse fixture truncates
the DB between tests.

Usage:
    python runner.py --target python [--profile default] [-- pytest args...]
    python runner.py --target go     [--profile no_spa]  [-- -k EXPR]

The Python target is migrated with Alembic first (the Flask app does NOT create tables
on import -- entrypoint.sh runs `alembic upgrade head` in production). The Go target
self-migrates (goose) at boot.
"""

from __future__ import annotations

import argparse
import os
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
BACKEND = REPO_ROOT / "backend"
CONTRACT = REPO_ROOT / "contract"

REQUIRED_TABLES = ("groups", "jobs", "config")

# AIDEV-NOTE: Per-profile extra server env (spec.md 8.3). STATSHED_TEST_HOOKS=1 is always
# set (build_env) so the tick hook exists and the 60s scheduler is off.
#   max_log_lines = 1500: one value covers all three log tests (a >1500-line log truncates
#     to 1500; a <1500-line log is untruncated; a 1500-line log retrieved with the default
#     tail caps at 1000 < 1500). See coverage-map.md.
#   max_page_size = 2: a `limit=100` request clamps to 2.
# The SPA profiles (with_spa/no_spa) are target-specific and resolved in build_env.
PROFILE_ENV: dict[str, dict[str, str]] = {
    "default": {},
    "log_disabled": {"LOG_UPLOAD_ENABLED": "false"},
    "max_log_lines": {"MAX_LOG_LINES": "1500"},
    "max_page_size": {"MAX_JOBS_PAGE_SIZE": "2"},
    "with_spa": {},
    "no_spa": {},
}


def write_synthetic_spa(dist: Path) -> Path:
    """Write a minimal SPA dist that the `with_spa` profile serves via STATIC_DIR.

    AIDEV-NOTE: The re-authored static-serving tests (test_spa.py) assert the shell
    contains "StatShed" and the asset contains "console.log". Both servers serve this
    same on-disk dist when STATIC_DIR points here (Python register_spa; Go STATIC_DIR
    override, Task 3.10).
    """
    (dist / "assets").mkdir(parents=True, exist_ok=True)
    (dist / "index.html").write_text(
        "<!doctype html><html><head><title>StatShed</title></head>"
        '<body><div id="root"></div></body></html>'
    )
    (dist / "assets" / "app.js").write_text("console.log('hi')\n")
    return dist


def free_port() -> int:
    """An ephemeral localhost port. Small TOCTOU window; fine for a test harness."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def build_env(
    target: str, profile: str, host: str, port: int, db_file: Path, tmpdir: Path
) -> dict[str, str]:
    env = dict(os.environ)
    # AIDEV-NOTE: runner.py runs inside the contract uv venv, so os.environ carries that
    # venv's VIRTUAL_ENV. Drop it before launching the server: the Python target's
    # `uv run` must resolve the backend project's own venv, and a mismatched VIRTUAL_ENV
    # makes uv warn and could shadow the wrong interpreter.
    env.pop("VIRTUAL_ENV", None)
    env["HOST"] = host
    env["PORT"] = str(port)
    # Absolute path -> SQLAlchemy's 4-slash form ("sqlite:///" + "/abs/path").
    env["DATABASE_URL"] = "sqlite:///" + str(db_file)
    env["STATSHED_TEST_HOOKS"] = "1"
    env.update(PROFILE_ENV[profile])
    if profile == "with_spa":
        env["STATIC_DIR"] = str(write_synthetic_spa(tmpdir / "spa-dist"))
    elif profile == "no_spa":
        if target == "go":
            env["STATIC_DISABLED"] = "1"
        else:
            # Python registers the SPA only when STATIC_DIR exists; point it at a
            # non-existent dir so no fallback is registered (bare paths -> JSON 404).
            env["STATIC_DIR"] = str(tmpdir / "no-such-spa")
    return env


def wait_healthy(
    base_url: str, proc: subprocess.Popen[bytes], timeout: float = 40.0
) -> None:
    deadline = time.monotonic() + timeout
    url = base_url + "/api/health"
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            raise RuntimeError(f"server exited early with code {proc.returncode}")
        try:
            with urllib.request.urlopen(url, timeout=2.0) as r:  # noqa: S310 (fixed localhost URL)
                if r.status == 200:
                    return
        except (urllib.error.URLError, ConnectionError, OSError):
            pass
        time.sleep(0.25)
    raise RuntimeError(f"server did not become healthy within {timeout}s at {url}")


def assert_tables(db_file: Path) -> None:
    """Fail fast if the schema is missing -- a green suite must not hide a missing DB."""
    con = sqlite3.connect(str(db_file))
    try:
        names = {
            row[0]
            for row in con.execute("SELECT name FROM sqlite_master WHERE type='table'")
        }
    finally:
        con.close()
    missing = [t for t in REQUIRED_TABLES if t not in names]
    if missing:
        raise RuntimeError(f"schema is missing tables {missing} (have {sorted(names)})")


def start_python(env: dict[str, str], host: str, port: int) -> subprocess.Popen[bytes]:
    # Migrate first: the Flask app does not create tables on import.
    # AIDEV-NOTE: Invoke tools as `uv run python -m <mod>` rather than the console script
    # (`uv run alembic`) -- the latter can fail to spawn if the venv's script shebang is
    # stale, whereas `python -m` always resolves the installed module.
    subprocess.run(
        ["uv", "run", "python", "-m", "alembic", "upgrade", "head"],
        cwd=str(BACKEND),
        env=env,
        check=True,
    )
    cmd = [
        "uv",
        "run",
        "python",
        "-m",
        "gunicorn",
        "-w",
        "1",
        "-k",
        "geventwebsocket.gunicorn.workers.GeventWebSocketWorker",
        "--bind",
        f"{host}:{port}",
        "app:app",
    ]
    # start_new_session so the whole gunicorn process group can be torn down cleanly.
    return subprocess.Popen(cmd, cwd=str(BACKEND), env=env, start_new_session=True)


def start_go(env: dict[str, str], workdir: Path) -> subprocess.Popen[bytes]:
    binary = workdir / "statshed-server"
    subprocess.run(
        ["go", "build", "-o", str(binary), "./cmd/statshed-server"],
        cwd=str(REPO_ROOT),
        check=True,
    )
    return subprocess.Popen(
        [str(binary)], cwd=str(REPO_ROOT), env=env, start_new_session=True
    )


def terminate(proc: subprocess.Popen[bytes]) -> None:
    if proc.poll() is not None:
        return
    try:
        pgid = os.getpgid(proc.pid)
    except ProcessLookupError:
        return
    os.killpg(pgid, signal.SIGTERM)
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(pgid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        proc.wait()


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Run the StatShed HTTP contract suite."
    )
    parser.add_argument("--target", required=True, choices=["python", "go"])
    parser.add_argument("--profile", default="default", choices=list(PROFILE_ENV))
    parser.add_argument(
        "pytest_args",
        nargs="*",
        help="extra args passed through to pytest (e.g. -k EXPR)",
    )
    args = parser.parse_args()

    host = "127.0.0.1"
    port = free_port()
    base_url = f"http://{host}:{port}"

    with tempfile.TemporaryDirectory(prefix="statshed-contract-") as tmp:
        tmpdir = Path(tmp)
        db_file = tmpdir / "statshed.db"
        env = build_env(args.target, args.profile, host, port, db_file, tmpdir)

        if args.target == "python":
            proc = start_python(env, host, port)
        else:
            proc = start_go(env, tmpdir)

        rc = 1
        try:
            wait_healthy(base_url, proc)
            assert_tables(db_file)

            test_env = dict(os.environ)
            test_env["STATSHED_BASE_URL"] = base_url
            test_env["STATSHED_DB_FILE"] = str(db_file)
            rc = subprocess.call(
                [sys.executable, "-m", "pytest", "-m", args.profile, *args.pytest_args],
                cwd=str(CONTRACT),
                env=test_env,
            )
        finally:
            terminate(proc)

    return rc


if __name__ == "__main__":
    sys.exit(main())
