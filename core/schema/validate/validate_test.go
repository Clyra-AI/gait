package validate

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateJSONFile(t *testing.T) {
	root := repoRoot(t)
	schema := filepath.Join(root, "schemas", "v1", "runpack", "manifest.schema.json")
	valid := filepath.Join(root, "core", "schema", "testdata", "manifest_valid.json")
	invalid := filepath.Join(root, "core", "schema", "testdata", "manifest_invalid.json")

	if err := ValidateJSONFile(schema, valid); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
	if err := ValidateJSONFile(schema, invalid); err == nil {
		t.Fatalf("expected invalid manifest to fail")
	}
}

func TestValidateJSONLFile(t *testing.T) {
	root := repoRoot(t)
	schema := filepath.Join(root, "schemas", "v1", "runpack", "intent.schema.json")
	valid := filepath.Join(root, "core", "schema", "testdata", "intent_valid.jsonl")
	invalid := filepath.Join(root, "core", "schema", "testdata", "intent_invalid.jsonl")

	if err := ValidateJSONLFile(schema, valid); err != nil {
		t.Fatalf("expected valid jsonl, got error: %v", err)
	}
	if err := ValidateJSONLFile(schema, invalid); err == nil {
		t.Fatalf("expected invalid jsonl to fail")
	}
}

func TestValidateSchemaFixtures(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		name       string
		schemaPath string
		validPath  string
		invalid    string
		isJSONL    bool
	}{
		{
			name:       "run",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "run.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "run_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "run_invalid.json"),
		},
		{
			name:       "result",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "result.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "result_valid.jsonl"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "result_invalid.jsonl"),
			isJSONL:    true,
		},
		{
			name:       "refs",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "refs.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "refs_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "refs_invalid.json"),
		},
		{
			name:       "gate_intent_request",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "intent_request.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_invalid.json"),
		},
		{
			name:       "gate_intent_request_endpoint",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "intent_request.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_endpoint_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_endpoint_invalid.json"),
		},
		{
			name:       "gate_intent_request_skill_provenance",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "intent_request.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_skill_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_skill_invalid.json"),
		},
		{
			name:       "gate_intent_request_enterprise_context",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "intent_request.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_enterprise_context_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_intent_request_enterprise_context_invalid.json"),
		},
		{
			name:       "gate_result",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "gate_result.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_result_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_result_invalid.json"),
		},
		{
			name:       "gate_policy",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "policy.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_policy_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_policy_invalid.json"),
		},
		{
			name:       "gate_trace_record",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "trace_record.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_trace_record_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_trace_record_invalid.json"),
		},
		{
			name:       "gate_trace_record_skill_provenance",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "trace_record.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_trace_record_skill_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_trace_record_skill_invalid.json"),
		},
		{
			name:       "gate_approval_token",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "approval_token.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_approval_token_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_approval_token_invalid.json"),
		},
		{
			name:       "gate_delegation_token",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "delegation_token.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_delegation_token_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_delegation_token_invalid.json"),
		},
		{
			name:       "gate_approval_audit_record",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "approval_audit_record.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_approval_audit_record_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_approval_audit_record_invalid.json"),
		},
		{
			name:       "gate_delegation_audit_record",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "delegation_audit_record.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_delegation_audit_record_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_delegation_audit_record_invalid.json"),
		},
		{
			name:       "gate_broker_credential_record",
			schemaPath: filepath.Join(root, "schemas", "v1", "gate", "broker_credential_record.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "gate_broker_credential_record_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "gate_broker_credential_record_invalid.json"),
		},
		{
			name:       "policy_test_result",
			schemaPath: filepath.Join(root, "schemas", "v1", "policytest", "policy_test_result.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "policy_test_result_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "policy_test_result_invalid.json"),
		},
		{
			name:       "regress_result",
			schemaPath: filepath.Join(root, "schemas", "v1", "regress", "regress_result.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "regress_result_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "regress_result_invalid.json"),
		},
		{
			name:       "scout_inventory_snapshot",
			schemaPath: filepath.Join(root, "schemas", "v1", "scout", "inventory_snapshot.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "scout_inventory_snapshot_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "scout_inventory_snapshot_invalid.json"),
		},
		{
			name:       "scout_adoption_event",
			schemaPath: filepath.Join(root, "schemas", "v1", "scout", "adoption_event.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "scout_adoption_event_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "scout_adoption_event_invalid.json"),
		},
		{
			name:       "scout_operational_event",
			schemaPath: filepath.Join(root, "schemas", "v1", "scout", "operational_event.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "scout_operational_event_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "scout_operational_event_invalid.json"),
		},
		{
			name:       "scout_run_fingerprint",
			schemaPath: filepath.Join(root, "schemas", "v1", "scout", "run_fingerprint.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "scout_run_fingerprint_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "scout_run_fingerprint_invalid.json"),
		},
		{
			name:       "scout_signal_report",
			schemaPath: filepath.Join(root, "schemas", "v1", "scout", "signal_report.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "scout_signal_report_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "scout_signal_report_invalid.json"),
		},
		{
			name:       "guard_pack_manifest",
			schemaPath: filepath.Join(root, "schemas", "v1", "guard", "pack_manifest.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "guard_pack_manifest_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "guard_pack_manifest_invalid.json"),
		},
		{
			name:       "pack_manifest",
			schemaPath: filepath.Join(root, "schemas", "v1", "pack", "manifest.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "pack_manifest_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "pack_manifest_invalid.json"),
		},
		{
			name:       "pack_run",
			schemaPath: filepath.Join(root, "schemas", "v1", "pack", "run.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "pack_run_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "pack_run_invalid.json"),
		},
		{
			name:       "pack_job",
			schemaPath: filepath.Join(root, "schemas", "v1", "pack", "job.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "pack_job_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "pack_job_invalid.json"),
		},
		{
			name:       "pack_diff",
			schemaPath: filepath.Join(root, "schemas", "v1", "pack", "diff.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "pack_diff_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "pack_diff_invalid.json"),
		},
		{
			name:       "guard_retention_report",
			schemaPath: filepath.Join(root, "schemas", "v1", "guard", "retention_report.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "guard_retention_report_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "guard_retention_report_invalid.json"),
		},
		{
			name:       "guard_encrypted_artifact",
			schemaPath: filepath.Join(root, "schemas", "v1", "guard", "encrypted_artifact.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "guard_encrypted_artifact_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "guard_encrypted_artifact_invalid.json"),
		},
		{
			name:       "registry_pack",
			schemaPath: filepath.Join(root, "schemas", "v1", "registry", "registry_pack.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "registry_pack_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "registry_pack_invalid.json"),
		},
		{
			name:       "registry_verification_report",
			schemaPath: filepath.Join(root, "schemas", "v1", "registry", "verification_report.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "registry_verification_report_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "registry_verification_report_invalid.json"),
		},
		{
			name:       "runpack_session_journal",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "session_journal.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "session_journal_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "session_journal_invalid.json"),
		},
		{
			name:       "runpack_session_checkpoint",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "session_checkpoint.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "session_checkpoint_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "session_checkpoint_invalid.json"),
		},
		{
			name:       "runpack_session_chain",
			schemaPath: filepath.Join(root, "schemas", "v1", "runpack", "session_chain.schema.json"),
			validPath:  filepath.Join(root, "core", "schema", "testdata", "session_chain_valid.json"),
			invalid:    filepath.Join(root, "core", "schema", "testdata", "session_chain_invalid.json"),
		},
	}

	for _, c := range cases {
		if c.isJSONL {
			if err := ValidateJSONLFile(c.schemaPath, c.validPath); err != nil {
				t.Fatalf("expected valid %s, got error: %v", c.name, err)
			}
			if err := ValidateJSONLFile(c.schemaPath, c.invalid); err == nil {
				t.Fatalf("expected invalid %s to fail", c.name)
			}
			continue
		}
		if err := ValidateJSONFile(c.schemaPath, c.validPath); err != nil {
			t.Fatalf("expected valid %s, got error: %v", c.name, err)
		}
		if err := ValidateJSONFile(c.schemaPath, c.invalid); err == nil {
			t.Fatalf("expected invalid %s to fail", c.name)
		}
	}
}

