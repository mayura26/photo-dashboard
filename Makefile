# Home HQ — local build & run
# Requires: go, make (Git Bash / MSYS2 on Windows)

.PHONY: help run build test clean

BIN := bin/server

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

run:
	go run ./cmd/server

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/server

test:
	go test ./...

clean:
	rm -rf bin
