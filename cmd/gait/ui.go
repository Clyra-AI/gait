package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	coreui "github.com/Clyra-AI/gait/core/ui"
)

type uiOutput struct {
	OK      bool   `json:"ok"`
	Listen  string `json:"listen,omitempty"`
	URL     string `json:"url,omitempty"`
	OpenURL bool   `json:"open_url,omitempty"`
	Error   string `json:"error,omitempty"`
}

func runUI(arguments []string) int {
	if hasExplainFlag(arguments) {
		return writeExplain("Run an optional localhost UI that orchestrates existing Gait commands for first-run, artifact, and regression workflows.")
	}
	arguments = reorderInterspersedFlags(arguments, map[string]bool{
		"listen":             true,
		"open-browser":       true,
		"allow-non-loopback": false,
	})

	flagSet := flag.NewFlagSet("ui", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var listenAddr string
	var openBrowser bool
	var allowNonLoopback bool
	var jsonOutput bool
	var helpFlag bool

	flagSet.StringVar(&listenAddr, "listen", "127.0.0.1:7980", "listen address for localhost UI server")
	flagSet.BoolVar(&openBrowser, "open-browser", true, "open UI URL in default browser after startup")
	flagSet.BoolVar(&allowNonLoopback, "allow-non-loopback", false, "allow non-loopback listen addresses")
	flagSet.BoolVar(&jsonOutput, "json", false, "emit startup JSON")
	flagSet.BoolVar(&helpFlag, "help", false, "show help")

	if err := flagSet.Parse(arguments); err != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInvalidInput))
	}
	if helpFlag {
		printUIUsage()
		return exitOK
	}
	if len(flagSet.Args()) > 0 {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: "unexpected positional arguments"}, exitInvalidInput)
	}
	listenAddr = strings.TrimSpace(listenAddr)
	if listenAddr == "" {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: "--listen must not be empty"}, exitInvalidInput)
	}
	isLoopback, loopbackErr := mcpServeIsLoopbackListen(listenAddr)
	if loopbackErr != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: loopbackErr.Error()}, exitInvalidInput)
	}
	if !isLoopback && !allowNonLoopback {
		return writeUIOutput(jsonOutput, uiOutput{
			OK:    false,
			Error: "non-loopback --listen requires --allow-non-loopback",
		}, exitInvalidInput)
	}

	executablePath, executableErr := os.Executable()
	if executableErr != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: executableErr.Error()}, exitCodeForError(executableErr, exitInternalFailure))
	}
	staticHandler, staticErr := coreui.NewStaticHandler()
	if staticErr != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: staticErr.Error()}, exitCodeForError(staticErr, exitInternalFailure))
	}
	apiHandler, apiErr := coreui.NewHandler(coreui.Config{
		ExecutablePath: executablePath,
		WorkDir:        ".",
		CommandTimeout: 2 * time.Minute,
	}, staticHandler)
	if apiErr != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: apiErr.Error()}, exitCodeForError(apiErr, exitInternalFailure))
	}

	listener, listenErr := net.Listen("tcp", listenAddr)
	if listenErr != nil {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: listenErr.Error()}, exitCodeForError(listenErr, exitInternalFailure))
	}
	url := "http://" + listener.Addr().String()
	if jsonOutput {
		if code := writeUIOutput(jsonOutput, uiOutput{
			OK:      true,
			Listen:  listener.Addr().String(),
			URL:     url,
			OpenURL: openBrowser,
		}, exitOK); code != exitOK {
			_ = listener.Close()
			return code
		}
	} else {
		fmt.Printf("ui listening=%s\n", listener.Addr().String())
		fmt.Printf("ui url=%s\n", url)
	}

	if openBrowser {
		_ = openInBrowser(url)
	}
	server := &http.Server{
		Handler:           apiHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !strings.Contains(err.Error(), "closed network connection") {
		return writeUIOutput(jsonOutput, uiOutput{OK: false, Error: err.Error()}, exitCodeForError(err, exitInternalFailure))
	}
	return exitOK
}

func writeUIOutput(jsonOutput bool, output uiOutput, exitCode int) int {
	if jsonOutput {
		return writeJSONOutput(output, exitCode)
	}
	if output.OK {
		return exitCode
	}
	fmt.Printf("ui error: %s\n", output.Error)
	return exitCode
}

func printUIUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gait ui [--listen 127.0.0.1:7980] [--open-browser=true|false] [--allow-non-loopback] [--json] [--explain]")
}

func openInBrowser(url string) error {
	trimmedURL := strings.TrimSpace(url)
	if trimmedURL == "" {
		return fmt.Errorf("empty url")
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", trimmedURL) // #nosec G204
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", trimmedURL) // #nosec G204
	default:
		command = exec.Command("xdg-open", trimmedURL) // #nosec G204
	}
	return command.Start()
}
