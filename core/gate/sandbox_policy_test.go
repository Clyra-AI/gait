package gate

import (
	"strings"
	"testing"

	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func TestEvaluatePolicySandboxConstraints(t *testing.T) {
	policy, err := ParsePolicyYAML([]byte(`
default_verdict: block
rules:
  - name: allow-sandboxed-exec
    priority: 10
    effect: allow
    sandbox:
      allowed_network_modes: [disabled, egress_allowlist]
      allowed_writable_path_prefixes: [/tmp/work, /var/tmp/cache]
      required_read_only_roots: [/repo, /usr/share]
      allowed_env_exposure_modes: [none, allowlist]
      max_timeout_seconds: 60
      allowed_filesystem_isolations: [workspace, container]
      allowed_user_modes: [unprivileged]
    match:
      endpoint_classes: [proc.exec]
      risk_classes: [high]
`))
	if err != nil {
		t.Fatalf("parse sandbox policy: %v", err)
	}

	tests := []struct {
		name        string
		mutate      func(*schemagate.IntentRequest)
		wantVerdict string
		wantReason  string
		wantStatus  string
	}{
		{
			name:        "valid_sandbox_allows",
			mutate:      func(intent *schemagate.IntentRequest) {},
			wantVerdict: "allow",
			wantStatus:  "valid",
		},
		{
			name: "missing_sandbox_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.Sandbox = nil
			},
			wantVerdict: "block",
			wantReason:  "sandbox_metadata_missing",
			wantStatus:  "missing",
		},
		{
			name: "full_network_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.Sandbox.NetworkMode = "full"
			},
			wantVerdict: "block",
			wantReason:  "sandbox_network_mode_disallowed",
			wantStatus:  "disallowed",
		},
		{
			name: "writable_path_outside_prefix_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.Sandbox.WritablePaths = []string{"/etc"}
			},
			wantVerdict: "block",
			wantReason:  "sandbox_writable_path_disallowed",
			wantStatus:  "disallowed",
		},
		{
			name: "full_env_exposure_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.Sandbox.EnvExposureMode = "full"
			},
			wantVerdict: "block",
			wantReason:  "sandbox_env_exposure_mode_disallowed",
			wantStatus:  "disallowed",
		},
		{
			name: "timeout_above_max_blocks",
			mutate: func(intent *schemagate.IntentRequest) {
				intent.Context.Sandbox.TimeoutSeconds = 120
			},
			wantVerdict: "block",
			wantReason:  "sandbox_timeout_exceeded",
			wantStatus:  "disallowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := sandboxPolicyIntent()
			test.mutate(&intent)

			outcome, evalErr := EvaluatePolicyDetailed(policy, intent, EvalOptions{ProducerVersion: "test"})
			if evalErr != nil {
				t.Fatalf("evaluate sandbox policy: %v", evalErr)
			}
			if outcome.Result.Verdict != test.wantVerdict {
				t.Fatalf("unexpected verdict: %#v", outcome.Result)
			}
			if test.wantReason != "" && !contains(outcome.Result.ReasonCodes, test.wantReason) {
				t.Fatalf("expected reason %q in %#v", test.wantReason, outcome.Result.ReasonCodes)
			}
			if outcome.Sandbox == nil || outcome.Sandbox.Status != test.wantStatus {
				t.Fatalf("unexpected sandbox decision: %#v", outcome.Sandbox)
			}
			if outcome.Sandbox != nil && outcome.Sandbox.Status == "valid" {
				if outcome.Sandbox.EvidenceDigest != strings.Repeat("a", 64) {
					t.Fatalf("unexpected sandbox evidence digest: %#v", outcome.Sandbox)
				}
			}
		})
	}
}

func sandboxPolicyIntent() schemagate.IntentRequest {
	intent := baseIntent()
	intent.ToolName = "tool.exec"
	intent.Context.RiskClass = "high"
	intent.Targets = []schemagate.IntentTarget{
		{Kind: "path", Value: "/tmp/work/run.sh", Operation: "execute", EndpointClass: "proc.exec"},
	}
	intent.Context.Sandbox = &schemagate.SandboxMetadata{
		NetworkMode:         "egress_allowlist",
		WritablePaths:       []string{"/tmp/work/tmp"},
		ReadOnlyRoots:       []string{"/repo", "/usr/share"},
		EnvExposureMode:     "allowlist",
		TimeoutSeconds:      30,
		FilesystemIsolation: "container",
		UserMode:            "unprivileged",
		EvidenceRef:         "sandbox:receipt:v1",
		EvidenceDigest:      strings.Repeat("a", 64),
	}
	return intent
}

func TestSandboxDecisionHelpers(t *testing.T) {
	if !sandboxPathHasPrefix("/tmp/work/tmp", "/tmp/work") {
		t.Fatalf("expected writable path prefix match")
	}
	if sandboxPathHasPrefix("/etc", "/tmp/work") {
		t.Fatalf("expected non-matching writable path prefix")
	}
	if status := classifySandboxDecisionStatus([]string{"sandbox_metadata_missing"}); status != "missing" {
		t.Fatalf("expected missing status, got %q", status)
	}
	if status := classifySandboxDecisionStatus([]string{"sandbox_timeout_exceeded"}); status != "disallowed" {
		t.Fatalf("expected disallowed status, got %q", status)
	}
	if status := classifySandboxDecisionStatus([]string{"sandbox_metadata_invalid"}); status != "invalid" {
		t.Fatalf("expected invalid status, got %q", status)
	}

	valid := &schemagate.SandboxDecision{Status: "valid", EvidenceDigest: strings.Repeat("b", 64)}
	missing := &schemagate.SandboxDecision{Status: "missing", EvidenceDigest: strings.Repeat("a", 64)}
	if picked := pickSandboxDecision(valid, missing); picked != missing {
		t.Fatalf("expected missing sandbox decision to outrank valid, got %#v", picked)
	}
	disallowed := &schemagate.SandboxDecision{Status: "disallowed", EvidenceDigest: strings.Repeat("c", 64)}
	if picked := pickSandboxDecision(disallowed, valid); picked != disallowed {
		t.Fatalf("expected disallowed sandbox decision to remain selected, got %#v", picked)
	}
}
