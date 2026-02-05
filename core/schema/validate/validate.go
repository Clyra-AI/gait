package validate

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/kaptinlin/jsonschema"
)

func ValidateJSONFile(schemaPath, jsonPath string) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return err
	}
	// #nosec G304 -- caller supplies local file path for validation by design
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read json: %w", err)
	}
	return validateJSON(schema, data)
}

func ValidateJSON(schemaPath string, data []byte) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return err
	}
	return validateJSON(schema, data)
}

func ValidateJSONLFile(schemaPath, jsonlPath string) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return err
	}
	// #nosec G304 -- caller supplies local file path for validation by design
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		return fmt.Errorf("read jsonl: %w", err)
	}
	return validateJSONL(schema, data)
}

func ValidateJSONL(schemaPath string, data []byte) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return err
	}
	return validateJSONL(schema, data)
}

func loadSchema(schemaPath string) (*jsonschema.Schema, error) {
	// #nosec G304 -- caller supplies local file path for validation by design
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	schema, err := compiler.Compile(data)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return schema, nil
}

func validateJSON(schema *jsonschema.Schema, data []byte) error {
	result := schema.ValidateJSON(data)
	if result.IsValid() {
		return nil
	}
	return fmt.Errorf("schema validation failed: %v", result.Errors)
}

func validateJSONL(schema *jsonschema.Schema, data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		b := bytes.TrimSpace(scanner.Bytes())
		if len(b) == 0 {
			continue
		}
		if err := validateJSON(schema, b); err != nil {
			return fmt.Errorf("jsonl line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read jsonl: %w", err)
	}
	return nil
}
