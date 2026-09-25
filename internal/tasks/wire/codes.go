package wire

import "fmt"

// Closed detail codes (SPEC §11). Only these strings may appear in a
// taskman-command-result/0 or taskman-outcome/0 codes array.
const CodeAdjudication = "ADJUDICATION"
const CodeAdoptUnsupportedField = "ADOPT_UNSUPPORTED_FIELD"
const CodeApprovalMissing = "APPROVAL_MISSING"
const CodeApprovalRevoked = "APPROVAL_REVOKED"
const CodeAttemptLive = "ATTEMPT_LIVE"
const CodeBootFenced = "BOOT_FENCED"
const CodeBootTimeout = "BOOT_TIMEOUT"
const CodeBudgetExceeded = "BUDGET_EXCEEDED"
const CodeBudgetUnknown = "BUDGET_UNKNOWN"
const CodeCapabilityUnavailable = "CAPABILITY_UNAVAILABLE"
const CodeCemMissing = "CEM_MISSING"
const CodeContaminated = "CONTAMINATED"
const CodeCoverageUnknown = "COVERAGE_UNKNOWN"
const CodeCutoverInProgress = "CUTOVER_IN_PROGRESS"
const CodeCutoverMissing = "CUTOVER_MISSING"
const CodeCycle = "CYCLE"
const CodeDependencyMissing = "DEPENDENCY_MISSING"
const CodeDependencyUnsatisfied = "DEPENDENCY_UNSATISFIED"
const CodeDevelopmentMode = "DEVELOPMENT_MODE"
const CodeDirtyWorktree = "DIRTY_WORKTREE"
const CodeDocsMissing = "DOCS_MISSING"
const CodeDuplicateID = "DUPLICATE_ID"
const CodeEffectOwned = "EFFECT_OWNED"
const CodeExternalUnbounded = "EXTERNAL_UNBOUNDED"
const CodeFenced = "FENCED"
const CodeGateFailed = "GATE_FAILED"
const CodeGateStale = "GATE_STALE"
const CodeGateUnknown = "GATE_UNKNOWN"
const CodeIndependenceUnverified = "INDEPENDENCE_UNVERIFIED"
const CodeIntentBranchMismatch = "INTENT_BRANCH_MISMATCH"
const CodeIntentDiverged = "INTENT_DIVERGED"
const CodeInvalidPriority = "INVALID_PRIORITY"
const CodeJournalForked = "JOURNAL_FORKED"
const CodeJournalSaturated = "JOURNAL_SATURATED"
const CodeLimitExceeded = "LIMIT_EXCEEDED"
const CodeLockTimeout = "LOCK_TIMEOUT"
const CodeMalformed = "MALFORMED"
const CodeMissingEvidence = "MISSING_EVIDENCE"
const CodeMissingGate = "MISSING_GATE"
const CodeNoexec = "NOEXEC"
const CodeOcmMissing = "OCM_MISSING"
const CodeOutOfScope = "OUT_OF_SCOPE"
const CodePaused = "PAUSED"
const CodePlanStale = "PLAN_STALE"
const CodeQuiescenceUnproved = "QUIESCENCE_UNPROVED"
const CodeRedoPending = "REDO_PENDING"
const CodeRequestIDConflict = "REQUEST_ID_CONFLICT"
const CodeResourceCollision = "RESOURCE_COLLISION"
const CodeRestored = "RESTORED"
const CodeRestoreIncomplete = "RESTORE_INCOMPLETE"
const CodeRetryExhausted = "RETRY_EXHAUSTED"
const CodeReviewIncomplete = "REVIEW_INCOMPLETE"
const CodeReviewRejected = "REVIEW_REJECTED"
const CodeSignalRefusedIdentity = "SIGNAL_REFUSED_IDENTITY"
const CodeSnapshotMoved = "SNAPSHOT_MOVED"
const CodeStalePolicy = "STALE_POLICY"
const CodeStaleTicket = "STALE_TICKET"
const CodeStaleTree = "STALE_TREE"
const CodeSupervisorLost = "SUPERVISOR_LOST"
const CodeSurvivors = "SURVIVORS"
const CodeTicketHeld = "TICKET_HELD"
const CodeTicketState = "TICKET_STATE"
const CodeUncertainEffect = "UNCERTAIN_EFFECT"
const CodeUninitialized = "UNINITIALIZED"
const CodeUnpublished = "UNPUBLISHED"
const CodeUnresolvedFinding = "UNRESOLVED_FINDING"
const CodeUnsupported = "UNSUPPORTED"
const CodeUnsupportedFilesystem = "UNSUPPORTED_FILESYSTEM"
const CodeUnsupportedVersion = "UNSUPPORTED_VERSION"

