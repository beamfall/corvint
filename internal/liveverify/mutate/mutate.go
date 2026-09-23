// Package mutate implements the test-kills-mutant falsifier described by
// docs/specs/falsifiable-packet-v0.md. An impact packet row of kind test claims
// that a test file covers a changed Go file; this package mutates the changed
// file's top-level declarations, runs the claimed tests against each mutant on
// an exported copy of the revision, and requires at least one mutant to die.
// A surviving population falsifies the claim that the test proves anything.
package mutate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Verdict is the falsifier's judgement about the claim. Every judgement about
// the code under test travels here; error is reserved for caller mistakes and
// infrastructure failure.
type Verdict string

const (
	// Killed reports that at least one mutant died: the falsifier passes.
	Killed Verdict = "KILLED"
	// Survived reports that every mutant lived: the falsifier fails.
	Survived Verdict = "SURVIVED"
	// NoMutants reports that the changed file held nothing mutable.
	NoMutants Verdict = "NO_MUTANTS"
	// BudgetExceeded reports that time ran out before any mutant was killed.
	BudgetExceeded Verdict = "BUDGET_EXCEEDED"
	// Unsupported reports that the claim could not be judged at all.
	Unsupported Verdict = "UNSUPPORTED"
)

// Request names one claim to falsify. Root is never modified: all work happens
// on an export of Revision under the system temporary directory.
type Request struct {
	Root        string
	Git         string
	Revision    string
	ChangedPath string
	TestPath    string
	// Lines, when set, confines mutation to sites whose line falls inside
	// one of the spans (1-based, inclusive), so a claim about a change is
	// judged on the change and not on the rest of the file.
	Lines      []LineSpan
	MaxMutants int
	Budget     time.Duration
	// Complete runs every mutant instead of stopping at the first kill.
	Complete bool
	// CacheDir, when set, is the absolute GOCACHE every run in this request
	// writes. Callers share it across requests of one invocation and remove
	// it; it must never be the host's own build cache.
	CacheDir string
}

// Report is the falsifier's answer. Detail is one deterministic human-readable
// line: it carries no timings and no temporary paths.
type Report struct {
	Verdict  Verdict
	Mutants  int
	Killed   int
	Survived int
	// Uncompilable counts mutants that never built, which are neither killed
	// nor survivors: the test was never asked about them.
	Uncompilable int
	Skipped      int
	Operators    []string
	Elapsed      time.Duration
	Detail       string
	// Witness identifies the first behavioural kill when the runner could
	// attribute it to one named test. It is nil for every non-kill and for an
	// unattributed package failure.
	Witness *Witness
	// Survivors lists every mutant that built and passed the tests, in plan
	// order; it is complete only for a Complete run.
	Survivors []Survivor
}

// Survivor is one mutant the tests let live: its operator, the 1-based line
// of the changed file it was applied at, and its zero-based, end-exclusive
// byte span in the original changed-file blob.
type Survivor struct {
	Operator string
	Line     int
	Start    int
	End      int
}

// Witness is the replayable part of one killed mutant. Start and End are a
// zero-based, end-exclusive byte span in the original changed-file blob.
type Witness struct {
	Operator    string
	Start       int
	End         int
	KillingTest string
}

const (
	defaultMaxMutants = 8
	defaultBudget     = 10 * time.Minute
	minimumPerRun     = 10 * time.Second
	outputLimit       = 64 << 10
)

// LineSpan is one inclusive 1-based line range of the changed file.
type LineSpan struct {
	Start, End int
}

type settings struct {
	exportSettings
	changedPath string
	testPath    string
	packageDir  string
	lines       []LineSpan
	maxMutants  int
	budget      time.Duration
	perRun      time.Duration
	complete    bool
}

// Export is one exported revision that judges many claims on one copy. The
// runner restores the changed file after every mutant, so no claim ever sees
// another's mutation, and every claim shares the export's build cache.
type Export struct {
	configuration exportSettings
	space         workspace
	remove        func()
	unsandboxed   string
}

// Open exports request.Revision once; only Root, Git, Revision, and CacheDir
// are read. A host without a sandbox opens nothing and every later Judge
// reports Unsupported. Close removes the copy.
func Open(ctx context.Context, request Request) (*Export, error) {
	configuration, err := normalizeExport(request)
	if err != nil {
		return nil, err
	}
	return open(ctx, configuration)
}

