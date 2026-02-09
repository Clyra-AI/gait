# Skill Provenance Contract (Normative)

Status: normative for OSS `v1.7+`.

Skill provenance allows Gate and policy to evaluate trust for skill-triggered actions without storing raw secret material.

## Purpose

When a tool call is initiated by a packaged skill, producers SHOULD attach `skill_provenance` to `IntentRequest`.

Gate copies this context into `TraceRecord` for deterministic auditability.

## `skill_provenance` fields

Required when present:

- `skill_name`
- `source`
- `publisher`

Optional:

- `skill_version`
- `digest` (sha256 hex)
- `signature_key_id`

## Validation rules

- Empty required fields are invalid.
- `digest`, when provided, must be a 64-char hex SHA-256.
- Values are normalized (`source` lowercased, whitespace trimmed).

## Policy hooks

Policy rules can match:

- `skill_publishers`
- `skill_sources`

This enables:

- allow trusted publishers
- block or require approval for unknown or untrusted sources

## Privacy and security

- Do not include raw credentials or secrets in provenance.
- Prefer immutable refs (`digest`, `signature_key_id`) over mutable metadata.
