// Package tracerecordrepo binds trace recording to stable Git repository authority.
package tracerecordrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

const (
	maximumAncestry       = 10_000
	maximumAncestryBytes  = 2 << 20
	maximumTreeBytes      = 64 << 20
	maximumHistoricalPath = 200_000
)

var replayAncestryLimit = maximumAncestry // unexported regression-test hook

type revisionBindingMode uint8

const (
	strictRevisionBinding revisionBindingMode = iota
	recordRevisionBinding
)

// Input is the public record command payload after argument parsing.
type Input struct {
	Task         string
	OpenedPaths  []string
	ChangedPaths []string
	Verification []string
	Outcome      string
}

// Result is the canonical trace, its resolved store path, and the number of
// ancestry rows the bounded replay probe truncated while binding revisions.
// A non-zero count discloses that candidate revisions beyond the window were
// not examined; it is derived disclosure, never an input to admission.
type Result struct {
	Record            trace.Record
	Store             string
	TruncatedAncestry int
}

// DogfoodResult binds the complete Git change set to the exact subset supplied
// to the shared record producer.
type DogfoodResult struct {
	Base, Target, Tree              string
	CandidateSHA256, AdmittedSHA256 string
	Candidates, Admitted            []string
	State                           string
	Recorded                        *Result
}

// Record validates against frozen repository authority and atomically records one trace.
func Record(ctx context.Context, root string, input Input) (Result, error) {
	index, tracked, err := recordIndex(ctx, root)
	if err != nil {
		return Result{}, err
	}
	return recordWithIndex(ctx, root, index, tracked, stabilityCheck(ctx, root, index, nil), input)
}

// RecordDogfood classifies and records one base-to-target change under a single
// pinned authority. It returns no-source-paths without opening the trace store.
func RecordDogfood(ctx context.Context, root, baseArg, targetArg string, input Input) (DogfoodResult, error) {
	index, tracked, err := recordIndex(ctx, root)
	if err != nil {
		return DogfoodResult{}, dogfoodFailure("record-index-failed", err)
	}
	base, err := resolveCommit(ctx, root, index.ObjectFormat, baseArg)
	if err != nil {
		return DogfoodResult{}, dogfoodFailure("invalid-base-revision", err)
	}
	target, err := resolveCommit(ctx, root, index.ObjectFormat, targetArg)
	if err != nil {
		return DogfoodResult{}, dogfoodFailure("invalid-target-revision", err)
	}
	if target != index.CommitRevision {
		return DogfoodResult{}, dogfoodFailure("repository-identity-changed", &repositoryChangedError{})
	}
	candidates, err := changedPaths(ctx, root, base, target)
	if err != nil {
		return DogfoodResult{}, dogfoodFailure("changed-path-acquisition-failed", err)
	}
	admitted, err := trace.AdmissibleCurrentPaths(candidates, tracked)
	if err != nil {
		reason := trace.AdmissionFailureReason(err)
		if reason == "" {
			reason = "changed-path-admission-failed"
		}
		return DogfoodResult{}, dogfoodFailure(reason, err)
	}
	authority := &dogfoodAuthority{
		base: base, target: target, candidates: digestPaths(candidates), admitted: digestPaths(admitted),
	}
	result := DogfoodResult{
		Base: base, Target: target, Tree: index.Revision,
		CandidateSHA256: authority.candidates, AdmittedSHA256: authority.admitted,
		Candidates: candidates, Admitted: admitted,
	}
	stable := stabilityCheck(ctx, root, index, authority)
	if len(admitted) == 0 {
		if err := stable(); err != nil {
			return DogfoodResult{}, dogfoodRecordFailure(err)
		}
		result.State = "no-source-paths"
		return result, nil
	}
	input.ChangedPaths = admitted
	recorded, err := recordWithIndex(ctx, root, index, tracked, stable, input)
	if err != nil {
		return DogfoodResult{}, dogfoodRecordFailure(err)
	}
	result.State = "recorded"
	result.Recorded = &recorded
	return result, nil
}

