package verify

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// HunkRevertProfile names the consumer-observed hunk-revert report (decision
// 0523 P2, CEM-V1-088, CEM-V1-089). It is reported beside the stable result,
// never inside it: an observation of test execution depends on the host, so it
// cannot enter the cross-consumer verdict digest (CEM-V1-066).
const HunkRevertProfile = "cem-hunk-revert/0"

// HunkRevertRunner is the one runner profile the falsifier executes. Every
// other runner is a recorded limit.
const HunkRevertRunner = "go-test"

// Hunk-revert results. Only RevertHeld projects a hunk to the held tier.
const (
	RevertHeld     = "held"
	RevertSurvived = "survived"
	RevertNotRun   = "not-run"
)

// Closed not-run reasons (CEM-V1-089).
const (
	RevertNoNamedTest        = "no-named-test"
	RevertNotModification    = "not-a-modification"
	RevertMismatch           = "revert-mismatch"
	RevertBreaksBuild        = "revert-breaks-build"
	RevertFailsWithoutRevert = "fails-without-revert"
	RevertSkipped            = "skipped"
	RevertNotObserved        = "test-not-observed"
	RevertTimeout            = "timeout"
	RevertTotalLimit         = "total-limit"
	RevertInfrastructure     = "infrastructure"
)

// Closed not-attempted reasons (CEM-V1-320): why a mapped hunk is not a
// falsifier candidate at all, so "not attempted" differs from "nothing to do".
const (
	NotAttemptedTestOnly    = "test-only-hunk"
	NotAttemptedLanguage    = "unsupported-language"
	NotAttemptedDisposition = "not-supported-disposition"
	NotAttemptedNoTestClaim = "no-test-claim"
	NotAttemptedNotInPatch  = "not-in-patch"
)

// HunkRevertLimits bound one falsifier run: PerRun bounds each go test
// invocation and Total bounds the whole run, export included.
type HunkRevertLimits struct {
	PerRun, Total time.Duration
}

// DefaultHunkRevertLimits are the recorded limits of `cem verify --hunk-revert`.
var DefaultHunkRevertLimits = HunkRevertLimits{PerRun: 5 * time.Minute, Total: 20 * time.Minute}

// HunkRevert is the falsifier report. HunksByClass is the accepted result's
// citation tiers with every held row moved to held; nothing else changes.
type HunkRevert struct {
	Profile        string          `json:"profile"`
	Runner         string          `json:"runner"`
	PerRunSeconds  int64           `json:"perRunSeconds"`
	TotalSeconds   int64           `json:"totalSeconds"`
	Hunks          []HunkRevertRow `json:"hunks"`
	Held           int             `json:"held"`
	Survived       int             `json:"survived"`
	NotRun         int             `json:"notRun"`
	HunksByClass   CitationClasses `json:"hunksByClass"`
	ElapsedSeconds float64         `json:"elapsedSeconds"`
	// NotAttempted lists, in hunk ID order, every mapped hunk that is not a
	// candidate, with its closed reason (CEM-V1-320, CEM-V1-321). It is
	// additive: Hunks and the counts keep their candidate-only meaning.
	NotAttempted []HunkNotAttempted `json:"notAttempted"`
}

