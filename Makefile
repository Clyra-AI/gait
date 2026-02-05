SHELL := /bin/sh

GO ?= go
PYTHON ?= python3

SDK_DIR := sdk/python
UV_PY := 3.13

.PHONY: fmt lint test test-e2e build
.PHONY: hooks

fmt:
	gofmt -w .
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff format)

lint:
	$(GO) vet ./...
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.1 run ./...
	$(GO) run github.com/securego/gosec/v2/cmd/gosec@v2.22.0 ./...
	$(GO) build ./cmd/gait
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest -mode=binary ./gait
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff check)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev mypy)

test:
	$(GO) test ./...
	$(GO) test ./core/... -coverprofile=coverage-go.out
	(cd $(SDK_DIR) && PYTHONPATH=. uv run --python $(UV_PY) --extra dev pytest --cov=gait --cov-report=term-missing)

coverage:
	$(GO) test ./core/... -coverprofile=coverage-go.out
	$(GO) tool cover -func=coverage-go.out | tail -n 1

test-e2e:
	$(GO) build ./cmd/gait
	./gait

build:
	$(GO) build ./cmd/gait

hooks:
	git config core.hooksPath .githooks
