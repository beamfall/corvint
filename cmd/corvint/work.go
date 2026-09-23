package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

const (
	workPolicyPath     = ".corvint/work-queue-policy.json"
	workAggregateLimit = 49 << 20
	workStderrLimit    = 1 << 20
	// workSelfDogfoodMapping is Corvint's own closed worklistadapter mapping
	// version (decision 0348, amending decision 0321/WQO-V0-046).
	workSelfDogfoodMapping = "decision-0046-v0"
)

type workOptions struct {
	operation string
	envelope  string
	limit     workqueue.Count
}

type workSource struct {
	qualified          *worksource.Source
	source             workqueue.RepositorySource
	manifest           workManifest
	policyRaw          []byte
	adapterRaw         []byte
	adapterOID         string
	adapterMode        string
	qualificationError error
}

type workCapture struct {
	snapshot        *workqueue.Snapshot
	details         *workqueue.DetailsDocument
	checkpoint      *workqueue.CheckpointDocument
	observation     *workqueue.Observation
	closure         workqueue.CollisionClosure
	runner          *workAdapterRunner
	materialization *workMaterialization
	opening         workSource
	monitoredRoots  []string
	closingError    error
	storeScope      bool
}

func parseWorkInvocation(arguments []string) (string, []string, bool, error) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "work" {
		return "", nil, false, nil
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
	} else {
		var err error
		root, err = normalizeRoot(root)
		if err != nil {
			return "", nil, true, err
		}
	}
	return root, arguments[index+1:], true, nil
}

func parseWorkOptions(arguments []string) (workOptions, error) {
	options := workOptions{}
	if len(arguments) == 0 {
		return options, errors.New("missing work operation")
	}
	options.operation = arguments[0]
	switch options.operation {
	case "observe":
		if len(arguments) != 1 {
			return options, errors.New("observe takes no arguments")
		}
	case "propose-wave":
		var limit string
		for index := 1; index < len(arguments); {
			name, value, inline := strings.Cut(arguments[index], "=")
			if name != "--envelope" && name != "--limit" {
				return options, errors.New("unrecognized work argument")
			}
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return options, errors.New("missing work argument value")
				}
				value, index = arguments[index+1], index+2
			} else {
				index++
			}
			if name == "--envelope" {
				if options.envelope != "" {
					return options, errors.New("duplicate envelope")
				}
				options.envelope = value
			} else {
				if limit != "" {
					return options, errors.New("duplicate limit")
				}
				limit = value
			}
		}
		if options.envelope == "" || limit == "" {
			return options, errors.New("propose-wave requires --envelope and --limit")
		}
		parsed, err := workqueue.ParseCount(limit)
		if err != nil || parsed < 1 || parsed > 128 {
			return options, errors.New("limit must be in 1..128")
		}
		options.limit = parsed
	default:
		return options, errors.New("unknown work operation")
	}
	return options, nil
}

// workHangBound bounds one `corvint work` invocation: source acquisition, every adapter
// operation, and the final freshness check together. It is a hang detector, not a
// performance budget (decision 0082). The former 30 s per operation failed the WQO-V0-004
// and WQO-V0-017 gate tests at host load 27-43 while nothing hung. It is a variable only so
// a test can reach expiry without waiting for it.
var workHangBound = 10 * time.Minute

var errWorkHang = errors.New("corvint work hang detector expired")