func open(ctx context.Context, configuration exportSettings) (*Export, error) {
	box, err := findSandbox()
	if err != nil {
		return &Export{configuration: configuration, unsandboxed: "cited tests cannot be sandboxed: " + err.Error()}, nil
	}
	space, remove, err := newWorkspace(ctx, configuration, box)
	if err != nil {
		return nil, err
	}
	return &Export{configuration: configuration, space: space, remove: remove}, nil
}

// Close removes the exported copy. It is safe to call more than once.
func (exported *Export) Close() {
	if exported.remove != nil {
		exported.remove()
		exported.remove = nil
	}
}

// Judge falsifies one claim on the exported copy. Root, Git, and Revision
// may be left empty; when given they must match the export.
func (exported *Export) Judge(ctx context.Context, request Request) (Report, error) {
	configuration, err := normalize(exported.claimed(request))
	if err != nil {
		return Report{}, err
	}
	if configuration.exportSettings != exported.configuration {
		return Report{}, fmt.Errorf("mutate: claim %s differs from the export of %s", configuration.revision, exported.configuration.revision)
	}
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, configuration.budget)
	defer cancel()
	return exported.judge(runCtx, configuration, started)
}

func (exported *Export) claimed(request Request) Request {
	if request.Root == "" {
		request.Root = exported.configuration.root
	}
	if request.Git == "" {
		request.Git = exported.configuration.git
	}
	if request.Revision == "" {
		request.Revision = exported.configuration.revision
	}
	if request.CacheDir == "" {
		request.CacheDir = exported.configuration.cacheDir
	}
	return request
}

func (exported *Export) judge(ctx context.Context, configuration settings, started time.Time) (Report, error) {
	if exported.unsandboxed != "" {
		return unsupportedReport(exported.unsandboxed), nil
	}
	report, err := falsify(ctx, configuration, exported.space)
	if err != nil && ctx.Err() != nil {
		report, err = budgetReport("budget exhausted during the run"), nil
	}
	if err != nil {
		return Report{}, err
	}
	report.Elapsed = time.Since(started)
	return report, nil
}

// Run exports Revision, establishes a baseline, and runs the generated mutants
// against the tests declared in TestPath: one claim on one copy.
func Run(ctx context.Context, request Request) (Report, error) {
	configuration, err := normalize(request)
	if err != nil {
		return Report{}, err
	}
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, configuration.budget)
	defer cancel()
	exported, err := open(runCtx, configuration.exportSettings)
	if err != nil && runCtx.Err() != nil {
		return budgetReport("budget exhausted before the export completed"), nil
	}
	if err != nil {
		return Report{}, err
	}
	defer exported.Close()
	return exported.judge(runCtx, configuration, started)
}

// exportSettings is the part of a request that names the copy.
type exportSettings struct {
	root, git, revision, cacheDir string
}

func normalizeExport(request Request) (exportSettings, error) {
	if strings.TrimSpace(request.Root) == "" {
		return exportSettings{}, errors.New("mutate: Root is required")
	}
	if strings.TrimSpace(request.Git) == "" {
		return exportSettings{}, errors.New("mutate: Git is required")
	}
	if strings.TrimSpace(request.Revision) == "" {
		return exportSettings{}, errors.New("mutate: Revision is required")
	}
	if request.CacheDir != "" && !filepath.IsAbs(request.CacheDir) {
		return exportSettings{}, fmt.Errorf("mutate: CacheDir %q is not absolute", request.CacheDir)
	}
	return exportSettings{
		root:     request.Root,
		git:      request.Git,
		revision: request.Revision,
		cacheDir: strings.TrimSpace(request.CacheDir),
	}, nil
}

