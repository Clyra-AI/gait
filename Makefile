SHELL := /bin/sh

GO ?= go
PYTHON ?= python3
GO_COVERAGE_THRESHOLD ?= 85
PYTHON_COVERAGE_THRESHOLD ?= 85
GAIT_BINARY ?= ./gait

ifeq ($(OS),Windows_NT)
GAIT_BINARY := ./gait.exe
endif

SDK_DIR := sdk/python
UV_PY := 3.13
GO_COVERAGE_PACKAGES := ./core/... ./cmd/gait
BENCH_PACKAGES := ./core/gate ./core/runpack ./core/scout ./core/guard ./core/registry ./core/mcp
BENCH_REGEX := Benchmark(EvaluatePolicyTypical|VerifyZipTypical|DiffRunpacksTypical|SnapshotTypical|DiffSnapshotsTypical|VerifyPackTypical|BuildIncidentPackTypical|InstallLocalTypical|VerifyInstalledTypical|DecodeToolCallOpenAITypical|EvaluateToolCallTypical)$$
BENCH_OUTPUT ?= perf/bench_output.txt
BENCH_BASELINE ?= perf/bench_baseline.json

.PHONY: fmt lint lint-fast codeql test test-fast prepush prepush-full github-guardrails github-guardrails-strict test-hardening test-hardening-acceptance test-chaos test-e2e test-acceptance test-v1-6-acceptance test-v1-7-acceptance test-v1-8-acceptance test-v2-3-acceptance test-v2-4-acceptance test-packspec-tck test-ui-acceptance test-adoption test-adapter-parity test-ecosystem-automation test-release-smoke test-install test-install-path-versions test-contracts test-intent-receipt-conformance test-ci-regress-template test-live-connectors test-skill-supply-chain test-runtime-slo test-ent-consumer-contract test-uat-local test-openclaw-skill-install test-beads-bridge openclaw-skill-install build bench bench-check bench-budgets skills-validate ecosystem-validate ecosystem-release-notes demo-90s demo-hero-gif homebrew-formula wiki-publish tool-allowlist-policy ui-build ui-sync ui-deps-check
.PHONY: hooks
.PHONY: docs-site-install docs-site-build docs-site-lint

fmt:
	gofmt -w .
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff format)

lint:
	$(PYTHON) scripts/validate_repo_skills.py
	$(PYTHON) scripts/validate_community_index.py
	bash scripts/check_repo_hygiene.sh
	bash scripts/check_hooks_config.sh
	$(GO) vet ./...
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.1 run ./...
	$(GO) run github.com/securego/gosec/v2/cmd/gosec@v2.22.0 ./...
	$(GO) build -o $(GAIT_BINARY) ./cmd/gait
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest -mode=binary $(GAIT_BINARY)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff check)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev mypy)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev bandit -q -r gait)

lint-fast:
	$(PYTHON) scripts/validate_repo_skills.py
	$(PYTHON) scripts/validate_community_index.py
	bash scripts/check_repo_hygiene.sh
	bash scripts/check_hooks_config.sh
	$(GO) vet ./...
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.1 run ./...
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev ruff check)
	(cd $(SDK_DIR) && uv run --python $(UV_PY) --extra dev mypy)

codeql:
	bash scripts/run_codeql_local.sh

test:
	$(GO) test ./...
	$(GO) test $(GO_COVERAGE_PACKAGES) -coverprofile=coverage-go.out
	$(PYTHON) scripts/check_go_coverage.py coverage-go.out $(GO_COVERAGE_THRESHOLD)
	(cd $(SDK_DIR) && PYTHONPATH=. uv run --python $(UV_PY) --extra dev pytest --cov=gait --cov-report=term-missing --cov-fail-under=$(PYTHON_COVERAGE_THRESHOLD))

test-fast:
	$(GO) test ./...
	(cd $(SDK_DIR) && PYTHONPATH=. uv run --python $(UV_PY) --extra dev pytest)

prepush:
	$(MAKE) lint-fast
	$(MAKE) test-fast

prepush-full:
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) codeql

github-guardrails:
	bash scripts/configure_github_guardrails.sh

github-guardrails-strict:
	GAIT_REQUIRED_REVIEWS=1 GAIT_REQUIRE_CODEOWNER_REVIEWS=true bash scripts/configure_github_guardrails.sh

test-hardening:
	bash scripts/test_hardening.sh

test-hardening-acceptance:
	bash scripts/test_hardening_acceptance.sh

