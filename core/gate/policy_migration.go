package gate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

type LegacyPolicyFieldMigration struct {
	Field             string   `json:"field"`
	ReplacementFields []string `json:"replacement_fields,omitempty"`
	Note              string   `json:"note,omitempty"`
}

type LegacyPolicyContractError struct {
	Fields []LegacyPolicyFieldMigration
}

func (err LegacyPolicyContractError) Error() string {
	if len(err.Fields) == 0 {
		return "legacy policy proposal fields are not supported"
	}
	fieldNames := make([]string, 0, len(err.Fields))
	details := make([]string, 0, len(err.Fields))
	for _, field := range err.Fields {
		fieldNames = append(fieldNames, field.Field)
		if len(field.ReplacementFields) == 0 {
			if strings.TrimSpace(field.Note) == "" {
				details = append(details, field.Field+"->remove")
				continue
			}
			details = append(details, fmt.Sprintf("%s->%s", field.Field, field.Note))
			continue
		}
		detail := fmt.Sprintf("%s->%s", field.Field, strings.Join(field.ReplacementFields, "|"))
		if strings.TrimSpace(field.Note) != "" {
			detail += " (" + field.Note + ")"
		}
		details = append(details, detail)
	}
	return fmt.Sprintf(
		"legacy policy proposal fields are not supported [%s]: %s",
		strings.Join(fieldNames, ", "),
		strings.Join(details, "; "),
	)
}

var legacyPolicyFieldOrder = []string{
	"version",
	"name",
	"boundaries",
	"defaults",
	"trust_sources",
	"unknown_server",
}

var legacyPolicyFieldDetails = map[string]LegacyPolicyFieldMigration{
	"version": {
		Field:             "version",
		ReplacementFields: []string{"schema_id", "schema_version"},
		Note:              "set schema_id=gait.gate.policy and schema_version=1.0.0",
	},
	"name": {
		Field: "name",
		Note:  "remove the top-level policy name; rule names remain under rules[].name when needed",
	},
	"boundaries": {
		Field:             "boundaries",
		ReplacementFields: []string{"rules", "mcp_trust"},
	},
	"defaults": {
		Field:             "defaults",
		ReplacementFields: []string{"default_verdict", "fail_closed"},
	},
	"trust_sources": {
		Field:             "trust_sources",
		ReplacementFields: []string{"mcp_trust.snapshot"},
		Note:              "render external trust data to a local snapshot before evaluation",
	},
	"unknown_server": {
		Field:             "unknown_server",
		ReplacementFields: []string{"mcp_trust.action"},
		Note:              "enforce unknown servers through local snapshot coverage and a fail-closed action",
	},
}

func LegacyPolicyFieldMigrations(fieldNames []string) []LegacyPolicyFieldMigration {
	seen := map[string]struct{}{}
	for _, fieldName := range fieldNames {
		trimmed := strings.TrimSpace(fieldName)
		if trimmed == "" {
			continue
		}
		if _, ok := legacyPolicyFieldDetails[trimmed]; !ok {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	details := make([]LegacyPolicyFieldMigration, 0, len(seen))
	orderIndex := map[string]int{}
	for idx, field := range legacyPolicyFieldOrder {
		orderIndex[field] = idx
	}
	ordered := make([]string, 0, len(seen))
	for field := range seen {
		ordered = append(ordered, field)
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, leftOK := orderIndex[ordered[i]]
		right, rightOK := orderIndex[ordered[j]]
		switch {
		case leftOK && rightOK:
			return left < right
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return ordered[i] < ordered[j]
		}
	})
	for _, field := range ordered {
		details = append(details, legacyPolicyFieldDetails[field])
	}
	return details
}

func detectLegacyPolicyFieldMigrations(data []byte) ([]LegacyPolicyFieldMigration, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}
	root, ok := raw.(map[string]any)
	if !ok {
		return nil, nil
	}
	fieldNames := make([]string, 0, len(root))
	for key := range root {
		trimmed := strings.TrimSpace(key)
		if _, exists := legacyPolicyFieldDetails[trimmed]; exists {
			fieldNames = append(fieldNames, trimmed)
		}
	}
	if len(fieldNames) == 0 {
		return nil, nil
	}
	return LegacyPolicyFieldMigrations(fieldNames), nil
}
