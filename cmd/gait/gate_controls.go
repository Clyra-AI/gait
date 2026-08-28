package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Clyra-AI/gait/core/actioncontract"
	"github.com/Clyra-AI/gait/core/fsx"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

func evaluateGateControls(policyPath, statePath, candidatePath, stateOut, circuitPath string, intent schemagate.IntentRequest) error {
	chainSelected := strings.TrimSpace(policyPath) != "" || strings.TrimSpace(statePath) != "" || strings.TrimSpace(candidatePath) != ""
	if chainSelected && (strings.TrimSpace(policyPath) == "" || strings.TrimSpace(statePath) == "" || strings.TrimSpace(candidatePath) == "") {
		return &actioncontract.ValidationError{Reasons: []string{"action_chain_input_incomplete"}}
	}
	if chainSelected {
		return fsx.WithFileLock(statePath, func() error {
			return evaluateGateControlsLocked(policyPath, statePath, candidatePath, stateOut, circuitPath, intent)
		})
	}
	return evaluateGateControlsLocked(policyPath, statePath, candidatePath, stateOut, circuitPath, intent)
}

func evaluateGateControlsLocked(policyPath, statePath, candidatePath, stateOut, circuitPath string, intent schemagate.IntentRequest) error {
	chainSelected := strings.TrimSpace(policyPath) != "" || strings.TrimSpace(statePath) != "" || strings.TrimSpace(candidatePath) != ""
	if chainSelected {
		read := func(path string, target any) error {
			raw, err := actioncontract.ReadRuntimeInput(path)
			if err != nil {
				return err
			}
			return actioncontract.DecodeStrictRuntimeJSON(raw, target)
		}
		var policy actioncontract.ChainPolicy
		var state actioncontract.ChainState
		var candidate actioncontract.ChainStep
		if err := read(policyPath, &policy); err != nil {
			return &actioncontract.ValidationError{Reasons: []string{"action_chain_policy_unreadable"}}
		}
		if err := read(statePath, &state); err != nil {
			return &actioncontract.ValidationError{Reasons: []string{"action_chain_state_unreadable"}}
		}
		if err := read(candidatePath, &candidate); err != nil {
			return &actioncontract.ValidationError{Reasons: []string{"action_chain_candidate_unreadable"}}
		}
		expected := deriveGateChainCandidate(intent)
		if candidate.ID != expected.ID || candidate.Target != expected.Target || !sameStringSet(candidate.Classes, expected.Classes) {
			return &actioncontract.ValidationError{Reasons: []string{"action_chain_candidate_binding_mismatch"}}
		}
		decision := actioncontract.EvaluateCandidate(state, candidate, policy)
		if !decision.Allowed {
			return &actioncontract.ValidationError{Reasons: decision.ReasonCodes}
		}
		if strings.TrimSpace(stateOut) != "" {
			raw, err := json.Marshal(decision.State)
			if err != nil {
				return fmt.Errorf("encode action chain state: %w", err)
			}
			if err := fsx.WriteFileAtomic(stateOut, append(raw, '\n'), 0600); err != nil {
				return fmt.Errorf("persist action chain state: %w", err)
			}
		}
	}
	if strings.TrimSpace(circuitPath) != "" {
		raw, err := actioncontract.ReadRuntimeInput(circuitPath)
		if err != nil {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_input_unreadable"}}
		}
		var input actioncontract.CircuitBreakerInput
		if err := actioncontract.DecodeStrictRuntimeJSON(raw, &input); err != nil {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_input_malformed"}}
		}
		decision := actioncontract.EvaluateCircuit(input)
		expected := deriveGateChainCandidate(intent)
		if len(input.Chain.State.StepIDs) == 0 || input.Chain.State.StepIDs[len(input.Chain.State.StepIDs)-1] != expected.ID {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_input_binding_mismatch"}}
		}
		expectedIntentDigest := intent.IntentDigest
		if expectedIntentDigest != "" && !strings.HasPrefix(expectedIntentDigest, "sha256:") {
			expectedIntentDigest = "sha256:" + expectedIntentDigest
		}
		if strings.TrimSpace(expectedIntentDigest) == "" || input.IntentDigest != expectedIntentDigest {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_intent_digest_mismatch"}}
		}
		stateDigest, digestErr := actioncontract.DigestChainState(input.Chain.State)
		if digestErr != nil || input.ChainStateDigest != stateDigest {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_state_digest_mismatch"}}
		}
		bindingDigest, bindingErr := actioncontract.CircuitBindingDigest(input)
		if bindingErr != nil || input.BindingDigest != bindingDigest {
			return &actioncontract.ValidationError{Reasons: []string{"circuit_binding_digest_mismatch"}}
		}
		if !decision.Allow {
			return &actioncontract.ValidationError{Reasons: decision.ReasonCodes}
		}
	}
	return nil
}

func deriveGateChainCandidate(intent schemagate.IntentRequest) actioncontract.ChainStep {
	id := strings.TrimSpace(intent.Context.RequestID)
	if id == "" {
		id = strings.TrimSpace(intent.IntentDigest)
	}
	id = strings.ToLower(strings.NewReplacer("sha256:", "", " ", "_").Replace(id))
	if id == "" {
		id = "request"
	}
	target := "intent"
	classes := []string{}
	for _, item := range intent.Targets {
		if target == "intent" && strings.TrimSpace(item.Value) != "" {
			target = item.Value
		}
		value := strings.ToLower(strings.TrimSpace(item.EndpointClass + " " + item.Operation + " " + item.Value))
		matched := false
		if strings.Contains(value, "delete") {
			classes = append(classes, "delete")
			matched = true
		}
		if strings.Contains(value, "write") || strings.Contains(value, "create") || strings.Contains(value, "update") {
			classes = append(classes, "write")
			matched = true
		}
		if strings.Contains(value, "exec") {
			classes = append(classes, "execute")
			matched = true
		}
		if strings.Contains(value, "http") || strings.Contains(value, "net.") {
			classes = append(classes, "egress")
			matched = true
		}
		if !matched {
			classes = append(classes, "read")
		}
	}
	if len(classes) == 0 {
		classes = []string{"read"}
	}
	return actioncontract.ChainStep{ID: id, Target: target, Classes: uniqueSortedStrings(classes)}
}

func sameStringSet(a, b []string) bool {
	return strings.Join(uniqueSortedStrings(a), "\x00") == strings.Join(uniqueSortedStrings(b), "\x00")
}
