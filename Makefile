.PHONY: build web go ext dev-api dev-web test lint fmt clean

BIN ?= kala

## build: frontend + single binary with the SPA embedded
build: web go

web:
	cd web && npm ci && npm run build

## ext: Chrome extension into extension/dist (load it unpacked)
ext:
	cd extension && npm ci && npm run build

go:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/kala

## dev-api: API on :8080 with dev cookies and open signup
dev-api:
	KALA_DEV=true KALA_ALLOW_SIGNUP=true KALA_DB_PATH=kala-dev.db go run ./cmd/kala

## dev-web: Vite on :5173, proxying /api and /mcp to :8080
dev-web:
	cd web && npm run dev

test:
	go test ./...
	cd web && npm test
	cd extension && npm test

lint:
	gofmt -l cmd internal web/embed.go | (! grep .)
	golangci-lint run ./...
	cd web && npm run lint && npm run typecheck
	cd extension && npm run lint && npm run typecheck

fmt:
	gofmt -w .

clean:
	rm -f $(BIN)
	find web/dist -mindepth 1 ! -name .gitkeep -delete
	rm -rf extension/dist
