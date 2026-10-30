package analyzercap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/Beamfall/corvint/internal/analyzerexec"
	"io"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"
)

type Stage string

const (
	StageLock              Stage = "lock"
	StageRegistry          Stage = "registry"
	StageResolve           Stage = "resolve"
	StageCompatibility     Stage = "compatibility"
	StageDependency        Stage = "dependency"
	StageLaunch            Stage = "launch"
	StageProtocol          Stage = "protocol"
	StageAdmission         Stage = "admission"
	StageExactVerification Stage = "exact-verification"
)

type StageDiagnostic struct {
	Stage                   Stage
	Event                   ReceiptEvent
	Result                  string
	Started, Ended, Elapsed time.Duration
	CleanupElapsed          time.Duration
	Cleanup                 analyzerexec.CleanupObservation
	CleanupError            analyzerexec.CleanupError
}

// ReceiptEvent is one actual operation on the one monotonic receipt clock.
// Encoding and decoding deliberately remain different events: treating them
// as one interval would make a launch between them appear to overlap protocol.
type ReceiptEvent string

const (
	EventRegistry       ReceiptEvent = "registry"
	EventLock           ReceiptEvent = "lock"
	EventResolve        ReceiptEvent = "resolve"
	EventCompatibility  ReceiptEvent = "compatibility"
	EventDependency     ReceiptEvent = "dependency"
	EventProtocolEncode ReceiptEvent = "protocol-encode"
	EventPreLaunch      ReceiptEvent = "pre-launch"
	EventLaunch         ReceiptEvent = "launch"
	EventProtocolDecode ReceiptEvent = "protocol-decode"
	EventAdmission      ReceiptEvent = "admission"
	EventExact          ReceiptEvent = "exact-verification"
)

type ReceiptPassKind string

const (
	PassAuthorityIssue      ReceiptPassKind = "authority-issue"
	PassPreLaunchRefresh    ReceiptPassKind = "pre-launch-refresh"
	PassPreAdmissionRefresh ReceiptPassKind = "pre-admission-refresh"
	PassInvocation          ReceiptPassKind = "invocation-execution"
	PassAdmission           ReceiptPassKind = "invocation-admission"
	PassInvocationGuard     ReceiptPassKind = "invocation-guard"
)

// ReceiptPass preserves one authority or invocation pass without flattening
// it into a prior pass. Events returns a defensive copy.
type ReceiptPass struct {
	Kind         ReceiptPassKind
	Started      time.Duration
	Ended        time.Duration
	FirstFailure Stage
	events       []StageDiagnostic
}

func (p ReceiptPass) Events() []StageDiagnostic { return clone(p.events) }

type Observation string

const (
	ObservationNone     Observation = "none"
	ObservationObserved Observation = "OBSERVED"
)

type Invocation struct {
	Authority *AdmissionAuthority
}
type ImmutableInput struct {
	Handle Opaque
	Digest Opaque
	Bytes  []byte
}
type Admission string

const (
	AdmissionAdmitted Admission = "ADMITTED"
	AdmissionDenied   Admission = "REJECTED"
)

type coreSource interface {
	get(context.Context) (coreState, error)
	hardWall() (time.Duration, error)
}
type coreState struct {
	r              *Request
	i              []ImmutableInput
	id, rev        Opaque
	lim            ProtocolLimits
	artifact       string
	exe            string
	repositoryRoot string
	stagingParent  string
	art, bin       Opaque
	v              AdmissionVerifier
}
type analyzerRunner func(context.Context, analyzerexec.Plan) (analyzerexec.Result, error)

var readAuthorityRandom = rand.Read
var coreInstanceSequence atomic.Uint64

type Core struct {
	s        coreSource
	run      analyzerRunner
	hardWall time.Duration
	instance Opaque
}

// CoreInput is the Core-owned invocation state for one fixed native analyzer
// selection. Request bytes are deliberately absent: Core derives and hashes
// them after resolver closure, rather than accepting a caller digest.
type CoreInput struct {
	Request                          Request
	Inputs                           []ImmutableInput
	RequestID, Revision              Opaque
	Limits                           ProtocolLimits
	ArtifactPath, ExecutablePath     string
	RepositoryRoot, StagingParent    string
	ArtifactDigest, HostBinaryDigest Opaque
	Verifier                         AdmissionVerifier
}

type staticCoreSource struct{ state coreState }

func (source staticCoreSource) get(ctx context.Context) (coreState, error) {
	if err := coreContextFailure(ctx); err != nil {
		return coreState{}, err
	}
	return ownCoreState(source.state), nil
}
func (source staticCoreSource) hardWall() (time.Duration, error) {
	if err := source.state.lim.validate(); err != nil {
		return 0, err
	}
	return time.Duration(source.state.lim.WallMilliseconds) * time.Millisecond, nil
}

// NewStaticCore is the production-facing constructor for a Core-owned,
// immutable native invocation selection. It does not qualify a profile.
func NewStaticCore(input CoreInput) (*Core, error) {
	request := cloneReq(input.Request)
	return NewCore(staticCoreSource{state: coreState{
		r: &request, i: cloneInputs(input.Inputs), id: input.RequestID, rev: input.Revision, lim: input.Limits,
		artifact: input.ArtifactPath, exe: input.ExecutablePath, repositoryRoot: input.RepositoryRoot, stagingParent: input.StagingParent,
		art: input.ArtifactDigest, bin: input.HostBinaryDigest, v: input.Verifier,
	}})
}

type AdmissionAuthority struct {
	c        *Core
	b        Identity
	cache    Identity
	request  Opaque
	rev      Opaque
	x        ExactVerificationResult
	origin   time.Time
	issue    ReceiptPass
	used     *atomic.Bool
	nonce    Opaque
	wall     time.Duration
	deadline time.Time
}
type AdmissionContext struct {
	Request AnalyzerRequest
}
type coreContext struct {
	AdmissionContext
	a, e                          string
	repositoryRoot, stagingParent string
	art, bin                      Opaque
	w                             []byte
	v                             AdmissionVerifier
	s                             []StageDiagnostic
	pass                          ReceiptPass
	previous                      []ReceiptPass
	b, cache                      Identity
	request                       Opaque
	rev                           Opaque
	origin                        time.Time
	receipt                       CoreReceipt
}
type AdmissionVerifier interface {
	Identity() Identity
	VerifyAdmission(context.Context, AdmissionContext, AnalyzerResponse) (Admission, error)
}

type frozenAdmissionVerifier interface {
	AdmissionVerifier
	CloneAdmissionVerifier() AdmissionVerifier
	coreBoundedAdmissionVerifier()
}

// ProtocolAdmissionVerifier is the bounded production admission rule for the
// experimental static runtime. Protocol/schema/echo validation has already
// completed before it runs; no caller callback can outlive the hard wall.
// A promoted profile replaces this rule with another Core-owned bounded
// implementation, never an in-process plugin callback.
type ProtocolAdmissionVerifier struct{ Verifier Identity }

func (verifier ProtocolAdmissionVerifier) Identity() Identity { return verifier.Verifier }
func (verifier ProtocolAdmissionVerifier) VerifyAdmission(ctx context.Context, _ AdmissionContext, _ AnalyzerResponse) (Admission, error) {
	if verifier.Verifier == "" || coreContextFailure(ctx) != nil {
		return AdmissionDenied, fail(AdmissionRejected)
	}
	return AdmissionAdmitted, nil
}
func (verifier ProtocolAdmissionVerifier) CloneAdmissionVerifier() AdmissionVerifier { return verifier }
func (ProtocolAdmissionVerifier) coreBoundedAdmissionVerifier()                      {}

type InvocationResult struct {
	Candidate         AnalyzerResponse
	Observation       Observation
	Admission         string
	ExactVerification ExactVerificationResult
	Stages            []StageDiagnostic
	Receipt           CoreReceipt
	StderrBytes       int
}

type ReceiptTerminalState string

const (
	ReceiptOpen      ReceiptTerminalState = "OPEN"
	ReceiptSucceeded ReceiptTerminalState = "SUCCEEDED"
	ReceiptFailed    ReceiptTerminalState = "FAILED"
)

