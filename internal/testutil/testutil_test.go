package testutil

import (
	"bytes"
	"testing"
)

func TestNormalizeNewlines(t *testing.T) {
	input := []byte("{\r\n  \"ok\": true\r\n}\r\n")
	expected := []byte("{\n  \"ok\": true\n}\n")

	actual := normalizeNewlines(input)
	if !bytes.Equal(actual, expected) {
		t.Fatalf("unexpected newline normalization: got=%q want=%q", string(actual), string(expected))
	}
}
