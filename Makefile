SHELL := /bin/sh

GO ?= go
PYTHON ?= python3
GO_COVERAGE_THRESHOLD ?= 85
PYTHON_COVERAGE_THRESHOLD ?= 85

SDK_DIR := sdk/python
UV_PY := 3.13
GO_COVERAGE_PACKAGES := ./core/... ./cmd/gait

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
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev bandit -q -r gait)

test:
	$(GO) test ./...
	$(GO) test $(GO_COVERAGE_PACKAGES) -coverprofile=coverage-go.out
	$(PYTHON) scripts/check_go_coverage.py coverage-go.out $(GO_COVERAGE_THRESHOLD)
	(cd $(SDK_DIR) && PYTHONPATH=. uv run --python $(UV_PY) --extra dev pytest --cov=gait --cov-report=term-missing --cov-fail-under=$(PYTHON_COVERAGE_THRESHOLD))

coverage:
	$(GO) test $(GO_COVERAGE_PACKAGES) -coverprofile=coverage-go.out
	$(GO) tool cover -func=coverage-go.out | tail -n 1

test-e2e:
	$(GO) test ./internal/e2e -count=1

build:
	$(GO) build ./cmd/gait

hooks:
	git config core.hooksPath .githooks
