package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveCLIVersionPrefersStampedRelease(t *testing.T) {
	got := resolveCLIVersion("1.2.3", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v9.9.9"},
		}, true
	})
	if got != "1.2.3" {
		t.Fatalf("resolveCLIVersion stamped release: got %q want %q", got, "1.2.3")
	}
}

func TestResolveCLIVersionUsesTrustedBuildInfoRelease(t *testing.T) {
	got := resolveCLIVersion(localDevVersion, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.3.5"},
		}, true
	})
	if got != "1.3.5" {
		t.Fatalf("resolveCLIVersion build info release: got %q want %q", got, "1.3.5")
	}
}

func TestResolveCLIVersionIgnoresUntrustedBuildInfo(t *testing.T) {
	testCases := []struct {
		name    string
		version string
	}{
		{name: "devel", version: "(devel)"},
		{name: "pseudo release bump", version: "v1.3.6-0.20260318215907-c90958e0d34e"},
		{name: "pseudo no prior tag", version: "v1.0.0-20260318215907-c90958e0d34e"},
		{name: "pseudo prerelease", version: "v1.3.5-rc.1.0.20260318215907-c90958e0d34e"},
		{name: "dirty", version: "v1.3.6-0.20260318215907-c90958e0d34e+dirty"},
		{name: "empty", version: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := resolveCLIVersion(localDevVersion, func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{
					Main: debug.Module{Version: testCase.version},
				}, true
			})
			if got != localDevVersion {
				t.Fatalf("resolveCLIVersion(%q): got %q want %q", testCase.version, got, localDevVersion)
			}
		})
	}
}

func TestNormalizeReleaseVersion(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expected  string
		normalize bool
	}{
		{name: "plain semver", input: "1.2.3", expected: "1.2.3", normalize: true},
		{name: "tagged semver", input: "v1.2.3", expected: "1.2.3", normalize: true},
		{name: "prerelease", input: "v1.2.3-rc.1", expected: "1.2.3-rc.1", normalize: true},
		{name: "dev", input: localDevVersion, normalize: false},
		{name: "custom", input: "feature-branch", normalize: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := normalizeReleaseVersion(testCase.input)
			if ok != testCase.normalize {
				t.Fatalf("normalizeReleaseVersion(%q) ok=%t want %t", testCase.input, ok, testCase.normalize)
			}
			if got != testCase.expected {
				t.Fatalf("normalizeReleaseVersion(%q) got %q want %q", testCase.input, got, testCase.expected)
			}
		})
	}
}
