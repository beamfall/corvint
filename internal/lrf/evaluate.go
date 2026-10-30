package lrf

// Evaluate is the retired lrf/0 evaluator (CEM-V1-055, CEM-V1-215, V1-2665). No
// context row is admitted, so it refuses every request as unsupported-lrf-context
// and never issues a result. ValidateStableRequest is the only live admission.
func Evaluate(request Request) (Result, error) {
	return Result{}, unsupported("lrf/0 is retired and evaluates no context; use ValidateStableRequest")
}
