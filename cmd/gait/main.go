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

	switch arguments[1] {
	case "approve":
		return runApprove(arguments[2:])
	case "demo":
		return runDemo(arguments[2:])
	case "gate":
		return runGate(arguments[2:])
	case "trace":
		return runTrace(arguments[2:])
	case "regress":
		return runRegress(arguments[2:])
	case "run":
		return runCommand(arguments[2:])
	case "verify":
		return runVerify(arguments[2:])
	case "version", "--version", "-v":
		fmt.Println("gait", version)
		return exitOK
	default:
		printUsage()
		return exitInvalidInput
	}
}
