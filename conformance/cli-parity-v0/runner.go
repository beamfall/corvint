package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const parityPrompt = "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"

const (
	replayWorkspacePrefix   = "corvint-cli-parity-replay-"
	staleReplayWorkspaceAge = time.Hour
	staleReplaySweepLimit   = 64
)

// authorityLearnedPrompt shares two terms with the fixture commit subject, so
// the oracle's project-operations packet carries advisory learned paths.
const authorityLearnedPrompt = "Orient the roadmap workflow gates for the atlas parity fixture"

type executionEvidence struct {
	Process procgroup.Observation
	Before  repositorySnapshot
	After   repositorySnapshot
	Fixture materializedFixture
}

type replayOptions struct {
	ManifestPath string
	Oracle       string
	Candidate    string
	Only         string
	Output       io.Writer
	workspace    string
}

type captureOptions struct {
	ManifestPath string
	Oracle       string
	Only         string
	Write        bool
	Output       io.Writer
}

type replayJob func() (string, error)

type replayResult struct {
	output string
	err    error
}

func replay(ctx context.Context, options replayOptions) error {
	manifest, moduleRoot, err := readManifest(options.ManifestPath, true)
	if err != nil {
		return err
	}
	if options.Oracle != "" {
		return errors.New("live oracle overrides are retired under decision 0088")
	}
	workspace := options.workspace
	if workspace == "" {
		workspace, err = newReplayWorkspace()
		if err != nil {
			return err
		}
	}
	defer removeWritableTree(workspace)
	candidateArgv, err := resolveCandidate(ctx, workspace, moduleRoot, options.Candidate, manifest.CandidateCommand)
	if err != nil {
		return fmt.Errorf("candidate command: %w", err)
	}
	candidateArgv, err = stageCandidate(workspace, candidateArgv)
	if err != nil {
		return fmt.Errorf("stage candidate: %w", err)
	}
	candidateIdentity, err := readCandidateIdentity(ctx, candidateArgv[0])
	if err != nil {
		return fmt.Errorf("candidate identity: %w", err)
	}
	fixturesRoot := filepath.Join(filepath.Dir(options.ManifestPath), "fixtures")
	selectedCases := 0
	jobs := make([]replayJob, 0, len(manifest.Cases)+len(manifest.Refusals))
	for index, item := range manifest.Cases {
		if !selectedByPrefix(item.ID, options.Only) {
			continue
		}
		selectedCases++
		item := item
		if item.Retired != nil {
			// Not executed and not scored: the frozen expectation cannot be met
			// by a candidate that carries one name (decision 0308).
			jobs = append(jobs, func() (string, error) {
				return fmt.Sprintf("RETIRED %s decision=%s reason=%s\n", item.ID, item.Retired.Decision, item.Retired.Reason), nil
			})
			continue
		}
		caseRoot, err := replayCaseRoot(workspace, index)
		if err != nil {
			return fmt.Errorf("case workspace %s: %w", item.ID, err)
		}
		jobs = append(jobs, func() (string, error) {
			return replayParityCase(ctx, caseRoot, fixturesRoot, item, candidateArgv)
		})
	}
	selectedRefusals := 0
	for index, item := range manifest.Refusals {
		if !selectedByPrefix(item.ID, options.Only) {
			continue
		}
		selectedRefusals++
		item := item
		if item.Retired != nil {
			jobs = append(jobs, func() (string, error) {
				return fmt.Sprintf("RETIRED %s decision=%s reason=%s\n", item.ID, item.Retired.Decision, item.Retired.Reason), nil
			})
			continue
		}
		caseRoot, err := replayCaseRoot(workspace, len(manifest.Cases)+index)
		if err != nil {
			return fmt.Errorf("case workspace %s: %w", item.ID, err)
		}
		jobs = append(jobs, func() (string, error) {
			return replayRefusalCase(ctx, caseRoot, fixturesRoot, item, candidateArgv)
		})
	}
	results := runReplayJobs(manifestWorkerCount(manifest.Workers), jobs)
	var replayErr error
	if options.Only != "" && selectedCases == 0 && selectedRefusals == 0 {
		replayErr = fmt.Errorf("--only %q selected no case", options.Only)
	}
	for _, result := range results {
		if replayErr != nil {
			break
		}
		if result.err != nil {
			replayErr = result.err
			break
		}
		fmt.Fprint(options.Output, result.output)
	}
	if replayErr == nil && options.Only == "" {
		for _, command := range manifest.Commands {
			if command.Status == "UNSUPPORTED" || command.Status == "NOT_RUN" || command.Status == "PARTIAL" {
				fmt.Fprintf(options.Output, "%s command:%s reason=%s\n", command.Status, command.Command, command.Reason)
			}
		}
	}
	if replayErr != nil {
		return replayErr
	}
	fmt.Fprintln(options.Output, candidateIdentity)
	if options.Only != "" {
		// Deliberately NOT the SUMMARY line, and the command inventory is withheld:
		// a filtered run verifies a slice and must never be readable as corpus
		// evidence or as a statement about any command's certification.
		fmt.Fprintf(options.Output, "SUMMARY-FILTERED only=%s selected-cases=%d of %d selected-refusals=%d of %d NOT-CORPUS-EVIDENCE\n", options.Only, selectedCases, len(manifest.Cases), selectedRefusals, len(manifest.Refusals))
		return nil
	}
	// `parity` counts executed cases only; every other count ranges over them.
	// Retired cases are named separately and never contribute to any count.
	retired := retiredCount(manifest.Cases)
	retiredRefusals := retiredRefusalCount(manifest.Refusals)
	fmt.Fprintf(options.Output, "SUMMARY parity=%d retired=%d identity-renames=%d accepted-divergences=%d known-divergences=%d location-normalizations=%d structural-fields=%d unsupported-refusals=%d retired-refusals=%d full-gpk-v0-005=%s detached-descendants=%s\n", len(manifest.Cases)-retired, retired, identityRenameCount(manifest.Cases), acceptedDivergenceCount(manifest.Cases), knownDivergenceCount(manifest.Cases), locationNormalizationCount(manifest.Cases), structuralComparisonCount(manifest.Cases), len(manifest.Refusals)-retiredRefusals, retiredRefusals, manifest.Qualification.FullGPKV0005Matrix, manifest.Qualification.DetachedDescendantCleanup)
	return nil
}

