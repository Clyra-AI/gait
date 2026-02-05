SHELL := /bin/sh

GO ?= go
PYTHON ?= python3

SDK_DIR := sdk/python
UV_PY := 3.13

.PHONY: fmt lint test test-e2e build

fmt:
	gofmt -w .
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff format)

lint:
	$(GO) vet ./...
	golangci-lint run ./...
	gosec ./...
	govulncheck ./...
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff check)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev mypy)

test:
	$(GO) test ./... -coverprofile=coverage-go.out
	(cd $(SDK_DIR) && PYTHONPATH=. uv run --python $(UV_PY) --extra dev pytest --cov=gait --cov-report=term-missing)

test-e2e:
	$(GO) build ./cmd/gait
	./gait

build:
	$(GO) build ./cmd/gait
