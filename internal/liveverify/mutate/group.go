package mutate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// claim is one test file of a group: the functions it declares and the
// report those functions earn. seen counts the mutants it has been credited
// for; charged is the wall time of every run it took part in.
type claim struct {
	configuration settings
	names         []string
	report        Report
	seen          int
	charged       time.Duration
	done          bool
}

// packageGroup is every live claim whose tests live in one package: they
// share one go test per mutant.
type packageGroup struct {
	dir    string
	claims []*claim
}

// JudgeGroup falsifies every claim that one of tests covers changed, on the
// exported copy, with one go test per package per mutant instead of one per
// claim. The mutants depend on changed and lines alone, so each is written
// once and every package's live claims run together with -json output, from
// which each claim's own functions decide its verdict exactly as Judge would:
// a function that fails after building kills, every function passing is a
// survivor, a package that did not build is uncompilable. A claim is decided
// at its first kill and leaves later runs; the mutant loop continues while any
// claim is undecided. A function the package run left without an outcome
// (the binary died in another test, the output was cut) is re-run alone under
// its claim's own pattern, so no claim inherits another's crash. Budget: every
// claim in a run is charged the run's wall time; a claim whose charge reaches
// the row budget before a run is BUDGET_EXCEEDED at that mutant; a package
// run's go test timeout is the per-run share times the claims in it, capped
// at the row budget, so a group is never slower per test than a claim alone.
// Reports are keyed by test path; a repeated path is judged once.
func (exported *Export) JudgeGroup(ctx context.Context, changed string, lines []LineSpan, tests []string) (map[string]Report, error) {
	claims, err := exported.groupClaims(changed, lines, tests)
	if err != nil {
		return nil, err
	}
	if err := exported.judgeGroup(ctx, claims); err != nil {
		return nil, err
	}
	reports := make(map[string]Report, len(claims))
	for _, entry := range claims {
		reports[entry.configuration.testPath] = entry.report
	}
	return reports, nil
}

func (exported *Export) groupClaims(changed string, lines []LineSpan, tests []string) ([]*claim, error) {
	claims := make([]*claim, 0, len(tests))
	known := make(map[string]struct{}, len(tests))
	for _, test := range tests {
		configuration, err := normalize(exported.claimed(Request{ChangedPath: changed, TestPath: test, Lines: lines}))
		if err != nil {
			return nil, err
		}
		if configuration.exportSettings != exported.configuration {
			return nil, fmt.Errorf("mutate: claim %s differs from the export of %s", configuration.revision, exported.configuration.revision)
		}
		if _, repeated := known[configuration.testPath]; repeated {
			continue
		}
		known[configuration.testPath] = struct{}{}
		claims = append(claims, &claim{configuration: configuration})
	}
	return claims, nil
}

func (exported *Export) judgeGroup(ctx context.Context, claims []*claim) error {
	if len(claims) == 0 {
		return nil
	}
	if exported.unsandboxed != "" {
		finishAll(claims, unsupportedReport(exported.unsandboxed))
		return nil
	}
	if ctx.Err() != nil {
		finishAll(claims, budgetReport("budget exhausted before the run started"))
		return nil
	}
	space := exported.space
	for _, entry := range claims {
		entry.prepare(space)
	}
	live := liveClaims(claims)
	if len(live) == 0 {
		return nil
	}
	shared := live[0].configuration
	changedFile := filepath.Join(space.export, filepath.FromSlash(shared.changedPath))
	source, err := os.ReadFile(changedFile)
	if err != nil {
		return fmt.Errorf("mutate: read exported %s: %w", shared.changedPath, err)
	}
	for _, group := range packagesOf(live) {
		if err := baselineGroup(ctx, group, space); err != nil {
			return err
		}
	}
	mutants, err := generateMutants(source, shared.maxMutants, shared.lines)
	for _, entry := range liveClaims(claims) {
		entry.plan(mutants, err)
	}
	return runMutants(ctx, claims, space, source, changedFile, mutants)
}

// prepare decides what Judge decides before any run: the files must sit in
// the export's module and the test file must declare tests.
func (entry *claim) prepare(space workspace) {
	if blocked, unsupported := inspectExport(space.export, entry.configuration); blocked {
		entry.finish(unsupported)
		return
	}
	names, err := testFunctionNames(filepath.Join(space.export, filepath.FromSlash(entry.configuration.testPath)))
	if err != nil {
		entry.finish(unsupportedReport("test file does not parse: " + entry.configuration.testPath))
		return
	}
	if len(names) == 0 {
		entry.finish(unsupportedReport("test file declares no tests: " + entry.configuration.testPath))
		return
	}
	entry.names = names
}

