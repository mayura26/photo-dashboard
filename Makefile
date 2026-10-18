# Home HQ — local build & run
# Requires: go, make (Git Bash / MSYS2 on Windows)

.PHONY: help run build test clean version

BIN := bin/server
APP_VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

ifneq (,$(wildcard .env))
include .env
export
endif

help:
	@echo "Targets:"
	@echo "  make run    — go run with .env loaded (if present)"
	@echo "  make build  — compile to $(BIN)"
	@echo "  make test   — go test ./..."
	@echo "  make clean  — remove bin/"
	@echo "  make version — write web/version.json from git SHA"

version:
	@printf '{"version":"%s"}\n' "$(APP_VERSION)" > web/version.json
	@echo "web/version.json → $(APP_VERSION)"

run: version
	APP_VERSION=$(APP_VERSION) go run ./cmd/server

build: version
	@mkdir -p bin
	APP_VERSION=$(APP_VERSION) go build -o $(BIN) ./cmd/server

test:
	go test ./...

clean:
	rm -rf bin
