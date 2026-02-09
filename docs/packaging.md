# OSS And Enterprise Packaging Boundary

This document defines what is in OSS v1 versus enterprise packaging, without changing runtime semantics.

## OSS v1 (Free Forever Core)

OSS is the default product surface and remains fully useful offline:

- `runpack`: capture, verify, diff, replay (stub default)
- `regress`: incident-to-regression workflow with CI-friendly outputs
- `gate`: execution-boundary policy evaluation with approvals and traces
- `doctor`: first-run diagnostics and fix guidance
- `scout`: local inventory and signal reports
- adapter examples, sidecar path, and CI templates

Contract commitments for OSS:

- offline-first core workflows
- default-safe privacy posture
- fail-closed execution posture for protected paths
- stable artifact schemas and exit codes within major versions

## Enterprise Packaging (v2 Direction)

Enterprise packaging is a separate layer for organizations operating many teams and agents at scale.

Typical enterprise-only capabilities:

- centralized policy distribution and approval workflows
- fleet-level inventory, coverage, and drift posture
- long-horizon retention, compliance automation, and governance integrations

## Non-Negotiable Boundary Rules

- OSS execution semantics are not downgraded by enterprise packaging.
- Enterprise consumes OSS artifacts and contracts; it does not redefine them.
- No multi-tenant control-plane dependency is introduced into OSS runtime commands.
- Vendor neutrality and local verifiability remain first-class OSS properties.
