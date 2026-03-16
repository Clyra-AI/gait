# LangChain Quickstart (Official Middleware)

This guide shows the shipped LangChain lane:

1. Build a real `create_agent(..., middleware=[GaitLangChainMiddleware(...)])` agent.
2. Enforce only in `wrap_tool_call`.
3. Surface additive correlation metadata with an optional callback handler.
4. Capture one Gait runpack for the whole agent run with `run_session(...)`.

## Run

From repo root:

```bash
go build -o ./gait ./cmd/gait
(cd sdk/python && uv sync --extra langchain --extra dev)
(cd sdk/python && uv run --python 3.11 --extra langchain python ../../examples/integrations/langchain/quickstart.py --scenario allow)
(cd sdk/python && uv run --python 3.11 --extra langchain python ../../examples/integrations/langchain/quickstart.py --scenario block)
(cd sdk/python && uv run --python 3.11 --extra langchain python ../../examples/integrations/langchain/quickstart.py --scenario require_approval)
```

Expected allow output:

```text
framework=langchain
scenario=allow
verdict=allow
executed=true
trace_path=/.../gait-out/integrations/langchain/trace_allow.json
executor_output=/.../gait-out/integrations/langchain/executor_allow.json
runpack_path=/.../gait-out/integrations/langchain/runpacks/runpack_run_langchain_allow.zip
```

Expected block output:

```text
framework=langchain
scenario=block
verdict=block
executed=false
trace_path=/.../gait-out/integrations/langchain/trace_block.json
```

Expected approval output:

```text
framework=langchain
scenario=require_approval
verdict=require_approval
executed=false
trace_path=/.../gait-out/integrations/langchain/trace_require_approval.json
```

Deterministic artifacts:

- `gait-out/integrations/langchain/trace_allow.json`
- `gait-out/integrations/langchain/trace_block.json`
- `gait-out/integrations/langchain/trace_require_approval.json`
- `gait-out/integrations/langchain/runpacks/runpack_run_langchain_allow.zip`
- `gait-out/integrations/langchain/runpacks/runpack_run_langchain_block.zip`
- `gait-out/integrations/langchain/runpacks/runpack_run_langchain_require_approval.zip`

Language contract:

- official wording: middleware with optional callback correlation
- callbacks do not make allow or block decisions
- only `allow` executes the tool
