package main

import (
	"strings"
	"testing"
)

func TestRunUIValidationAndHelp(t *testing.T) {
	if code := runUI([]string{"--help"}); code != exitOK {
		t.Fatalf("run ui help: expected %d got %d", exitOK, code)
	}
	if code := runUI([]string{"--listen", "0.0.0.0:7980"}); code != exitInvalidInput {
		t.Fatalf("run ui non-loopback guard: expected %d got %d", exitInvalidInput, code)
	}
}

func TestRunUIInputValidation(t *testing.T) {
	if code := runUI([]string{"--listen", ""}); code != exitInvalidInput {
		t.Fatalf("run ui empty listen: expected %d got %d", exitInvalidInput, code)
	}
	if code := runUI([]string{"--listen", "127.0.0.1:7980", "extra"}); code != exitInvalidInput {
		t.Fatalf("run ui unexpected args: expected %d got %d", exitInvalidInput, code)
	}
	if code := runUI([]string{"--listen", "localhost"}); code != exitInvalidInput {
		t.Fatalf("run ui invalid listen address: expected %d got %d", exitInvalidInput, code)
	}
}

func TestRunUIExplainAndOutputWriter(t *testing.T) {
	if code := runUI([]string{"--explain"}); code != exitOK {
		t.Fatalf("run ui explain: expected %d got %d", exitOK, code)
	}
	if code := writeUIOutput(true, uiOutput{OK: true, URL: "http://127.0.0.1:7980"}, exitOK); code != exitOK {
		t.Fatalf("writeUIOutput json: expected %d got %d", exitOK, code)
	}
	if code := writeUIOutput(false, uiOutput{OK: true}, exitOK); code != exitOK {
		t.Fatalf("writeUIOutput text ok: expected %d got %d", exitOK, code)
	}
	raw := captureStdout(t, func() {
		if code := writeUIOutput(false, uiOutput{OK: false, Error: "bad"}, exitInvalidInput); code != exitInvalidInput {
			t.Fatalf("writeUIOutput text err: expected %d got %d", exitInvalidInput, code)
		}
	})
	if !strings.Contains(raw, "ui error: bad") {
		t.Fatalf("expected ui error output, got %q", raw)
	}
}

func TestOpenInBrowserValidation(t *testing.T) {
	if err := openInBrowser(""); err == nil {
		t.Fatalf("expected empty URL error")
	}
}