// ReceiptTerminal is a closed, deterministic summary of a terminal attempt.
// It carries no raw error text, filesystem path, command, environment, or
// source body.
type ReceiptTerminal struct {
	State            ReceiptTerminalState
	Reason           Reason
	ProcessStarted   bool
	ProcessCompleted bool
	Termination      analyzerexec.Termination
	Cleanup          analyzerexec.CleanupObservation
	CleanupError     analyzerexec.CleanupError
}

// CoreReceipt owns canonical receipt bytes. Its accessors return copies so a
// verifier or caller cannot mutate the authority-bound record after issue.
type CoreReceipt struct {
	authority, liveAuthority, cache Identity
	revision, request               Opaque
	terminal                        ReceiptTerminal
	digest                          Identity
	raw                             []byte
	passes                          []ReceiptPass
}

func (r CoreReceipt) Digest() Identity {
	if !r.valid() {
		return ""
	}
	return r.digest
}
func (r CoreReceipt) Authority() Identity {
	if !r.valid() {
		return ""
	}
	return r.authority
}
func (r CoreReceipt) LiveAuthority() Identity {
	if !r.valid() {
		return ""
	}
	return r.liveAuthority
}
func (r CoreReceipt) CacheIdentity() Identity {
	if !r.valid() {
		return ""
	}
	return r.cache
}
func (r CoreReceipt) Revision() Opaque {
	if !r.valid() {
		return ""
	}
	return r.revision
}
func (r CoreReceipt) RequestBytesDigest() Opaque {
	if !r.valid() {
		return ""
	}
	return r.request
}
func (r CoreReceipt) Terminal() ReceiptTerminal {
	if !r.valid() {
		return ReceiptTerminal{}
	}
	return r.terminal
}
func (r CoreReceipt) CanonicalBytes() []byte {
	if !r.valid() {
		return nil
	}
	return clone(r.raw)
}
func (r CoreReceipt) Passes() []ReceiptPass {
	if !r.valid() {
		return nil
	}
	return clonePasses(r.passes)
}
func (r CoreReceipt) StageDiagnostics() []StageDiagnostic {
	if !r.valid() {
		return nil
	}
	var events []StageDiagnostic
	for _, pass := range r.passes {
		events = append(events, pass.events...)
	}
	return events
}

type ExactVerificationResult struct {
	State             exactState
	Reason            exactReason
	Verifier, Profile Identity
	Revision          Opaque
}
type exactState string
type exactReason Reason

const ExactVerificationNotRun exactState = "NOT_RUN"

// AuthorityIssueError preserves closed stage diagnostics when the immutable
// authority could not be issued. It contains only canonical receipt evidence.
type AuthorityIssueError struct {
	cause   error
	receipt CoreReceipt
}

func (err *AuthorityIssueError) Error() string { return err.cause.Error() }
func (err *AuthorityIssueError) Unwrap() error { return err.cause }
func (err *AuthorityIssueError) Receipt() CoreReceipt {
	return err.receipt
}

func InvokeAnalyzer(ctx context.Context, invocation Invocation) (InvocationResult, error) {
	authority := invocation.Authority
	if authority == nil || authority.used == nil {
		return failedInvocation(nil, coreContext{}, initialInvocationGuardStages(), StageAdmission, 0, analyzerexec.Result{CleanupState: analyzerexec.CleanupNotRun, Termination: analyzerexec.TerminationNotRun, CleanupError: analyzerexec.CleanupErrorNone}, fail(AdmissionRejected))
	}
	if err := coreContextFailure(ctx); err != nil {
		state := authority.guardFailureContext()
		return failedInvocation(authority, state, state.s, StageAdmission, 0, noStartExecutionResult(), err)
	}
	if !authority.used.CompareAndSwap(false, true) {
		state := authority.guardFailureContext()
		return failedInvocation(authority, state, state.s, StageAdmission, 0, analyzerexec.Result{CleanupState: analyzerexec.CleanupNotRun, Termination: analyzerexec.TerminationNotRun, CleanupError: analyzerexec.CleanupErrorNone}, fail(AdmissionRejected))
	}
	deadline := ctx
	cancel := func() {}
	if authority.deadline.IsZero() {
		deadline, cancel = context.WithTimeout(ctx, authority.wall)
	} else {
		deadline, cancel = context.WithDeadline(ctx, authority.deadline)
	}
	defer cancel()
	state, err := authority.ctxPass(deadline, PassPreLaunchRefresh, nil)
	if err != nil {
		stage := StageAdmission
		if len(state.s) != 0 {
			stage = firstFailedStage(state.s, StageResolve)
		}
		return failedInvocation(authority, state, state.s, stage, 0, analyzerexec.Result{CleanupState: analyzerexec.CleanupNotRun, Termination: analyzerexec.TerminationNotRun, CleanupError: analyzerexec.CleanupErrorNone}, err)
	}
	request, encoded := state.Request, clone(state.w)
	if len(encoded) == 0 {
		return failedInvocation(authority, state, state.s, StageProtocol, 0, analyzerexec.Result{CleanupState: analyzerexec.CleanupNotRun, Termination: analyzerexec.TerminationNotRun, CleanupError: analyzerexec.CleanupErrorNone}, fail(ProtocolEchoMismatch))
	}
	origin := state.origin
	host := actualHostPlatform()
	executionHost := analyzerexec.NativePlatform{OS: string(host.OS), Architecture: string(host.Architecture), ABI: string(host.ABI)}
	executionTarget := analyzerexec.NativePlatform{OS: string(request.Target.OS), Architecture: string(request.Target.Architecture), ABI: string(request.Target.ABI)}
	state = state.nextPass(PassInvocation, initialInvocationStages())
	launchStarted := time.Since(origin)
	run, err := authority.c.run(deadline, analyzerexec.Plan{
		Artifact: state.a, ExpectedArtifactSHA256: string(state.art),
		Executable: state.e, ExpectedExecutableSHA256: string(state.bin), Request: encoded,
		RepositoryRoot: state.repositoryRoot, StagingParent: state.stagingParent,
		Host:                    executionHost,
		Target:                  executionTarget,
		InvocationBindingSHA256: analyzerexec.InvocationBindingSHA256(encoded, executionHost, executionTarget),
		Timeout:                 time.Duration(request.Limits.WallMilliseconds) * time.Millisecond,
		MaxStdoutBytes:          int(request.Limits.MaxResponseBytes), MaxStderrBytes: int(request.Limits.MaxStderrBytes),
		MemoryBytes: request.Limits.MemoryBytes, MaxChildren: request.Limits.MaxChildren,
	})
	if !run.ValidCleanupObservation() {
		run = rejectedExecutionResult(run)
		state.s = recordLaunchInterval(state.s, "FAILED", run, launchStarted, time.Since(origin))
		return failedInvocation(authority, state, state.s, StageLaunch, len(run.Stderr), run, fail(AnalyzerFailure))
	}
	if len(run.Stdout) > int(request.Limits.MaxResponseBytes) || len(run.Stderr) > int(request.Limits.MaxStderrBytes) {
		state.s = recordLaunchInterval(state.s, "FAILED", run, launchStarted, time.Since(origin))
		return failedInvocation(authority, state, state.s, StageLaunch, len(run.Stderr), run, fail(LimitExceeded))
	}
	if err != nil {
		state.s = recordLaunchInterval(state.s, "FAILED", run, launchStarted, time.Since(origin))
		return failedInvocation(authority, state, state.s, StageLaunch, len(run.Stderr), run, mapExecutionFailure(err))
	}
	if !run.Started || !run.Completed || run.Termination != analyzerexec.TerminationExited || run.CleanupState != analyzerexec.CleanupObserved || run.CleanupError != analyzerexec.CleanupErrorNone {
		state.s = recordLaunchInterval(state.s, "FAILED", run, launchStarted, time.Since(origin))
		return failedInvocation(authority, state, state.s, StageLaunch, len(run.Stderr), run, fail(AnalyzerFailure))
	}
	state.s = recordLaunchInterval(state.s, "OK", run, launchStarted, time.Since(origin))
	decodeStarted := time.Since(origin)
	response, err := DecodeAnalyzerResponse(run.Stdout, request)
	if err != nil {
		state.s = recordProtocolInterval(state.s, "FAILED", decodeStarted, time.Since(origin), false)
		return failedInvocation(authority, state, state.s, StageProtocol, len(run.Stderr), run, err)
	}
	state.s = recordProtocolInterval(state.s, "OK", decodeStarted, time.Since(origin), false)
	state.syncPass()
	context, refreshErr := authority.ctxPass(deadline, PassPreAdmissionRefresh, append(state.previous, state.pass))
	if refreshErr != nil {
		return failedInvocation(authority, context, context.s, firstFailedStage(context.s, StageRegistry), len(run.Stderr), run, refreshErr)
	}
	context = context.nextPass(PassAdmission, admissionStages())
	admissionStarted := time.Since(origin)
	candidate, admission, err := verifyCandidate(deadline, context.v, context.AdmissionContext, response)
	if err != nil {
		context.s = recordStageInterval(context.s, StageAdmission, "FAILED", admissionStarted, time.Since(origin))
		result, _ := failedInvocation(authority, context, context.s, StageAdmission, len(run.Stderr), run, err)
		result.Admission = string(admission)
		return result, err
	}
	if err = coreContextFailure(deadline); err != nil {
		context.s = recordStageInterval(context.s, StageAdmission, "FAILED", admissionStarted, time.Since(origin))
		result, _ := failedInvocation(authority, context, context.s, StageAdmission, len(run.Stderr), run, err)
		result.Admission = string(AdmissionDenied)
		return result, err
	}
	context.s = recordStageInterval(context.s, StageAdmission, string(admission), admissionStarted, time.Since(origin))
	context.syncPass()
	receipt := finalizeReceipt(invocationReceipt(context), successfulTerminal(run))
	return InvocationResult{
		Candidate: candidate, Observation: ObservationObserved, Admission: string(admission), ExactVerification: exactNotRun(authority),
		Stages: receipt.StageDiagnostics(), Receipt: receipt, StderrBytes: len(run.Stderr),
	}, nil
}
func NewCore(source coreSource) (*Core, error) {
	if source == nil {
		return nil, fail(AdmissionRejected)
	}
	hardWall, err := source.hardWall()
	if err != nil || hardWall <= 0 {
		return nil, fail(LimitExceeded)
	}
	instance, err := newCoreInstance()
	if err != nil {
		return nil, err
	}
	return &Core{s: source, run: analyzerexec.Run, hardWall: hardWall, instance: instance}, nil
}
func (core *Core) IssueAdmissionAuthority() (*AdmissionAuthority, error) {
	return core.IssueAdmissionAuthorityContext(context.TODO())
}

