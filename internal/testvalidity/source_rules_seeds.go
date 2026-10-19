package testvalidity

// Seed rule IDs (docs/specs/test-source-analysis-v0.md TSA-V0-240..246; V1-1935 / GitHub #1196).
// Every seed finding is advisory: it names a seed the project's declared server validation rejects
// before any run, or the tests a seeding hook's failure would skip, and never changes an axis.
const (
	RuleSeedValidationRejected   = "SEED-VALIDATION-REJECTED"
	RuleSeedSiblingCollision     = "SEED-SIBLING-COLLISION"
	RuleSeedHookBlastRadius      = "SEED-HOOK-BLAST-RADIUS"
	RuleSeedValidationUnresolved = "SEED-VALIDATION-UNRESOLVED"
)