func runWork(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) (exit int) {
	if len(arguments) > 0 && arguments[0] == "plan-fixture" {
		return runTaskmanFixture(ctx, root, arguments[1:], stdout, stderr)
	}
	ctx, cancel := context.WithTimeoutCause(ctx, workHangBound, errWorkHang)
	defer func() {
		// The stdout code stays the closed INPUT_LIMIT (WQO-V0-015); stderr names the
		// cause, because no input bound was measured when the detector fires.
		if exit != 0 && errors.Is(context.Cause(ctx), errWorkHang) {
			io.WriteString(stderr, "corvint work: stopped by the "+workHangBound.String()+" hang detector; no input bound was exceeded\n")
		}
		cancel()
	}()
	options, err := parseWorkOptions(arguments)
	if err != nil {
		return emitWorkError(stdout, "MALFORMED_INPUT")
	}
	var envelope *workqueue.CapacityEnvelope
	if options.operation == "propose-wave" {
		raw, readErr := workReadBoundedFile(options.envelope, (1<<20)+1)
		if readErr != nil {
			return emitWorkError(stdout, "MALFORMED_INPUT")
		}
		envelope, err = workqueue.ParseEnvelope(raw)
		if err != nil {
			return emitWorkError(stdout, workCommandError(err))
		}
	}
	capture, commandCode := observeWork(ctx, root)
	if commandCode != "" {
		return emitWorkError(stdout, commandCode)
	}
	defer capture.Close()
	if options.operation == "observe" {
		result := &workqueue.CommandResult{Observation: capture.observation, State: "OK"}
		workqueue.RefreshCommandResult(result)
		return writeWorkResult(stdout, result)
	}
	return proposeWork(ctx, root, capture, envelope, options.limit, stdout)
}

func proposeWork(ctx context.Context, root string, capture *workCapture, envelope *workqueue.CapacityEnvelope, limit workqueue.Count, stdout io.Writer) int {
	capture.snapshot.ObservationID = capture.observation.ID
	capture.snapshot.QueueSourceID = capture.observation.QueueSourceID
	capture.snapshot.ObservationState = capture.observation.State
	capture.snapshot.ObservationUnknowns = append([]string(nil), capture.observation.Unknowns...)
	proposal, err := workqueue.ProposeWave(capture.snapshot, envelope, capture.closure, limit)
	if err != nil {
		return emitWorkError(stdout, workCommandError(err))
	}
	drift, commandCode := workFreshAtReturn(ctx, root, capture)
	if commandCode != "" {
		return emitWorkError(stdout, commandCode)
	}
	if proposal.ObservationID != capture.observation.ID {
		capture.snapshot.ObservationID = capture.observation.ID
		capture.snapshot.QueueSourceID = capture.observation.QueueSourceID
		capture.snapshot.ObservationState = capture.observation.State
		capture.snapshot.ObservationUnknowns = append([]string(nil), capture.observation.Unknowns...)
		proposal, err = workqueue.ProposeWave(capture.snapshot, envelope, capture.closure, limit)
		if err != nil {
			return emitWorkError(stdout, workCommandError(err))
		}
	}
	if drift {
		proposal.MarkStale()
	}
	result := &workqueue.CommandResult{Proposal: proposal, State: "OK"}
	workqueue.RefreshCommandResult(result)
	return writeWorkResult(stdout, result)
}