// IssueAdmissionAuthorityContext binds issuance, source retrieval, snapshot
// resolution, and request hashing to the caller's deadline.
func (core *Core) IssueAdmissionAuthorityContext(ctx context.Context) (*AdmissionAuthority, error) {
	origin := time.Now()
	if core == nil || core.hardWall <= 0 {
		state, _, _, err := core.currentAt(ctx, origin, PassAuthorityIssue, "0")
		return nil, &AuthorityIssueError{cause: err, receipt: authorityIssueReceipt(state, err)}
	}
	deadline, cancel := context.WithTimeout(ctx, core.hardWall)
	defer cancel()
	deadlineAt, _ := deadline.Deadline()
	nonce, err := newAuthorityNonce()
	if err != nil {
		state, _, _, stateErr := core.currentAt(deadline, origin, PassAuthorityIssue, "0")
		if stateErr != nil {
			err = stateErr
		}
		return nil, &AuthorityIssueError{cause: err, receipt: authorityIssueReceipt(state, err)}
	}
	state, binding, exact, err := core.currentAt(deadline, origin, PassAuthorityIssue, nonce)
	if err != nil {
		return nil, &AuthorityIssueError{cause: err, receipt: authorityIssueReceipt(state, err)}
	}
	return &AdmissionAuthority{c: core, b: binding, cache: state.cache, request: state.request, rev: state.rev, x: exact, origin: origin, issue: state.pass, used: &atomic.Bool{}, nonce: nonce, wall: core.hardWall, deadline: deadlineAt}, nil
}

func newAuthorityNonce() (Opaque, error) {
	var bytes [32]byte
	if _, err := io.ReadFull(randomReader(readAuthorityRandom), bytes[:]); err != nil {
		return "", fail(AdmissionRejected)
	}
	return Opaque(hex.EncodeToString(bytes[:])), nil
}

func newCoreInstance() (Opaque, error) {
	nonce, err := newAuthorityNonce()
	if err != nil {
		return "", err
	}
	sequence := coreInstanceSequence.Add(1)
	return Opaque(digest("analyzer-core-instance/v1", string(nonce), strconv.FormatUint(sequence, 10))), nil
}

type randomReader func([]byte) (int, error)

func (reader randomReader) Read(value []byte) (int, error) { return reader(value) }

func authorityIssueReceipt(state coreContext, err error) CoreReceipt {
	authority := state.b
	if mustOpaque(Opaque(authority)) != nil {
		authority = digest("authority-issue-failure/v1", string(state.rev))
	}
	cache := state.cache
	if mustOpaque(Opaque(cache)) != nil {
		cache = digest("authority-issue-cache/v1", string(state.rev))
	}
	revision := state.rev
	if mustOpaque(revision) != nil {
		revision = "authority-issue-failed"
	}
	request := state.request
	if mustOpaque(request) != nil {
		request = "authority-issue-failed"
	}
	terminal := failedTerminal(noStartExecutionResult(), err)
	return makeCoreReceiptTerminal(authority, authority, cache, revision, request, terminal, []ReceiptPass{state.pass})
}