func replayParityCase(ctx context.Context, caseRoot, fixturesRoot string, item parityCase, candidateArgv []string) (string, error) {
	candidate, err := executeCase(ctx, caseRoot, fixturesRoot, item.Repository, item.Setup, item.Argv, item.StdinBase64, item.TimeoutMilliseconds, candidateArgv, "")
	if err != nil {
		return "", fmt.Errorf("FAIL %s candidate: %w", item.ID, err)
	}
	if err := compareNativeManifestExpectation(item, candidate); err != nil {
		return "", err
	}
	var output strings.Builder
	if item.AcceptedDivergence == nil && item.LocationNormalization == nil && item.StructuralComparison == nil && item.KnownDivergence == nil && item.ExclusionCountDivergence == nil && item.IdentityRenameDivergence == nil {
		fmt.Fprintf(&output, "PASS %s\n", item.ID)
		return output.String(), nil
	}
	// A case may carry more than one declaration; name every one, so that no
	// qualification is hidden behind another.
	if item.StructuralComparison != nil {
		fmt.Fprintf(&output, "PASS-WITH-STRUCTURAL-FIELD %s fields=%s kind=%s reason=%s\n", item.ID, strings.Join(item.StructuralComparison.Fields, ","), item.StructuralComparison.Kind, item.StructuralComparison.Reason)
	}
	if item.AcceptedDivergence != nil {
		fmt.Fprintf(&output, "PASS-WITH-ACCEPTED-DIVERGENCE %s oracle-only-created-path=%s reason=%s\n", item.ID, item.AcceptedDivergence.OracleOnlyCreatedPath, item.AcceptedDivergence.Reason)
	}
	if item.LocationNormalization != nil {
		fmt.Fprintf(&output, "PASS-WITH-LOCATION-NORMALIZATION %s fields=%s kind=%s reason=%s\n", item.ID, strings.Join(item.LocationNormalization.Fields, ","), item.LocationNormalization.Kind, item.LocationNormalization.Reason)
	}
	if item.KnownDivergence != nil {
		fmt.Fprintf(&output, "PASS-WITH-KNOWN-DIVERGENCE %s register=%s clause=%s rewrites=%d reason=%s\n", item.ID, item.KnownDivergence.Register, item.KnownDivergence.Clause, len(item.KnownDivergence.Rewrites)+len(item.KnownDivergence.StderrRewrites), item.KnownDivergence.Reason)
	}
	if item.ExclusionCountDivergence != nil {
		fmt.Fprintf(&output, "PASS-WITH-KNOWN-DIVERGENCE %s register=%s clause=%s rewrites=%d reason=%s\n", item.ID, item.ExclusionCountDivergence.Register, item.ExclusionCountDivergence.Clause, len(item.ExclusionCountDivergence.Rewrites), item.ExclusionCountDivergence.Reason)
	}
	if item.IdentityRenameDivergence != nil {
		fmt.Fprintf(&output, "PASS-WITH-IDENTITY-RENAME %s register=%s clause=%s rewrites=%d reason=%s\n", item.ID, item.IdentityRenameDivergence.Register, item.IdentityRenameDivergence.Clause, len(item.IdentityRenameDivergence.Rewrites), item.IdentityRenameDivergence.Reason)
	}
	return output.String(), nil
}

func replayRefusalCase(ctx context.Context, caseRoot, fixturesRoot string, item refusalCase, candidateArgv []string) (string, error) {
	evidence, err := executeCase(ctx, caseRoot, fixturesRoot, item.Repository, item.Setup, item.Argv, "", item.TimeoutMilliseconds, candidateArgv, "")
	if err != nil {
		return "", fmt.Errorf("FAIL %s candidate-refusal: %w", item.ID, err)
	}
	code, err := validateGenericRefusal(evidence, item.ExpectedType)
	if err != nil {
		return "", fmt.Errorf("FAIL %s refusal: %w", item.ID, err)
	}
	return fmt.Sprintf("UNSUPPORTED %s code=%s\n", item.ID, code), nil
}

func runReplayJobs(workerCount int, jobs []replayJob) []replayResult {
	results := make([]replayResult, len(jobs))
	indexes := make(chan int)
	var workers sync.WaitGroup
	for range min(workerCount, len(jobs)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range indexes {
				results[index].output, results[index].err = jobs[index]()
			}
		}()
	}
	for index := range jobs {
		indexes <- index
	}
	close(indexes)
	workers.Wait()
	return results
}

func manifestWorkerCount(requested int) int {
	return min(requested, runtime.GOMAXPROCS(0))
}

func replayCaseRoot(workspace string, index int) (string, error) {
	root := filepath.Join(workspace, "jobs", fmt.Sprintf("%06d", index), "case")
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func stageCandidate(workspace string, argv []string) ([]string, error) {
	raw, err := os.ReadFile(argv[0])
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(workspace, "candidate")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "candidate"+filepath.Ext(argv[0]))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o500); err != nil {
		return nil, err
	}
	staged := append([]string(nil), argv...)
	staged[0] = path
	return staged, nil
}

func readCandidateIdentity(ctx context.Context, candidate string) (string, error) {
	raw, err := os.ReadFile(candidate)
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "go", "version", "-m", candidate)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	buildInfo, err := command.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSuffix(string(buildInfo), "\n"), "\n")
	prefix := candidate + ": "
	if len(lines) == 0 || !strings.HasPrefix(lines[0], prefix) {
		return "", errors.New("go version -m returned an unexpected executable identity")
	}
	lines[0] = strings.TrimPrefix(lines[0], prefix)
	encodedBuildInfo, err := json.Marshal(lines)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("CANDIDATE-IDENTITY sha256=%x go-version-m=%s", digest, encodedBuildInfo), nil
}

func capture(_ context.Context, _ captureOptions) error {
	return errors.New("capture retired under decision 0088; frozen expectations are immutable and cannot be regenerated from the candidate")
}

// selectedByPrefix filters cases to one id prefix so an author can iterate on a
// slice in seconds instead of re-running the whole corpus. An empty prefix
// selects everything, which is the only shape that produces corpus evidence.
func selectedByPrefix(id, prefix string) bool {
	return prefix == "" || strings.HasPrefix(id, prefix)
}

