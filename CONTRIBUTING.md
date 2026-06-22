# Contributing to StatShed

Thanks for your interest in StatShed! This repo is a monorepo:

- `cmd/statshed-server/` + `internal/` — the **Go server**: REST API, SSE stream, and
  embedded SPA host.
- `frontend/` — React 19 + Vite + TypeScript single-page app (built into the Go binary).
- `contracttest/` — an HTTP contract suite that drives the Go server over HTTP.

## Development setup

The fastest way to a working stack is Docker (builds from source):

```bash
docker compose up --build -d   # http://localhost:7827   (or: make up)
```

For hot-reload while working on the dashboard, run the Go server and the Vite dev server
side by side:

```bash
# Terminal 1 — Go server on :7828
go run ./cmd/statshed-server

# Terminal 2 — Vite dev server on :7827 (proxies /api and /api/events to :7828)
cd frontend && npm ci && npm run dev
```

Or `make dev` to run the Go server with a freshly built, embedded dashboard (no frontend
hot-reload). `make help` lists all shortcut targets.

## Before opening a pull request

Run the same checks CI runs. For the Go server and dashboard (the common case):

```bash
# Go server
go build ./...
go vet ./...
golangci-lint run ./...     # also enforces gofmt/goimports formatting
go test -race ./...

# Frontend
cd frontend
npm run lint
npm run typecheck
npm run test:ci
npm run build
```

Cross-server and end-to-end gates:

```bash
make contract-test                  # HTTP contract suite against the Go server
make e2e                            # hermetic Playwright e2e
make live-e2e                       # non-mocked live-SSE proxy + reconnect gate
```

This project uses `AIDEV-NOTE:` / `AIDEV-TODO:` anchor comments — keep them updated when you
touch the code they annotate, and don't remove them without reason.

## License

StatShed is released under [CC0 1.0 Universal](LICENSE) (public domain). By contributing, you
agree that your contributions are dedicated to the public domain under the same terms.