func observeWork(parent context.Context, root string) (*workCapture, string) {
	if _, err := worksource.PlatformPath(); err != nil {
		return nil, "UNSUPPORTED_PLATFORM"
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	opening, err := acquireWorkSource(ctx, root)
	if err != nil {
		return nil, workSourceCommandError(ctx, err)
	}
	policy, err := workqueue.ParsePolicy(opening.policyRaw)
	if err != nil {
		opening.qualified.Close()
		return nil, workCommandError(err)
	}
	if opening.adapterMode != "100755" {
		opening.qualified.Close()
		return nil, "SOURCE_UNQUALIFIED"
	}
	materialization, err := newWorkMaterialization(ctx, opening.qualified)
	if err != nil {
		opening.qualified.Close()
		return nil, "SOURCE_UNQUALIFIED"
	}
	capture := &workCapture{opening: opening, materialization: materialization}
	completed := false
	defer func() {
		if !completed {
			capture.Close()
		}
	}()
	adapterPath := filepath.Join(materialization.target, filepath.FromSlash(policy.AdapterPath))
	runner, err := newWorkAdapterRunner(ctx, materialization.target, adapterPath, opening, policy)
	if err != nil {
		return nil, "SOURCE_UNQUALIFIED"
	}
	runner.env = append([]string(nil), materialization.environment...)
	runner.verifyTarget = materialization.Verify
	capture.runner = runner
	if err := runner.qualifyBoundExecutable(); err != nil {
		return nil, "SOURCE_UNQUALIFIED"
	}
	capture.monitoredRoots = []string{opening.qualified.Root, opening.qualified.GitDir, opening.qualified.CommonDir, materialization.target}
	opening.manifest = workMutationManifest(ctx, capture.monitoredRoots)
	capture.opening = opening
	snapshotRaw, snapshotReceipt, err := runner.run("snapshot", policy.Operations.Snapshot, 16<<20)
	if err != nil {
		return nil, runner.commandError(err)
	}
	snapshot, err := workqueue.ParseSnapshot(snapshotRaw)
	if err != nil {
		return nil, workCommandError(err)
	}
	detailsRaw, detailsReceipt, err := runner.run("details", policy.Operations.Details, 32<<20)
	if err != nil {
		return nil, runner.commandError(err)
	}
	details, err := workqueue.ParseDetails(detailsRaw)
	if err != nil {
		return nil, workCommandError(err)
	}
	closure := deriveWorkCollisions(ctx, materialization.target, snapshot, opening.qualified.GitPath, opening.qualified.GitEnvironment)
	checkpointRaw, verifyReceipt, err := runner.run("verify", policy.Operations.Verify, 64<<10)
	if err != nil {
		return nil, runner.commandError(err)
	}
	checkpoint, err := workqueue.ParseCheckpoint(checkpointRaw)
	if err != nil {
		return nil, workCommandError(err)
	}
	closing := closeWorkSource(ctx, root, opening, capture.monitoredRoots)
	capture.closingError = closing.qualificationError
	if code := workClosingCommandError(closing); code != "" {
		return nil, code
	}
	capture.storeScope = workMappingReproduced(opening.qualified, policy, snapshot.Canonical(), details.Canonical(), checkpoint.Canonical())
	opening.manifest.complete = capture.storeScope && opening.manifest.monitoredComplete
	closing.manifest.complete = capture.storeScope && closing.manifest.monitoredComplete
	observation := validateWorkCapture(policy, opening, closing, snapshot, details, checkpoint, []workqueue.AdapterReceipt{snapshotReceipt, detailsReceipt, verifyReceipt}, closure)
	capture.snapshot, capture.details, capture.checkpoint = snapshot, details, checkpoint
	capture.observation, capture.closure = observation, closure
	completed = true
	return capture, ""
}

func (capture *workCapture) Close() {
	if capture.runner != nil {
		capture.runner.Close()
	}
	if capture.materialization != nil {
		capture.materialization.Close()
	}
	if capture.opening.qualified != nil {
		capture.opening.qualified.Close()
	}
}

func closeWorkSource(ctx context.Context, root string, opening workSource, roots []string) workSource {
	// Read mutation evidence first: a dirty closing source must not erase a
	// positively observed change after complete adapter payload acquisition.
	manifest := workCloseManifest(opening.manifest, workMutationManifest(ctx, roots))
	closing, err := acquireWorkSource(ctx, root)
	if err != nil {
		closing = opening
		manifest.qualificationFailed = true
		closing.qualificationError = errors.Join(err, ctx.Err())
	} else {
		closing.qualified.Close()
		closing.qualified = nil
	}
	closing.manifest = manifest
	return closing
}

// Drift requires an observed qualified source/checkpoint change. A failed final
// operation has no comparison witness and retains its closed command error.
func workFreshAtReturn(parent context.Context, root string, capture *workCapture) (bool, string) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	source, err := acquireWorkSource(ctx, root)
	if err != nil {
		return false, workSourceCommandError(ctx, err)
	}
	defer source.qualified.Close()
	if source.source != capture.opening.source {
		return true, ""
	}
	policy, err := workqueue.ParsePolicy(source.policyRaw)
	if err != nil {
		return false, workCommandError(err)
	}
	if policy.ID != capture.observation.PolicyID {
		return false, "SOURCE_UNQUALIFIED"
	}
	opening := source
	opening.manifest = workCloseManifest(capture.opening.manifest, workMutationManifest(ctx, capture.monitoredRoots))
	opening.manifest.changed = opening.manifest.changed || capture.observation.MutationState == "CHANGED"
	capture.runner.ctx = ctx
	raw, receipt, err := capture.runner.run("verify", policy.Operations.Verify, 64<<10)
	if err != nil {
		return false, capture.runner.commandError(err)
	}
	document, err := workqueue.ParseCheckpoint(raw)
	if err != nil {
		return false, workCommandError(err)
	}
	closing := closeWorkSource(ctx, root, opening, capture.monitoredRoots)
	capture.closingError = closing.qualificationError
	if code := workClosingCommandError(closing); code != "" {
		return false, code
	}
	receipts := append([]workqueue.AdapterReceipt(nil), capture.observation.AdapterReceipts...)
	receipts[len(receipts)-1] = receipt
	storeScope := capture.storeScope && workMappingReproduced(source.qualified, policy, nil, nil, document.Canonical())
	opening.manifest.complete = storeScope && opening.manifest.monitoredComplete
	closing.manifest.complete = storeScope && closing.manifest.monitoredComplete
	observation := validateWorkCapture(policy, opening, closing, capture.snapshot, capture.details, document, receipts, capture.closure)
	capture.observation = observation
	checkpointDrift := document.PolicyID == policy.ID && document.SnapshotID == capture.snapshot.ID &&
		document.RepositorySource == opening.source && document.Checkpoint != capture.checkpoint.Checkpoint
	sourceDrift := !closing.manifest.qualificationFailed && closing.source != opening.source
	return checkpointDrift || sourceDrift, ""
}