func noStartExecutionResult() analyzerexec.Result {
	return analyzerexec.Result{CleanupState: analyzerexec.CleanupNotRun, Termination: analyzerexec.TerminationNotRun, CleanupError: analyzerexec.CleanupErrorNone}
}
func (authority *AdmissionAuthority) ctx() (coreContext, error) {
	return authority.ctxPass(context.TODO(), PassPreLaunchRefresh, nil)
}
func (authority *AdmissionAuthority) guardFailureContext() coreContext {
	started := time.Since(authority.origin)
	stages := initialInvocationGuardStages()
	stages = recordStageInterval(stages, StageAdmission, "FAILED", started, time.Since(authority.origin))
	pass := makeReceiptPass(PassInvocationGuard, stages)
	previous := []ReceiptPass{authority.issue}
	receipt := makeCoreReceiptTerminal(authority.b, authority.b, authority.cache, authority.rev, authority.request, ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, append(previous, pass))
	return coreContext{s: stages, pass: pass, previous: previous, b: authority.b, cache: authority.cache, request: authority.request, rev: authority.rev, origin: authority.origin, receipt: receipt}
}
func (authority *AdmissionAuthority) ctxPass(ctx context.Context, kind ReceiptPassKind, previous []ReceiptPass) (coreContext, error) {
	if authority == nil || authority.c == nil || authority.b == "" {
		return coreContext{}, fail(AdmissionRejected)
	}
	state, binding, _, err := authority.c.currentAt(ctx, authority.origin, kind, authority.nonce)
	if len(previous) == 0 {
		state.previous = []ReceiptPass{authority.issue}
	} else {
		state.previous = clonePasses(previous)
	}
	if err != nil {
		state.receipt = makeCoreReceiptTerminal(authority.b, receiptIdentity(binding, authority.b), receiptIdentity(state.cache, authority.cache), receiptOpaque(state.rev, authority.rev), receiptOpaque(state.request, authority.request), ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, append(state.previous, state.pass))
		return state, err
	}
	bindingStarted := time.Since(authority.origin)
	if binding != authority.b {
		state.s = recordStageInterval(state.s, StageAdmission, "FAILED", bindingStarted, time.Since(authority.origin))
		state.syncPass()
		state.receipt = makeCoreReceiptTerminal(authority.b, receiptIdentity(binding, authority.b), receiptIdentity(state.cache, authority.cache), receiptOpaque(state.rev, authority.rev), receiptOpaque(state.request, authority.request), ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, append(state.previous, state.pass))
		return state, fail(AdmissionRejected)
	}
	state.s = recordStageInterval(state.s, StageAdmission, "OK", bindingStarted, time.Since(authority.origin))
	state.syncPass()
	state.receipt = makeCoreReceiptTerminal(authority.b, receiptIdentity(binding, authority.b), receiptIdentity(state.cache, authority.cache), receiptOpaque(state.rev, authority.rev), receiptOpaque(state.request, authority.request), ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, append(state.previous, state.pass))
	return state, nil
}
func (core *Core) current() (coreContext, Identity, ExactVerificationResult, error) {
	return core.currentAt(context.TODO(), time.Now(), PassAuthorityIssue, "direct")
}
func (core *Core) currentAt(ctx context.Context, origin time.Time, kind ReceiptPassKind, nonce Opaque) (coreContext, Identity, ExactVerificationResult, error) {
	stages := initialAuthorityStages()
	failed := func(stages []StageDiagnostic, binding, cache Identity, request, revision Opaque, err error) (coreContext, Identity, ExactVerificationResult, error) {
		return coreContext{s: stages, pass: makeReceiptPass(kind, stages), b: binding, cache: cache, request: request, rev: revision, origin: origin}, binding, ExactVerificationResult{}, err
	}
	registryStarted := time.Since(origin)
	if err := coreContextFailure(ctx); err != nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", "", err)
	}
	if core == nil || core.s == nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", "", fail(AdmissionRejected))
	}
	s, err := core.s.get(ctx)
	if err != nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", "", closedFailure(err, RegistrySnapshotUnavailable))
	}
	if err := coreContextFailure(ctx); err != nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", s.rev, err)
	}
	s = ownCoreState(s)
	if s.r == nil || s.v == nil || mustOpaque(s.id) != nil || mustOpaque(s.rev) != nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", s.rev, fail(AdmissionRejected))
	}
	if s.lim.validate() != nil {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", s.rev, fail(LimitExceeded))
	}
	snapshotVerifier, snapshotFrozen := s.r.Verifier.(frozen)
	admissionVerifier, admissionFrozen := s.v.(frozenAdmissionVerifier)
	if !snapshotFrozen || !admissionFrozen {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", s.rev, fail(AdmissionRejected))
	}
	s.r.Verifier, s.v = snapshotVerifier.CloneSnapshotVerifier(), admissionVerifier.CloneAdmissionVerifier()
	if s.r.Verifier == nil || s.v == nil || s.v.Identity() == "" || s.v.Identity() != Identity(s.r.Cache.AdmissionVerifierDigest) {
		stages = recordStageInterval(stages, StageRegistry, "FAILED", registryStarted, time.Since(origin))
		return failed(stages, "", "", "", s.rev, fail(AdmissionRejected))
	}
	// The source may hold stale caller-shaped cache data, but Core owns the
	// final request bytes and overwrites this field only after canonical encode.
	s.r.Cache.RequestBytesDigest = ""
	r, stages, err := resolveWithReceiptStagesContext(ctx, *s.r, origin, stages, registryStarted, false, true)
	if err != nil {
		return failed(stages, "", "", "", s.rev, err)
	}
	protocolStarted := time.Since(origin)
	// The input body snapshot, hashing, input binding, and request encoding are
	// one measured protocol operation after resolver closure. Do not copy body
	// bytes while gathering registry state: that would falsely place actual work
	// before the declared pipeline event.
	iSnapshot := cloneInputs(s.i)
	i, err := bindInputsContext(ctx, iSnapshot, &s.r.Cache)
	var q AnalyzerRequest
	var b []byte
	if err == nil {
		inputBinding, bindingErr := inputProjectionBindingContext(ctx, r, s.r.Cache, iSnapshot)
		if bindingErr != nil {
			err = bindingErr
		}
		if err == nil {
			requestID := authorityRequestID(s.id, core.instance, nonce, r, s.r.Cache, s.v.Identity(), s.rev, inputBinding, s.art, s.bin)
			q, b, err = deriveRequest(r, *s.r, i, requestID, inputBinding, s.lim)
		}
	}
	if err == nil {
		err = coreContextFailure(ctx)
	}
	if err != nil {
		stages = recordProtocolInterval(stages, "FAILED", protocolStarted, time.Since(origin), true)
		return failed(stages, "", "", "", s.rev, err)
	}
	requestDigest := sha256Digest(b)
	s.r.Cache.RequestBytesDigest = requestDigest
	r.CacheIdentity = cacheIdentity(r, s.r.Cache)
	stages = recordProtocolInterval(stages, "OK", protocolStarted, time.Since(origin), true)
	preLaunchStarted := time.Since(origin)
	h := authBinding(core.instance, r, s.r.Cache, s.v.Identity(), s.rev, nonce, iSnapshot, s.art, s.bin)
	if err := preLaunchIdentityFailure(s, r); err != nil {
		stages = recordEventInterval(stages, EventPreLaunch, "FAILED", preLaunchStarted, time.Since(origin))
		return failed(stages, h, r.CacheIdentity, requestDigest, s.rev, err)
	}
	stages = recordEventInterval(stages, EventPreLaunch, "OK", preLaunchStarted, time.Since(origin))
	x := ExactVerificationResult{State: ExactVerificationNotRun, Reason: exactReason(ExactVerifierNotRun), Verifier: Identity(s.r.Cache.ExactVerifierDigest), Profile: q.ProfileIdentity, Revision: s.rev}
	context := coreContext{AdmissionContext: AdmissionContext{Request: q}, a: s.artifact, e: s.exe, repositoryRoot: s.repositoryRoot, stagingParent: s.stagingParent, art: s.art, bin: s.bin, w: clone(b), v: s.v, s: stages, b: h, cache: r.CacheIdentity, request: requestDigest, rev: s.rev, origin: origin}
	context.pass = makeReceiptPass(kind, stages)
	return context, h, x, nil
}

func preLaunchIdentityFailure(state coreState, resolution Resolution) error {
	if state.artifact == "" || state.exe == "" || state.repositoryRoot == "" || state.stagingParent == "" || !filepath.IsAbs(state.artifact) || filepath.Clean(state.artifact) != state.artifact || !filepath.IsAbs(state.exe) || filepath.Clean(state.exe) != state.exe || !filepath.IsAbs(state.repositoryRoot) || filepath.Clean(state.repositoryRoot) != state.repositoryRoot || !filepath.IsAbs(state.stagingParent) || filepath.Clean(state.stagingParent) != state.stagingParent {
		return fail(ExecutableIdentityUnsafe)
	}
	if state.art != resolution.Release.ArtifactDigest {
		return fail(BinaryDigestMismatch)
	}
	if state.bin != resolution.Release.HostBinaryDigest {
		return fail(BinaryDigestMismatch)
	}
	return nil
}
func bindInputs(inputs []ImmutableInput, cache *CacheInputs) ([]InputHandle, error) {
	return bindInputsContext(context.TODO(), inputs, cache)
}