func executeCase(ctx context.Context, canonicalRoot, fixturesRoot, repository string, setups []fixtureSetup, argv []string, stdinBase64 string, timeoutMilliseconds int, command []string, oracleSourceRoot string) (executionEvidence, error) {
	var result executionEvidence
	if err := destroyFixture(canonicalRoot); err != nil {
		return result, err
	}
	fixture, err := materializeFixture(ctx, fixturesRoot, repository, canonicalRoot)
	if err != nil {
		return result, err
	}
	result.Fixture = fixture
	if err := applySetups(canonicalRoot, setups); err != nil {
		return result, err
	}
	runState := filepath.Join(filepath.Dir(canonicalRoot), "run-state")
	if err := destroyFixture(runState); err != nil {
		return result, err
	}
	for _, directory := range []string{"cache", "home", "tmp"} {
		if err := os.MkdirAll(filepath.Join(runState, directory), 0o700); err != nil {
			return result, err
		}
	}
	stdin, err := base64.StdEncoding.DecodeString(stdinBase64)
	if err != nil {
		return result, err
	}
	before, err := snapshotRepository(ctx, canonicalRoot)
	if err != nil {
		return result, err
	}
	processArgv := append([]string(nil), command...)
	processArgv = append(processArgv, "--root", canonicalRoot)
	processArgv = append(processArgv, argv...)
	environment := parityEnvironment(runState, oracleSourceRoot)
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: processArgv, Dir: canonicalRoot, Env: environment, Stdin: stdin,
		Timeout: time.Duration(timeoutMilliseconds) * time.Millisecond, ShutdownTimeout: 2 * time.Second,
		InputLimit: maximumStdin, OutputLimit: procgroup.DefaultOutputLimit,
	})
	after, snapshotErr := snapshotRepository(ctx, canonicalRoot)
	if snapshotErr != nil {
		return result, snapshotErr
	}
	result.Process, result.Before, result.After = observation, before, after
	if err := validateCache(filepath.Join(runState, "cache")); err != nil {
		return result, err
	}
	if observation.Err != nil {
		return result, observation.Err
	}
	return result, nil
}

func captureExpectation(item *parityCase, evidence executionEvidence) error {
	stdout, err := replayExpectationStdout(*item, evidence)
	if err != nil {
		return err
	}
	item.CommitRevision = evidence.Fixture.CommitRevision
	item.TreeRevision = evidence.Fixture.TreeRevision
	item.FixtureSHA256 = evidence.Fixture.FixtureSHA256
	item.ExitStatus = evidence.Process.ExitStatus
	item.StatusBeforeSHA256 = evidence.Before.StatusSHA256
	item.StatusAfterSHA256 = evidence.After.StatusSHA256
	item.RepositoryBeforeSHA256 = evidence.Before.RepositorySHA256
	item.RepositoryAfterSHA256 = evidence.After.RepositorySHA256
	item.FileModesBeforeSHA256 = evidence.Before.FileModesSHA256
	item.FileModesAfterSHA256 = evidence.After.FileModesSHA256
	item.StdoutSHA256 = sha256Hex(stdout)
	item.StdoutBytes = len(stdout)
	item.StderrSHA256 = sha256Hex(evidence.Process.Stderr)
	item.StderrBytes = len(evidence.Process.Stderr)
	stdin, _ := base64.StdEncoding.DecodeString(item.StdinBase64)
	item.StdinSHA256 = sha256Hex(stdin)
	item.TimedOut = evidence.Process.TimedOut
	item.ProcessStarted = evidence.Process.Started
	item.WaitCompleted = evidence.Process.WaitCompleted
	item.PipesDrained = evidence.Process.PipesDrained
	item.OwnedProcessGroupCleanup = evidence.Process.OwnedProcessGroupCleanup
	item.CleanupScope = evidence.Process.DescendantCleanupStatus
	return nil
}

// Only the comparison projection changes; the independent frozen receipt stays
// immutable. Its digest checks every byte after the accepted rewrites.
func compareNativeManifestExpectation(item parityCase, actual executionEvidence) error {
	if item.AcceptedDivergence != nil {
		if !validAcceptedLockDivergence(item) && !validAcceptedPrivateParentDivergence(item) {
			return errors.New("invalid accepted native divergence")
		}
		if err := compareSnapshotDigests(item.ID, "native read-only", actual, executionEvidence{Before: actual.Before, After: actual.Before}); err != nil {
			return err
		}
		item.AcceptedDivergence = nil
		item.StatusAfterSHA256 = item.StatusBeforeSHA256
		item.RepositoryAfterSHA256 = item.RepositoryBeforeSHA256
		item.FileModesAfterSHA256 = item.FileModesBeforeSHA256
	}
	if item.IdentityRenameDivergence != nil {
		// The profile member lies outside the counted packet, so the rename is
		// applied before, and independently of, the packet_bytes reconciliation.
		if !validIdentityRenameDivergence(item) {
			return errors.New("invalid identity rename divergence")
		}
		rewrite := item.IdentityRenameDivergence.Rewrites[0]
		if bytes.Count(actual.Process.Stdout, []byte(rewrite.Candidate)) != 1 {
			return fmt.Errorf("FAIL %s identity rename: declared profile must occur exactly once", item.ID)
		}
		actual.Process.Stdout = bytes.Replace(actual.Process.Stdout, []byte(rewrite.Candidate), []byte(rewrite.Oracle), 1)
	}
	if rewrites := declaredStdoutRewrites(item); len(rewrites) > 0 {
		member, count, err := packetBytesValue(actual.Process.Stdout)
		if err != nil {
			return err
		}
		delta := 0
		for _, rewrite := range rewrites {
			delta += len(rewrite.Candidate) - len(rewrite.Oracle)
		}
		projected := count - delta
		for range 8 {
			next := count - delta - len(strconv.Itoa(count)) + len(strconv.Itoa(projected))
			if next == projected {
				break
			}
			projected = next
		}
		expectedMember := []byte(packetBytesPrefix + strconv.Itoa(projected))
		projection := bytes.Replace(actual.Process.Stdout, member, expectedMember, 1)
		actual.Process.Stdout, err = applyKnownDivergence(item, actual.Process.Stdout, projection)
		if err != nil {
			return err
		}
	}
	if item.KnownDivergence != nil {
		if !validKnownDivergence(item) {
			return errors.New("invalid known native divergence")
		}
		for _, rewrite := range item.KnownDivergence.StderrRewrites {
			if bytes.Count(actual.Process.Stderr, []byte(rewrite.Candidate)) != 1 {
				return errors.New("declared stderr divergence must occur exactly once")
			}
			actual.Process.Stderr = bytes.Replace(actual.Process.Stderr, []byte(rewrite.Candidate), []byte(rewrite.Oracle), 1)
		}
	}
	return compareManifestExpectation(item, actual)
}

