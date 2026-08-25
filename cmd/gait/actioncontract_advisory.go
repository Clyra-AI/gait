package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

type advisoryCLIOutput struct {
	OK     bool                           `json:"ok"`
	Report *actioncontract.AdvisoryReport `json:"report,omitempty"`
	Error  string                         `json:"error,omitempty"`
}

func runActionContractAdvisory(args []string) int {
	if hasExplainFlag(args) {
		return writeExplain("Evaluate or verify signed, advisory-only Action Contract reports. Advisory reports never authorize execution.")
	}
	if len(args) == 0 || isTopLevelHelp(args[0]) {
		fmt.Println("Usage: gait action-contract advisory evaluate|verify")
		return exitOK
	}
	switch args[0] {
	case "evaluate":
		return runAdvisoryEvaluate(args[1:])
	case "verify":
		return runAdvisoryVerify(args[1:])
	default:
		return exitInvalidInput
	}
}
func runAdvisoryEvaluate(args []string) int {
	f := flag.NewFlagSet("advisory-evaluate", flag.ContinueOnError)
	var input, out, key, action, contract, corr string
	var js bool
	f.StringVar(&input, "input", "", "input JSON")
	f.StringVar(&out, "out", "", "report output")
	f.StringVar(&key, "private-key", "", "Ed25519 private key")
	f.StringVar(&action, "action-id", "", "Action Contract action ID")
	f.StringVar(&contract, "contract-digest", "", "expected contract digest")
	f.StringVar(&corr, "correlation-digest", "", "expected correlation digest")
	f.BoolVar(&js, "json", false, "JSON")
	if err := f.Parse(args); err != nil {
		return advisoryOutput(js, nil, err.Error(), exitInvalidInput)
	}
	if input == "" || out == "" || key == "" || action == "" {
		return advisoryOutput(js, nil, "--input, --out, --private-key, and --action-id are required", exitInvalidInput)
	}
	raw, e := os.ReadFile(input)
	if e != nil {
		return advisoryOutput(js, nil, e.Error(), exitInvalidInput)
	}
	var in actioncontract.AdvisoryInput
	if e = json.Unmarshal(raw, &in); e != nil {
		return advisoryOutput(js, nil, e.Error(), exitInvalidInput)
	}
	in.ActionID = action
	in.ContractDigest = contract
	in.CorrelationDigest = corr
	r, e := (actioncontract.OfflineAdvisoryEvaluator{}).Evaluate(in)
	if e == nil {
		var k []byte
		k, e = proofsign.LoadPrivateKeyBase64(key)
		if e == nil {
			r, e = r.Sign(k)
		}
	}
	if e == nil {
		var b []byte
		b, e = json.Marshal(r)
		if e == nil {
			e = os.WriteFile(out, b, 0600)
		}
	}
	return advisoryOutput(js, &r, errString(e), mapErr(e))
}
func runAdvisoryVerify(args []string) int {
	f := flag.NewFlagSet("advisory-verify", flag.ContinueOnError)
	var path, pub, contract, corr string
	var js bool
	f.StringVar(&path, "report", "", "report JSON")
	f.StringVar(&pub, "trusted-key", "", "trusted Ed25519 public key")
	f.StringVar(&contract, "expected-contract-digest", "", "expected contract digest")
	f.StringVar(&corr, "expected-correlation-digest", "", "expected correlation digest")
	f.BoolVar(&js, "json", false, "JSON")
	if err := f.Parse(args); err != nil {
		return advisoryOutput(js, nil, err.Error(), exitInvalidInput)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return advisoryOutput(js, nil, e.Error(), exitInvalidInput)
	}
	var r actioncontract.AdvisoryReport
	if e = json.Unmarshal(raw, &r); e != nil {
		return advisoryOutput(js, nil, e.Error(), exitInvalidInput)
	}
	k, e := proofsign.LoadPublicKeyBase64(pub)
	if e != nil {
		return advisoryOutput(js, nil, e.Error(), exitInvalidInput)
	}
	e = actioncontract.VerifyAdvisoryReport(r, k, contract, corr)
	if e != nil {
		return advisoryOutput(js, &r, e.Error(), exitVerifyFailed)
	}
	return advisoryOutput(js, &r, "", exitOK)
}
func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
func mapErr(e error) int {
	if e == nil {
		return exitOK
	}
	return exitInvalidInput
}
func advisoryOutput(js bool, r *actioncontract.AdvisoryReport, e string, code int) int {
	o := advisoryCLIOutput{OK: e == "", Report: r, Error: e}
	if js {
		b, _ := json.Marshal(o)
		fmt.Println(string(b))
	} else if e != "" {
		fmt.Println("advisory error: " + e)
	} else {
		fmt.Println("advisory report: " + r.Status)
	}
	return code
}