func normalize(request Request) (settings, error) {
	exported, err := normalizeExport(request)
	if err != nil {
		return settings{}, err
	}
	changedPath, err := relativeGoPath(request.ChangedPath, "ChangedPath")
	if err != nil {
		return settings{}, err
	}
	testPath, err := relativeGoPath(request.TestPath, "TestPath")
	if err != nil {
		return settings{}, err
	}
	if !strings.HasSuffix(testPath, "_test.go") {
		return settings{}, fmt.Errorf("mutate: TestPath %q is not a _test.go file", testPath)
	}
	for _, span := range request.Lines {
		if span.Start < 1 || span.End < span.Start {
			return settings{}, fmt.Errorf("mutate: Lines span %d-%d is not a 1-based inclusive range", span.Start, span.End)
		}
	}
	maxMutants, err := mutantCap(request.MaxMutants)
	if err != nil {
		return settings{}, err
	}
	budget, err := runBudget(request.Budget)
	if err != nil {
		return settings{}, err
	}
	return settings{
		exportSettings: exported,
		changedPath:    changedPath,
		testPath:       testPath,
		packageDir:     packageArgument(testPath),
		lines:          append([]LineSpan(nil), request.Lines...),
		maxMutants:     maxMutants,
		budget:         budget,
		perRun:         perRunTimeout(budget, maxMutants),
		complete:       request.Complete,
	}, nil
}

func mutantCap(requested int) (int, error) {
	if requested < 0 {
		return 0, fmt.Errorf("mutate: MaxMutants %d is negative", requested)
	}
	if requested == 0 {
		return defaultMaxMutants, nil
	}
	return requested, nil
}

func runBudget(requested time.Duration) (time.Duration, error) {
	if requested < 0 {
		return 0, fmt.Errorf("mutate: Budget %s is negative", requested)
	}
	if requested == 0 {
		return defaultBudget, nil
	}
	return requested, nil
}

func relativeGoPath(candidate, field string) (string, error) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return "", fmt.Errorf("mutate: %s is required", field)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("mutate: %s %q is not a repository-relative path", field, candidate)
	}
	if !strings.HasSuffix(cleaned, ".go") {
		return "", fmt.Errorf("mutate: %s %q is not a .go file", field, candidate)
	}
	return cleaned, nil
}

func packageArgument(testPath string) string {
	directory := path.Dir(testPath)
	if directory == "." {
		return "."
	}
	return "./" + directory
}

func perRunTimeout(budget time.Duration, maxMutants int) time.Duration {
	share := budget / time.Duration(maxMutants+1)
	if share < minimumPerRun {
		return minimumPerRun
	}
	return share
}

// falsify judges one claim on the export: baseline first, then the mutants.
func falsify(ctx context.Context, configuration settings, space workspace) (Report, error) {
	if ctx.Err() != nil {
		return budgetReport("budget exhausted before the run started"), nil
	}
	if blocked, unsupported := inspectExport(space.export, configuration); blocked {
		return unsupported, nil
	}
	source, err := os.ReadFile(filepath.Join(space.export, filepath.FromSlash(configuration.changedPath)))
	if err != nil {
		return Report{}, fmt.Errorf("mutate: read exported %s: %w", configuration.changedPath, err)
	}
	names, err := testFunctionNames(filepath.Join(space.export, filepath.FromSlash(configuration.testPath)))
	if err != nil {
		return unsupportedReport("test file does not parse: " + configuration.testPath), nil
	}
	if len(names) == 0 {
		return unsupportedReport("test file declares no tests: " + configuration.testPath), nil
	}
	pattern := runPattern(names)
	baseline, output := runTests(ctx, configuration, space, pattern)
	if baseline != runPassed && ctx.Err() != nil {
		return budgetReport("budget exhausted during the baseline run"), nil
	}
	if baseline == runBroken {
		return Report{}, inconclusive(output)
	}
	if baseline != runPassed {
		return unsupportedReport(baselineDetail(output, configuration)), nil
	}
	mutants, err := generateMutants(source, configuration.maxMutants, configuration.lines)
	if err != nil {
		return unsupportedReport("changed file does not parse: " + configuration.changedPath), nil
	}
	if len(mutants) == 0 {
		return Report{
			Verdict:   NoMutants,
			Operators: []string{},
			Detail:    noMutantsDetail(configuration),
		}, nil
	}
	return judgeMutants(ctx, configuration, space, source, pattern, names, mutants)
}

func noMutantsDetail(configuration settings) string {
	if len(configuration.lines) != 0 {
		return "no mutable declarations inside the changed lines of " + configuration.changedPath
	}
	return "no mutable declarations in " + configuration.changedPath
}