func compareManifestExpectation(item parityCase, actual executionEvidence) error {
	if err := validateOracleAcceptedDivergence(item, actual); err != nil {
		return fmt.Errorf("FAIL %s accepted divergence: %w", item.ID, err)
	}
	if actual.Process.TimedOut != item.TimedOut {
		return fmt.Errorf("FAIL %s timedOut: candidate=%t manifest=%t", item.ID, actual.Process.TimedOut, item.TimedOut)
	}
	stdout, err := replayExpectationStdout(item, actual)
	if err != nil {
		return fmt.Errorf("FAIL %s location normalization: %w", item.ID, err)
	}
	checks := []struct{ field, got, want string }{
		{"commitRevision", actual.Fixture.CommitRevision, item.CommitRevision},
		{"treeRevision", actual.Fixture.TreeRevision, item.TreeRevision},
		{"fixtureSha256", actual.Fixture.FixtureSHA256, item.FixtureSHA256},
		{"stdoutSha256", sha256Hex(stdout), item.StdoutSHA256},
		{"stderrSha256", sha256Hex(replayExpectationStderr(item, actual.Process.Stderr)), item.StderrSHA256},
		{"statusBeforeSha256", actual.Before.StatusSHA256, item.StatusBeforeSHA256},
		{"statusAfterSha256", actual.After.StatusSHA256, item.StatusAfterSHA256},
		{"repositoryBeforeSha256", actual.Before.RepositorySHA256, item.RepositoryBeforeSHA256},
		{"repositoryAfterSha256", actual.After.RepositorySHA256, item.RepositoryAfterSHA256},
		{"fileModesBeforeSha256", actual.Before.FileModesSHA256, item.FileModesBeforeSHA256},
		{"fileModesAfterSha256", actual.After.FileModesSHA256, item.FileModesAfterSHA256},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf("FAIL %s %s: candidate=%s manifest=%s", item.ID, check.field, check.got, check.want)
		}
	}
	if actual.Process.ExitStatus != item.ExitStatus {
		return fmt.Errorf("FAIL %s exitStatus: candidate=%d manifest=%d", item.ID, actual.Process.ExitStatus, item.ExitStatus)
	}
	if len(stdout) != item.StdoutBytes {
		return fmt.Errorf("FAIL %s stdoutBytes: candidate=%d manifest=%d", item.ID, len(stdout), item.StdoutBytes)
	}
	if len(actual.Process.Stderr) != item.StderrBytes {
		return fmt.Errorf("FAIL %s stderrBytes: candidate=%d manifest=%d", item.ID, len(actual.Process.Stderr), item.StderrBytes)
	}
	return compareProcessEvidence(item.ID, "manifest", actual.Process, procgroup.Observation{
		Started: item.ProcessStarted, TimedOut: item.TimedOut, WaitCompleted: item.WaitCompleted,
		PipesDrained: item.PipesDrained, OwnedProcessGroupCleanup: item.OwnedProcessGroupCleanup,
		DescendantCleanupStatus: item.CleanupScope,
	})
}

const repositoryRootPlaceholder = "<repository-root>"
const measuredValuePlaceholder = "<measured>"

// rawFieldValue walks a dotted path of object keys through canonical JSON and
// returns the exact encoded bytes of the value at that path.
func rawFieldValue(body []byte, path []string) (json.RawMessage, error) {
	current := json.RawMessage(body)
	for _, name := range path {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(current, &object); err != nil {
			return nil, fmt.Errorf("declared field parent %q is not an object", name)
		}
		value, exists := object[name]
		if !exists {
			return nil, fmt.Errorf("declared field %q is missing", name)
		}
		current = value
	}
	return current, nil
}

// declaredFieldPath splits a declaration such as "stdout.evaluation.metrics.latency_ms"
// into the object key path beneath stdout.
func declaredFieldPath(declaration string) ([]string, error) {
	parts := strings.Split(declaration, ".")
	if len(parts) < 2 || parts[0] != "stdout" {
		return nil, fmt.Errorf("declared field %q must name a stdout path", declaration)
	}
	for _, part := range parts[1:] {
		if part == "" {
			return nil, fmt.Errorf("declared field %q has an empty segment", declaration)
		}
	}
	return parts[1:], nil
}

// substituteUnique replaces the single occurrence of old with replacement,
// refusing when the encoded value is not unique in the document.
func substituteUnique(raw, body, old, replacement []byte, declaration string) ([]byte, error) {
	if bytes.Count(body, old) != 1 {
		return nil, fmt.Errorf("declared field %s value is not unique", declaration)
	}
	return bytes.Replace(raw, old, replacement, 1), nil
}

func stdoutJSONBody(raw []byte) ([]byte, error) {
	if !bytes.HasSuffix(raw, []byte{'\n'}) {
		return nil, errors.New("stdout is not one line with a terminal LF")
	}
	body := bytes.TrimSuffix(raw, []byte{'\n'})
	if err := validateJSONValue(body); err != nil {
		return nil, fmt.Errorf("stdout JSON: %w", err)
	}
	return body, nil
}

