# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project
- `kala` is a Go 1.27 server/service (module path `kala`). It's a new project, so the structure isn't settled yet.

## Dependencies
- Stdlib first: use the standard library (`net/http`, `log/slog`, `encoding/json`, etc.). Add a third-party module only when the stdlib clearly falls short, and say why when you add it.

## Checks (run before considering a change done)
- Format: `gofmt -w .` (or `goimports -w .` if installed; it isn't currently)
- Lint: `golangci-lint run ./...` (v2 config in `.golangci.yml`)

## Git
- Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`.