// plan seats the shared mutant list on a claim that passed its baseline.
func (entry *claim) plan(mutants []mutant, parseErr error) {
	if parseErr != nil {
		entry.finish(unsupportedReport("changed file does not parse: " + entry.configuration.changedPath))
		return
	}
	if len(mutants) == 0 {
		entry.finish(Report{Verdict: NoMutants, Operators: []string{}, Detail: noMutantsDetail(entry.configuration)})
		return
	}
	entry.report = Report{Mutants: len(mutants), Operators: operatorNames(mutants)}
}

// baselineGroup runs one package's claims unmutated. A failing function
// leaves its claim UNSUPPORTED as it would alone; a function without an
// outcome earns its claim its own baseline run.
func baselineGroup(ctx context.Context, group packageGroup, space workspace) error {
	if ctx.Err() != nil {
		finishAll(group.claims, budgetReport("budget exhausted before the run started"))
		return nil
	}
	outcome, output, outcomes := runGroup(ctx, group, space)
	if outcome != runPassed && ctx.Err() != nil {
		finishAll(group.claims, budgetReport("budget exhausted during the baseline run"))
		return nil
	}
	if outcome == runBroken {
		return inconclusive(output)
	}
	if outcome == runPassed {
		return nil
	}
	if outcome == runUnbuildable {
		for _, entry := range group.claims {
			entry.finish(unsupportedReport(baselineDetail(output, entry.configuration)))
		}
		return nil
	}
	for _, entry := range group.claims {
		result := entry.outcome(outcomes)
		if result == runPassed {
			continue
		}
		if result == runFailed {
			entry.finish(unsupportedReport(baselineDetail(output, entry.configuration)))
			continue
		}
		if err := baselineAlone(ctx, entry, space); err != nil {
			return err
		}
	}
	return nil
}

func baselineAlone(ctx context.Context, entry *claim, space workspace) error {
	outcome, output := runClaim(ctx, entry, space)
	if outcome != runPassed && ctx.Err() != nil {
		entry.finish(budgetReport("budget exhausted during the baseline run"))
		return nil
	}
	if outcome == runBroken {
		return inconclusive(output)
	}
	if outcome != runPassed {
		entry.finish(unsupportedReport(baselineDetail(output, entry.configuration)))
	}
	return nil
}

// runMutants writes each mutant once and runs every package that still has an
// undecided claim against it, restoring the changed file afterwards.
func runMutants(ctx context.Context, claims []*claim, space workspace, source []byte, changedFile string, mutants []mutant) error {
	defer func() { _ = os.WriteFile(changedFile, source, 0o644) }()
	for _, candidate := range mutants {
		live := liveClaims(claims)
		if len(live) == 0 {
			return nil
		}
		if ctx.Err() != nil {
			partialAll(live)
			return nil
		}
		if err := os.WriteFile(changedFile, candidate.Source, 0o644); err != nil {
			return fmt.Errorf("mutate: write mutant: %w", err)
		}
		err := judgeMutantAcrossPackages(ctx, live, space, candidate)
		if restoreErr := os.WriteFile(changedFile, source, 0o644); restoreErr != nil {
			return fmt.Errorf("mutate: restore %s: %w", live[0].configuration.changedPath, restoreErr)
		}
		if err != nil {
			return err
		}
	}
	for _, entry := range liveClaims(claims) {
		entry.finish(finishedReport(entry.report, entry.configuration, ""))
	}
	return nil
}

func judgeMutantAcrossPackages(ctx context.Context, live []*claim, space workspace, candidate mutant) error {
	for _, group := range packagesOf(live) {
		group = withinBudget(group)
		if len(group.claims) == 0 {
			continue
		}
		if ctx.Err() != nil {
			partialAll(liveClaims(live))
			return nil
		}
		outcome, output, outcomes := runGroup(ctx, group, space)
		if outcome != runPassed && ctx.Err() != nil {
			partialAll(liveClaims(live))
			return nil
		}
		if outcome == runBroken {
			return inconclusive(output)
		}
		if err := creditGroup(ctx, group, space, outcome, outcomes, candidate); err != nil {
			return err
		}
	}
	return nil
}

// creditGroup credits one package run to each of its claims from that claim's
// own functions, re-running alone any claim the run left unattributed.
func creditGroup(ctx context.Context, group packageGroup, space workspace, outcome runOutcome, outcomes map[string]runOutcome, candidate mutant) error {
	if outcome != runFailed {
		for _, entry := range group.claims {
			entry.credit(outcome, candidate, "")
		}
		return nil
	}
	for _, entry := range group.claims {
		result, output := entry.outcome(outcomes), ""
		killingTest := firstFailedTest(entry.names, outcomes)
		if result == runUnattributed {
			result, output = judgeAlone(ctx, entry, space)
			killingTest = ""
		}
		if result == runUnattributed {
			entry.finish(budgetPartial(entry.report, entry.seen))
			continue
		}
		if result == runBroken {
			return inconclusive(output)
		}
		entry.credit(result, candidate, killingTest)
	}
	return nil
}