// workMappingReproduced reports whether the adapter documents are exactly one
// of the two closed worklistadapter mappings of the qualified committed tree
// (WQO-V0-046): the adoptable repository worklist, or Corvint's own
// decision-0046-v0 self-dogfood worklist (decision 0348, amending decision
// 0321). Only then is that tree the queue's whole store, so complete
// monitored manifests are complete store scope. A nil document is not
// compared.
func workMappingReproduced(source *worksource.Source, policy *workqueue.Policy, snapshot, details, checkpoint []byte) bool {
	if policy.MappingVersion != worklistadapter.RepositoryMapping && policy.MappingVersion != workSelfDogfoodMapping {
		return false
	}
	expectedSnapshot, expectedDetails, expectedCheckpoint, err := worklistadapter.DocumentsFromSource(source, policy)
	if err != nil {
		return false
	}
	pairs := [][2][]byte{{snapshot, expectedSnapshot.Canonical()}, {details, expectedDetails.Canonical()}, {checkpoint, expectedCheckpoint.Canonical()}}
	for _, pair := range pairs {
		if pair[0] != nil && !bytes.Equal(pair[0], pair[1]) {
			return false
		}
	}
	return true
}

func workSourceCommandError(ctx context.Context, err error) string {
	if workSourceLimitError(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "INPUT_LIMIT"
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return "CANCELLED"
	}
	return "SOURCE_UNQUALIFIED"
}

func workSourceLimitError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, exec.ErrWaitDelay)
}

func workClosingCommandError(closing workSource) string {
	if workSourceLimitError(closing.qualificationError) {
		return "INPUT_LIMIT"
	}
	if errors.Is(closing.qualificationError, context.Canceled) {
		return "CANCELLED"
	}
	return ""
}

