# Python Reference Adapter

This path is the minimal Python wrapper integration contract for `gait-py`:
one decorator for tool calls plus one run-level context manager for deterministic artifacts.

## Canonical Flow

1. decorate tool functions with `@gate_tool(...)`
2. wrap the run in `with run_session(...):`
3. execute side effects only on `allow`
4. emit a deterministic runpack and initialize regress fixture

Run from repo root:

```bash
cd sdk/python
PYTHONPATH=. uv run --python 3.13 python ../../examples/python/reference_adapter_demo.py
```

## 15-Minute Checklist

Stop if any expected field is missing:

- `tool output=...`
- `runpack run_id=... bundle=...`
- `ticket_footer=GAIT run_id=...`
- `regress fixture=... config=...`

Decorator examples:

- `sdk/python/examples/openai_style_tool_decorator.py`
- `sdk/python/examples/langchain_style_tool_decorator.py`
