package flag

import (
	"leaflag/backend/internal/service/strategy"
)

// EvaluationResult is the outcome of evaluating a flag against a context.
// The real evaluation engine lives in the strategy package (see
// strategy.Evaluate); this alias keeps Service's public surface (and its
// many external callers — OFREP/playground routes, the runtime data plane)
// unchanged.
type EvaluationResult = strategy.EvaluationResult
