package main

import "testing"

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
}

func TestOpenInBrowserValidation(t *testing.T) {
	if err := openInBrowser(""); err == nil {
		t.Fatalf("expected empty URL error")
	}
}
