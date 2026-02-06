package main

import (
	"fmt"
	"os"
)

const version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args))
}

func run(arguments []string) int {
	if len(arguments) < 2 {
		fmt.Println("gait", version)
		return exitOK
	}
	if arguments[1] == "--explain" {
		return writeExplain("Gait is an offline-first CLI for deterministic run artifacts, policy-gated tool execution, and reproducible debugging workflows.")
	}

	switch arguments[1] {
	case "approve":
		return runApprove(arguments[2:])
	case "demo":
		return runDemo(arguments[2:])
	case "doctor":
		return runDoctor(arguments[2:])
	case "gate":
		return runGate(arguments[2:])
	case "policy":
		return runPolicy(arguments[2:])
	case "trace":
		return runTrace(arguments[2:])
	case "regress":
		return runRegress(arguments[2:])
	case "run":
		return runCommand(arguments[2:])
	case "scout":
		return runScout(arguments[2:])
	case "guard":
		return runGuard(arguments[2:])
	case "incident":
		return runIncident(arguments[2:])
	case "registry":
		return runRegistry(arguments[2:])
	case "migrate":
		return runMigrate(arguments[2:])
	case "mcp":
		return runMCP(arguments[2:])
	case "verify":
		return runVerify(arguments[2:])
	case "version", "--version", "-v":
		if hasExplainFlag(arguments[2:]) {
			return writeExplain("Print the CLI version.")
		}
		fmt.Println("gait", version)
		return exitOK
	default:
		printUsage()
		return exitInvalidInput
	}
}
