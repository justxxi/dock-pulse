.PHONY: dev build test lint web docker clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -s -w \
	-X github.com/owner/dock-pulse/internal/version.Version=$(VERSION) \
	-X github.com/owner/dock-pulse/internal/version.Commit=$(COMMIT) \
	-X github.com/owner/dock-pulse/internal/version.BuildDate=$(BUILD_DATE)

web:
	cd web && npm install && npm run build

build: web
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/dock-pulse ./cmd/dock-pulse

dev:
	go run ./cmd/dock-pulse -listen-addr 127.0.0.1:8080 -log-level debug

test:
	go test -v ./...
	cd web && npm test

lint:
	golangci-lint run
	cd web && npm run lint

docker:
	docker build -t dock-pulse:latest .

clean:
	rm -rf bin/ internal/web/dist web/node_modules