type dogfoodError struct {
	reason string
	cause  error
}

func (err *dogfoodError) Error() string { return err.cause.Error() }
func (err *dogfoodError) Unwrap() error { return err.cause }

func dogfoodFailure(reason string, cause error) error {
	return &dogfoodError{reason: reason, cause: cause}
}

func dogfoodRecordFailure(err error) error {
	var changed *repositoryChangedError
	if errors.As(err, &changed) || trace.IsDrift(err) {
		return dogfoodFailure("repository-identity-changed", err)
	}
	if reason := trace.VerificationFailureReason(err); reason != "" {
		return dogfoodFailure(reason, err)
	}
	return dogfoodFailure("record-failed", err)
}

// DogfoodFailureReason returns the bounded coordination reason retained by the
// hidden dogfood producer. It is empty for errors outside that transaction.
func DogfoodFailureReason(err error) string {
	var failure *dogfoodError
	if errors.As(err, &failure) {
		return failure.reason
	}
	return ""
}

func recordIndex(ctx context.Context, root string) (*contextindex.Index, []string, error) {
	index, err := contextindex.Build(ctx, root)
	if err != nil {
		return nil, nil, recordIndexError(root, err)
	}
	if len(index.DirtyPaths) != 0 {
		return nil, nil, fmt.Errorf("trace recording requires a clean Git tree")
	}
	tracked := recordPaths(index)
	return index, tracked, nil
}

func recordPaths(index *contextindex.Index) []string {
	paths := sortedSourcePaths(index.Sources)
	for value := range index.Tracked {
		if value == ".gitignore" || strings.HasSuffix(value, "/.gitignore") {
			paths = append(paths, value)
		}
	}
	sort.Strings(paths)
	return paths
}

func recordWithIndex(ctx context.Context, root string, index *contextindex.Index, tracked []string, stable func() error, input Input) (Result, error) {
	validated, err := trace.NewRecord(trace.Input{
		Revision: index.CommitRevision, TreeRevision: index.Revision,
		Task: "record validation", Verification: input.Verification, Outcome: "passed",
	}, tracked)
	if err != nil {
		return Result{}, err
	}
	if !validOutcome(input.Outcome) {
		return Result{}, fmt.Errorf("trace outcome must be one of: blocked, failed, passed")
	}
	var record trace.Record
	recordValidated := false
	validateRecord := func() error {
		built, err := trace.NewRecord(trace.Input{
			Revision: index.CommitRevision, TreeRevision: index.Revision,
			Task: input.Task, OpenedPaths: input.OpenedPaths, ChangedPaths: input.ChangedPaths,
			Verification: validated.Verification, Outcome: input.Outcome,
		}, tracked)
		if err != nil {
			return err
		}
		if _, err := trace.Encode(built); err != nil {
			return err
		}
		record = built
		recordValidated = true
		return nil
	}
	if err := trace.RecoverInterruptedAppend(root, stable, validateRecord); err != nil {
		return Result{}, err
	}
	revisions, truncatedAncestry, err := bindRevisions(ctx, root, index, tracked, recordRevisionBinding)
	if err != nil {
		return Result{}, err
	}
	if !recordValidated {
		if err := validateRecord(); err != nil {
			return Result{}, err
		}
	}
	store, err := trace.NewStore(root, revisions, stable)
	if err != nil {
		return Result{}, err
	}
	if _, err := store.Append(record); err != nil {
		return Result{}, err
	}
	return Result{
		Record: record, Store: trace.StorePath(root, index.CommitRevision),
		TruncatedAncestry: truncatedAncestry,
	}, nil
}

