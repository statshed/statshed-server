"""Shared fixtures for the StatShed HTTP contract suite.

AIDEV-NOTE: Tests drive a LIVE server over real HTTP (httpx), not an in-process test
client, so the identical assertions run against the Python oracle and then verbatim
against the Go server (spec.md 8.1). The server is started by runner.py, which exports
STATSHED_BASE_URL (the server root) and STATSHED_DB_FILE (the SQLite file) into the env.

Per-test isolation is by direct-SQLite truncation (spec.md 8.2): the autouse reset_db
fixture empties the tables in FK order before each test. There is deliberately NO
`DELETE FROM sqlite_sequence` -- the schema has no AUTOINCREMENT, so ids reset to 1 after
a full DELETE and that table does not exist. This is language-neutral and identical
against Python and Go (both read fresh per request, safe under WAL).
"""

from __future__ import annotations

import os
import sqlite3
from collections.abc import Iterator
from datetime import UTC, datetime

import httpx
import pytest


def _require_env(name: str) -> str:
    try:
        return os.environ[name]
    except KeyError:
        raise RuntimeError(
            f"{name} is not set -- run the contract suite via runner.py "
            "(e.g. `make contract-test TARGET=python`), not pytest directly."
        ) from None


def _db_path() -> str:
    """The SQLite DB file the server is using (exported by runner.py)."""
    return _require_env("STATSHED_DB_FILE")


# AIDEV-NOTE: FK order is jobs (child) -> groups (parent) -> config. No sqlite_sequence.
_TRUNCATE_SQL = "DELETE FROM jobs; DELETE FROM groups; DELETE FROM config;"


@pytest.fixture(autouse=True)
def reset_db() -> Iterator[None]:
    """Truncate all tables to a pristine state before each test."""
    con = sqlite3.connect(_db_path())
    try:
        con.executescript(_TRUNCATE_SQL)
        con.commit()
    finally:
        con.close()
    yield


@pytest.fixture
def client() -> Iterator[httpx.Client]:
    """An httpx client bound to the server root.

    Tests use explicit paths (`/api/health`, `/`, ...); there is no implicit /api
    prefixing -- the suite addresses the real URL space both servers expose.
    """
    with httpx.Client(base_url=_require_env("STATSHED_BASE_URL"), timeout=10.0) as c:
        yield c


def backdate(table: str, where: str, **cols: object) -> None:
    """Set columns on the rows matching `where`, via direct SQL -- used to age rows.

    AIDEV-NOTE: The server always writes 'now' timestamps; tests that need aged rows
    (jobs ordering, pagination by offset, cleanup older_than_days, timeout/expiry
    transitions) backdate them here instead of sleeping (spec.md 8.3 bucket 3). `table`
    and `where` are trusted literals authored by tests, never user input.
    """
    assignments = ", ".join(f"{name} = ?" for name in cols)
    con = sqlite3.connect(_db_path())
    try:
        con.execute(
            f"UPDATE {table} SET {assignments} WHERE {where}",
            list(cols.values()),
        )
        con.commit()
    finally:
        con.close()


def insert_group(name: str) -> None:
    """Insert a bare group row (no jobs) via direct SQL.

    AIDEV-NOTE: A group with zero jobs cannot be created through the API (a group is
    created implicitly by the first job POST), yet GET /api/groups must still return it
    with zeroed aggregates. Used by the empty-group test (spec.md 8.3 bucket 2).
    created_at uses the app's stored text format.
    """
    now = datetime.now(UTC).strftime("%Y-%m-%d %H:%M:%S.%f")
    con = sqlite3.connect(_db_path())
    try:
        con.execute(
            "INSERT INTO groups (name, staleness_enabled, created_at) VALUES (?, 0, ?)",
            (name, now),
        )
        con.commit()
    finally:
        con.close()
