package wire

import "errors"

// Retry is the Tasks-owned retryability of one §11 code (CAL-V0-078). A
// retryable code names the condition under which reissuing the same command,
// with the same --request-id, after a bounded backoff can succeed. A code that
// is not retryable names why a plain retry cannot change the result: the
// attempt was fenced, the input or a recorded fact must change first, another
// actor or the owner must act, or the code is reserved with no in-tree
// producer. Uncertain codes are not retryable.
type Retry struct {
	Retryable bool
	Condition string
}

const (
	whyFenced      = "fencing: the attempt or generation really lost and a retry cannot recover it; start a new attempt"
	whyInput       = "the request, payload or recorded input must change first; an identical retry repeats the refusal"
	whyReread      = "a recorded fact moved; re-read and rebuild the request before trying again"
	whyActor       = "another actor, the owner or a recorded state must change first; retry only after that change is observed"
	whyResult      = "an execution or review result, not a contention failure; the same inputs reproduce it"
	whyReserved    = "reserved in §11 with no in-tree producer; unclassified codes are not retryable"
	whyCapacity    = "capacity or storage condition that backoff alone is not known to clear; uncertain, so not retryable"
	whyDisposition = "a disposition, not a failure; the outcome is final for this request"
)

// retries classifies every member of Codes. TestCALV0078_ClassificationCoversEveryCode
// pins it so a new §11 code cannot ship unclassified.
var retries = map[string]Retry{
	CodeResourceExhausted: {true, "process or system file descriptors, disk space or inotify watches are exhausted (CAL-V0-900); after bounded backoff the identical request with the same --request-id can succeed when pressure clears; explicit post-program or unfinished-commit marks still forbid retry (CAL-V0-215)"},
	CodeLockTimeout:       {true, "the store lock or a preparation admission was held by another writer past the wait budget, or the write's context ended before it committed (ATR-V0-050); nothing was locked or written, so the same command with the same --request-id can succeed once the holder releases"},
	CodeRedoPending:       {true, "a writer is between receipt link-in and head rename; the read or command can succeed once that writer finishes, and if it outlives the budget the next mutating command redoes the pending receipt"},
	CodeSnapshotMoved:     {true, "the store, head, intent tree or worktree changed during a read or before commit; nothing was decided, so the same command can succeed against a quiet snapshot (a release or attestation candidate whose head no longer matches, and a criterion capture wrapping a refused read, repeat until their input changes)"},

	CodeFenced:         {false, whyFenced},
	CodeBootFenced:     {false, whyFenced},
	CodeSupervisorLost: {false, whyFenced},

	CodeLimitExceeded:         {false, "mostly a static size bound; the transient preparation-slot and active-attempt caps share this code, so it is not retryable until they are split"},
	CodeJournalSaturated:      {false, whyCapacity},
	CodeUnsupportedFilesystem: {false, whyCapacity},

	CodeStaleTicket:           {false, whyReread},
	CodeStalePolicy:           {false, whyReread},
	CodeStaleTree:             {false, whyReread},
	CodeCommitNotIntegrated:   {false, "the commit is in neither the intent branch nor its remote-tracking upstream; integrate it or fetch the upstream, then complete again"},
	CodeRequestIDConflict:     {false, whyInput},
	CodeMalformed:             {false, whyInput},
	CodeMilestoneRequired:     {false, whyInput},
	CodeSecretDetected:        {false, whyInput},
	CodeProvenanceUnverified:  {false, whyInput},
	CodeDuplicateID:           {false, whyInput},
	CodeCycle:                 {false, whyInput},
	CodeDependencyMissing:     {false, whyInput},
	CodeInvalidPriority:       {false, whyInput},
	CodeGateUnknown:           {false, whyInput},
	CodeAdoptUnsupportedField: {false, whyInput},
	CodeOutOfScope:            {false, whyInput},
	CodeUnsupported:           {false, whyInput},
	CodeUnsupportedVersion:    {false, whyInput},
	CodeIntentBranchMismatch:  {false, whyInput},
	CodeDirtyWorktree:         {false, whyInput},

	CodeAttemptLive:             {false, whyActor},
	CodeResourceCollision:       {false, whyActor},
	CodePaused:                  {false, whyActor},
	CodeDependencyUnsatisfied:   {false, whyActor},
	CodeTicketHeld:              {false, whyActor},
	CodeTicketState:             {false, whyActor},
	CodeApprovalMissing:         {false, whyActor},
	CodeApprovalRevoked:         {false, whyActor},
	CodeCutoverMissing:          {false, whyActor},
	CodeQuiescenceUnproved:      {false, whyActor},
	CodeRetryExhausted:          {false, whyActor},
	CodeGateFailed:              {false, whyActor},
	CodeGateStale:               {false, whyActor},
	CodeMissingGate:             {false, whyActor},
	CodeMissingEvidence:         {false, whyActor},
	CodeBudgetUnknown:           {false, whyActor},
	CodeCoverageUnknown:         {false, whyActor},
	CodeExternalUnbounded:       {false, whyActor},
	CodeUninitialized:           {false, whyActor},
	CodeRestoreIncomplete:       {false, whyActor},
	CodeIntentDiverged:          {false, whyActor},
	CodeJournalForked:           {false, whyActor},
	CodeEscalationPending:       {false, whyActor},
	CodePrerequisiteUnsatisfied: {false, whyActor},
	CodeLoopDetected:            {false, whyActor},

	CodePoolMemberRecoveryStalled: {false, whyActor},

	CodeNoexec:    {false, whyResult},
	CodeSurvivors: {false, whyResult},

	CodeHandoff:         {false, whyDisposition},
	CodeReviewReturned:  {false, whyDisposition},
	CodeDevelopmentMode: {false, whyDisposition},

	CodeAdjudication:           {false, whyReserved},
	CodeBootTimeout:            {false, whyReserved},
	CodeBudgetExceeded:         {false, whyReserved},
	CodeCapabilityUnavailable:  {false, whyReserved},
	CodeCemMissing:             {false, whyReserved},
	CodeContaminated:           {false, whyReserved},
	CodeCutoverInProgress:      {false, whyReserved},
	CodeDocsMissing:            {false, whyReserved},
	CodeEffectOwned:            {false, whyReserved},
	CodeIndependenceUnverified: {false, whyReserved},
	CodeOcmMissing:             {false, whyReserved},
	CodePlanStale:              {false, whyReserved},
	CodeRestored:               {false, whyReserved},
	CodeReviewIncomplete:       {false, whyReserved},
	CodeReviewRejected:         {false, whyReserved},
	CodeSignalRefusedIdentity:  {false, whyReserved},
	CodeUncertainEffect:        {false, whyReserved},
	CodeUnpublished:            {false, whyReserved},
	CodeUnresolvedFinding:      {false, whyReserved},
}

