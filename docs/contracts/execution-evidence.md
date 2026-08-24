---
title: "Execution and Containment Evidence"
description: "Signed, digest-bound lifecycle evidence emitted and verified at caller execution boundaries."
---

# Execution and Containment Evidence

Gait defines additive v1 artifacts for execution, effect events, containment, and compensation. Gait records and verifies evidence supplied at a caller-owned execution boundary; it does not execute tools or infer that an effect occurred.

Each new artifact binds the exact Wrkr contract family and revision, Gait activation, runtime action, readiness decision, policy, target, environment, Proof correlation references, and causal predecessor. New lifecycle event kinds enforce the sequence from verified activation through execution, validated effects, containment, and required compensation.

## Compatibility and migration

- Existing v1 lifecycle records require no conversion or rewrite. Records that use only the earlier proposal, readiness, activation, rejection, revocation, or supersession events remain valid.
- The new typed fields are optional on earlier event kinds and required only for their matching new event kinds.
- Consumers that understand the new event kinds should update to the current v1 lifecycle schema and verify embedded signatures plus causal bindings.
- Older consumers that encounter an unknown event kind must report it as unsupported or inconclusive. They must not treat an unknown execution, effect, containment, or compensation event as successful.
- Identifier-only auxiliary Proof correlation refs remain accepted where the profile content digest provides the binding. Authoritative contract and evidence relationships remain digest-bound.
- No persisted artifact is automatically upgraded. Producers emit new typed artifacts only when recording the post-activation lifecycle.

This foundation does not by itself declare Gait v1.5.0 release conformance. Exact cross-product fixtures and downstream consumer receipts remain separate release gates.
