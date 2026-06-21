# Contributing to StatShed

Thanks for your interest in StatShed! This repo is a monorepo:

- `cmd/` + `internal/` — the StatShed server, written in Go (1.26+). It serves the REST API
  under `/api`, the Server-Sent Events stream at `/api/events`, and the embedded React SPA,
  all from one binary backed by SQLite.
- `frontend/` — React 19 + Vite + TypeScript single-page app (built into the binary).
- `contract/` — the black-box HTTP contract suite that drives a running server over HTTP.

## Development setup

The fastest way to get a working stack is Docker:

```bash
cp .env.example .env          # SECRET_KEY is optional; the Go server ignores it
docker compose up --build -d  # http://localhost:7827
```

Or run it natively. `make dev` builds the SPA into the binary and runs the server with the
real dashboard on `:7828`:

```bash
make dev                      # builds the SPA, then `go run ./cmd/statshed-server`
```

For frontend hot-reload, run the Go API and the Vite dev server side by side:

```bash
# Terminal 1 — API only on :7828 (Vite serves the SPA, so disable the embedded one)
STATIC_DISABLED=1 DATABASE_URL=sqlite:///dev.db go run ./cmd/statshed-server

# Terminal 2 — frontend on :7827 (Vite proxies /api to :7828)
cd frontend && npm ci && npm run dev
```

> **Fresh-DB-only:** the server creates and migrates an **empty** SQLite database on first
> start and refuses a pre-existing one (the production cutover behavior, C1). In dev, point
> `DATABASE_URL` at a throwaway path and delete it between runs (`rm -f dev.db*`).

`make help` lists shortcut targets (`make up`, `make test`, `make lint`, `make e2e`,
`make contract-test`, `make live-e2e`, …).

## Before opening a pull request

Run the same checks CI runs:

```bash
# Go server
go build ./...
go vet ./...
golangci-lint run ./...        # also enforces gofmt/goimports
go test -race ./...            # or: make test

# Frontend
cd frontend
npm run lint
npm run typecheck
npm run test:ci
npm run build
npm run test:e2e
cd ..

# Contract suite (boots the Go binary, drives it over HTTP, across every profile)
for p in default max_page_size max_log_lines log_disabled no_spa with_spa; do
  make contract-test TARGET=go PROFILE=$p
done

# Live SSE gate (non-mocked: real server through the Vite proxy + reconnect)
make live-e2e
```

Keep new Go code `gofmt`-clean with reasonable type signatures; the race detector
(`-race`) is the baseline for the concurrent worker and SSE hub. This project uses
`AIDEV-NOTE:` / `AIDEV-TODO:` / `AIDEV-QUESTION:` anchor comments — keep them updated when
you touch the code they annotate, and don't remove them without reason.

## License

StatShed is released under [CC0 1.0 Universal](LICENSE) (public domain). By
contributing, you agree that your contributions are dedicated to the public domain
under the same terms.