// RetryOf returns the classification of a §11 code. An unknown code is not
// retryable.
func RetryOf(code string) Retry {
	if r, ok := retries[code]; ok {
		return r
	}
	return Retry{Condition: "not a §11 code; not retryable"}
}

// ResultRetryable derives the envelope's retryable member: present only on a
// non-OK result that carries at least one code, and true only when every code
// is retryable (CAL-V0-078).
func ResultRetryable(outcome string, codes []string) (retryable, present bool) {
	if outcome == OutcomeOK || len(codes) == 0 {
		return false, false
	}
	for _, c := range codes {
		if !RetryOf(c).Retryable {
			return false, true
		}
	}
	return true, true
}

// WithoutRetry returns err marked so that its result is never retryable
// (CAL-V0-078), for a caller that carries the fact only through an error. A
// *Error, or the *Error an error wraps, is copied with NotRetryable set; any
// other retryable error is wrapped without losing its cause (CAL-V0-215).
// Errors whose classification already forbids retry are returned unchanged.
func WithoutRetry(err error) error {
	var e *Error
	if !errors.As(err, &e) {
		if RetryOf(CodeOf(err)).Retryable {
			return &retryForbiddenError{err}
		}
		return err
	}
	marked := *e
	marked.NotRetryable = true
	return &marked
}

// Annotatef is Errorf with the code of cause, for a caller that restates cause
// with its own location and message. The WithoutRetry mark of cause survives,
// so an annotated error from a call that already ran stays not retryable
// (CAL-V0-078, V1-0908).
func Annotatef(cause error, where, format string, args ...interface{}) *Error {
	e := Errorf(CodeOf(cause), where, format, args...)
	e.NotRetryable = RetryForbidden(cause)
	return e
}

type retryForbiddenError struct{ error }

func (e *retryForbiddenError) Unwrap() error { return e.error }

// RetryForbidden reports whether err carries the WithoutRetry mark.
func RetryForbidden(err error) bool {
	var marked *retryForbiddenError
	if errors.As(err, &marked) {
		return true
	}
	var e *Error
	return errors.As(err, &e) && e.NotRetryable
}
