"""Test fixtures for StatShed backend tests.

AIDEV-NOTE: Tests use an in-memory SQLite database. The DATABASE_URL environment
variable is set before importing models to ensure the test database is used.
"""

import os
import sys

# Set test database URL BEFORE importing any application modules
os.environ["DATABASE_URL"] = "sqlite:///:memory:"

import pytest


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


@pytest.fixture
def client(app):
    """Create a test client."""
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
