# SDK Docs

Gait SDKs are adoption layers over the local Go CLI contracts.

- Python SDK contract and usage: `docs/sdk/python.md`

Guardrails:

- SDKs do not re-implement policy logic.
- SDKs call local `gait` commands and return structured outputs.
- Core trust contracts remain Go CLI artifacts and schemas.
