package testvalidity

// Seed-parity rule vocabulary (docs/specs/test-source-analysis-v0.md, TSA-V0-100..109).
// internal/testsource compares the setup a new test performs with the setup of the legacy test it
// is linked to. Every rule here is advisory: it never changes a hygiene or strength axis.

// Seed-parity rule IDs.
const (
	// RuleSeedParityDiverges: the two linked tests seed a different entity kind, a different
	// count of a kind, or a different explicitly set attribute (TSA-V0-104, TSA-V0-105).
	RuleSeedParityDiverges = "SEED-PARITY-DIVERGES"
	// RuleSeedAggregateSingleMember: an assertion about a group aggregate reads a kind that one
	// side seeds with a single member (TSA-V0-106).
	RuleSeedAggregateSingleMember = "SEED-AGGREGATE-SINGLE-MEMBER"
	// RuleSeedParityNotAnalysed: one side's setup could not be extracted, so parity is not
	// reported (TSA-V0-107).
	RuleSeedParityNotAnalysed = "SEED-PARITY-NOT-ANALYSED"
	// RuleSeedParityUncertain: both sides set an attribute, and one side's value is an
	// expression the reading does not know, so the shapes are neither equal nor shown to differ
	// (TSA-V0-105).
	RuleSeedParityUncertain = "SEED-PARITY-UNCERTAIN"
)

// Seed-parity statuses (TSA-V0-103). Parity is the equality of the normalised setup shapes only,
// never behavioural parity.
const (
	SeedParityStatusParity      = "parity"
	SeedParityStatusDiverges    = "diverges"
	SeedParityStatusNotAnalysed = "not-analysed"
	SeedParityStatusUncertain   = "uncertain"
)
