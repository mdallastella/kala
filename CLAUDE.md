# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project
- `kala` is a Go 1.27 server/service (module path `kala`) with a React + MUI frontend in `web/` (embedded into the binary from `web/dist`).
- `extension/` is a Chrome (MV3) extension, Preact + Vite, that brings quick add (with an optional timer) to any page over the REST API with a personal access token.
- Business rules live in `internal/service`, shared by the REST API (`internal/api`) and the MCP server (`internal/mcp`); see README.md for the domain model.

## Dependencies
- Stdlib first: use the standard library (`net/http`, `log/slog`, `encoding/json`, etc.). Add a third-party module only when the stdlib clearly falls short, and say why when you add it.

## Checks (run before considering a change done)
- Format: `gofmt -w .` (or `goimports -w .` if installed; it isn't currently)
- Lint: `golangci-lint run ./...` (v2 config in `.golangci.yml`)
- Test: `go test ./...`
- Frontend (in `web/` and `extension/`): `npm run lint`, `npm run typecheck`, `npm test`
- `make lint` / `make test` run all of the above
- CI (`.github/workflows/docker.yml`) gates the image push on Trivy: fixable CRITICAL/HIGH in the repo or built image fails the build. Accepted findings go in `.trivyignore` with a comment. Commands to preview locally are in README.md ("CI security gate").

## Git
- Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`.
