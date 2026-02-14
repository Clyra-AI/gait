package gate

import schemagate "github.com/davidahmann/gait/core/schema/v1/gate"

// Evaluate is a stable alias for policy evaluation used by integration callers.
func Evaluate(policy Policy, intent schemagate.IntentRequest, options EvalOptions) (schemagate.GateResult, error) {
	return EvaluatePolicy(policy, intent, options)
}