// applyStructuralComparison enforces decision 0005: a declared measured field
// MUST be present, numeric, and non-negative; its value is then replaced by a
// fixed placeholder so it is not compared. Unlike location normalization this
// also applies to the cross-runtime comparison, because a measured value never
// repeats across runs or runtimes.
func applyStructuralComparison(item parityCase, raw []byte) ([]byte, error) {
	if item.StructuralComparison == nil {
		return raw, nil
	}
	if !validStructuralComparison(item) {
		return nil, errors.New("structural declaration is invalid")
	}
	for _, declaration := range item.StructuralComparison.Fields {
		path, err := declaredFieldPath(declaration)
		if err != nil {
			return nil, err
		}
		body, err := stdoutJSONBody(raw)
		if err != nil {
			return nil, err
		}
		value, err := rawFieldValue(body, path)
		if err != nil {
			return nil, err
		}
		var measured float64
		if err := json.Unmarshal(value, &measured); err != nil {
			return nil, fmt.Errorf("declared field %s is not a number", declaration)
		}
		if measured < 0 {
			return nil, fmt.Errorf("declared field %s is negative", declaration)
		}
		replacement := []byte(`"` + measuredValuePlaceholder + `"`)
		raw, err = substituteUnique(raw, body, value, replacement, declaration)
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

// applyLocationNormalization enforces decision 0004: a declared location-dependent
// field has exactly its repository-root prefix replaced. Every other byte, and the
// path suffix, stay exact.
func applyLocationNormalization(item parityCase, raw []byte, root string) ([]byte, error) {
	if item.LocationNormalization == nil {
		return raw, nil
	}
	if !validLocationNormalization(item) {
		return nil, errors.New("declaration is invalid")
	}
	if root == "" {
		return nil, errors.New("repository root is unavailable")
	}
	for _, declaration := range item.LocationNormalization.Fields {
		path, err := declaredFieldPath(declaration)
		if err != nil {
			return nil, err
		}
		body, err := stdoutJSONBody(raw)
		if err != nil {
			return nil, err
		}
		value, err := rawFieldValue(body, path)
		if err != nil {
			return nil, err
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, fmt.Errorf("declared field %s is not a string", declaration)
		}
		if !strings.HasPrefix(text, root+string(filepath.Separator)) {
			return nil, fmt.Errorf("declared field %s does not start with the repository root", declaration)
		}
		prefixEnd, err := jsonStringPrefixEnd(value, root)
		if err != nil {
			return nil, err
		}
		normalized := make([]byte, 0, len(value)-prefixEnd+len(repositoryRootPlaceholder)+1)
		normalized = append(normalized, '"')
		normalized = append(normalized, repositoryRootPlaceholder...)
		normalized = append(normalized, value[prefixEnd:]...)
		raw, err = substituteUnique(raw, body, value, normalized, declaration)
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

// replayExpectationStderr applies a known divergence's stderr rewrites, each
// of which must occur exactly once; a declaration that does not fit the bytes
// leaves them unrewritten so the digest check reports the mismatch.
func replayExpectationStderr(item parityCase, stderr []byte) []byte {
	if item.KnownDivergence == nil || !validKnownDivergence(item) {
		return stderr
	}
	rewritten := stderr
	for _, rewrite := range item.KnownDivergence.StderrRewrites {
		from := []byte(rewrite.Candidate)
		if bytes.Count(rewritten, from) != 1 {
			return stderr
		}
		rewritten = bytes.Replace(rewritten, from, []byte(rewrite.Oracle), 1)
	}
	return rewritten
}

func replayExpectationStdout(item parityCase, evidence executionEvidence) ([]byte, error) {
	raw, err := applyStructuralComparison(item, evidence.Process.Stdout)
	if err != nil {
		return nil, err
	}
	return applyLocationNormalization(item, raw, evidence.Fixture.Root)
}

func jsonStringPrefixEnd(raw json.RawMessage, prefix string) (int, error) {
	for end := 1; end < len(raw); end++ {
		candidate := make([]byte, 0, end+1)
		candidate = append(candidate, raw[:end]...)
		candidate = append(candidate, '"')
		var decoded string
		if json.Unmarshal(candidate, &decoded) == nil && decoded == prefix {
			return end, nil
		}
	}
	return 0, errors.New("declared root prefix encoding is not exact")
}

func compareExecutions(item parityCase, subject string, actual, oracle executionEvidence) error {
	if err := compareExecutionPayload(item, subject, actual, oracle); err != nil {
		return err
	}
	return compareSnapshotDigests(item.ID, subject, actual, oracle)
}

// applyKnownDivergence rewrites the candidate's declared divergent bytes into the
// oracle's, so the rest of stdout is still compared byte-for-byte. It is applied
// to the candidate only: the oracle-repeat comparison stays untouched, and the
// frozen manifest expectation remains exactly the oracle's captured bytes.
//
// Each declared rewrite must match exactly once. A candidate that stops emitting
// the form the cited clause requires therefore fails here rather than passing
// quietly, which is what keeps the declaration from becoming a blanket exclusion.
// The receipt's own `packet_bytes` count is reconciled arithmetically rather than
// declared -- see reconcilePacketBytes.
func applyKnownDivergence(item parityCase, candidate, oracle []byte) ([]byte, error) {
	if item.KnownDivergence != nil && !validKnownDivergence(item) {
		return nil, errors.New("known divergence declaration is invalid")
	}
	if item.ExclusionCountDivergence != nil && !validExclusionCountDivergence(item) {
		return nil, errors.New("exclusion count divergence declaration is invalid")
	}
	rewrites := declaredStdoutRewrites(item)
	if len(rewrites) == 0 {
		// No declaration, or a stderr-only divergence (DR-0016): stdout is
		// untouched, and an empty refusal stdout carries no packet_bytes member.
		return candidate, nil
	}
	accounted := 0
	rewritten := candidate
	for _, rewrite := range rewrites {
		from, to := []byte(rewrite.Candidate), []byte(rewrite.Oracle)
		if count := bytes.Count(rewritten, from); count != 1 {
			return nil, fmt.Errorf("declared divergence %q occurs %d times in candidate stdout", rewrite.Candidate, count)
		}
		rewritten = bytes.Replace(rewritten, from, to, 1)
		accounted += len(from) - len(to)
	}
	return reconcilePacketBytes(rewritten, oracle, accounted)
}

// declaredStdoutRewrites lists every stdout rewrite a case declares, across its
// `knownDivergence` and its `DR-0023` exclusion-count declaration, so the
// packet_bytes identity accounts for all of them at once.
func declaredStdoutRewrites(item parityCase) []divergenceRewrite {
	var rewrites []divergenceRewrite
	if item.KnownDivergence != nil {
		rewrites = append(rewrites, item.KnownDivergence.Rewrites...)
	}
	if item.ExclusionCountDivergence != nil {
		rewrites = append(rewrites, item.ExclusionCountDivergence.Rewrites...)
	}
	return rewrites
}

// reconcilePacketBytes closes the one member no clause can author: the receipt's
// canonical byte count, which moves because the declared rewrites moved the
// packet it counts. Transcribing the candidate's number would take an expected
// byte from the candidate, so instead the two counts must differ by EXACTLY the
// length the declared rewrites account for, plus the width of the count's own
// encoded member. Any unaccounted byte anywhere in the receipt breaks the
// identity and fails the case.
func reconcilePacketBytes(candidate, oracle []byte, accounted int) ([]byte, error) {
	candidateMember, candidateCount, err := packetBytesValue(candidate)
	if err != nil {
		return nil, fmt.Errorf("candidate %w", err)
	}
	oracleMember, oracleCount, err := packetBytesValue(oracle)
	if err != nil {
		return nil, fmt.Errorf("oracle %w", err)
	}
	// The count includes its own encoded member, so a rewrite that moves the
	// count across a digit boundary also changes the packet's length by the width
	// of that member. That self-reference is a consequence of the declared
	// rewrites, not an unaccounted byte, so it belongs on the accounted side of
	// the identity. It is zero whenever the two counts have the same width, which
	// is why DR-0007 reconciles without it and DR-0008 (838 against 1720) does
	// not.
	accounted += len(candidateMember) - len(oracleMember)
	if candidateCount-oracleCount != accounted {
		return nil, fmt.Errorf("packet_bytes delta %d is not accounted for by the declared divergence (%d)", candidateCount-oracleCount, accounted)
	}
	return bytes.Replace(candidate, candidateMember, oracleMember, 1), nil
}

// packetBytesValue returns the single encoded `"packet_bytes":N` member and its
// value, refusing anything but exactly one occurrence.
func packetBytesValue(raw []byte) ([]byte, int, error) {
	index := bytes.Index(raw, []byte(packetBytesPrefix))
	if index < 0 || bytes.Count(raw, []byte(packetBytesPrefix)) != 1 {
		return nil, 0, errors.New("stdout does not carry exactly one packet_bytes member")
	}
	end := index + len(packetBytesPrefix)
	for end < len(raw) && raw[end] >= '0' && raw[end] <= '9' {
		end++
	}
	member := raw[index:end]
	value, err := strconv.Atoi(string(member[len(packetBytesPrefix):]))
	if err != nil {
		return nil, 0, errors.New("packet_bytes is not an integer")
	}
	return member, value, nil
}

func compareCandidateExecution(item parityCase, candidate, oracle executionEvidence) error {
	if item.KnownDivergence != nil || item.ExclusionCountDivergence != nil {
		rewritten, err := applyKnownDivergence(item, candidate.Process.Stdout, oracle.Process.Stdout)
		if err != nil {
			return fmt.Errorf("FAIL %s known divergence: %w", item.ID, err)
		}
		candidate.Process.Stdout = rewritten
	}
	if item.AcceptedDivergence == nil {
		return compareExecutions(item, "candidate", candidate, oracle)
	}
	if err := compareExecutionPayload(item, "candidate", candidate, oracle); err != nil {
		return err
	}
	if err := compareDirectionalSnapshots(item.ID, candidate, oracle); err != nil {
		return err
	}
	if err := validateOracleAcceptedDivergence(item, oracle); err != nil {
		return fmt.Errorf("FAIL %s accepted divergence: %w", item.ID, err)
	}
	return nil
}

func compareExecutionPayload(item parityCase, subject string, actual, oracle executionEvidence) error {
	id := item.ID
	if actual.Process.ExitStatus != oracle.Process.ExitStatus {
		return fmt.Errorf("FAIL %s exitStatus: %s=%d oracle=%d", id, subject, actual.Process.ExitStatus, oracle.Process.ExitStatus)
	}
	// Decision 0005: a declared measured field is compared structurally here too,
	// because its value never repeats across runs or runtimes. Location
	// normalization deliberately does NOT apply to cross-runtime comparison:
	// both runtimes observe the same absolute path, so it must stay byte-exact.
	actualStdout, err := applyStructuralComparison(item, actual.Process.Stdout)
	if err != nil {
		return fmt.Errorf("FAIL %s %s structural: %w", id, subject, err)
	}
	oracleStdout, err := applyStructuralComparison(item, oracle.Process.Stdout)
	if err != nil {
		return fmt.Errorf("FAIL %s oracle structural: %w", id, err)
	}
	if !bytes.Equal(actualStdout, oracleStdout) {
		return fmt.Errorf("FAIL %s stdout: %s=%s oracle=%s", id, subject, sha256Hex(actualStdout), sha256Hex(oracleStdout))
	}
	// A declared stderr divergence (DR-0016) is the candidate's; applied to the
	// oracle's own bytes it matches nothing and leaves them as they are.
	actualStderr := replayExpectationStderr(item, actual.Process.Stderr)
	if !bytes.Equal(actualStderr, oracle.Process.Stderr) {
		return fmt.Errorf("FAIL %s stderr: %s=%s oracle=%s", id, subject, sha256Hex(actualStderr), sha256Hex(oracle.Process.Stderr))
	}
	if actual.Fixture != oracle.Fixture {
		return fmt.Errorf("FAIL %s fixture identity: %s=%#v oracle=%#v", id, subject, actual.Fixture, oracle.Fixture)
	}
	return compareProcessEvidence(id, subject, actual.Process, oracle.Process)
}

func compareSnapshotDigests(id, subject string, actual, oracle executionEvidence) error {
	for _, check := range []struct{ field, got, want string }{
		{"statusBeforeSha256", actual.Before.StatusSHA256, oracle.Before.StatusSHA256},
		{"statusAfterSha256", actual.After.StatusSHA256, oracle.After.StatusSHA256},
		{"repositoryBeforeSha256", actual.Before.RepositorySHA256, oracle.Before.RepositorySHA256},
		{"repositoryAfterSha256", actual.After.RepositorySHA256, oracle.After.RepositorySHA256},
		{"fileModesBeforeSha256", actual.Before.FileModesSHA256, oracle.Before.FileModesSHA256},
		{"fileModesAfterSha256", actual.After.FileModesSHA256, oracle.After.FileModesSHA256},
	} {
		if check.got != check.want {
			return fmt.Errorf("FAIL %s %s: %s=%s oracle=%s", id, check.field, subject, check.got, check.want)
		}
	}
	return nil
}

func compareDirectionalSnapshots(id string, candidate, oracle executionEvidence) error {
	for _, check := range []struct{ field, got, want string }{
		{"statusBeforeSha256", candidate.Before.StatusSHA256, oracle.Before.StatusSHA256},
		{"statusAfterSha256", candidate.After.StatusSHA256, oracle.After.StatusSHA256},
		{"repositoryBeforeSha256", candidate.Before.RepositorySHA256, oracle.Before.RepositorySHA256},
		{"repositoryAfterSha256", candidate.After.RepositorySHA256, oracle.Before.RepositorySHA256},
		{"fileModesBeforeSha256", candidate.Before.FileModesSHA256, oracle.Before.FileModesSHA256},
		{"fileModesAfterSha256", candidate.After.FileModesSHA256, oracle.Before.FileModesSHA256},
	} {
		if check.got != check.want {
			return fmt.Errorf("FAIL %s %s: candidate=%s required-no-mutation=%s", id, check.field, check.got, check.want)
		}
	}
	return nil
}

// oracleOnlyCreation says where a declared oracle-only path lands in the snapshot
// and what shape it must have there. snapshotRepository files the Git directory
// under "git" and everything else under "worktree", so the prefix is derived from
// the declared path rather than assumed.
type oracleOnlyCreation struct {
	snapshotPath string
	entryType    string
	mode         uint32
	description  string
}

func declaredOracleOnlyCreation(item parityCase) (oracleOnlyCreation, bool) {
	declared := item.AcceptedDivergence.OracleOnlyCreatedPath
	switch {
	case validAcceptedLockDivergence(item):
		return oracleOnlyCreation{"worktree/" + declared, "file", 0o600, "an empty regular 0600 file"}, true
	case validAcceptedPrivateParentDivergence(item):
		return oracleOnlyCreation{"git/" + strings.TrimPrefix(declared, ".git/"), "directory", 0o700, "an empty 0700 directory"}, true
	}
	return oracleOnlyCreation{}, false
}

func validateOracleAcceptedDivergence(item parityCase, evidence executionEvidence) error {
	if item.AcceptedDivergence == nil {
		return nil
	}
	declared, ok := declaredOracleOnlyCreation(item)
	if !ok {
		return errors.New("declaration is invalid")
	}
	if evidence.Before.StatusSHA256 != evidence.After.StatusSHA256 {
		return errors.New("oracle changed Git status")
	}
	remainder := make([]snapshotEntry, 0, len(evidence.After.Entries))
	created := 0
	for _, entry := range evidence.After.Entries {
		if entry.Path != declared.snapshotPath {
			remainder = append(remainder, entry)
			continue
		}
		created++
		if entry.Type != declared.entryType || entry.Mode != declared.mode || len(entry.Content) != 0 || entry.LinkTarget != "" {
			return fmt.Errorf("oracle-only path is not %s", declared.description)
		}
	}
	if created != 1 || !reflect.DeepEqual(evidence.Before.Entries, remainder) {
		return errors.New("oracle mutation is not exactly the declared creation")
	}
	return nil
}

func retiredCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.Retired != nil {
			count++
		}
	}
	return count
}

func retiredRefusalCount(refusals []refusalCase) int {
	count := 0
	for _, item := range refusals {
		if item.Retired != nil {
			count++
		}
	}
	return count
}

func identityRenameCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.IdentityRenameDivergence != nil {
			count++
		}
	}
	return count
}

func knownDivergenceCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.Retired != nil {
			continue
		}
		if item.KnownDivergence != nil {
			count++
		}
		if item.ExclusionCountDivergence != nil {
			count++
		}
	}
	return count
}

func acceptedDivergenceCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.Retired != nil {
			continue
		}
		if item.AcceptedDivergence != nil {
			count++
		}
	}
	return count
}

func structuralComparisonCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.Retired != nil {
			continue
		}
		if item.StructuralComparison != nil {
			count++
		}
	}
	return count
}

func locationNormalizationCount(cases []parityCase) int {
	count := 0
	for _, item := range cases {
		if item.Retired != nil {
			continue
		}
		if item.LocationNormalization != nil {
			count++
		}
	}
	return count
}

func compareProcessEvidence(id, subject string, actual, oracle procgroup.Observation) error {
	checks := []struct {
		field     string
		got, want bool
	}{
		{"processStarted", actual.Started, oracle.Started}, {"timedOut", actual.TimedOut, oracle.TimedOut},
		{"waitCompleted", actual.WaitCompleted, oracle.WaitCompleted}, {"pipesDrained", actual.PipesDrained, oracle.PipesDrained},
		{"ownedProcessGroupCleanup", actual.OwnedProcessGroupCleanup, oracle.OwnedProcessGroupCleanup},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf("FAIL %s %s: %s=%t oracle=%t", id, check.field, subject, check.got, check.want)
		}
	}
	if actual.DescendantCleanupStatus != oracle.DescendantCleanupStatus {
		return fmt.Errorf("FAIL %s cleanupScope: %s=%s oracle=%s", id, subject, actual.DescendantCleanupStatus, oracle.DescendantCleanupStatus)
	}
	return nil
}

