package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

func runActionContractOTel(args []string) int {
	f := flag.NewFlagSet("action-contract-otel", flag.ContinueOnError)
	var in, out, version, keyPath string
	var js bool
	f.StringVar(&in, "lifecycle", "", "lifecycle JSON array")
	f.StringVar(&out, "otel-out", "", "OTEL JSONL output")
	f.StringVar(&version, "source-version", "0.0.0-dev", "source version")
	f.StringVar(&keyPath, "trusted-key", "", "trusted lifecycle public key")
	f.BoolVar(&js, "json", false, "JSON")
	if e := f.Parse(args); e != nil {
		return writeOTelCLI(js, e.Error(), exitInvalidInput)
	}
	if in == "" || out == "" || keyPath == "" || version == "" {
		return writeOTelCLI(js, "--lifecycle and --otel-out are required", exitInvalidInput)
	}
	raw, e := actioncontract.ReadRuntimeInput(in)
	if e != nil {
		return writeOTelCLI(js, e.Error(), exitInvalidInput)
	}
	var records []actioncontract.LifecycleRecord
	if e = actioncontract.DecodeStrictRuntimeJSON(raw, &records); e != nil {
		return writeOTelCLI(js, e.Error(), exitInvalidInput)
	}
	pub, e := proofsign.LoadPublicKeyBase64(keyPath)
	if e != nil {
		return writeOTelCLI(js, e.Error(), exitVerifyFailed)
	}
	if _, e = actioncontract.ReduceVerifiedLifecycle(records, pub); e != nil {
		return writeOTelCLI(js, e.Error(), exitVerifyFailed)
	}
	e = actioncontract.ExportLifecycleOTel(out, records, version)
	if e != nil {
		return writeOTelCLI(js, e.Error(), exitInternalFailure)
	}
	return writeOTelCLI(js, "", exitOK)
}
func writeOTelCLI(js bool, e string, code int) int {
	if js {
		b, _ := json.Marshal(map[string]any{"ok": e == "", "error": e})
		fmt.Println(string(b))
	} else if e != "" {
		fmt.Println("action-contract otel error: " + e)
	} else {
		fmt.Println("action-contract otel export ok")
	}
	return code
}