func bindInputsContext(ctx context.Context, inputs []ImmutableInput, cache *CacheInputs) ([]InputHandle, error) {
	if err := coreContextFailure(ctx); err != nil {
		return nil, err
	}
	if len(inputs) != len(cache.CompilationInputDigests) {
		return nil, fail(InputEvidenceMismatch)
	}
	handles := make([]InputHandle, len(inputs))
	total := 0
	for index, input := range inputs {
		if err := coreContextFailure(ctx); err != nil {
			return nil, err
		}
		total += len(input.Bytes)
		if mustOpaque(input.Handle) != nil || mustOpaque(input.Digest) != nil || len(input.Bytes) > MaxProtocolBytes || total > MaxProtocolBytes || input.Digest != cache.CompilationInputDigests[index] || sha256Digest(input.Bytes) != input.Digest {
			return nil, fail(InputEvidenceMismatch)
		}
		handles[index] = InputHandle{Handle: input.Handle, Digest: input.Digest}
	}
	if validateInputs(handles) != nil {
		return nil, fail(InputEvidenceMismatch)
	}
	cache.InputHandles, cache.SelectedInputSetDigest = handles, inputSetDigest(handles)
	if err := coreContextFailure(ctx); err != nil {
		return nil, err
	}
	return handles, nil
}
func deriveRequest(resolution Resolution, resolver Request, inputs []InputHandle, requestID Opaque, inputBinding Identity, limits ProtocolLimits) (AnalyzerRequest, []byte, error) {
	if limits.validate() != nil {
		return AnalyzerRequest{}, nil, fail(LimitExceeded)
	}
	if resolver.Lock == nil || mustOpaque(requestID) != nil || mustOpaque(Opaque(inputBinding)) != nil || resolution.Release.Protocol != AnalyzerProtocolV0 || resolver.Lock.Protocol != resolution.Release.Protocol || resolution.Clause.Protocol != resolution.Release.Protocol {
		return AnalyzerRequest{}, nil, fail(AdmissionRejected)
	}
	request := AnalyzerRequest{Protocol: resolution.Release.Protocol, RequestID: requestID, ProfileIdentity: resolution.Profile.Identity(), LockDigest: Opaque(lockDigest(*resolver.Lock)), RegistrySnapshotDigest: resolution.RegistrySnapshotDigest, TrustEpoch: resolution.TrustEpoch, ResolverDigest: resolution.Release.ResolverDigest, ClauseID: resolution.Clause.ID, CompilationUnitID: resolution.Unit.ID, Target: resolution.Target, ProjectionDigest: resolution.Release.ProjectionDigest, ProjectionScheme: resolution.Release.ProjectionScheme, Inputs: inputs, InputBinding: inputBinding, Features: clone(resolution.Profile.Features), Limits: limits}
	encoded, err := MarshalAnalyzerRequest(request)
	if err != nil {
		return AnalyzerRequest{}, nil, err
	}
	return request, encoded, nil
}
func inputProjectionBinding(resolution Resolution, cache CacheInputs, inputs []ImmutableInput) Identity {
	binding, _ := inputProjectionBindingContext(context.TODO(), resolution, cache, inputs)
	return binding
}

func inputProjectionBindingContext(ctx context.Context, resolution Resolution, cache CacheInputs, inputs []ImmutableInput) (Identity, error) {
	parts := make([]string, 0, 6+len(inputs)*3)
	parts = append(parts, string(resolution.Release.ProjectionDigest), string(resolution.Release.ProjectionScheme), string(resolution.Release.ClosedInputFamilyDigest), string(cache.FullSnapshotDigest), string(cache.ClosedInputFamilyDigest), string(cache.SelectedInputSetDigest))
	for _, input := range inputs {
		if err := coreContextFailure(ctx); err != nil {
			return "", err
		}
		parts = append(parts, string(input.Handle), string(input.Digest), string(sha256Digest(input.Bytes)))
	}
	if err := coreContextFailure(ctx); err != nil {
		return "", err
	}
	return digest("analyzer-input-projection/v1", parts...), nil
}
func authorityRequestID(seed, core, nonce Opaque, resolution Resolution, cache CacheInputs, verifier Identity, revision Opaque, inputBinding Identity, artifact, binary Opaque) Opaque {
	parts := [...]string{string(seed), string(core), string(nonce), string(resolution.Release.PluginID), string(resolution.Release.ReleaseID), string(resolution.Release.BuildID), string(resolution.Release.ManifestDigest), string(resolution.Identity), string(resolution.CacheIdentity), string(cache.FullSnapshotDigest), string(cache.ClosedInputFamilyDigest), string(cache.SelectedInputSetDigest), string(verifier), string(revision), string(inputBinding), string(artifact), string(binary)}
	return Opaque(digest("analyzer-authority-request/v2", parts[:]...))
}
func authBinding(core Opaque, resolution Resolution, cache CacheInputs, verifier Identity, revision, nonce Opaque, inputs []ImmutableInput, artifact, binary Opaque) Identity {
	parts := []string{string(core), string(resolution.Release.PluginID), string(resolution.Release.ReleaseID), string(resolution.Release.BuildID), string(resolution.Release.ManifestDigest), string(resolution.Identity), string(resolution.CacheIdentity), string(cache.RequestBytesDigest), string(cache.AdmissionVerifierDigest), string(cache.ExactVerifierDigest), string(verifier), string(revision), string(nonce), string(artifact), string(binary), string(resolution.Release.ArtifactDigest), string(resolution.Release.HostBinaryDigest), string(resolution.Release.Protocol)}
	for _, input := range inputs {
		parts = append(parts, string(input.Handle), string(input.Digest), string(sha256Digest(input.Bytes)))
	}
	return digest("admission-authority/v4", parts...)
}
func receiptParts(authority, live, cache Identity, revision, request Opaque, terminal ReceiptTerminal, passes []ReceiptPass) []string {
	count := 12
	for _, pass := range passes {
		count += 4 + len(pass.events)*9
	}
	parts := make([]string, 0, count)
	parts = append(parts, string(authority), string(live), string(cache), string(revision), string(request), string(terminal.State), string(terminal.Reason), strconv.FormatBool(terminal.ProcessStarted), strconv.FormatBool(terminal.ProcessCompleted), string(terminal.Termination), string(terminal.Cleanup), string(terminal.CleanupError))
	for _, pass := range passes {
		parts = append(parts, string(pass.Kind), strconv.FormatInt(pass.Started.Nanoseconds(), 10), strconv.FormatInt(pass.Ended.Nanoseconds(), 10), string(pass.FirstFailure))
		for _, event := range pass.events {
			parts = append(parts, string(event.Stage), string(event.Event), event.Result, strconv.FormatInt(event.Started.Nanoseconds(), 10), strconv.FormatInt(event.Ended.Nanoseconds(), 10), strconv.FormatInt(event.Elapsed.Nanoseconds(), 10), strconv.FormatInt(event.CleanupElapsed.Nanoseconds(), 10), string(event.Cleanup), string(event.CleanupError))
		}
	}
	return parts
}
func makeCoreReceipt(authority, live, cache Identity, revision Opaque, passes []ReceiptPass) CoreReceipt {
	return makeCoreReceiptTerminal(authority, live, cache, revision, "", ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, passes)
}
func makeCoreReceiptTerminal(authority, live, cache Identity, revision, request Opaque, terminal ReceiptTerminal, passes []ReceiptPass) CoreReceipt {
	passes = clonePasses(passes)
	parts := receiptParts(authority, live, cache, revision, request, terminal, passes)
	return CoreReceipt{authority: authority, liveAuthority: live, cache: cache, revision: revision, request: request, terminal: terminal, digest: digest("core-receipt/v3", parts...), raw: []byte(canonical("core-receipt/v3", parts...)), passes: passes}
}
func invocationReceipt(context coreContext) CoreReceipt {
	context.syncPass()
	if context.receipt.authority != "" {
		return makeCoreReceiptTerminal(context.receipt.authority, receiptIdentity(context.b, context.receipt.liveAuthority), receiptIdentity(context.cache, context.receipt.cache), receiptOpaque(context.rev, context.receipt.revision), receiptOpaque(context.request, context.receipt.request), context.receipt.terminal, append(context.previous, context.pass))
	}
	if context.b == "" || context.cache == "" || context.rev == "" {
		return CoreReceipt{}
	}
	return makeCoreReceiptTerminal(context.b, context.b, context.cache, context.rev, context.request, ReceiptTerminal{State: ReceiptOpen, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}, append(context.previous, context.pass))
}
func (r CoreReceipt) valid() bool {
	if mustOpaque(Opaque(r.authority)) != nil || mustOpaque(Opaque(r.liveAuthority)) != nil || mustOpaque(Opaque(r.cache)) != nil || mustOpaque(r.revision) != nil || len(r.passes) == 0 || !validReceiptTerminal(r.terminal) {
		return false
	}
	for index, pass := range r.passes {
		if !validReceiptPass(pass) || index > 0 && pass.Started < r.passes[index-1].Ended || !validReceiptPassSequence(r.passes[:index+1]) {
			return false
		}
	}
	parts := receiptParts(r.authority, r.liveAuthority, r.cache, r.revision, r.request, r.terminal, r.passes)
	return r.digest == digest("core-receipt/v3", parts...) && string(r.raw) == canonical("core-receipt/v3", parts...)
}

