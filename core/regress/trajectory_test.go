package regress

import (
	"testing"

	"github.com/davidahmann/gait/core/runpack"
	schemarunpack "github.com/davidahmann/gait/core/schema/v1/runpack"
)

func TestToolSequenceFromRunpack(t *testing.T) {
	bundle := runpack.Runpack{
		Intents: []schemarunpack.IntentRecord{
			{ToolName: "tool.search"},
			{ToolName: "  "},
			{ToolName: "tool.write"},
		},
	}
	sequence := toolSequenceFromRunpack(bundle)
	if len(sequence) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(sequence))
	}
	if sequence[0] != "tool.search" || sequence[1] != "tool.write" {
		t.Fatalf("unexpected tool sequence: %#v", sequence)
	}
}

func TestVerdictSequenceFromRunpack(t *testing.T) {
	bundle := runpack.Runpack{
		Results: []schemarunpack.ResultRecord{
			{Result: map[string]any{"verdict": "allow"}},
			{Result: map[string]any{"verdict": "needs_approval"}},
			{Status: "blocked"},
			{Status: "simulate"},
			{Status: "unknown"},
		},
	}
	sequence := verdictSequenceFromRunpack(bundle)
	expected := []string{"allow", "require_approval", "block", "dry_run", "error"}
	if !equalOrderedSequence(expected, sequence) {
		t.Fatalf("unexpected verdict sequence: expected %#v got %#v", expected, sequence)
	}
}

func TestDeriveTrajectoryVerdictPrefersStructuredVerdict(t *testing.T) {
	result := schemarunpack.ResultRecord{
		Status: "blocked",
		Result: map[string]any{"verdict": "allow"},
	}
	if got := deriveTrajectoryVerdict(result); got != "allow" {
		t.Fatalf("expected structured verdict to win, got %q", got)
	}
}

func TestDeriveTrajectoryVerdictFromStatusFallbacks(t *testing.T) {
	for _, candidate := range []struct {
		status string
		want   string
	}{
		{status: "ok", want: "allow"},
		{status: "passed", want: "allow"},
		{status: "deny", want: "block"},
		{status: "approval_required", want: "require_approval"},
		{status: "dry-run", want: "dry_run"},
		{status: "simulated", want: "dry_run"},
		{status: "other", want: "error"},
	} {
		t.Run(candidate.status, func(t *testing.T) {
			if got := deriveTrajectoryVerdict(schemarunpack.ResultRecord{Status: candidate.status}); got != candidate.want {
				t.Fatalf("expected %q got %q", candidate.want, got)
			}
		})
	}
}

func TestNormalizeTrajectorySequence(t *testing.T) {
	normalized := normalizeTrajectorySequence([]string{" tool.a ", "", "tool.b", "   "})
	expected := []string{"tool.a", "tool.b"}
	if !equalOrderedSequence(expected, normalized) {
		t.Fatalf("unexpected normalized sequence: %#v", normalized)
	}
}

func TestNormalizeTrajectoryVerdictSequence(t *testing.T) {
	normalized, err := normalizeTrajectoryVerdictSequence([]string{"allow", "needs_approval", "dry run", "failure"})
	if err != nil {
		t.Fatalf("normalize verdict sequence: %v", err)
	}
	expected := []string{"allow", "require_approval", "dry_run", "error"}
	if !equalOrderedSequence(expected, normalized) {
		t.Fatalf("unexpected normalized verdict sequence: %#v", normalized)
	}

	_, err = normalizeTrajectoryVerdictSequence([]string{"allow", "unknown_verdict"})
	if err == nil {
		t.Fatalf("expected invalid verdict error")
	}
}

func TestNormalizeTrajectoryVerdictAliases(t *testing.T) {
	for _, candidate := range []struct {
		input string
		want  string
	}{
		{input: "allow", want: "allow"},
		{input: "PASS", want: "allow"},
		{input: "blocked", want: "block"},
		{input: "needs approval", want: "require_approval"},
		{input: "dry-run", want: "dry_run"},
		{input: "failed", want: "error"},
		{input: "unknown", want: ""},
	} {
		t.Run(candidate.input, func(t *testing.T) {
			if got := normalizeTrajectoryVerdict(candidate.input); got != candidate.want {
				t.Fatalf("expected %q got %q", candidate.want, got)
			}
		})
	}
}

func TestSequenceHelpers(t *testing.T) {
	if !equalOrderedSequence([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatalf("expected equal ordered sequence")
	}
	if equalOrderedSequence([]string{"a", "b"}, []string{"a"}) {
		t.Fatalf("expected unequal sequence by length")
	}
	if equalOrderedSequence([]string{"a", "b"}, []string{"a", "c"}) {
		t.Fatalf("expected unequal sequence by value")
	}

	index, expected, actual := firstSequenceMismatch([]string{"a", "b"}, []string{"a", "c"})
	if index != 1 || expected != "b" || actual != "c" {
		t.Fatalf("unexpected mismatch values: %d %q %q", index, expected, actual)
	}

	index, expected, actual = firstSequenceMismatch([]string{"a", "b"}, []string{"a"})
	if index != 1 || expected != "b" || actual != "<missing>" {
		t.Fatalf("unexpected missing actual mismatch: %d %q %q", index, expected, actual)
	}

	index, expected, actual = firstSequenceMismatch([]string{"a"}, []string{"a", "b"})
	if index != 1 || expected != "<missing>" || actual != "b" {
		t.Fatalf("unexpected missing expected mismatch: %d %q %q", index, expected, actual)
	}

	index, expected, actual = firstSequenceMismatch([]string{"a"}, []string{"a"})
	if index != -1 || expected != "<none>" || actual != "<none>" {
		t.Fatalf("unexpected no mismatch values: %d %q %q", index, expected, actual)
	}
}