func validateGenericRefusal(evidence executionEvidence, expectedType string) (string, error) {
	process := evidence.Process
	if process.ExitStatus == 0 || len(process.Stdout) != 0 || !process.Started || process.TimedOut || !process.WaitCompleted || !process.PipesDrained || !process.OwnedProcessGroupCleanup {
		return "", fmt.Errorf("not a bounded nonzero typed refusal")
	}
	if !bytes.HasSuffix(process.Stderr, []byte{'\n'}) || bytes.Count(process.Stderr, []byte{'\n'}) != 1 {
		return "", fmt.Errorf("stderr is not exactly one JSON line")
	}
	var envelope struct {
		Code  string `json:"code"`
		Error string `json:"error"`
		OK    bool   `json:"ok"`
	}
	if err := decodeStrictJSON(bytes.TrimSuffix(process.Stderr, []byte{'\n'}), &envelope); err != nil {
		return "", err
	}
	if envelope.OK || envelope.Code != expectedType || envelope.Error == "" {
		return "", fmt.Errorf("stderr is not a generic unsupported refusal")
	}
	if refusalMutatedRepository(evidence.Before, evidence.After) {
		return "", fmt.Errorf("refusal mutated repository")
	}
	return envelope.Code, nil
}

// selfObservationLedgerEntry is the one path an unsupported-* refusal may write:
// the gitignored SOL-V0-007 ledger that GPK-V0-008 excepts from its byte-identical
// rule (AGENTS.md invariant 4). An unignored ledger still changes Git status.
const selfObservationLedgerEntry = "worktree/.corvint/self-observations.jsonl"