func judgeMutants(ctx context.Context, configuration settings, space workspace, source []byte, pattern string, names []string, mutants []mutant) (Report, error) {
	report := Report{Mutants: len(mutants), Operators: operatorNames(mutants)}
	changedFile := filepath.Join(space.export, filepath.FromSlash(configuration.changedPath))
	defer func() { _ = os.WriteFile(changedFile, source, 0o644) }()
	firstKill := ""
	for index, candidate := range mutants {
		if ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		if err := os.WriteFile(changedFile, candidate.Source, 0o644); err != nil {
			return Report{}, fmt.Errorf("mutate: write mutant: %w", err)
		}
		outcome, output, outcomes := runGoTest(ctx, configuration.perRun, space, configuration.packageDir, pattern, true)
		if err := os.WriteFile(changedFile, source, 0o644); err != nil {
			return Report{}, fmt.Errorf("mutate: restore %s: %w", configuration.changedPath, err)
		}
		if outcome != runPassed && ctx.Err() != nil {
			return budgetPartial(report, index), nil
		}
		if outcome == runBroken {
			return Report{}, inconclusive(output)
		}
		if outcome == runPassed {
			report.Survived++
			report.Survivors = append(report.Survivors, survivor(candidate))
			continue
		}
		if outcome == runUnbuildable {
			report.Uncompilable++
			continue
		}
		report.Killed++
		if firstKill == "" {
			firstKill = fmt.Sprintf("%s at %s:%d", candidate.Operator, configuration.changedPath, candidate.Line)
			if killingTest := firstFailedTest(names, outcomes); killingTest != "" {
				report.Witness = mutationWitness(candidate, killingTest)
			}
		}
		if !configuration.complete {
			report.Skipped = len(mutants) - index - 1
			break
		}
	}
	return finishedReport(report, configuration, firstKill), nil
}

func firstFailedTest(names []string, outcomes map[string]runOutcome) string {
	for _, name := range names {
		if outcomes[name] == runFailed {
			return name
		}
	}
	return ""
}

func survivor(candidate mutant) Survivor {
	return Survivor{Operator: candidate.Operator, Line: candidate.Line, Start: candidate.Start, End: candidate.End}
}

func mutationWitness(candidate mutant, killingTest string) *Witness {
	return &Witness{
		Operator: candidate.Operator, Start: candidate.Start, End: candidate.End,
		KillingTest: killingTest,
	}
}

func finishedReport(report Report, configuration settings, firstKill string) Report {
	if report.Killed > 0 {
		report.Verdict = Killed
		report.Detail = fmt.Sprintf("killed %d of %d mutants; first kill by %s", report.Killed, report.Mutants, firstKill) + uncompilableSuffix(report)
		return report
	}
	if report.Survived == 0 {
		report.Verdict = NoMutants
		report.Detail = fmt.Sprintf("every one of %d mutants of %s failed to compile", report.Mutants, configuration.changedPath)
		return report
	}
	report.Verdict = Survived
	report.Detail = fmt.Sprintf("all %d mutants survived %s", report.Survived, configuration.testPath) + uncompilableSuffix(report)
	return report
}

func uncompilableSuffix(report Report) string {
	if report.Uncompilable == 0 {
		return ""
	}
	return fmt.Sprintf("; %d did not compile", report.Uncompilable)
}

func budgetPartial(report Report, index int) Report {
	report.Skipped = report.Mutants - index
	report.Verdict = BudgetExceeded
	report.Detail = fmt.Sprintf("budget exhausted after %d of %d mutants", index, report.Mutants)
	return report
}

func budgetReport(detail string) Report {
	return Report{Verdict: BudgetExceeded, Operators: []string{}, Detail: detail}
}

func unsupportedReport(detail string) Report {
	return Report{Verdict: Unsupported, Operators: []string{}, Detail: detail}
}

func operatorNames(mutants []mutant) []string {
	seen := make(map[string]struct{}, len(mutants))
	names := make([]string, 0, len(mutants))
	for _, candidate := range mutants {
		if _, known := seen[candidate.Operator]; known {
			continue
		}
		seen[candidate.Operator] = struct{}{}
		names = append(names, candidate.Operator)
	}
	sort.Strings(names)
	return names
}