// HunkNotAttempted is one mapped hunk the falsifier never attempted.
type HunkNotAttempted struct {
	HunkID string `json:"hunkId"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// HunkRevertRow is one candidate hunk: a supported Go source hunk whose basis
// cites a _test.go span as test-claim. Tests are the top-level Go tests the
// cited spans declare; Failing names the tests the consumer itself observed
// pass at the target and fail with only this hunk reverted.
type HunkRevertRow struct {
	HunkID  string   `json:"hunkId"`
	Tests   []string `json:"tests"`
	Result  string   `json:"result"`
	Reason  string   `json:"reason"`
	Failing []string `json:"failing"`
}

// stableCapture receives the admitted mechanics of an accepted Stable call.
type stableCapture struct {
	change *wire.Map
	parsed *patch.Patch
	target string
}

// StableWithHunkRevert runs Stable and, only when it accepts, the hunk-revert
// falsifier against the accepted target. The result and exit are exactly what
// Stable returns; the report is nil unless the result accepted.
func StableWithHunkRevert(ctx context.Context, raw []byte, o StableOptions, limits HunkRevertLimits) (StableResult, int, *HunkRevert) {
	capture := &stableCapture{}
	o.capture = capture
	result, exit := Stable(ctx, raw, o)
	if result.Outcome != "ACCEPT" || capture.change == nil || result.Citation == nil {
		return result, exit, nil
	}
	report := runHunkRevert(ctx, o.Repository, capture, result.Citation, limits, goTestRunner(o.Repository))
	return result, exit, report
}

// revertCandidate is one hunk the falsifier will judge.
type revertCandidate struct {
	hunk  *patch.Hunk
	kind  patch.GroupKind
	units []testUnit
	tests []string
}

// testUnit is one package directory and the tests named in it.
type testUnit struct {
	dir   string
	names []string
}

// testRunner executes the fixed go test profile for names in directory dir of
// the disposable copy.
type testRunner func(ctx context.Context, tree, dir string, names []string) runOutcome

func runHunkRevert(ctx context.Context, repository string, c *stableCapture, citation *Citation, limits HunkRevertLimits, run testRunner) *HunkRevert {
	started := time.Now()
	report := &HunkRevert{
		Profile: HunkRevertProfile, Runner: HunkRevertRunner,
		PerRunSeconds: int64(limits.PerRun / time.Second), TotalSeconds: int64(limits.Total / time.Second),
		Hunks: []HunkRevertRow{}, HunksByClass: citation.HunksByClass, NotAttempted: []HunkNotAttempted{},
	}
	defer func() { report.ElapsedSeconds = time.Since(started).Round(time.Millisecond).Seconds() }()
	candidates, notAttempted := revertCandidates(c, func(blobOID string) ([]byte, error) {
		return gitOutput(ctx, repository, nil, "cat-file", "blob", blobOID)
	})
	report.NotAttempted = notAttempted
	if len(candidates) == 0 {
		return report
	}
	total, cancel := context.WithTimeout(ctx, limits.Total)
	defer cancel()
	rows := make([]HunkRevertRow, len(candidates))
	for i, candidate := range candidates {
		rows[i] = HunkRevertRow{HunkID: candidate.hunk.ID, Tests: candidate.tests, Result: RevertNotRun, Failing: []string{}}
	}
	runnable := false
	for _, candidate := range candidates {
		runnable = runnable || candidate.kind == patch.KindModify && len(candidate.tests) > 0
	}
	var err error
	if !runnable {
		// Every row is decided before any file is read: no copy is exported.
		judgeCandidates(total, "", candidates, rows, limits, run)
	} else if workspace, mkErr := disposableWorkspace(total, repository, "corvint-hunk-revert-"); mkErr != nil {
		err = mkErr
	} else {
		// The disposable copy is removed on every exit path; the caller's
		// worktree, index and refs are only ever read through git ls-tree and
		// git cat-file (product invariant 4).
		defer os.RemoveAll(workspace)
		tree := filepath.Join(workspace, "tree")
		err = exportTree(total, repository, c.target, tree, filepath.Join(workspace, "tmp"))
		if err == nil {
			judgeCandidates(total, tree, candidates, rows, limits, run)
		}
	}
	tiers := map[string]string{}
	for _, hunk := range citation.Hunks {
		tiers[hunk.HunkID] = hunk.Tier
	}
	for i := range rows {
		if err != nil {
			rows[i].Reason = RevertInfrastructure
			if total.Err() != nil {
				rows[i].Reason = RevertTotalLimit
			}
		}
		report.count(rows[i], tiers[rows[i].HunkID])
	}
	report.Hunks = rows
	return report
}

// count tallies one row and moves a held hunk from its accepted tier to held.
func (report *HunkRevert) count(row HunkRevertRow, tier string) {
	switch row.Result {
	case RevertHeld:
		report.Held++
		classes := &report.HunksByClass
		for name, slot := range map[string]*int{TierAnchored: &classes.Anchored, TierCited: &classes.Cited} {
			if tier == name && *slot > 0 {
				*slot--
				classes.Held++
			}
		}
	case RevertSurvived:
		report.Survived++
	default:
		report.NotRun++
	}
}

// judgeCandidates judges every candidate against tree, the pristine export of
// the target. No test ever runs in tree itself: each baseline run and each
// reverted run gets a fresh snapshot of it, with only the reverted file
// changed for a reverted run (CEM-V1-692), so a file one run's tests create or
// change can never reach another run and forge a held or survived verdict.
func judgeCandidates(total context.Context, tree string, candidates []revertCandidate, rows []HunkRevertRow, limits HunkRevertLimits, run testRunner) {
	bounded := func(dir string, names []string, revert *revertedFile) runOutcome {
		snapshot, cleanup, err := snapshotTree(tree, revert)
		if err != nil {
			return runOutcome{err: err}
		}
		defer cleanup()
		perRun, cancel := context.WithTimeout(total, limits.PerRun)
		defer cancel()
		outcome := run(perRun, snapshot, dir, names)
		if perRun.Err() != nil {
			outcome.timedOut = true
		}
		return outcome
	}
	atTarget := map[string]runOutcome{}
	for i, candidate := range candidates {
		row := &rows[i]
		switch {
		case total.Err() != nil:
			row.Reason = RevertTotalLimit
			continue
		case candidate.kind != patch.KindModify:
			row.Reason = RevertNotModification
			continue
		case len(candidate.tests) == 0:
			row.Reason = RevertNoNamedTest
			continue
		}
		file := filepath.Join(tree, filepath.FromSlash(*candidate.hunk.NewPath))
		original, perm, err := readCopyFile(tree, file)
		if err != nil {
			row.Reason = RevertInfrastructure
			continue
		}
		reverted, ok := reverseHunk(original, candidate.hunk)
		if !ok {
			row.Reason = RevertMismatch
			continue
		}
		baseline := make([]runOutcome, len(candidate.units))
		for u, unit := range candidate.units {
			key := unit.dir + "\x00" + strings.Join(unit.names, "\x00")
			if _, seen := atTarget[key]; !seen {
				atTarget[key] = bounded(unit.dir, unit.names, nil)
			}
			baseline[u] = atTarget[key]
		}
		revert := &revertedFile{path: *candidate.hunk.NewPath, data: reverted, perm: perm}
		var verdicts []unitVerdict
		for u, unit := range candidate.units {
			eligible := baseline[u].passed(unit.names)
			if len(eligible) == 0 {
				verdicts = append(verdicts, unitVerdict{result: RevertNotRun, reason: baseline[u].notRunReason(unit.names), noEligible: true})
				continue
			}
			verdicts = append(verdicts, classifyRevert(bounded(unit.dir, eligible, revert), eligible))
		}
		combine(row, verdicts)
	}
}

// revertedFile is the one file a reverted run's snapshot replaces: a
// slash-separated path below the copy, its reverted bytes and its mode.
type revertedFile struct {
	path string
	data []byte
	perm os.FileMode
}

// snapshotTree copies the pristine export tree into a fresh run directory
// beside it, <run>/tree with an empty <run>/tmp, so the run's TMPDIR and any
// redirected cache stay inside that run (goTestEnvironment). The export holds
// only regular files and directories, and no test runs in it, so the copy is
// exact. revert, when set, replaces its one file in the snapshot. cleanup
// removes the whole run directory.
func snapshotTree(pristine string, revert *revertedFile) (string, func(), error) {
	run, err := os.MkdirTemp(filepath.Dir(pristine), "run-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(run) }
	snapshot := filepath.Join(run, "tree")
	if err := os.Mkdir(filepath.Join(run, "tmp"), 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.CopyFS(snapshot, os.DirFS(pristine)); err != nil {
		cleanup()
		return "", nil, err
	}
	if revert != nil {
		file := filepath.Join(snapshot, filepath.FromSlash(revert.path))
		if err := replaceCopyFile(snapshot, file, revert.data, revert.perm); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return snapshot, cleanup, nil
}

// readCopyFile reads a regular file of the copy reached through real
// directories only. A test run may have replaced a path component with a
// symlink; such a path is refused, never followed out of the copy.
func readCopyFile(tree, file string) ([]byte, os.FileMode, error) {
	if err := copyPathIsReal(tree, file); err != nil {
		return nil, 0, err
	}
	info, err := os.Lstat(file)
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(file)
	return data, info.Mode().Perm(), err
}

// replaceCopyFile replaces a regular file of the copy without following a
// symlink: every component is checked, the old entry is unlinked, and the new
// file is created exclusively, so a write never leaves the copy. The test
// processes that could race it have exited before this runs.
func replaceCopyFile(tree, file string, data []byte, perm os.FileMode) error {
	if err := copyPathIsReal(tree, file); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}

// copyPathIsReal requires every directory from tree down to file to be a real
// directory and file itself to be a regular file.
func copyPathIsReal(tree, file string) error {
	relative, err := filepath.Rel(tree, file)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path is outside the copy")
	}
	current := tree
	parts := strings.Split(relative, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if last := i == len(parts)-1; last && !info.Mode().IsRegular() || !last && !info.IsDir() {
			return errors.New("copy path component is not a real file or directory")
		}
	}
	return nil
}

// unitVerdict is one package's judgement of one reverted hunk. noEligible
// marks a package none of whose named tests passed at the target, so it had
// no reverted run.
type unitVerdict struct {
	result, reason string
	failing        []string
	noEligible     bool
}

// combine folds per-package verdicts: held when any package held; survived
// only when every package with eligible tests survived, so every eligible test
// of the row passed reverted (CEM-V1-693); otherwise the not-run reason of the
// first package with eligible tests that did not survive, or of the first
// package when none had eligible tests.
func combine(row *HunkRevertRow, verdicts []unitVerdict) {
	for _, verdict := range verdicts {
		if verdict.result == RevertHeld {
			row.Result, row.Reason = RevertHeld, ""
			row.Failing = append(row.Failing, verdict.failing...)
		}
	}
	if row.Result == RevertHeld {
		sort.Strings(row.Failing)
		return
	}
	survived := false
	for _, verdict := range verdicts {
		switch {
		case verdict.result == RevertSurvived:
			survived = true
		case !verdict.noEligible:
			row.Reason = verdict.reason
			return
		}
	}
	if survived {
		row.Result, row.Reason = RevertSurvived, ""
		return
	}
	if len(verdicts) > 0 {
		row.Reason = verdicts[0].reason
	}
}

// classifyRevert judges one reverted run over tests that passed at the target.
// held needs a test-level fail event the consumer itself decoded and a
// package that failed with it; survived needs a terminal pass for every
// eligible test (CEM-V1-693), so one skipped or unreported test makes the
// unit skipped or test-not-observed, never survived; a build failure, a
// timeout or an undecodable run is not-run (CEM-V1-089).
func classifyRevert(outcome runOutcome, eligible []string) unitVerdict {
	switch {
	case outcome.timedOut:
		return unitVerdict{result: RevertNotRun, reason: RevertTimeout}
	case outcome.err != nil:
		return unitVerdict{result: RevertNotRun, reason: RevertInfrastructure}
	case outcome.buildFailed:
		return unitVerdict{result: RevertNotRun, reason: RevertBreaksBuild}
	}
	var failing []string
	passed := 0
	for _, name := range eligible {
		switch outcome.states[name] {
		case "fail":
			failing = append(failing, name)
		case "pass":
			passed++
		}
	}
	switch {
	case len(failing) > 0 && !outcome.packageFailed:
		// A test-level fail whose package still passed is a stream the test
		// process forged (a printed framing line), not a failure: go sets the
		// package result from the binary's exit status.
		return unitVerdict{result: RevertNotRun, reason: RevertInfrastructure}
	case len(failing) > 0:
		return unitVerdict{result: RevertHeld, failing: failing}
	case passed == len(eligible):
		return unitVerdict{result: RevertSurvived}
	case passed == 0 && outcome.packageFailed:
		return unitVerdict{result: RevertNotRun, reason: RevertBreaksBuild}
	}
	for _, name := range eligible {
		if outcome.states[name] == "skip" {
			return unitVerdict{result: RevertNotRun, reason: RevertSkipped}
		}
	}
	return unitVerdict{result: RevertNotRun, reason: RevertNotObserved}
}

// revertCandidates selects, in hunk ID order, every supported hunk on Go
// source (not _test.go) whose basis cites a _test.go span as test-claim. load
// returns a cited test file's base blob; a blob it cannot load names no test.
// Every other mapped hunk is returned as not attempted with its closed reason.
func revertCandidates(c *stableCapture, load func(blobOID string) ([]byte, error)) ([]revertCandidate, []HunkNotAttempted) {
	derived := map[string]*patch.Hunk{}
	kinds := map[string]patch.GroupKind{}
	for _, group := range c.parsed.Groups {
		for _, hunk := range group.Hunks {
			derived[hunk.ID] = hunk
			kinds[hunk.ID] = group.Kind
		}
	}
	records := map[string]wire.Evidence{}
	for _, record := range c.change.Evidence {
		records[record.ID] = record
	}
	blobs := map[string][]byte{}
	hunks := append([]wire.Hunk(nil), c.change.Hunks...)
	sort.Slice(hunks, func(i, j int) bool { return hunks[i].ID < hunks[j].ID })
	var candidates []revertCandidate
	notAttempted := []HunkNotAttempted{}
	skip := func(mapped wire.Hunk, reason string) {
		notAttempted = append(notAttempted, HunkNotAttempted{HunkID: mapped.ID, Path: mapped.Path, Reason: reason})
	}
	for _, mapped := range hunks {
		hunk := derived[mapped.ID]
		switch {
		case strings.HasSuffix(mapped.Path, "_test.go"):
			skip(mapped, NotAttemptedTestOnly)
			continue
		case !goSource(mapped.Path):
			skip(mapped, NotAttemptedLanguage)
			continue
		case mapped.Disposition != "supported":
			skip(mapped, NotAttemptedDisposition)
			continue
		case hunk == nil:
			skip(mapped, NotAttemptedNotInPatch)
			continue
		}
		named := map[string]map[string]bool{}
		cites := false
		for _, basis := range mapped.Basis {
			record := records[basis.EvidenceID]
			evidencePath := record.Path
			if basis.Relation != "test-claim" || !strings.HasSuffix(evidencePath, "_test.go") {
				continue
			}
			cites = true
			dir := path.Dir(evidencePath)
			if named[dir] == nil {
				named[dir] = map[string]bool{}
			}
			if _, loaded := blobs[record.BlobOid]; !loaded {
				blobs[record.BlobOid], _ = load(record.BlobOid)
			}
			for _, name := range declaredTests(blobs[record.BlobOid], record.Span) {
				named[dir][name] = true
			}
		}
		if !cites {
			skip(mapped, NotAttemptedNoTestClaim)
			continue
		}
		candidate := revertCandidate{hunk: hunk, kind: kinds[mapped.ID], tests: []string{}}
		for dir, names := range named {
			if len(names) == 0 {
				continue
			}
			unit := testUnit{dir: dir}
			for name := range names {
				unit.names = append(unit.names, name)
				candidate.tests = append(candidate.tests, dir+"::"+name)
			}
			sort.Strings(unit.names)
			candidate.units = append(candidate.units, unit)
		}
		sort.Slice(candidate.units, func(i, j int) bool { return candidate.units[i].dir < candidate.units[j].dir })
		sort.Strings(candidate.tests)
		candidates = append(candidates, candidate)
	}
	return candidates, notAttempted
}

func goSource(p string) bool {
	return strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
}

// declaredTests returns the top-level Go tests whose declaration header, from
// the func keyword through the parameter list, lies inside span of the parsed
// test file: func TestX(t *testing.T), the names go test runs. Text shaped like
// a declaration inside a comment or string is never a declaration
// (TCQ-V0-061); an unparsable file declares nothing.
func declaredTests(file []byte, span wire.Span) []string {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "", file, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var names []string
	for _, decl := range parsed.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || !goTestDecl(function) {
			continue
		}
		start := int64(fileSet.Position(function.Pos()).Offset)
		end := int64(fileSet.Position(function.Type.Params.End()).Offset)
		if start >= span.Start && end <= span.End {
			names = append(names, function.Name.Name)
		}
	}
	sort.Strings(names)
	return names
}

// goTestDecl reports a top-level func TestX(t *testing.T) that go test runs.
func goTestDecl(function *ast.FuncDecl) bool {
	return function.Recv == nil && function.Type.TypeParams == nil && function.Type.Results == nil &&
		testName(function.Name.Name) && testingTParameter(function.Type.Params)
}

// testName reports whether go test runs a function of this name: Test followed
// by nothing or by a rune that is not a lowercase letter.
func testName(name string) bool {
	return strings.HasPrefix(name, "Test") && !(len(name) > 4 && name[4] >= 'a' && name[4] <= 'z')
}

// testingTParameter reports a parameter list of exactly one *testing.T.
func testingTParameter(params *ast.FieldList) bool {
	if params == nil || len(params.List) != 1 || len(params.List[0].Names) > 1 {
		return false
	}
	star, ok := params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && selector.Sel.Name == "T"
}

// reverseHunk returns data with exactly one hunk reverse-applied: the hunk's
// new-side lines at its new range are replaced by its old-side lines. It
// refuses when the target bytes there are not the hunk's new side.
func reverseHunk(data []byte, hunk *patch.Hunk) ([]byte, bool) {
	lines, err := patch.SplitLF(data)
	if err != nil {
		return nil, false
	}
	var oldSide, newSide [][]byte
	for _, line := range hunk.Body {
		if line.Prefix != '+' {
			oldSide = append(oldSide, line.OldPayload())
		}
		if line.Prefix != '-' {
			newSide = append(newSide, line.NewPayload())
		}
	}
	start := hunk.NewRange.Start - 1
	if hunk.NewRange.Count == 0 {
		start = hunk.NewRange.Start
	}
	end := start + hunk.NewRange.Count
	if start < 0 || end > int64(len(lines)) || int64(len(newSide)) != hunk.NewRange.Count {
		return nil, false
	}
	for i, line := range newSide {
		if !bytes.Equal(lines[start+int64(i)], line) {
			return nil, false
		}
	}
	var out bytes.Buffer
	for _, line := range lines[:start] {
		out.Write(line)
	}
	for _, line := range oldSide {
		out.Write(line)
	}
	for _, line := range lines[end:] {
		out.Write(line)
	}
	return out.Bytes(), true
}
