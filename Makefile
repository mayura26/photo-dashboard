# Home HQ — local build & run
# Requires: go, make (Git Bash / MSYS2 on Windows)

.PHONY: help run watch build test clean version garmin-run

BIN := bin/server
APP_VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

ifneq (,$(wildcard .env))
include .env
export
endif

help:
	@echo "Targets:"
	@echo "  make run        — go run with .env loaded (if present)"
	@echo "  make watch      — auto-restart Go server on .go changes (Air)"
	@echo "  make build      — compile to $(BIN)"
	@echo "  make test       — go test ./..."
	@echo "  make clean      — remove bin/ and tmp/"
	@echo "  make version    — write web/version.json from git SHA"
	@echo "  make garmin-run — run Garmin sidecar (services/garmin/.venv)"

version:
	@printf '{"version":"%s"}\n' "$(APP_VERSION)" > web/version.json
	@echo "web/version.json → $(APP_VERSION)"

run: version
	APP_VERSION=$(APP_VERSION) go run ./cmd/server

watch: version
	go run github.com/air-verse/air@latest -c .air.toml

build: version
	@mkdir -p bin
	APP_VERSION=$(APP_VERSION) go build -o $(BIN) ./cmd/server

test:
	go test ./...

clean:
	rm -rf bin tmp

garmin-run:
	@if [ -x services/garmin/.venv/Scripts/python.exe ]; then \
		services/garmin/.venv/Scripts/python.exe services/garmin/poller.py; \
	elif [ -x services/garmin/.venv/bin/python ]; then \
		services/garmin/.venv/bin/python services/garmin/poller.py; \
	else \
		echo "Create venv first:"; \
		echo "  cd services/garmin && python -m venv .venv"; \
		echo "  .venv/Scripts/activate   # Windows"; \
		echo "  # or: source .venv/bin/activate"; \
		echo "  pip install -r requirements.txt"; \
		exit 1; \
	fi