func validateWorkCapture(policy *workqueue.Policy, opening, closing workSource, snapshot *workqueue.Snapshot, details *workqueue.DetailsDocument, checkpoint *workqueue.CheckpointDocument, receipts []workqueue.AdapterReceipt, closure workqueue.CollisionClosure) *workqueue.Observation {
	unknowns := []string{workqueue.UnknownContainmentUnqualified, workqueue.UnknownMutationEnforcementUnqualified, workqueue.UnknownNetworkUnobserved}
	facts := workqueue.StateFacts{}
	// These validators produce only contradiction, positive partial, or inability;
	// mutation/adapter failure is measured separately below and never merged by state.
	for _, validation := range []workqueue.ValidationResult{workqueue.ValidateSnapshot(snapshot), workqueue.ValidateDetailRequests(snapshot, policy), workqueue.ValidateDetailCoverage(snapshot, details)} {
		facts.IdentityContradiction = facts.IdentityContradiction || validation.State == workqueue.StateConflicted
		facts.PositivePartial = facts.PositivePartial || validation.State == workqueue.StatePartial
		facts.Unable = facts.Unable || validation.State == workqueue.StateUnknown
		unknowns = append(unknowns, validation.Unknowns...)
	}
	bindingsConflict := snapshot.PolicyID != policy.ID || snapshot.RepositoryAuthorityID != policy.RepositoryAuthorityID || snapshot.QueueAuthorityID != policy.QueueAuthorityID || snapshot.AccessContextID != policy.AccessContextID || snapshot.Scope.ID != policy.ScopeID || checkpoint.PolicyID != policy.ID || checkpoint.SnapshotID != snapshot.ID
	facts.IdentityContradiction = facts.IdentityContradiction || bindingsConflict
	for _, source := range []workqueue.RepositorySource{opening.source, closing.source, checkpoint.RepositorySource} {
		refreshed := source
		workqueue.RefreshRepositorySource(&refreshed)
		facts.IdentityContradiction = facts.IdentityContradiction || source.ID != refreshed.ID
	}
	facts.Stale = snapshot.RepositorySource != opening.source || snapshot.Checkpoint != checkpoint.Checkpoint
	if !closing.manifest.qualificationFailed {
		facts.Stale = facts.Stale || checkpoint.RepositorySource != closing.source || opening.source != closing.source
	}
	if facts.Stale {
		unknowns = append(unknowns, workqueue.UnknownCheckpointChanged)
	}
	mutationState := "UNKNOWN"
	completeManifests := opening.manifest.complete && closing.manifest.complete
	if completeManifests {
		mutationState = "UNCHANGED_OBSERVED"
	}
	if !completeManifests || opening.manifest.qualificationFailed || closing.manifest.qualificationFailed {
		facts.Unable = true
		unknowns = append(unknowns, workqueue.UnknownSourceUnqualified)
	}
	manifestChanged := opening.manifest.changed || closing.manifest.changed || (opening.manifest.monitoredComplete && closing.manifest.monitoredComplete && opening.manifest.digest != closing.manifest.digest)
	if manifestChanged {
		mutationState = "CHANGED"
		facts.MutationOrAdapterFailure = true
		unknowns = append(unknowns, workqueue.UnknownMutationDetected)
	}
	if opening.source.StatusSHA256 != workqueue.SHA256Hex(nil) || closing.source.StatusSHA256 != workqueue.SHA256Hex(nil) {
		facts.Unable = true
		unknowns = append(unknowns, workqueue.UnknownRepositoryDirty)
	}
	operations := []string{"snapshot", "details", "verify"}
	adapterFailed := len(receipts) != len(operations)
	for i, receipt := range receipts {
		if i >= len(operations) || receipt.Operation != operations[i] || receipt.State != "PASSED" || receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.Signal != nil {
			adapterFailed = true
		}
		if receipt.ExecutableQualification == "UNQUALIFIED" {
			unknowns = append(unknowns, workqueue.UnknownExecutableIdentityUnqualified)
		}
	}
	if adapterFailed {
		facts.MutationOrAdapterFailure = true
		unknowns = append(unknowns, workqueue.UnknownAdapterInvalid)
	}
	if !closure.Complete {
		facts.Unable = true
	}
	unknowns = append(unknowns, closure.Unknowns...)
	containment := workContainmentClass()
	detailIDs := make([]string, 0, len(details.Details))
	for _, detail := range details.Details {
		detailIDs = append(detailIDs, detail.DetailID)
	}
	observation := &workqueue.Observation{AdapterReceipts: receipts, ContainmentClass: containment, DetailIDs: detailIDs,
		EndCheckpoint: checkpoint.Checkpoint, MutationState: mutationState, NetworkState: "HOST_UNOBSERVED", PolicyID: policy.ID,
		QueueSourceID: workqueue.QueueSourceIdentity(policy, snapshot), SnapshotID: snapshot.ID, StartCheckpoint: snapshot.Checkpoint, State: workqueue.ResolveState(facts), Unknowns: unknowns}
	workqueue.RefreshObservation(observation)
	return observation
}