test-chaos:
	bash scripts/test_chaos_exporters.sh
	bash scripts/test_chaos_service_boundary.sh
	bash scripts/test_chaos_payload_limits.sh
	bash scripts/test_chaos_sessions.sh
	bash scripts/test_chaos_trace_uniqueness.sh
	bash scripts/test_job_runtime_chaos.sh

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

test-v1-7-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v1_7_acceptance.sh ./gait

test-v1-8-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v1_8_acceptance.sh ./gait

test-v2-3-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v2_3_acceptance.sh ./gait

test-v2-4-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_v2_4_acceptance.sh ./gait

test-packspec-tck:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_packspec_tck.sh ./gait

test-ui-acceptance:
	$(GO) build -o ./gait ./cmd/gait
	bash scripts/test_ui_acceptance.sh ./gait

test-adoption:
	bash scripts/test_adoption_smoke.sh

test-adapter-parity:
	bash scripts/test_adapter_parity.sh

test-ecosystem-automation:
	bash scripts/test_ecosystem_release_automation.sh
	bash scripts/test_ci_regress_template.sh

test-release-smoke: build
	bash scripts/test_release_smoke.sh ./gait

test-install: build
	bash scripts/test_install.sh ./gait

test-install-path-versions: build
	bash scripts/test_cli_version_install_paths.sh ./gait

test-contracts: build
	bash scripts/test_contracts.sh ./gait
	bash scripts/test_ent_consumer_contract.sh ./gait

test-intent-receipt-conformance: build
	bash scripts/test_intent_receipt_conformance.sh ./gait

test-ci-regress-template: build
	bash scripts/test_ci_regress_template.sh

test-ent-consumer-contract: build
	bash scripts/test_ent_consumer_contract.sh ./gait

test-live-connectors:
	bash scripts/test_live_connectors.sh

test-skill-supply-chain:
	bash scripts/test_skill_supply_chain.sh

test-uat-local:
	bash scripts/test_uat_local.sh

test-openclaw-skill-install:
	bash scripts/test_openclaw_skill_install.sh

test-beads-bridge:
	bash scripts/test_beads_bridge.sh

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
	$(PYTHON) scripts/check_command_budgets.py ./gait perf/command_budget_report.json perf/runtime_slo_budgets.json

test-runtime-slo: bench-budgets

hooks:
	git config core.hooksPath .githooks

skills-validate:
	$(PYTHON) scripts/validate_repo_skills.py

ecosystem-validate:
	$(PYTHON) scripts/validate_community_index.py

ecosystem-release-notes:
	$(PYTHON) scripts/render_ecosystem_release_notes.py

demo-90s: build
	bash scripts/demo_90s.sh

demo-hero-gif: build
	bash scripts/record_runpack_hero_demo.sh

homebrew-formula:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required (example: make homebrew-formula VERSION=vX.Y.Z)"; exit 2; fi
	bash scripts/render_homebrew_formula.sh --version "$(VERSION)" --checksums dist/checksums.txt --out dist/gait.rb

wiki-publish:
	@if [ -z "$(REPO)" ]; then REPO=davidahmann/gait; else REPO=$(REPO); fi; \
	bash scripts/publish_wiki.sh --repo "$$REPO"

tool-allowlist-policy:
	@if [ -z "$(INPUT)" ]; then echo "INPUT is required (example: make tool-allowlist-policy INPUT=examples/policy/external_tool_allowlist.json OUTPUT=gait-out/policy_external_allowlist.yaml)"; exit 2; fi
	@if [ -z "$(OUTPUT)" ]; then echo "OUTPUT is required (example: make tool-allowlist-policy INPUT=examples/policy/external_tool_allowlist.json OUTPUT=gait-out/policy_external_allowlist.yaml)"; exit 2; fi
	python3 scripts/render_tool_allowlist_policy.py --input "$(INPUT)" --output "$(OUTPUT)"

openclaw-skill-install:
	@if [ -z "$(TARGET_DIR)" ]; then bash scripts/install_openclaw_skill.sh; else bash scripts/install_openclaw_skill.sh --target-dir "$(TARGET_DIR)"; fi

docs-site-install:
	cd docs-site && npm ci

docs-site-build:
	cd docs-site && npm run build

docs-site-lint:
	cd docs-site && npm run lint

ui-build:
	bash scripts/ui_build.sh

ui-sync:
	bash scripts/ui_sync_assets.sh

ui-deps-check:
	bash scripts/check_ui_deps_freshness.sh
