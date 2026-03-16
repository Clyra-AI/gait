package main

import (
	"path/filepath"
	"testing"
)

func TestRunGateEvalDelegationEnforcement(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "delegation_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: block
rules:
  - name: allow-delegated-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.specialist]
      delegation_scopes: [write]
`)
	intentPath := filepath.Join(workDir, "intent.json")
	mustWriteFile(t, intentPath, `{
  "schema_id":"gait.gate.intent_request",
  "schema_version":"1.0.0",
  "created_at":"2026-02-11T00:00:00Z",
  "producer_version":"test",
  "tool_name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"write"}],
  "delegation":{
    "requester_identity":"agent.specialist",
    "scope_class":"write",
    "token_refs":["delegation_demo"],
    "chain":[{"delegator_identity":"agent.lead","delegate_identity":"agent.specialist","scope_class":"write"}]
  },
  "context":{"identity":"agent.specialist","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--json",
	}); code != exitPolicyBlocked {
		t.Fatalf("runGateEval missing delegation token expected %d got %d", exitPolicyBlocked, code)
	}

	delegationTokenPath := filepath.Join(workDir, "delegation_token.json")
	if code := runDelegate([]string{
		"mint",
		"--delegator", "agent.lead",
		"--delegate", "agent.specialist",
		"--scope", "tool:tool.write",
		"--scope-class", "write",
		"--ttl", "1h",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--out", delegationTokenPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runDelegate mint expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--delegation-token", delegationTokenPath,
		"--delegation-private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval with valid delegation token expected %d got %d", exitOK, code)
	}
}

func TestRunGateEvalRejectsIncompleteDelegationChain(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "delegation_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: block
rules:
  - name: allow-delegated-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.manager, agent.specialist]
      delegation_scopes: [write]
      max_delegation_depth: 2
`)
	intentPath := filepath.Join(workDir, "intent.json")
	mustWriteFile(t, intentPath, `{
  "schema_id":"gait.gate.intent_request",
  "schema_version":"1.0.0",
  "created_at":"2026-02-11T00:00:00Z",
  "producer_version":"test",
  "tool_name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"write"}],
  "delegation":{
    "requester_identity":"agent.specialist",
    "scope_class":"write",
    "token_refs":["delegation_mid","delegation_leaf"],
    "chain":[
      {"delegator_identity":"agent.lead","delegate_identity":"agent.manager","scope_class":"write","token_ref":"delegation_mid"},
      {"delegator_identity":"agent.manager","delegate_identity":"agent.specialist","scope_class":"write","token_ref":"delegation_leaf"}
    ]
  },
  "context":{"identity":"agent.specialist","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)

	delegationTokenPath := filepath.Join(workDir, "delegation_token.json")
	if code := runDelegate([]string{
		"mint",
		"--delegator", "agent.manager",
		"--delegate", "agent.specialist",
		"--scope", "tool:tool.write",
		"--scope-class", "write",
		"--ttl", "1h",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--out", delegationTokenPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runDelegate mint expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--delegation-token", delegationTokenPath,
		"--delegation-private-key", privateKeyPath,
		"--json",
	}); code != exitPolicyBlocked {
		t.Fatalf("runGateEval with incomplete delegation chain expected %d got %d", exitPolicyBlocked, code)
	}
}

func TestRunGateEvalRejectsDelegationScopeMismatch(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "delegation_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: block
rules:
  - name: allow-delegated-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.specialist]
      delegation_scopes: [write]
`)
	intentPath := filepath.Join(workDir, "intent.json")
	mustWriteFile(t, intentPath, `{
  "schema_id":"gait.gate.intent_request",
  "schema_version":"1.0.0",
  "created_at":"2026-02-11T00:00:00Z",
  "producer_version":"test",
  "tool_name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"write"}],
  "delegation":{
    "requester_identity":"agent.specialist",
    "scope_class":"write",
    "token_refs":["delegation_demo"],
    "chain":[{"delegator_identity":"agent.lead","delegate_identity":"agent.specialist","scope_class":"write","token_ref":"delegation_demo"}]
  },
  "context":{"identity":"agent.specialist","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)

	delegationTokenPath := filepath.Join(workDir, "delegation_token.json")
	if code := runDelegate([]string{
		"mint",
		"--delegator", "agent.lead",
		"--delegate", "agent.specialist",
		"--scope", "tool:tool.read",
		"--scope-class", "read",
		"--ttl", "1h",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--out", delegationTokenPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runDelegate mint expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--delegation-token", delegationTokenPath,
		"--delegation-private-key", privateKeyPath,
		"--json",
	}); code != exitPolicyBlocked {
		t.Fatalf("runGateEval with delegation scope mismatch expected %d got %d", exitPolicyBlocked, code)
	}
}

func TestRunGateEvalAllowsAnyDelegationScopeMatch(t *testing.T) {
	workDir := t.TempDir()
	withWorkingDir(t, workDir)

	privateKeyPath := filepath.Join(workDir, "delegation_private.key")
	writePrivateKey(t, privateKeyPath)

	policyPath := filepath.Join(workDir, "policy.yaml")
	mustWriteFile(t, policyPath, `default_verdict: block
rules:
  - name: allow-delegated-read-or-write
    effect: allow
    match:
      tool_names: [tool.write]
      require_delegation: true
      allowed_delegator_identities: [agent.lead]
      allowed_delegate_identities: [agent.specialist]
      delegation_scopes: [read, write]
`)
	intentPath := filepath.Join(workDir, "intent.json")
	mustWriteFile(t, intentPath, `{
  "schema_id":"gait.gate.intent_request",
  "schema_version":"1.0.0",
  "created_at":"2026-02-11T00:00:00Z",
  "producer_version":"test",
  "tool_name":"tool.write",
  "args":{"path":"/tmp/out.txt"},
  "targets":[{"kind":"path","value":"/tmp/out.txt","operation":"write"}],
  "delegation":{
    "requester_identity":"agent.specialist",
    "scope_class":"write",
    "token_refs":["delegation_demo"],
    "chain":[{"delegator_identity":"agent.lead","delegate_identity":"agent.specialist","scope_class":"write","token_ref":"delegation_demo"}]
  },
  "context":{"identity":"agent.specialist","workspace":"/repo/gait","risk_class":"high","session_id":"sess-1"}
}`)

	delegationTokenPath := filepath.Join(workDir, "delegation_token.json")
	if code := runDelegate([]string{
		"mint",
		"--delegator", "agent.lead",
		"--delegate", "agent.specialist",
		"--scope", "tool:tool.write",
		"--scope-class", "write",
		"--ttl", "1h",
		"--key-mode", "prod",
		"--private-key", privateKeyPath,
		"--out", delegationTokenPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runDelegate mint expected %d got %d", exitOK, code)
	}

	if code := runGateEval([]string{
		"--policy", policyPath,
		"--intent", intentPath,
		"--delegation-token", delegationTokenPath,
		"--delegation-private-key", privateKeyPath,
		"--json",
	}); code != exitOK {
		t.Fatalf("runGateEval with any matching delegation scope expected %d got %d", exitOK, code)
	}
}