// judgeAlone runs one claim's own pattern against the current mutant; a run
// the budget cut stays unattributed.
func judgeAlone(ctx context.Context, entry *claim, space workspace) (runOutcome, string) {
	outcome, output := runClaim(ctx, entry, space)
	if outcome != runPassed && ctx.Err() != nil {
		return runUnattributed, output
	}
	return outcome, output
}

// outcome folds the package run's per-function results into the claim's:
// any failure kills, every pass survives, a missing function is unattributed.
func (entry *claim) outcome(outcomes map[string]runOutcome) runOutcome {
	folded := runPassed
	for _, name := range entry.names {
		result, known := outcomes[name]
		if !known {
			return runUnattributed
		}
		if result == runFailed {
			folded = runFailed
		}
	}
	return folded
}

// credit records one mutant's outcome exactly as judgeMutants does, deciding
// the claim at its first kill.
func (entry *claim) credit(outcome runOutcome, candidate mutant, killingTest string) {
	entry.seen++
	if outcome == runPassed {
		entry.report.Survived++
		entry.report.Survivors = append(entry.report.Survivors, survivor(candidate))
		return
	}
	if outcome == runUnbuildable {
		entry.report.Uncompilable++
		return
	}
	entry.report.Killed++
	entry.report.Skipped = entry.report.Mutants - entry.seen
	if killingTest != "" {
		entry.report.Witness = mutationWitness(candidate, killingTest)
	}
	firstKill := fmt.Sprintf("%s at %s:%d", candidate.Operator, entry.configuration.changedPath, candidate.Line)
	entry.finish(finishedReport(entry.report, entry.configuration, firstKill))
}

func (entry *claim) finish(report Report) {
	report.Elapsed = entry.charged
	entry.report = report
	entry.done = true
}

// runGroup runs the union of the group's functions in one go test, charges
// every claim the run's wall time, and returns each function's own result.
func runGroup(ctx context.Context, group packageGroup, space workspace) (runOutcome, string, map[string]runOutcome) {
	started := time.Now()
	outcome, output, outcomes := runGoTest(ctx, groupTimeout(group), space, group.dir, runPattern(groupNames(group)), true)
	charge(group.claims, time.Since(started))
	return outcome, output, outcomes
}

// runClaim runs one claim's own pattern, as Judge would, charging it alone.
func runClaim(ctx context.Context, entry *claim, space workspace) (runOutcome, string) {
	started := time.Now()
	outcome, output := runTests(ctx, entry.configuration, space, runPattern(entry.names))
	charge([]*claim{entry}, time.Since(started))
	return outcome, output
}

func groupTimeout(group packageGroup) time.Duration {
	shared := group.claims[0].configuration
	timeout := shared.perRun * time.Duration(len(group.claims))
	if timeout > shared.budget {
		return shared.budget
	}
	return timeout
}

func groupNames(group packageGroup) []string {
	seen := map[string]struct{}{}
	names := make([]string, 0, len(group.claims))
	for _, entry := range group.claims {
		for _, name := range entry.names {
			if _, known := seen[name]; known {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

func charge(claims []*claim, elapsed time.Duration) {
	for _, entry := range claims {
		entry.charged += elapsed
	}
}

// withinBudget drops from the group every claim whose charge has reached its
// row budget, deciding it as the row's own clock would have.
func withinBudget(group packageGroup) packageGroup {
	kept := make([]*claim, 0, len(group.claims))
	for _, entry := range group.claims {
		if entry.charged >= entry.configuration.budget {
			entry.finish(budgetPartial(entry.report, entry.seen))
			continue
		}
		kept = append(kept, entry)
	}
	return packageGroup{dir: group.dir, claims: kept}
}

// packagesOf groups live claims by test package in first-appearance order.
func packagesOf(claims []*claim) []packageGroup {
	groups := make([]packageGroup, 0, len(claims))
	byDir := map[string]int{}
	for _, entry := range claims {
		dir := entry.configuration.packageDir
		index, known := byDir[dir]
		if !known {
			index = len(groups)
			byDir[dir] = index
			groups = append(groups, packageGroup{dir: dir})
		}
		groups[index].claims = append(groups[index].claims, entry)
	}
	return groups
}

func liveClaims(claims []*claim) []*claim {
	live := make([]*claim, 0, len(claims))
	for _, entry := range claims {
		if !entry.done {
			live = append(live, entry)
		}
	}
	return live
}

func finishAll(claims []*claim, report Report) {
	for _, entry := range claims {
		entry.finish(report)
	}
}

func partialAll(claims []*claim) {
	for _, entry := range claims {
		entry.finish(budgetPartial(entry.report, entry.seen))
	}
}
