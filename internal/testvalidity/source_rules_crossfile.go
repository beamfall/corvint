package testvalidity

// Cross-file abstentions of the advisory source rules (docs/specs/test-source-analysis-v0.md,
// TSA-V0-300..312). internal/testsource reads one committed file at a time; when what decides an
// advisory rule is declared in another file, the reading is withheld as a RuleAbstention instead of
// being reported or silently dropped.

// Cross-file abstention reasons, owned here alone (TSA-V0-311).
const (
	// AbstainStubTableCrossFile: the file imports a sibling module that may declare a stub
	// covering the input, so UNSTUBBED-OUTBOUND-INPUT is withheld (TSA-V0-301, TSA-V0-302).
	AbstainStubTableCrossFile = "stub-table-cross-file"
	// AbstainCleanupRegistryCrossFile: the cleanup registry is a fixture the file does not declare,
	// so its scope is unknown and CLEANUP-HANDLE-OUTLIVED is withheld (TSA-V0-303).
	AbstainCleanupRegistryCrossFile = "cleanup-registry-cross-file"
	// AbstainFixtureCrossFile: the test requests a fixture the file does not declare, which may seed
	// the compared collections, so VACUOUS-EMPTY-EQUALITY is withheld (TSA-V0-307).
	AbstainFixtureCrossFile = "fixture-cross-file"
)

// RuleAbstention is one advisory rule reading withheld because what decides it is declared in
// another file (TSA-V0-300). Test and Location are `path:line` of the test and the statement the
// rule would have flagged; Cause is `path:line` of the import that names the other file, or the
// location when the file has none. It is not a finding and changes no axis.
type RuleAbstention struct {
	Rule     string `json:"rule"`
	Reason   string `json:"reason"`
	Test     string `json:"test"`
	Location string `json:"location"`
	Cause    string `json:"cause"`
	Message  string `json:"message"`
}