func bindRevisions(ctx context.Context, root string, index *contextindex.Index, currentPaths []string, mode revisionBindingMode) (map[string]trace.Revision, int, error) {
	var candidates []string
	var err error
	if mode == recordRevisionBinding {
		candidates, err = trace.CandidateRevisionsForAppend(root)
	} else {
		candidates, err = trace.CandidateRevisions(root)
	}
	if err != nil {
		kind := ReadFailureTraceState
		if trace.IsDrift(err) {
			kind = ReadFailureDrift
		}
		return nil, 0, readFailure(kind, err)
	}
	revisions := map[string]trace.Revision{
		index.CommitRevision: {TreeRevision: index.Revision, TrackedPaths: currentPaths},
	}
	// The ancestry read runs even for an empty store so a failing Git
	// surfaces through the same path and message as the Python oracle.
	budget := gitrun.NewDefaultBudget()
	raw, err := runGit(ctx, budget, root, maximumAncestryBytes,
		"log", "--topo-order", fmt.Sprintf("--max-count=%d", replayAncestryLimit+1), "--format=%H %T %P", "HEAD")
	if err != nil {
		return nil, 0, readFailure(ReadFailureHistory, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	truncatedAncestry := 0
	if len(lines) > replayAncestryLimit {
		truncatedAncestry = len(lines) - replayAncestryLimit
		lines = lines[:replayAncestryLimit]
	}
	length := objectIDLength(index.ObjectFormat)
	ancestry := make(map[string]ancestryRevision, len(lines))
	for _, line := range lines {
		fields := strings.Split(strings.TrimSuffix(line, " "), " ")
		if len(fields) < 2 || !lowerHex(fields[0], length) || !lowerHex(fields[1], length) {
			return nil, 0, readFailure(ReadFailureHistory, fmt.Errorf("Git ancestry contains malformed commit/tree identity"))
		}
		for _, parent := range fields[2:] {
			if !lowerHex(parent, length) {
				return nil, 0, readFailure(ReadFailureHistory, fmt.Errorf("Git ancestry contains malformed commit/tree identity"))
			}
		}
		ancestry[fields[0]] = ancestryRevision{tree: fields[1], parents: fields[2:]}
	}
	distances, err := shortestAncestryDistances(index.CommitRevision, ancestry)
	if err != nil {
		return nil, 0, readFailure(ReadFailureHistory, err)
	}
	commits := make(map[string]string, len(distances))
	treeDistances := make(map[string]int, len(distances))
	for commit, distance := range distances {
		tree := ancestry[commit].tree
		commits[commit] = tree
		current, exists := treeDistances[tree]
		if !exists || distance < current {
			treeDistances[tree] = distance
		}
	}
	trackedCache := map[string][]string{index.Revision: currentPaths}
	for _, revision := range candidates {
		treeRevision, canonical := commits[revision]
		distance := distances[revision]
		if !canonical {
			legacyDistance, legacy := treeDistances[revision]
			if !legacy {
				if mode == recordRevisionBinding {
					continue
				}
				if truncatedAncestry != 0 {
					predates, err := candidatePredatesReplayWindow(ctx, budget, root, index.CommitRevision, revision)
					if err != nil {
						return nil, 0, readFailure(ReadFailureHistory, err)
					}
					if predates {
						return nil, 0, readFailureWithTruncatedAncestry(ReadFailureReplayWindow,
							fmt.Errorf("candidate revision predates the bounded replay window"), truncatedAncestry)
					}
				}
				return nil, 0, readFailure(ReadFailureTraceState, fmt.Errorf("local trace store contains unreachable revision: %s", revision))
			}
			treeRevision = revision
			distance = legacyDistance
		}
		paths, ok := trackedCache[treeRevision]
		if !ok {
			paths, err = trackedPaths(ctx, budget, root, treeRevision)
			if err != nil {
				return nil, 0, readFailure(ReadFailureHistory, err)
			}
			trackedCache[treeRevision] = paths
		}
		revisions[revision] = trace.Revision{
			TreeRevision: treeRevision, Legacy: !canonical, TrackedPaths: paths, AncestryDistance: distance,
		}
	}
	return revisions, truncatedAncestry, nil
}

type ancestryRevision struct {
	tree    string
	parents []string
}

func shortestAncestryDistances(head string, ancestry map[string]ancestryRevision) (map[string]int, error) {
	if _, ok := ancestry[head]; !ok {
		return nil, fmt.Errorf("Git ancestry does not contain captured HEAD")
	}
	distances := map[string]int{head: 0}
	queue := []string{head}
	for len(queue) != 0 {
		commit := queue[0]
		queue = queue[1:]
		for _, parent := range ancestry[commit].parents {
			if _, admitted := ancestry[parent]; !admitted {
				continue
			}
			distance := distances[commit] + 1
			current, visited := distances[parent]
			if visited && current <= distance {
				continue
			}
			distances[parent] = distance
			queue = append(queue, parent)
		}
	}
	return distances, nil
}

func candidatePredatesReplayWindow(ctx context.Context, budget *gitrun.Budget, root, head, candidate string) (bool, error) {
	raw, err := runGit(ctx, budget, root, len(candidate)+1, "rev-parse", "--verify", candidate+"^{commit}")
	if err != nil {
		return false, err
	}
	if strings.TrimSuffix(string(raw), "\n") != candidate {
		return false, fmt.Errorf("Git candidate revision probe returned an unexpected identity")
	}
	ancestor, err := probeGitAncestor(ctx, budget, root, candidate, head)
	if err != nil {
		return false, err
	}
	return ancestor, nil
}

func trackedPaths(ctx context.Context, budget *gitrun.Budget, root, tree string) ([]string, error) {
	raw, err := runGit(ctx, budget, root, maximumTreeBytes+1, "ls-tree", "-r", "--name-only", "-z", tree)
	if err != nil {
		return nil, err
	}
	if len(raw) > maximumTreeBytes {
		return nil, fmt.Errorf("historical trace tree exceeds %d bytes", maximumTreeBytes)
	}
	if bytes.Count(raw, []byte{0}) > maximumHistoricalPath {
		return nil, fmt.Errorf("historical trace tree exceeds %d paths", maximumHistoricalPath)
	}
	paths := make([]string, 0, bytes.Count(raw, []byte{0}))
	for _, path := range bytes.Split(raw, []byte{0}) {
		if len(path) != 0 {
			paths = append(paths, string(path))
		}
	}
	return paths, nil
}

type repositoryChangedError struct{}

func (*repositoryChangedError) Error() string { return "repository identity changed" }

type dogfoodAuthority struct {
	base, target, candidates, admitted string
}

// observedStabilityCheck is stabilityCheck's header comparison against one
// observation already taken, for the shared query bracket: the same fields,
// the same error, no spawn.
func observedStabilityCheck(root string, expected *contextindex.Index, current contextindex.Observation) func() error {
	dirty := append([]string(nil), expected.DirtyPaths...)
	return func() error {
		if current.CommitRevision != expected.CommitRevision || current.Revision != expected.Revision ||
			current.ObjectFormat != expected.ObjectFormat ||
			current.StatusSHA256 != expected.StatusSHA256 || !equalStrings(current.DirtyPaths, dirty) {
			return &repositoryChangedError{}
		}
		return nil
	}
}

func stabilityCheck(ctx context.Context, root string, expected *contextindex.Index, authority *dogfoodAuthority) func() error {
	dirty := append([]string(nil), expected.DirtyPaths...)
	// Without an authority the closing check reads only header fields, so it
	// observes them instead of compiling a second index whose sources it would
	// discard. ProfileID is not compared because it is a pure function of the
	// tree: an equal Revision already implies an equal ProfileID.
	if authority == nil {
		return func() error {
			current, err := contextindex.Observe(ctx, root)
			if err != nil {
				return recordIndexError(root, err)
			}
			if current.CommitRevision != expected.CommitRevision || current.Revision != expected.Revision ||
				current.ObjectFormat != expected.ObjectFormat ||
				current.StatusSHA256 != expected.StatusSHA256 || !equalStrings(current.DirtyPaths, dirty) {
				return &repositoryChangedError{}
			}
			return nil
		}
	}
	return func() error {
		current, err := contextindex.Build(ctx, root)
		if err != nil {
			return recordIndexError(root, err)
		}
		if current.CommitRevision != expected.CommitRevision || current.Revision != expected.Revision ||
			current.ObjectFormat != expected.ObjectFormat || current.ProfileID != expected.ProfileID ||
			current.StatusSHA256 != expected.StatusSHA256 || !equalStrings(current.DirtyPaths, dirty) {
			return &repositoryChangedError{}
		}
		if current.CommitRevision != authority.target {
			return &repositoryChangedError{}
		}
		candidates, err := changedPaths(ctx, root, authority.base, authority.target)
		if err != nil {
			return err
		}
		tracked := recordPaths(current)
		admitted, err := trace.AdmissibleCurrentPaths(candidates, tracked)
		if err != nil {
			return err
		}
		if digestPaths(candidates) != authority.candidates || digestPaths(admitted) != authority.admitted {
			return &repositoryChangedError{}
		}
		return nil
	}
}

func resolveCommit(ctx context.Context, root, objectFormat, revision string) (string, error) {
	raw, err := runGit(ctx, gitrun.NewDefaultBudget(), root, 256, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSuffix(string(raw), "\n")
	if strings.Contains(resolved, "\n") || !lowerHex(resolved, objectIDLength(objectFormat)) {
		return "", fmt.Errorf("Git returned an invalid commit identity")
	}
	return resolved, nil
}

func changedPaths(ctx context.Context, root, base, target string) ([]string, error) {
	raw, err := runGit(ctx, gitrun.NewDefaultBudget(), root, maximumTreeBytes+1,
		"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv",
		"--ignore-submodules=none", base, target, "--", ".")
	if err != nil {
		return nil, err
	}
	if len(raw) > maximumTreeBytes {
		return nil, fmt.Errorf("changed-path candidates exceed %d bytes", maximumTreeBytes)
	}
	if len(raw) != 0 && raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("Git changed-path output is malformed")
	}
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	if len(raw) == 0 {
		fields = nil
	}
	paths := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if len(field) == 0 {
			return nil, fmt.Errorf("Git changed-path output contains an empty path")
		}
		value := string(field)
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("Git changed-path output contains a duplicate path")
		}
		seen[value] = struct{}{}
		paths = append(paths, value)
		if len(paths) > trace.MaxAdmissionCandidates {
			return nil, fmt.Errorf("changed-path candidates exceed %d paths", trace.MaxAdmissionCandidates)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func digestPaths(paths []string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte("corvint-record-path-admission/v0\x00"))
	for _, path := range paths {
		_, _ = digest.Write([]byte(path))
		_, _ = digest.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil))
}

func runGit(ctx context.Context, budget *gitrun.Budget, root string, limit int, arguments ...string) ([]byte, error) {
	args := gitArguments(root, arguments...)
	result, err := gitrun.Run(ctx, budget, gitrun.Options{Env: gitEnvironment(), StdoutLimit: limit}, args...)
	if err == nil {
		return result, nil
	}
	if startError, ok := cemcode.GitStartFailureDetails(err); ok {
		if len(arguments) != 0 && arguments[0] == "log" {
			if detail, exact := pythonGitStartDetail(startError); exact {
				return nil, fmt.Errorf("Git error: %s", detail)
			}
		}
		if len(arguments) != 0 && arguments[0] == "ls-tree" {
			return nil, fmt.Errorf("cannot start Git")
		}
		return nil, err
	}
	details, ok := cemcode.GitExitFailureDetails(err)
	if !ok {
		return nil, err
	}
	detail := contextindex.TrimPythonSpace(contextindex.DecodePythonUTF8(details.Stderr))
	if detail == "" && len(arguments) != 0 && arguments[0] == "log" {
		detail = pythonCalledProcess(root, pythonAncestryLogArguments(arguments), details.ExitCode)
	}
	if detail == "" {
		detail = "git command failed"
	}
	return nil, fmt.Errorf("Git error: %s", detail)
}

func pythonAncestryLogArguments(arguments []string) []string {
	if len(arguments) == 5 && arguments[0] == "log" && arguments[1] == "--topo-order" &&
		strings.HasPrefix(arguments[2], "--max-count=") && arguments[3] == "--format=%H %T %P" && arguments[4] == "HEAD" {
		return []string{"log", arguments[2], "--format=%H %T", "HEAD"}
	}
	return arguments
}

func gitArguments(root string, arguments ...string) []string {
	args := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-C", root,
		"-c", "advice.graftFileDeprecated=false",
	}
	return append(args, arguments...)
}

