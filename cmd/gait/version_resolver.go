package main

import (
	"regexp"
	"runtime/debug"
	"strings"
	"unicode"
)

const localDevVersion = "0.0.0-dev"

var readBuildInfo = debug.ReadBuildInfo

var (
	semverLikeVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	baseVersionPattern       = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
	revisionPattern          = regexp.MustCompile(`^[0-9a-f]{12,}$`)
)

func currentVersion() string {
	return resolveCLIVersion(version, readBuildInfo)
}

func resolveCLIVersion(stamped string, reader func() (*debug.BuildInfo, bool)) string {
	if candidate := normalizeExplicitVersion(stamped); candidate != "" && candidate != localDevVersion {
		return candidate
	}
	if reader != nil {
		if info, ok := reader(); ok {
			if candidate := trustedBuildInfoVersion(info); candidate != "" {
				return candidate
			}
		}
	}
	if candidate := normalizeExplicitVersion(stamped); candidate != "" {
		return candidate
	}
	return localDevVersion
}

func normalizeExplicitVersion(candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return ""
	}
	if normalized, ok := normalizeReleaseVersion(trimmed); ok {
		return normalized
	}
	return trimmed
}

func trustedBuildInfoVersion(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	candidate := strings.TrimSpace(info.Main.Version)
	if candidate == "" || candidate == "(devel)" || strings.Contains(candidate, "+dirty") {
		return ""
	}
	if !semverLikeVersionPattern.MatchString(candidate) || isPseudoVersion(candidate) {
		return ""
	}
	normalized, ok := normalizeReleaseVersion(candidate)
	if !ok {
		return ""
	}
	return normalized
}

func normalizeReleaseVersion(candidate string) (string, bool) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == localDevVersion {
		return "", false
	}
	if !semverLikeVersionPattern.MatchString(trimmed) {
		return "", false
	}
	return strings.TrimPrefix(trimmed, "v"), true
}

func isPseudoVersion(candidate string) bool {
	core := strings.TrimSpace(candidate)
	if buildIndex := strings.IndexByte(core, '+'); buildIndex >= 0 {
		core = core[:buildIndex]
	}

	lastDash := strings.LastIndexByte(core, '-')
	if lastDash <= 0 || lastDash == len(core)-1 {
		return false
	}
	revision := core[lastDash+1:]
	if !revisionPattern.MatchString(revision) {
		return false
	}

	prefix := core[:lastDash]
	for _, separator := range []string{"-0.", ".0.", "-"} {
		timestampStart := len(prefix) - 14
		if timestampStart <= len(separator)-1 {
			continue
		}
		if prefix[timestampStart-len(separator):timestampStart] != separator {
			continue
		}
		timestamp := prefix[timestampStart:]
		if !isAllDigits(timestamp) {
			continue
		}
		base := prefix[:timestampStart-len(separator)]
		if baseVersionPattern.MatchString(base) {
			return true
		}
	}

	return false
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
