package testvalidity

// Write-response rule IDs (docs/specs/test-source-analysis-v0.md, TSA-V0-230..234; V1-1933 /
// GitHub #1194). Both are advisory: they are reported and change no axis.
const (
	// RuleWriteResponseUnasserted: a step's user action triggers a first-party POST, PUT, PATCH or
	// DELETE request that the step awaits, and no assertion in the step binds its response
	// (TSA-V0-230, TSA-V0-231).
	RuleWriteResponseUnasserted = "WRITE-RESPONSE-UNASSERTED"
	// RuleWriteResponseUnresolved: the step awaits a request the reader cannot resolve to a
	// method or a first-party route, so the rule abstains for it (TSA-V0-233).
	RuleWriteResponseUnresolved = "WRITE-RESPONSE-UNRESOLVED"
)
