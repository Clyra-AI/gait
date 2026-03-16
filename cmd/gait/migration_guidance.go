package main

import (
	"fmt"
	"strings"

	gatecore "github.com/Clyra-AI/gait/core/gate"
)

const (
	legacyCommandPrefix = "deprecated command spelling: "
	legacyFlagPrefix    = "deprecated flag spelling: "
	legacyPolicyPrefix  = "legacy policy proposal fields are not supported ["
)

type migrationMetadata struct {
	Kind               string                                `json:"kind"`
	DeprecatedCommand  string                                `json:"deprecated_command,omitempty"`
	ReplacementCommand string                                `json:"replacement_command,omitempty"`
	DeprecatedFlag     string                                `json:"deprecated_flag,omitempty"`
	ReplacementFlag    string                                `json:"replacement_flag,omitempty"`
	DeprecatedFields   []string                              `json:"deprecated_fields,omitempty"`
	FieldReplacements  []gatecore.LegacyPolicyFieldMigration `json:"field_replacements,omitempty"`
}

func legacyCommandError(deprecated string, replacement string) string {
	return legacyCommandPrefix + strings.TrimSpace(deprecated) + " -> " + strings.TrimSpace(replacement)
}

func legacyFlagError(deprecated string, replacement string) string {
	return legacyFlagPrefix + strings.TrimSpace(deprecated) + " -> " + strings.TrimSpace(replacement)
}

func migrationMetadataFromError(errorText string) (string, *migrationMetadata) {
	trimmed := strings.TrimSpace(errorText)
	switch {
	case strings.HasPrefix(trimmed, legacyCommandPrefix):
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, legacyCommandPrefix))
		parts := strings.SplitN(body, "->", 2)
		if len(parts) != 2 {
			return "", nil
		}
		deprecated := strings.TrimSpace(parts[0])
		replacement := strings.TrimSpace(parts[1])
		return fmt.Sprintf("use `%s` instead", replacement), &migrationMetadata{
			Kind:               "command",
			DeprecatedCommand:  deprecated,
			ReplacementCommand: replacement,
		}
	case strings.HasPrefix(trimmed, legacyFlagPrefix):
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, legacyFlagPrefix))
		parts := strings.SplitN(body, "->", 2)
		if len(parts) != 2 {
			return "", nil
		}
		deprecated := strings.TrimSpace(parts[0])
		replacement := strings.TrimSpace(parts[1])
		return fmt.Sprintf("use `%s` instead", replacement), &migrationMetadata{
			Kind:            "flag",
			DeprecatedFlag:  deprecated,
			ReplacementFlag: replacement,
		}
	case strings.Contains(trimmed, legacyPolicyPrefix):
		fieldNames := parseLegacyPolicyFieldNames(trimmed)
		if len(fieldNames) == 0 {
			return "", nil
		}
		replacements := gatecore.LegacyPolicyFieldMigrations(fieldNames)
		return "use the repo-root `.gait.yaml` contract: `schema_id`, `schema_version`, `default_verdict`, optional `fail_closed`, optional `mcp_trust`, and `rules`", &migrationMetadata{
			Kind:              "policy_fields",
			DeprecatedFields:  fieldNames,
			FieldReplacements: replacements,
		}
	default:
		return "", nil
	}
}

func parseLegacyPolicyFieldNames(errorText string) []string {
	start := strings.Index(errorText, legacyPolicyPrefix)
	if start == -1 {
		return nil
	}
	start += len(legacyPolicyPrefix)
	end := strings.Index(errorText[start:], "]")
	if end == -1 {
		return nil
	}
	body := errorText[start : start+end]
	parts := strings.Split(body, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fields = append(fields, trimmed)
	}
	return fields
}

func containsArgument(arguments []string, needle string) bool {
	for _, argument := range arguments {
		trimmed := strings.TrimSpace(argument)
		if trimmed == needle || strings.HasPrefix(trimmed, needle+"=") {
			return true
		}
	}
	return false
}

func writeRootInputError(arguments []string, errorText string) int {
	output := map[string]any{
		"ok":    false,
		"error": strings.TrimSpace(errorText),
	}
	if hasJSONFlag(arguments[1:]) {
		return writeJSONOutput(output, exitInvalidInput)
	}
	fmt.Printf("error: %s\n", strings.TrimSpace(errorText))
	if hint, _ := migrationMetadataFromError(errorText); hint != "" {
		fmt.Printf("hint: %s\n", hint)
	}
	printUsage()
	return exitInvalidInput
}