func probeGitAncestor(ctx context.Context, budget *gitrun.Budget, root, candidate, head string) (bool, error) {
	args := gitArguments(root, "merge-base", "--is-ancestor", candidate, head)
	result, err := gitrun.Run(ctx, budget, gitrun.Options{Env: gitEnvironment(), StdoutLimit: 1}, args...)
	if err == nil {
		if len(result) != 0 {
			return false, fmt.Errorf("Git candidate ancestry probe returned unexpected output")
		}
		return true, nil
	}
	details, exited := cemcode.GitExitFailureDetails(err)
	if exited && details.ExitCode == 1 && len(details.Stderr) == 0 {
		return false, nil
	}
	return false, err
}

func recordIndexError(root string, err error) error {
	details, ok := contextindex.GitFailureDetails(err)
	if !ok {
		return err
	}
	detail := contextindex.TrimPythonSpace(contextindex.DecodePythonUTF8(details.Stderr))
	if detail != "" {
		return fmt.Errorf("Git error: %s", detail)
	}
	if details.StartError != nil {
		if detail, exact := pythonGitStartDetail(details.StartError); exact {
			return fmt.Errorf("Git error: %s", detail)
		}
		return err
	}
	if hasArgument(details.Arguments, "status") {
		return fmt.Errorf("Git error: git status exited %d", details.ExitCode)
	}
	arguments := details.Arguments
	if equalStrings(arguments, []string{"rev-parse", "--show-object-format", "--is-shallow-repository", "HEAD", "HEAD^{tree}", "--git-path", "info/grafts"}) {
		arguments = []string{"rev-parse", "--show-object-format", "HEAD^{tree}"}
	}
	return fmt.Errorf("Git error: %s", pythonCalledProcess(root, arguments, details.ExitCode))
}

