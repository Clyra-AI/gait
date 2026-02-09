SHELL := /bin/sh

GO ?= go
PYTHON ?= python3
GO_COVERAGE_THRESHOLD ?= 85
PYTHON_COVERAGE_THRESHOLD ?= 85

SDK_DIR := sdk/python
UV_PY := 3.13
GO_COVERAGE_PACKAGES := ./core/... ./cmd/gait
BENCH_PACKAGES := ./core/gate ./core/runpack ./core/scout ./core/guard ./core/registry ./core/mcp
BENCH_REGEX := Benchmark(EvaluatePolicyTypical|VerifyZipTypical|DiffRunpacksTypical|SnapshotTypical|DiffSnapshotsTypical|VerifyPackTypical|BuildIncidentPackTypical|InstallLocalTypical|VerifyInstalledTypical|DecodeToolCallOpenAITypical|EvaluateToolCallTypical)$$
BENCH_OUTPUT ?= perf/bench_output.txt
BENCH_BASELINE ?= perf/bench_baseline.json

.PHONY: fmt lint test test-hardening test-hardening-acceptance test-e2e test-acceptance test-v1-6-acceptance test-adoption test-release-smoke test-contracts test-live-connectors test-skill-supply-chain build bench bench-check bench-budgets skills-validate
.PHONY: hooks

fmt:
	gofmt -w .
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff format)

lint:
	$(PYTHON) scripts/validate_repo_skills.py
	bash scripts/check_repo_hygiene.sh
	bash scripts/check_hooks_config.sh
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

test-hardening:
	bash scripts/test_hardening.sh

test-hardening-acceptance:
	bash scripts/test_hardening_acceptance.sh

coverage:
	$(GO) test $(GO_COVERAGE_PACKAGES) -coverprofile=coverage-go.out
	$(GO) tool cover -func=coverage-go.out | tail -n 1

test-e2e:
	$(GO) test ./internal/e2e -count=1

test-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v1_acceptance.sh ./gait

test-v1-6-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v1_6_acceptance.sh ./gait

test-adoption:
	bash scripts/test_adoption_smoke.sh

test-release-smoke: build
	bash scripts/test_release_smoke.sh ./gait

test-contracts: build
	bash scripts/test_contracts.sh ./gait

test-live-connectors:
	bash scripts/test_live_connectors.sh

test-skill-supply-chain:
	bash scripts/test_skill_supply_chain.sh

build:
	$(GO) build ./cmd/gait

bench:
	mkdir -p perf
	GOMAXPROCS=1 $(GO) test $(BENCH_PACKAGES) -run '^$$' -bench '$(BENCH_REGEX)' -benchmem -count=5 | tee $(BENCH_OUTPUT)

bench-check: bench
	$(PYTHON) scripts/check_bench_regression.py $(BENCH_OUTPUT) $(BENCH_BASELINE) perf/bench_report.json
	$(PYTHON) scripts/check_resource_budgets.py $(BENCH_OUTPUT) perf/resource_budgets.json perf/resource_budget_report.json

bench-budgets:
	$(GO) build -o ./gait ./cmd/gait
	$(PYTHON) scripts/check_command_budgets.py ./gait perf/command_budget_report.json

hooks:
	git config core.hooksPath .githooks

skills-validate:
	$(PYTHON) scripts/validate_repo_skills.py