func validReceiptTerminal(terminal ReceiptTerminal) bool {
	switch terminal.State {
	case ReceiptOpen:
		return terminal.Reason == "" && !terminal.ProcessStarted && !terminal.ProcessCompleted && terminal.Termination == analyzerexec.TerminationNotRun && terminal.Cleanup == analyzerexec.CleanupNotRun && terminal.CleanupError == analyzerexec.CleanupErrorNone
	case ReceiptSucceeded:
		return terminal.Reason == "" && terminal.ProcessStarted && terminal.ProcessCompleted && terminal.Termination == analyzerexec.TerminationExited && terminal.Cleanup == analyzerexec.CleanupObserved && terminal.CleanupError == analyzerexec.CleanupErrorNone
	case ReceiptFailed:
		if !validReason(terminal.Reason) {
			return false
		}
		if !terminal.ProcessStarted {
			if terminal.ProcessCompleted || terminal.Termination != analyzerexec.TerminationNotRun {
				return false
			}
			if terminal.Cleanup == analyzerexec.CleanupNotRun {
				return terminal.CleanupError == analyzerexec.CleanupErrorNone
			}
			return terminal.Cleanup == analyzerexec.CleanupFailed && terminal.CleanupError != analyzerexec.CleanupErrorNone || terminal.Cleanup == analyzerexec.CleanupRejected && terminal.CleanupError == analyzerexec.CleanupErrorNone
		}
		if terminal.Termination == analyzerexec.TerminationNotRun || terminal.Cleanup == analyzerexec.CleanupNotRun {
			return false
		}
		return terminal.Cleanup == analyzerexec.CleanupObserved && terminal.CleanupError == analyzerexec.CleanupErrorNone || terminal.Cleanup == analyzerexec.CleanupFailed && terminal.CleanupError != analyzerexec.CleanupErrorNone || terminal.Cleanup == analyzerexec.CleanupRejected && terminal.CleanupError == analyzerexec.CleanupErrorNone
	default:
		return false
	}
}
func validReceiptPass(pass ReceiptPass) bool {
	schema := receiptPassSchema(pass.Kind)
	if len(schema) == 0 || len(pass.events) != len(schema) || pass.Started < 0 || pass.Ended < 0 || pass.Ended < pass.Started {
		return false
	}
	var end time.Duration
	first := Stage("")
	started := false
	terminated := false
	for index, event := range pass.events {
		if event.Stage != schema[index].stage || event.Event != schema[index].event {
			return false
		}
		if !validReceiptEventResult(pass.Kind, event.Event, event.Result) {
			return false
		}
		if event.Result == "NOT_RUN" {
			if event.Started != 0 || event.Ended != 0 || event.Elapsed != 0 || event.CleanupElapsed != 0 || event.Cleanup != analyzerexec.CleanupNotRun || event.CleanupError != analyzerexec.CleanupErrorNone {
				return false
			}
			terminated = true
			continue
		}
		if terminated || event.Started < 0 || event.Ended < 0 || event.Elapsed < 0 || event.CleanupElapsed < 0 || started && event.Started < end || event.Ended < event.Started || event.Elapsed != event.Ended-event.Started {
			return false
		}
		if event.Cleanup != analyzerexec.CleanupNotRun && event.Cleanup != analyzerexec.CleanupObserved && event.Cleanup != analyzerexec.CleanupFailed && event.Cleanup != analyzerexec.CleanupRejected {
			return false
		}
		if event.Event != EventLaunch && (event.Cleanup != analyzerexec.CleanupNotRun || event.CleanupElapsed != 0 || event.CleanupError != analyzerexec.CleanupErrorNone) {
			return false
		}
		switch event.Cleanup {
		case analyzerexec.CleanupNotRun:
			if event.CleanupElapsed != 0 || event.CleanupError != analyzerexec.CleanupErrorNone {
				return false
			}
		case analyzerexec.CleanupObserved:
			if event.CleanupElapsed > event.Elapsed || event.CleanupError != analyzerexec.CleanupErrorNone {
				return false
			}
		case analyzerexec.CleanupFailed:
			if event.Event != EventLaunch || event.Result != "FAILED" || event.CleanupElapsed > event.Elapsed || event.CleanupError == analyzerexec.CleanupErrorNone {
				return false
			}
		case analyzerexec.CleanupRejected:
			if event.Result != "FAILED" || event.CleanupElapsed != 0 || event.CleanupError != analyzerexec.CleanupErrorNone {
				return false
			}
		default:
			return false
		}
		started = true
		end = event.Ended
		if event.Result == "FAILED" {
			terminated, first = true, event.Stage
		}
	}
	if !started || pass.FirstFailure != first || pass.Started != pass.events[0].Started || pass.Ended != end {
		return false
	}
	if first != "" && !terminated {
		return false
	}
	if first == "" {
		endpoint := successfulPassEndpoint(pass.Kind)
		if endpoint < 0 || endpoint >= len(pass.events) {
			return false
		}
		for index, event := range pass.events {
			if index <= endpoint && event.Result == "NOT_RUN" || index > endpoint && event.Result != "NOT_RUN" {
				return false
			}
		}
	}
	return pass.Started <= pass.Ended
}
func clonePasses(value []ReceiptPass) []ReceiptPass {
	result := clone(value)
	for index := range result {
		result[index].events = clone(result[index].events)
	}
	return result
}
func receiptIdentity(value, fallback Identity) Identity {
	if mustOpaque(Opaque(value)) == nil {
		return value
	}
	return fallback
}
func receiptOpaque(value, fallback Opaque) Opaque {
	if mustOpaque(value) == nil {
		return value
	}
	return fallback
}
func sha256Digest(value []byte) Opaque {
	sum := sha256.Sum256(value)
	return Opaque(hex.EncodeToString(sum[:]))
}
func verifyCandidate(ctx context.Context, verifier AdmissionVerifier, state AdmissionContext, response AnalyzerResponse) (AnalyzerResponse, Admission, error) {
	frozen, candidate := cloneResponse(response), cloneResponse(response)
	state.Request = cloneWire(state.Request)
	if verifier == nil || verifier.Identity() == "" || frozen.validate(state.Request) != nil {
		return AnalyzerResponse{}, AdmissionDenied, fail(AdmissionRejected)
	}
	admission, err := verifier.VerifyAdmission(ctx, state, candidate)
	if err != nil || admission != AdmissionAdmitted || !sameResponse(candidate, frozen) || frozen.validate(state.Request) != nil {
		return AnalyzerResponse{}, AdmissionDenied, fail(AdmissionRejected)
	}
	return frozen, AdmissionAdmitted, nil
}
func lockDigest(lock RepositoryLock) Identity {
	return digest("terminal-lock/v1", string(lock.CapabilityID), string(lock.PluginID), string(lock.ReleaseID), string(lock.BuildID), string(lock.ManifestDigest), string(lock.Protocol), string(lock.ArtifactDigest), string(lock.HostBinaryDigest), string(lock.ProjectionDigest), string(lock.ProjectionScheme), string(lock.ResolverDigest), string(lock.Profile))
}
func ownCoreState(value coreState) coreState {
	if value.r != nil {
		request := cloneReq(*value.r)
		value.r = &request
	}
	// Immutable input bytes are owned only at the measured protocol boundary.
	// Their handles/digests are inert until then and are cloned with the bytes.
	return value
}
func cloneInputs(value []ImmutableInput) []ImmutableInput {
	result := clone(value)
	for index := range result {
		result[index].Bytes = clone(result[index].Bytes)
	}
	return result
}
func cloneReg(value RegistrySnapshot) RegistrySnapshot {
	value.Releases = clone(value.Releases)
	for index := range value.Releases {
		value.Releases[index] = cloneRel(value.Releases[index])
	}
	return value
}
func cloneReq(value Request) Request {
	if value.Lock != nil {
		lock := *value.Lock
		value.Lock = &lock
	}
	value.Snapshot.Releases = clone(value.Snapshot.Releases)
	for i := range value.Snapshot.Releases {
		value.Snapshot.Releases[i] = cloneRel(value.Snapshot.Releases[i])
	}
	value.Profile = cloneProf(value.Profile)
	value.Profiles = clone(value.Profiles)
	for i := range value.Profiles {
		value.Profiles[i] = cloneProf(value.Profiles[i])
	}
	value.Unit.Targets = clone(value.Unit.Targets)
	value.Cache.CompilationInputDigests = clone(value.Cache.CompilationInputDigests)
	value.Cache.InputHandles = clone(value.Cache.InputHandles)
	return value
}