// refusalMutatedRepository reports any change besides the ledger becoming or
// staying an owner-only regular file. Git status must be unchanged, so a ledger
// the fixture does not ignore, and every other entry, still count as mutation.
func refusalMutatedRepository(before, after repositorySnapshot) bool {
	if before.StatusSHA256 != after.StatusSHA256 {
		return true
	}
	if before.RepositorySHA256 == after.RepositorySHA256 && before.FileModesSHA256 == after.FileModesSHA256 {
		return false
	}
	beforeRemainder, _ := withoutSelfObservationLedger(before.Entries)
	afterRemainder, ledger := withoutSelfObservationLedger(after.Entries)
	if ledger == nil || ledger.Type != "file" || ledger.Mode != 0o600 {
		return true
	}
	return !reflect.DeepEqual(beforeRemainder, afterRemainder)
}

func withoutSelfObservationLedger(entries []snapshotEntry) ([]snapshotEntry, *snapshotEntry) {
	remainder := make([]snapshotEntry, 0, len(entries))
	var ledger *snapshotEntry
	for index := range entries {
		if entries[index].Path == selfObservationLedgerEntry {
			ledger = &entries[index]
			continue
		}
		remainder = append(remainder, entries[index])
	}
	return remainder, ledger
}

func applySetups(root string, setups []fixtureSetup) error {
	for _, setup := range setups {
		if err := validateRelativePath(setup.Path); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(setup.Path))
		switch setup.Kind {
		case "mkdir":
			if err := os.MkdirAll(path, 0o700); err != nil {
				return err
			}
		case "write":
			data, err := base64.StdEncoding.DecodeString(setup.DataBase64)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, os.FileMode(setup.Mode)); err != nil {
				return err
			}
		case "remove":
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func readManifest(path string, validate bool) (parityManifest, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return parityManifest{}, "", err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return parityManifest{}, "", err
	}
	var manifest parityManifest
	if err := decodeStrictJSON(raw, &manifest); err != nil {
		return manifest, "", err
	}
	if validate {
		if err := validateManifest(manifest); err != nil {
			return manifest, "", err
		}
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(absolute), "..", ".."))
	return manifest, moduleRoot, nil
}

func resolveCommand(command string) ([]string, error) {
	parts, err := splitCommand(command)
	if err != nil {
		return nil, err
	}
	executable, err := exec.LookPath(parts[0])
	if err != nil {
		return nil, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	parts[0] = filepath.Clean(executable)
	return parts, nil
}

func splitCommand(command string) ([]string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 || strings.Join(parts, " ") != command {
		return nil, errors.New("command must be a space-separated argv without quoting")
	}
	return parts, nil
}

// resolveCandidate returns the argv replay will exercise as the candidate.
// An explicit override is trusted and resolved like any other command,
// PATH included. With no override, replay never trusts a PATH-resolved
// installation of the manifest's bare command name -- a stale install can
// silently outlive the tree under test and score as the candidate (seen
// during AT-02 slice 2) -- and instead builds the command fresh from
// moduleRoot's cmd/<name> package into the replay workspace.
func resolveCandidate(ctx context.Context, workspace, moduleRoot, override, fallback string) ([]string, error) {
	if override != "" {
		return resolveCommand(override)
	}
	parts, err := splitCommand(fallback)
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(workspace, "scratch-build")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	binary := filepath.Join(directory, parts[0])
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/"+parts[0])
	build.Dir = moduleRoot
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(workspace, "scratch-gocache"))
	if output, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("go build ./cmd/%s: %w\n%s", parts[0], err, output)
	}
	parts[0] = binary
	return parts, nil
}

func parityEnvironment(runState, oracleSourceRoot string) []string {
	allowed := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT"}
	environment := make([]string, 0, len(allowed)+16)
	for _, key := range allowed {
		if value, exists := os.LookupEnv(key); exists {
			environment = append(environment, key+"="+value)
		}
	}
	environment = append(environment,
		"HOME="+filepath.Join(runState, "home"), "TMPDIR="+filepath.Join(runState, "tmp"),
		"TMP="+filepath.Join(runState, "tmp"), "TEMP="+filepath.Join(runState, "tmp"),
		"CORVINT_CACHE_DIR="+filepath.Join(runState, "cache"), "LANG=C", "LC_ALL=C", "TZ=UTC",
		"PYTHONDONTWRITEBYTECODE=1", "PYTHONHASHSEED=0", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_ATTR_NOSYSTEM=1", "GIT_NO_REPLACE_OBJECTS=1",
		"GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0",
	)
	if oracleSourceRoot != "" {
		environment = append(environment, "PYTHONPATH="+filepath.Join(oracleSourceRoot, "src"))
	}
	return environment
}

func validateCache(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cache contains symlink %q", path)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("cache path %q is not owner-private: %o", path, info.Mode().Perm())
		}
		return nil
	})
}

func destroyFixture(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("prior fixture was not removed: %s", path)
	}
	return nil
}

func newPrivateTemp(pattern string) (string, error) {
	return os.MkdirTemp(privateTempBase(), pattern)
}

func privateTempBase() string {
	if runtime.GOOS == "darwin" {
		return "/private/tmp"
	}
	return os.TempDir()
}

func newReplayWorkspace() (string, error) {
	base := privateTempBase()
	sweepStaleReplayWorkspaces(base, time.Now().Add(-staleReplayWorkspaceAge), staleReplaySweepLimit)
	return os.MkdirTemp(base, replayWorkspacePrefix)
}

func sweepStaleReplayWorkspaces(base string, cutoff time.Time, limit int) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	attempted := 0
	for _, entry := range entries {
		if attempted >= limit {
			return
		}
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), replayWorkspacePrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		attempted++
		removeWritableTree(filepath.Join(base, entry.Name()))
	}
}

func removeWritableTree(root string) {
	_ = makeTreeWritable(root)
	_ = os.RemoveAll(root)
}

func writeFileAtomically(path string, raw []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