func TestValidateJSON(t *testing.T) {
	root := repoRoot(t)
	schema := filepath.Join(root, "schemas", "v1", "runpack", "manifest.schema.json")
	valid := []byte(`{
		"schema_id":"gait.runpack.manifest",
		"schema_version":"1.0.0",
		"created_at":"2026-02-05T00:00:00Z",
		"producer_version":"0.0.0-dev",
		"run_id":"run_demo",
		"capture_mode":"reference",
		"files":[{"path":"run.json","sha256":"0000000000000000000000000000000000000000000000000000000000000000"}],
		"manifest_digest":"1111111111111111111111111111111111111111111111111111111111111111"
	}`)
	invalid := []byte(`{`)

	if err := ValidateJSON(schema, valid); err != nil {
		t.Fatalf("expected valid json, got error: %v", err)
	}
	if err := ValidateJSON(schema, invalid); err == nil {
		t.Fatalf("expected invalid json to fail")
	}
}

func TestValidateJSONL(t *testing.T) {
	root := repoRoot(t)
	schema := filepath.Join(root, "schemas", "v1", "runpack", "intent.schema.json")
	data := []byte("\n" +
		`{"schema_id":"gait.runpack.intent","schema_version":"1.0.0","created_at":"2026-02-05T00:00:00Z","producer_version":"0.0.0-dev","run_id":"run_demo","intent_id":"intent_1","tool_name":"tool.demo","args_digest":"2222222222222222222222222222222222222222222222222222222222222222","args":{"foo":"bar"}}` +
		"\n")
	if err := ValidateJSONL(schema, data); err != nil {
		t.Fatalf("expected valid jsonl, got error: %v", err)
	}
}

func TestValidateSchemaMissing(t *testing.T) {
	err := ValidateJSONFile("does-not-exist.json", "also-missing.json")
	if err == nil {
		t.Fatalf("expected error for missing schema file")
	}
}

func repoRoot(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to locate test file")
	}
	dir := filepath.Dir(filename)
	return filepath.Clean(filepath.Join(dir, "..", "..", ".."))
}