func cloneRel(value Release) Release {
	value.Clauses = clone(value.Clauses)
	for i := range value.Clauses {
		value.Clauses[i] = cloneC(value.Clauses[i])
	}
	return value
}

func cloneC(value Clause) Clause {
	value.Components = clone(value.Components)
	value.Features = clone(value.Features)
	value.Dependencies = clone(value.Dependencies)
	return value
}

func cloneProf(value Profile) Profile {
	value.Components = clone(value.Components)
	value.Features = clone(value.Features)
	value.Edges = clone(value.Edges)
	return value
}

//go:noinline
func cloneResponse(value AnalyzerResponse) AnalyzerResponse {
	value.Inputs, value.Facts, value.ProfileEvidence, value.Diagnostics = clone(value.Inputs), clone(value.Facts), clone(value.ProfileEvidence), clone(value.Diagnostics)
	return value
}

//go:noinline
func cloneWire(value AnalyzerRequest) AnalyzerRequest {
	value.Inputs, value.Features = clone(value.Inputs), clone(value.Features)
	return value
}

func failedTerminal(run analyzerexec.Result, err error) ReceiptTerminal {
	reason := AnalyzerFailure
	var failure *Failure
	if errors.As(err, &failure) && validReason(failure.Reason) {
		reason = failure.Reason
	}
	if !run.Started {
		if run.CleanupState == analyzerexec.CleanupFailed && run.CleanupError != analyzerexec.CleanupErrorNone {
			return ReceiptTerminal{State: ReceiptFailed, Reason: reason, Termination: analyzerexec.TerminationNotRun, Cleanup: run.CleanupState, CleanupError: run.CleanupError}
		}
		if run.CleanupState == analyzerexec.CleanupRejected && run.CleanupError == analyzerexec.CleanupErrorNone {
			return ReceiptTerminal{State: ReceiptFailed, Reason: reason, Termination: analyzerexec.TerminationNotRun, Cleanup: run.CleanupState, CleanupError: run.CleanupError}
		}
		return ReceiptTerminal{State: ReceiptFailed, Reason: reason, Termination: analyzerexec.TerminationNotRun, Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}
	}
	termination := run.Termination
	if termination == analyzerexec.TerminationNotRun {
		termination = analyzerexec.TerminationFailed
	}
	cleanup, cleanupError := run.CleanupState, run.CleanupError
	if cleanup == analyzerexec.CleanupObserved && cleanupError == analyzerexec.CleanupErrorNone {
		return ReceiptTerminal{State: ReceiptFailed, Reason: reason, ProcessStarted: true, ProcessCompleted: run.Completed, Termination: termination, Cleanup: cleanup, CleanupError: cleanupError}
	}
	if cleanup == analyzerexec.CleanupFailed && cleanupError != analyzerexec.CleanupErrorNone {
		return ReceiptTerminal{State: ReceiptFailed, Reason: reason, ProcessStarted: true, ProcessCompleted: run.Completed, Termination: termination, Cleanup: cleanup, CleanupError: cleanupError}
	}
	return ReceiptTerminal{State: ReceiptFailed, Reason: reason, ProcessStarted: true, ProcessCompleted: run.Completed, Termination: termination, Cleanup: analyzerexec.CleanupRejected, CleanupError: analyzerexec.CleanupErrorNone}
}

func successfulTerminal(run analyzerexec.Result) ReceiptTerminal {
	return ReceiptTerminal{State: ReceiptSucceeded, ProcessStarted: true, ProcessCompleted: true, Termination: analyzerexec.TerminationExited, Cleanup: analyzerexec.CleanupObserved, CleanupError: analyzerexec.CleanupErrorNone}
}

func finalizeReceipt(receipt CoreReceipt, terminal ReceiptTerminal) CoreReceipt {
	if receipt.authority == "" {
		return CoreReceipt{}
	}
	return makeCoreReceiptTerminal(receipt.authority, receipt.liveAuthority, receipt.cache, receipt.revision, receipt.request, terminal, receipt.passes)
}

//go:noinline
func failedInvocation(authority *AdmissionAuthority, state coreContext, stages []StageDiagnostic, stage Stage, stderrBytes int, run analyzerexec.Result, err error) (InvocationResult, error) {
	stages = recordStage(stages, stage, "FAILED", 0)
	state.s = stages
	state.syncPass()
	receipt := finalizeReceipt(invocationReceipt(state), failedTerminal(run, err))
	resultStages := clone(stages)
	if receipt.Digest() != "" {
		resultStages = receipt.StageDiagnostics()
	}
	return InvocationResult{Observation: ObservationNone, Admission: "NOT_RUN", ExactVerification: exactNotRun(authority), Stages: resultStages, Receipt: receipt, StderrBytes: stderrBytes}, err
}
func closedFailure(err error, fallback Reason) error {
	var failure *Failure
	if errors.As(err, &failure) {
		return err
	}
	return fail(fallback)
}
func firstFailedStage(stages []StageDiagnostic, fallback Stage) Stage {
	for _, stage := range stages {
		if stage.Result == "FAILED" {
			return stage.Stage
		}
	}
	return fallback
}
func mergeStages(left, right []StageDiagnostic) []StageDiagnostic {
	// Legacy helper retained for test compatibility. A refresh is another pass,
	// never an elapsed-time aggregate of the prior pass.
	return append(clone(left), clone(right)...)
}
func exactNotRun(authority *AdmissionAuthority) ExactVerificationResult {
	if authority == nil {
		return ExactVerificationResult{State: ExactVerificationNotRun, Reason: exactReason(ExactVerifierNotRun)}
	}
	return authority.x
}

type receiptEventSpec struct {
	stage Stage
	event ReceiptEvent
}

var authorityPassSchema = []receiptEventSpec{
	{StageRegistry, EventRegistry},
	{StageLock, EventLock},
	{StageResolve, EventResolve},
	{StageCompatibility, EventCompatibility},
	{StageDependency, EventDependency},
	{StageProtocol, EventProtocolEncode},
	{StageLaunch, EventPreLaunch},
	{StageAdmission, EventAdmission},
	{StageExactVerification, EventExact},
}

var executionPassSchema = []receiptEventSpec{
	{StageLaunch, EventLaunch},
	{StageProtocol, EventProtocolDecode},
}

var admissionPassSchema = []receiptEventSpec{
	{StageAdmission, EventAdmission},
	{StageExactVerification, EventExact},
}

func receiptPassSchema(kind ReceiptPassKind) []receiptEventSpec {
	switch kind {
	case PassAuthorityIssue, PassPreLaunchRefresh, PassPreAdmissionRefresh:
		return authorityPassSchema
	case PassInvocation:
		return executionPassSchema
	case PassAdmission, PassInvocationGuard:
		return admissionPassSchema
	default:
		return nil
	}
}

func initialPassStages(kind ReceiptPassKind) []StageDiagnostic {
	schema := receiptPassSchema(kind)
	stages := make([]StageDiagnostic, len(schema))
	for index, spec := range schema {
		stages[index] = StageDiagnostic{Stage: spec.stage, Event: spec.event, Result: "NOT_RUN", Cleanup: analyzerexec.CleanupNotRun, CleanupError: analyzerexec.CleanupErrorNone}
	}
	return stages
}

func initialStages() []StageDiagnostic                { return initialAuthorityStages() }
func initialAuthorityStages() []StageDiagnostic       { return initialPassStages(PassAuthorityIssue) }
func initialInvocationStages() []StageDiagnostic      { return initialPassStages(PassInvocation) }
func initialInvocationGuardStages() []StageDiagnostic { return initialPassStages(PassInvocationGuard) }
func admissionStages() []StageDiagnostic              { return initialPassStages(PassAdmission) }

func validReceiptPassSequence(passes []ReceiptPass) bool {
	if len(passes) == 0 || passes[0].Kind != PassAuthorityIssue {
		return false
	}
	for index := 1; index < len(passes); index++ {
		if passes[index-1].FirstFailure != "" {
			return false
		}
		previous, current := passes[index-1].Kind, passes[index].Kind
		if previous == PassAuthorityIssue && (current == PassPreLaunchRefresh || current == PassInvocationGuard) {
			continue
		}
		if previous == PassPreLaunchRefresh && current == PassInvocation {
			continue
		}
		if previous == PassInvocation && current == PassPreAdmissionRefresh {
			continue
		}
		if previous == PassPreAdmissionRefresh && current == PassAdmission {
			continue
		}
		return false
	}
	return true
}

