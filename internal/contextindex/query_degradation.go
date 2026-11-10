package contextindex

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
