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