func successfulPassEndpoint(kind ReceiptPassKind) int {
	switch kind {
	case PassAuthorityIssue:
		return 6 // pre-launch identity is the final authority-issue operation.
	case PassPreLaunchRefresh, PassPreAdmissionRefresh:
		return 7 // live authority equality check completes each refresh.
	case PassInvocation:
		return 1 // launch and decode are both required for successful execution.
	case PassAdmission:
		return 0 // exact verification remains explicitly NOT_RUN.
	default:
		return -1
	}
}

func validReceiptEventResult(kind ReceiptPassKind, event ReceiptEvent, result string) bool {
	if event == EventExact {
		return result == "NOT_RUN"
	}
	if result == "NOT_RUN" || result == "FAILED" {
		return true
	}
	switch kind {
	case PassAuthorityIssue, PassPreLaunchRefresh, PassPreAdmissionRefresh:
		return result == "OK" && event != EventExact
	case PassInvocation:
		return result == "OK"
	case PassAdmission:
		return event == EventAdmission && result == string(AdmissionAdmitted)
	case PassInvocationGuard:
		return false
	default:
		return false
	}
}
func resultFor(err error) string {
	if err != nil {
		return "FAILED"
	}
	return "OK"
}

//go:noinline
func recordStage(stages []StageDiagnostic, stage Stage, result string, elapsed time.Duration) []StageDiagnostic {
	for index := range stages {
		if stages[index].Stage == stage {
			if stages[index].Result == "NOT_RUN" {
				stages[index].Started, stages[index].Ended, stages[index].Elapsed = 0, elapsed, elapsed
			}
			if stages[index].Result == "NOT_RUN" || result == "FAILED" {
				stages[index].Result = result
			}
			return stages
		}
	}
	return stages
}
func recordStageInterval(stages []StageDiagnostic, stage Stage, result string, started, ended time.Duration) []StageDiagnostic {
	if started < 0 || ended < started {
		return recordStage(stages, stage, "FAILED", 0)
	}
	for index := range stages {
		if stages[index].Stage != stage {
			continue
		}
		if stages[index].Result != "NOT_RUN" {
			return stages
		}
		stages[index].Started, stages[index].Ended, stages[index].Elapsed = started, ended, ended-started
		if stages[index].Result == "NOT_RUN" || result == "FAILED" {
			stages[index].Result = result
		}
		return stages
	}
	return stages
}
func recordEventInterval(stages []StageDiagnostic, event ReceiptEvent, result string, started, ended time.Duration) []StageDiagnostic {
	if started < 0 || ended < started {
		return stages
	}
	for index := range stages {
		if stages[index].Event != event || stages[index].Result != "NOT_RUN" {
			continue
		}
		stages[index].Started, stages[index].Ended, stages[index].Elapsed, stages[index].Result = started, ended, ended-started, result
		return stages
	}
	return stages
}
func recordLaunch(stages []StageDiagnostic, result string, run analyzerexec.Result, started time.Time) []StageDiagnostic {
	if run.Elapsed == 0 {
		run.Elapsed = time.Since(started)
	}
	stages = recordLaunchInterval(stages, result, run, 0, run.Elapsed)
	return stages
}
func recordLaunchInterval(stages []StageDiagnostic, result string, run analyzerexec.Result, started, ended time.Duration) []StageDiagnostic {
	stages = recordEventInterval(stages, EventLaunch, result, started, ended)
	for index := range stages {
		if stages[index].Event == EventLaunch {
			if !run.Started {
				// The backend observation is rejected by InvokeAnalyzer, but no
				// process started. Preserve that closed fact in the receipt rather
				// than manufacturing a started cleanup state.
				if run.CleanupState == analyzerexec.CleanupFailed && run.CleanupError != analyzerexec.CleanupErrorNone {
					stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError = run.Cleanup, analyzerexec.CleanupFailed, run.CleanupError
				} else if run.CleanupState == analyzerexec.CleanupRejected && run.CleanupError == analyzerexec.CleanupErrorNone {
					stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError = 0, analyzerexec.CleanupRejected, analyzerexec.CleanupErrorNone
				} else {
					stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError = 0, analyzerexec.CleanupNotRun, analyzerexec.CleanupErrorNone
				}
			} else if run.CleanupState == analyzerexec.CleanupObserved && run.CleanupError == analyzerexec.CleanupErrorNone && run.Cleanup >= 0 {
				stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError = run.Cleanup, analyzerexec.CleanupObserved, analyzerexec.CleanupErrorNone
			} else if run.CleanupState == analyzerexec.CleanupFailed && run.CleanupError != analyzerexec.CleanupErrorNone && run.Cleanup >= 0 {
				stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError = run.Cleanup, analyzerexec.CleanupFailed, run.CleanupError
			} else {
				stages[index].CleanupElapsed, stages[index].Cleanup, stages[index].CleanupError, stages[index].Result = 0, analyzerexec.CleanupRejected, analyzerexec.CleanupErrorNone, "FAILED"
			}
			return stages
		}
	}
	return stages
}
func recordProtocol(stages []StageDiagnostic, result string, elapsed time.Duration, encode bool) []StageDiagnostic {
	return recordProtocolInterval(stages, result, 0, elapsed, encode)
}
func recordProtocolInterval(stages []StageDiagnostic, result string, started, ended time.Duration, encode bool) []StageDiagnostic {
	want := EventProtocolDecode
	if encode {
		want = EventProtocolEncode
	}
	for index := range stages {
		if stages[index].Event != want {
			continue
		}
		if stages[index].Result != "NOT_RUN" || started < 0 || ended < started {
			return stages
		}
		stages[index].Started, stages[index].Ended, stages[index].Elapsed, stages[index].Result = started, ended, ended-started, result
		return stages
	}
	return stages
}
func makeReceiptPass(kind ReceiptPassKind, events []StageDiagnostic) ReceiptPass {
	pass := ReceiptPass{Kind: kind, events: clone(events)}
	var started bool
	for _, event := range pass.events {
		if event.Result == "NOT_RUN" {
			continue
		}
		if !started {
			pass.Started, started = event.Started, true
		}
		pass.Ended = event.Ended
		if pass.FirstFailure == "" && event.Result == "FAILED" {
			pass.FirstFailure = event.Stage
		}
	}
	return pass
}
func (context *coreContext) syncPass() {
	context.pass = makeReceiptPass(context.pass.Kind, context.s)
}
func (context coreContext) nextPass(kind ReceiptPassKind, stages []StageDiagnostic) coreContext {
	context.syncPass()
	context.previous = append(context.previous, context.pass)
	context.s, context.pass = clone(stages), makeReceiptPass(kind, stages)
	return context
}
func mapExecutionFailure(err error) error {
	switch {
	case analyzerexec.Is(err, analyzerexec.IdentityUnsafe):
		return fail(ExecutableIdentityUnsafe)
	case analyzerexec.Is(err, analyzerexec.DigestMismatch):
		return fail(BinaryDigestMismatch)
	case analyzerexec.Is(err, analyzerexec.Race):
		return fail(ExecutableRace)
	case analyzerexec.Is(err, analyzerexec.Unsupported):
		return fail(HostPlatformUnsupported)
	case analyzerexec.Is(err, analyzerexec.Limit):
		return fail(LimitExceeded)
	case analyzerexec.Is(err, analyzerexec.Timeout):
		return fail(Timeout)
	case analyzerexec.Is(err, analyzerexec.Cancelled), errors.Is(err, context.Canceled):
		return fail(Cancelled)
	default:
		return fail(AnalyzerFailure)
	}
}

func rejectedExecutionResult(result analyzerexec.Result) analyzerexec.Result {
	result.Cleanup = 0
	result.CleanupState = analyzerexec.CleanupRejected
	result.CleanupError = analyzerexec.CleanupErrorNone
	return result
}

func coreContextFailure(ctx context.Context) error {
	if ctx == nil {
		return fail(Cancelled)
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fail(Timeout)
		}
		return fail(Cancelled)
	}
	return nil
}
