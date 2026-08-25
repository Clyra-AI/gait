package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/fsx"
	"os"
	"strings"
)

func runActionContractChain(a []string) int {
	if len(a) == 0 || a[0] != "evaluate" {
		fmt.Println("Usage: gait action-contract chain evaluate --policy policy.json --state state.json --candidate candidate.json [--out decision.json] [--json]")
		return exitInvalidInput
	}
	f := flag.NewFlagSet("chain-evaluate", flag.ContinueOnError)
	var pp, sp, cp, out string
	var js, help, explain bool
	f.StringVar(&pp, "policy", "", "policy")
	f.StringVar(&sp, "state", "", "state")
	f.StringVar(&cp, "candidate", "", "candidate")
	f.StringVar(&out, "out", "", "output")
	f.BoolVar(&js, "json", false, "JSON")
	f.BoolVar(&help, "help", false, "show help")
	f.BoolVar(&explain, "explain", false, "explain decision")
	if e := f.Parse(a[1:]); e != nil {
		return exitInvalidInput
	}
	if help {
		f.PrintDefaults()
		return exitOK
	}
	if len(f.Args()) > 0 || strings.TrimSpace(pp) == "" || strings.TrimSpace(sp) == "" || strings.TrimSpace(cp) == "" {
		return exitInvalidInput
	}
	read := func(p string, v any) error {
		b, e := actioncontract.ReadRuntimeInput(p)
		if e != nil {
			return e
		}
		return actioncontract.DecodeStrictRuntimeJSON(b, v)
	}
	var p actioncontract.ChainPolicy
	var s actioncontract.ChainState
	var c actioncontract.ChainStep
	if e := read(pp, &p); e != nil {
		return exitInvalidInput
	}
	if e := read(sp, &s); e != nil {
		return exitInvalidInput
	}
	if e := read(cp, &c); e != nil {
		return exitInvalidInput
	}
	if reasons := actioncontract.ValidateChainPolicy(p); len(reasons) > 0 {
		return exitInvalidInput
	}
	if reasons := actioncontract.ValidateChainState(s); len(reasons) > 0 {
		return exitInvalidInput
	}
	if reasons := actioncontract.ValidateChainCandidate(c); len(reasons) > 0 {
		return exitInvalidInput
	}
	d := actioncontract.EvaluateCandidate(s, c, p)
	if out != "" {
		b, _ := json.Marshal(d)
		if e := fsx.WriteFileAtomic(out, append(b, '\n'), 0600); e != nil {
			return exitInternalFailure
		}
	}
	if js {
		b, _ := json.Marshal(d)
		fmt.Println(string(b))
	}
	if explain {
		fmt.Fprintf(os.Stderr, "chain decision: allowed=%t reasons=%s\n", d.Allowed, strings.Join(d.ReasonCodes, ","))
	}
	if !js {
		fmt.Printf("chain: %s\n", map[bool]string{true: "allowed", false: "denied"}[d.Allowed])
	}
	if d.Allowed {
		return exitOK
	}
	return exitVerifyFailed
}
func runActionContractCircuit(a []string) int {
	if len(a) == 0 || a[0] != "evaluate" {
		return exitInvalidInput
	}
	f := flag.NewFlagSet("circuit-evaluate", flag.ContinueOnError)
	var in, out string
	var js, help, explain bool
	f.StringVar(&in, "input", "", "input")
	f.StringVar(&out, "out", "", "output")
	f.BoolVar(&js, "json", false, "JSON")
	f.BoolVar(&help, "help", false, "show help")
	f.BoolVar(&explain, "explain", false, "explain decision")
	if e := f.Parse(a[1:]); e != nil {
		return exitInvalidInput
	}
	if help {
		f.PrintDefaults()
		return exitOK
	}
	if len(f.Args()) > 0 || strings.TrimSpace(in) == "" {
		return exitInvalidInput
	}
	b, e := actioncontract.ReadRuntimeInput(in)
	if e != nil {
		return exitInvalidInput
	}
	var i actioncontract.CircuitBreakerInput
	if e = actioncontract.DecodeStrictRuntimeJSON(b, &i); e != nil {
		return exitInvalidInput
	}
	if reasons := actioncontract.ValidateCircuitInput(i); len(reasons) > 0 {
		return exitInvalidInput
	}
	d := actioncontract.EvaluateCircuit(i)
	if reasons := actioncontract.ValidateCircuitDecision(d); len(reasons) > 0 {
		return exitInternalFailure
	}
	if out != "" {
		x, _ := json.Marshal(d)
		if e = fsx.WriteFileAtomic(out, append(x, '\n'), 0600); e != nil {
			return exitInternalFailure
		}
	}
	if js {
		x, _ := json.Marshal(d)
		fmt.Println(string(x))
	}
	if explain {
		fmt.Fprintf(os.Stderr, "circuit decision: allow=%t tripped=%t reasons=%s\n", d.Allow, d.Tripped, strings.Join(d.ReasonCodes, ","))
	}
	if !js {
		fmt.Printf("circuit: %s\n", map[bool]string{true: "allow", false: "tripped"}[d.Allow])
	}
	if d.Allow {
		return exitOK
	}
	return exitVerifyFailed
}
