package contextindex

import "context"

// QueryAuthorityMissingRootMessage is the one authority-start refusal the
// standalone command adapter degrades instead of returning (GPK-V0-091): the
// tree ranks no uniquely highest root AGENTS.md. The symlink and
// unsupported-content refusals keep their own messages and are never degraded.
const QueryAuthorityMissingRootMessage = "native Go authority-start query requires one uniquely highest-ranked root AGENTS.md"

// QueryDegradation is a degradation the command adapter announces before the
// repository profile ranks (GPK-V0-091). EvalQuery records Code in the
// receipt's `degradations` member and passes Uncertainty through the
// budget-aware coverage list, so a budgeted packet still fits its budget.
type QueryDegradation struct {
	Code, Uncertainty string
}

type queryDegradationKey struct{}

// WithQueryDegradation carries one announced degradation to EvalQuery.
func WithQueryDegradation(ctx context.Context, degradation QueryDegradation) context.Context {
	return context.WithValue(ctx, queryDegradationKey{}, degradation)
}

func queryDegradation(ctx context.Context) (QueryDegradation, bool) {
	degradation, ok := ctx.Value(queryDegradationKey{}).(QueryDegradation)
	return degradation, ok && degradation.Code != ""
}

// StabilizePacketBytes re-derives `coverage.packet_bytes` and
// `coverage.within_budget` after an adapter adds a member to a compiled
// receipt (GPK-V0-090). The caller keeps the packet inside its budget; this
// only makes the receipt describe its own bytes again.
func StabilizePacketBytes(receipt map[string]any, budget *int) error {
	if err := stabilizePacketBytes(receipt); err != nil {
		return err
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		return &Error{Message: "cannot encode context receipt"}
	}
	receipt["coverage"].(map[string]any)["within_budget"] = budget == nil || len(encoded) <= *budget
	return stabilizePacketBytes(receipt)
}
