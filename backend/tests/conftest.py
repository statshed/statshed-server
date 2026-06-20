"""Test fixtures for StatShed backend tests.

AIDEV-NOTE: Tests use an in-memory SQLite database. The DATABASE_URL environment
variable is set before importing models to ensure the test database is used.
"""

import os
import sys

# Set test database URL BEFORE importing any application modules
os.environ["DATABASE_URL"] = "sqlite:///:memory:"

import pytest
from flask.testing import FlaskClient


@pytest.fixture(scope="function")
def app():
    """Create a test Flask application with fresh database for each test."""
    # Import models and create fresh engine/session for this test
    from models import Base, create_db_engine, create_db_session

    # Create a new in-memory database for each test
    test_engine = create_db_engine("sqlite:///:memory:")
    test_session = create_db_session(test_engine)

    # Create tables
    Base.metadata.create_all(test_engine)

    # Patch the global db_session in models
    import models

    original_session = models.db_session
    original_engine = models.engine
    models.db_session = test_session
    models.engine = test_engine

    # Import app after setting up the test database
    # Need to reload app to use the patched session
    if "app" in sys.modules:
        del sys.modules["app"]
    if "background" in sys.modules:
        del sys.modules["background"]

    from app import app as flask_app

    flask_app.config["TESTING"] = True
    flask_app.config["DEBUG"] = True

    yield flask_app

    # Cleanup
    test_session.remove()
    Base.metadata.drop_all(test_engine)

    # Restore original
    models.db_session = original_session
    models.engine = original_engine


class _ApiPrefixClient(FlaskClient):
    """Test client that transparently prepends /api to request paths.

    AIDEV-NOTE: Production serves the REST API under an /api Blueprint so the
    React SPA can own root (the /jobs *page* vs the GET /jobs *API*). The test
    suite predates that split and addresses routes at root (/jobs, /status, ...).
    This wrapper injects /api so those ~371 calls keep working unchanged. Paths
    under /socket.io or already under /api are passed through untouched.
    """

    def open(self, *args, **kwargs):  # type: ignore[override]
        path = None
        pos = None
        if args and isinstance(args[0], str):
            path, pos = args[0], 0
        elif isinstance(kwargs.get("path"), str):
            path = kwargs["path"]
        if (
            path is not None
            and path.startswith("/")
            and not path.startswith("/api")
            and not path.startswith("/socket.io")
        ):
            new = "/api" + path
            if pos == 0:
                args = (new,) + args[1:]
            else:
                kwargs["path"] = new
        return super().open(*args, **kwargs)


@pytest.fixture
def client(app):
    """Test client that addresses the API at root (auto-prefixed to /api)."""
    app.test_client_class = _ApiPrefixClient
    return app.test_client()


@pytest.fixture
def raw_client(app):
    """Test client with NO prefixing — for testing real URL routing/SPA."""
    app.test_client_class = FlaskClient
    return app.test_client()


@pytest.fixture
def db_session(app):
    """Get the test database session."""
    import models

    return models.db_session


@pytest.fixture
def socketio_client(app):
    """Create a Socket.IO test client."""
    from app import socketio

    return socketio.test_client(app)
