package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Clyra-AI/gait/core/actioncontract"
	proofsign "github.com/Clyra-AI/proof/signing"
)

// runActionContractLifecycleVerify authenticates a pre-execution lifecycle
// prefix before a caller crosses into its executor. The prefix is owned by
// Gate and therefore uses the Gate trace public key.
func runActionContractLifecycleVerify(args []string) int {
	f := flag.NewFlagSet("contract-lifecycle-verify", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var journalPath, publicKeyPath string
	var js, help bool
	f.StringVar(&journalPath, "journal", "", "existing lifecycle JSONL journal")
	f.StringVar(&publicKeyPath, "public-key", "", "Gate trace public key")
	f.BoolVar(&js, "json", false, "emit JSON")
	f.BoolVar(&help, "help", false, "show help")
	if err := f.Parse(args); err != nil {
		return lifecycleVerifyOutput(js, err, exitInvalidInput)
	}
	if help {
		fmt.Println("Usage: gait action-contract lifecycle-verify --journal lifecycle.jsonl --public-key key [--json]")
		return exitOK
	}
	if strings.TrimSpace(journalPath) == "" || strings.TrimSpace(publicKeyPath) == "" {
		return lifecycleVerifyOutput(js, fmt.Errorf("--journal and --public-key are required"), exitInvalidInput)
	}
	public, err := proofsign.LoadPublicKeyBase64(publicKeyPath)
	if err != nil {
		return lifecycleVerifyOutput(js, err, exitInvalidInput)
	}
	records, err := actioncontract.ReadLifecycleJournal(journalPath)
	if err != nil {
		return lifecycleVerifyOutput(js, err, exitVerifyFailed)
	}
	if err = actioncontract.VerifyLifecycleRecords(records, public); err != nil {
		return lifecycleVerifyOutput(js, err, exitVerifyFailed)
	}
	snapshot, err := actioncontract.ReduceLifecycleChecked(records)
	if err != nil {
		return lifecycleVerifyOutput(js, err, exitVerifyFailed)
	}
	if !snapshot.Activated || !snapshot.DecisionReady {
		return lifecycleVerifyOutput(js, fmt.Errorf("lifecycle prefix is not activated and decision-ready"), exitVerifyFailed)
	}
	return lifecycleVerifyOutput(js, nil, exitOK)
}

func lifecycleVerifyOutput(js bool, err error, code int) int {
	if js {
		fmt.Printf("{\"ok\":%t,\"error\":%q}\n", err == nil, errorString(err, ""))
	} else if err != nil {
		fmt.Println("lifecycle verify error: " + err.Error())
	} else {
		fmt.Println("lifecycle verify: ok")
	}
	return code
}
