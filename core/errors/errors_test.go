package errors

import (
	stderrors "errors"
	"testing"
)

func TestWrapRoundTrip(t *testing.T) {
	base := stderrors.New("boom")
	err := Wrap(base, CategoryIOFailure, "io_write_failed", "check directory permissions", true)
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if CategoryOf(err) != CategoryIOFailure {
		t.Fatalf("unexpected category: %s", CategoryOf(err))
	}
	if CodeOf(err) != "io_write_failed" {
		t.Fatalf("unexpected code: %s", CodeOf(err))
	}
	if HintOf(err) != "check directory permissions" {
		t.Fatalf("unexpected hint: %s", HintOf(err))
	}
	if !RetryableOf(err) {
		t.Fatal("expected retryable true")
	}
	if !stderrors.Is(err, base) {
		t.Fatal("expected wrapped error to preserve cause")
	}
}

func TestUnknownErrorDefaults(t *testing.T) {
	err := stderrors.New("plain")
	if CategoryOf(err) != "" {
		t.Fatalf("unexpected category: %s", CategoryOf(err))
	}
	if CodeOf(err) != "" {
		t.Fatalf("unexpected code: %s", CodeOf(err))
	}
	if HintOf(err) != "" {
		t.Fatalf("unexpected hint: %s", HintOf(err))
	}
	if RetryableOf(err) {
		t.Fatal("unexpected retryable true")
	}
}

func TestWrapNilCauseReturnsNil(t *testing.T) {
	if got := Wrap(nil, CategoryInternalFailure, "internal_failure", "retry later", false); got != nil {
		t.Fatalf("expected nil wrapped error, got=%v", got)
	}
}

func TestClassifiedErrorNilCauseDefaults(t *testing.T) {
	err := &classifiedError{
		category:  CategoryNetworkTransient,
		code:      "network_transient",
		hint:      "retry request",
		retryable: true,
	}
	if err.Error() != "unknown error" {
		t.Fatalf("unexpected nil-cause error text: %s", err.Error())
	}
	if err.Unwrap() != nil {
		t.Fatalf("expected unwrap nil for nil cause")
	}
	if err.Category() != CategoryNetworkTransient {
		t.Fatalf("unexpected category: %s", err.Category())
	}
	if err.Code() != "network_transient" {
		t.Fatalf("unexpected code: %s", err.Code())
	}
	if err.Hint() != "retry request" {
		t.Fatalf("unexpected hint: %s", err.Hint())
	}
	if !err.Retryable() {
		t.Fatalf("expected retryable=true")
	}
}

func TestCategorySetIsStableAndUnique(t *testing.T) {
	categories := []Category{
		CategoryInvalidInput,
		CategoryVerification,
		CategoryPolicyBlocked,
		CategoryApprovalRequired,
		CategoryDependencyMissing,
		CategoryIOFailure,
		CategoryStateContention,
		CategoryNetworkTransient,
		CategoryNetworkPermanent,
		CategoryInternalFailure,
	}
	seen := map[Category]struct{}{}
	for _, category := range categories {
		if category == "" {
			t.Fatalf("category must not be empty")
		}
		if _, exists := seen[category]; exists {
			t.Fatalf("duplicate category: %s", category)
		}
		seen[category] = struct{}{}
	}
	if len(seen) != 10 {
		t.Fatalf("expected 10 categories, got %d", len(seen))
	}
}