// Codes is the closed §11 set.
var Codes = []string{
	CodeAdjudication, CodeAdoptUnsupportedField, CodeApprovalMissing, CodeApprovalRevoked,
	CodeAttemptLive, CodeBootFenced, CodeBootTimeout, CodeBudgetExceeded, CodeBudgetUnknown,
	CodeCapabilityUnavailable, CodeCemMissing, CodeContaminated, CodeCoverageUnknown,
	CodeCutoverInProgress, CodeCutoverMissing, CodeCycle, CodeDependencyMissing,
	CodeDependencyUnsatisfied, CodeDevelopmentMode, CodeDirtyWorktree, CodeDocsMissing,
	CodeDuplicateID, CodeEffectOwned, CodeExternalUnbounded, CodeFenced, CodeGateFailed,
	CodeGateStale, CodeGateUnknown, CodeIndependenceUnverified, CodeIntentBranchMismatch,
	CodeIntentDiverged, CodeInvalidPriority, CodeJournalForked, CodeJournalSaturated,
	CodeLimitExceeded, CodeLockTimeout, CodeMalformed, CodeMissingEvidence, CodeMissingGate,
	CodeNoexec, CodeOcmMissing, CodeOutOfScope, CodePaused, CodePlanStale,
	CodeQuiescenceUnproved, CodeRedoPending, CodeRequestIDConflict, CodeResourceCollision,
	CodeRestored, CodeRestoreIncomplete, CodeRetryExhausted, CodeReviewIncomplete,
	CodeReviewRejected, CodeSignalRefusedIdentity, CodeSnapshotMoved, CodeStalePolicy,
	CodeStaleTicket, CodeStaleTree, CodeSupervisorLost, CodeSurvivors, CodeTicketHeld,
	CodeTicketState, CodeUncertainEffect, CodeUninitialized, CodeUnpublished,
	CodeUnresolvedFinding, CodeUnsupported, CodeUnsupportedFilesystem, CodeUnsupportedVersion,
}

var codeSet = func() map[string]bool {
	m := make(map[string]bool, len(Codes))
	for _, c := range Codes {
		m[c] = true
	}
	return m
}()

// IsCode reports whether s is a member of the closed §11 set.
func IsCode(s string) bool { return codeSet[s] }

// Error is a stable, actionable failure: a closed §11 code, the location in
// the document (a JSON-pointer-like path or a byte offset) and a message.
type Error struct {
	Code  string
	Where string
	Msg   string
}

func (e *Error) Error() string {
	if e.Where == "" {
		return e.Code + ": " + e.Msg
	}
	return e.Code + ": " + e.Where + ": " + e.Msg
}

// Errorf builds an Error. The code must be a §11 code; anything else is a
// programming error and is reported as MALFORMED so it can never leak an
// unknown code onto the wire. The parameter is not named `code`: Corvint's
// error-code ownership ratchet matches callees by name (ECO-V0-001), so that
// name would make every fmt.Errorf literal in the module an emitted code.
func Errorf(detailCode, where, format string, args ...interface{}) *Error {
	if !IsCode(detailCode) {
		detailCode = CodeMalformed
	}
	return &Error{Code: detailCode, Where: where, Msg: fmt.Sprintf(format, args...)}
}

// CodeOf returns the §11 code carried by err, or MALFORMED for any other
// error, or "" for nil.
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return CodeMalformed
}
