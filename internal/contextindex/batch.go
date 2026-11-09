package contextindex

// The SBQ-V0-008 verdicts. Every possessed entry gets exactly one, matched by
// literal repository-relative path against the loaded index's tables: no
// normalization, case folding, symlink resolution or Git process.
const (
	VerdictUnframable  = "unframable"
	VerdictUnsupported = "unsupported"
	VerdictDirty       = "dirty"
	VerdictStale       = "stale"
	VerdictRetained    = "retained"
	VerdictAbsent      = "absent"
)

// The SBQ-V0-010(d) fallback reasons. A fallback disables suppression for one
// whole operation; there is no partial fallback.
const (
	FallbackBudgetCompacted       = "budget-compacted"
	FallbackUnsupportedPossession = "unsupported-possession"
	FallbackMixedWorktree         = "mixed-worktree"
)

// PossessedEntry is one caller-supplied possession row (SBQ-V0-007(a)). The
// optional `line` is validated and ignored in V0, so it is not carried here: a
// stored line and one dropped after validation are indistinguishable on the
// wire.
type PossessedEntry struct {
	Path     string
	BlobHash string
}

// SuppressedResult is one result that left `results` for `delta.suppressed`
// (SBQ-V0-009(f)). Rows is the count of that result's evidence rows.
type SuppressedResult struct {
	Kind string
	ID   string
	Rows int
}

// InvalidatedResult is one (result, invalidation-matched path) pair
// (SBQ-V0-009(h)). It is the one delta list keyed without an operation.
type InvalidatedResult struct {
	Kind    string
	ID      string
	Path    string
	Verdict string
}

// PossessionOutcome is what one operation's possession list did to its receipt.
// A non-empty Fallback means suppression was disabled for that whole operation
// and every entry is echoed under Ignored.
type PossessionOutcome struct {
	Fallback    string
	Suppressed  []SuppressedResult
	Invalidated []InvalidatedResult
	Ignored     []PossessedEntry
}