func deriveWorkCollisions(ctx context.Context, root string, snapshot *workqueue.Snapshot, executable string, environment []string) workqueue.CollisionClosure {
	index, err := contextindex.BuildWithGitExecution(ctx, root, executable, environment)
	if err != nil || index.Revision != snapshot.RepositorySource.Tree {
		return workqueue.DeriveCollisions(snapshot, nil)
	}
	return workqueue.DeriveCollisions(snapshot, workqueue.IndexCollisionSource(index))
}

func emitWorkError(stdout io.Writer, code string) int {
	result := &workqueue.CommandResult{ErrorCode: &code, State: "ERROR"}
	workqueue.RefreshCommandResult(result)
	if writeWorkResult(stdout, result) != 0 {
		return 2
	}
	return 2
}

func writeWorkResult(stdout io.Writer, result *workqueue.CommandResult) int {
	raw := result.Canonical()
	written, err := stdout.Write(raw)
	if err != nil || written != len(raw) {
		return 2
	}
	return 0
}

func workCommandError(err error) string {
	switch workqueue.ErrorCode(err) {
	case workqueue.CodeHostileInput:
		return "HOSTILE_INPUT"
	case workqueue.CodeInputLimit:
		return "INPUT_LIMIT"
	case workqueue.CodeMalformedInput, workqueue.CodeConflicted:
		return "MALFORMED_INPUT"
	default:
		return "MALFORMED_INPUT"
	}
}

func workReadBoundedFile(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit)))
	if err != nil || len(raw) == limit {
		return nil, errors.New("input exceeds bound")
	}
	return raw, nil
}

func acquireWorkSource(ctx context.Context, root string) (workSource, error) {
	var result workSource
	source, err := worksource.Acquire(ctx, root)
	if err != nil {
		return result, err
	}
	result.qualified = source
	ok := false
	defer func() {
		if !ok {
			source.Close()
		}
	}()
	var policyEntry, adapterEntry *worksource.Entry
	for i := range source.Entries {
		if source.Entries[i].Path == workPolicyPath {
			policyEntry = &source.Entries[i]
		}
	}
	if policyEntry == nil || policyEntry.Mode != "100644" {
		return result, errors.New("invalid policy source")
	}
	policy, err := workqueue.ParsePolicy(policyEntry.Raw)
	if err != nil {
		return result, err
	}
	if policy.MappingVersion == worklistadapter.RepositoryMapping {
		worklistPath, _ := worklistadapter.WorklistPath(policy.MappingVersion)
		worklistFound := false
		for i := range source.Entries {
			if source.Entries[i].Path == worklistPath && source.Entries[i].Mode == "100644" {
				worklistFound = true
				break
			}
		}
		if !worklistFound {
			return result, errors.New("invalid worklist source")
		}
	}
	for i := range source.Entries {
		if source.Entries[i].Path == policy.AdapterPath {
			adapterEntry = &source.Entries[i]
		}
	}
	if adapterEntry == nil || adapterEntry.Mode != "100755" {
		return result, errors.New("invalid adapter source")
	}
	result.source = source.Identity
	result.policyRaw, result.adapterRaw = policyEntry.Raw, adapterEntry.Raw
	result.adapterOID, result.adapterMode = adapterEntry.BlobOID, adapterEntry.Mode
	ok = true
	return result, nil
}
