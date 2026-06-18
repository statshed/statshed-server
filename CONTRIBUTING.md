# Contributing to StatShed

Thanks for your interest in StatShed! This repo is a monorepo with two parts:

- `backend/` — Flask REST API + Socket.IO server (Python 3.13, managed with [uv](https://docs.astral.sh/uv/))
- `frontend/` — React 19 + Vite + TypeScript single-page app

## Development setup

The fastest way to get a working stack is Docker:

```bash
cp .env.example .env          # set SECRET_KEY
docker compose up --build -d  # http://localhost:7827
```

Or run each side natively for hot-reload:

```bash
# Terminal 1 — backend on :7828
cd backend && uv sync && uv run python app.py

# Terminal 2 — frontend on :7827 (proxies /api and /socket.io to :7828)
cd frontend && npm ci && npm run dev
```

`make help` lists shortcut targets (`make up`, `make test`, `make e2e`, …).

## Before opening a pull request

Run the same checks CI runs:

```bash
# Backend
cd backend
uv run ruff format --check .
uv run ruff check .
uv run mypy app.py models.py config.py background.py
uv run pytest

# Frontend
cd frontend
npm run lint
npm run typecheck
npm run test:ci
npm run build
npm run test:e2e
```

Please format Python with `ruff format` and keep new code type-checked (`mypy`).
This project uses `AIDEV-NOTE:` / `AIDEV-TODO:` anchor comments — keep them updated
when you touch the code they annotate, and don't remove them without reason.

## License

StatShed is released under [CC0 1.0 Universal](LICENSE) (public domain). By
contributing, you agree that your contributions are dedicated to the public domain
under the same terms.