func pythonGitStartDetail(err error) (string, bool) {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "[Errno 2] No such file or directory: 'git'", true
	}
	if errors.Is(err, os.ErrPermission) {
		return "[Errno 13] Permission denied: 'git'", true
	}
	return "", false
}

func pythonCalledProcess(root string, arguments []string, exitCode int) string {
	command := append([]string{"git", "-C", root}, arguments...)
	quoted := make([]string, len(command))
	for index, value := range command {
		quoted[index] = pythonStringRepr(value)
	}
	return fmt.Sprintf("Command '[%s]' returned non-zero exit status %d.", strings.Join(quoted, ", "), exitCode)
}

func pythonStringRepr(value string) string {
	quote := '\''
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var output strings.Builder
	output.WriteRune(quote)
	for _, character := range value {
		switch character {
		case '\\':
			output.WriteString(`\\`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character == quote {
				output.WriteByte('\\')
			}
			output.WriteRune(character)
		}
	}
	output.WriteRune(quote)
	return output.String()
}

func hasArgument(arguments []string, wanted string) bool {
	for _, argument := range arguments {
		if argument == wanted {
			return true
		}
	}
	return false
}

func gitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull,
		"GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func sortedSourcePaths(sources map[string]contextindex.Source) []string {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func validOutcome(value string) bool {
	return value == "passed" || value == "failed" || value == "blocked"
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func objectIDLength(format string) int {
	if format == "sha256" {
		return 64
	}
	return 40
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
