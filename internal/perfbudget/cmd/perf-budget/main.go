// Command perf-budget measures the read budgets of performance-profiling-and-read-budget-v0.md
// (PRB-V0-020..036) over a generated realistic-size fixture: `make perf-budget`.
//
//	perf-budget [--budgets FILE] [--scale full|smoke] [--fixture DIR] [--corvint BIN]
//	            [--corpus-mcp BIN] [--out FILE] [--record-baseline] [--require-judgement]
//
// It prints the report JSON to stdout and exits 0 when every enforced limit holds (pending
// targets are reported, never failing), 1 when an enforced limit is exceeded or unobserved or a
// scenario failed, and 2 when the harness itself cannot run. When the host-contention record cannot
// support a latency or memory verdict the run is NOT_JUDGED (PRB-V0-039): status 0 with a
// stderr line, or 3 under --require-judgement.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/perfbudget"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("perf-budget", flag.ContinueOnError)
	flags.SetOutput(stderr)
	budgetsPath := flags.String("budgets", "internal/perfbudget/budgets.json", "committed budgets file")
	scaleName := flags.String("scale", "", "fixture scale: full or smoke (default: the budgets file's)")
	fixtureDir := flags.String("fixture", "", "fixture directory, reused when it holds fixture.json (default: a removed temporary directory)")
	corvint := flags.String("corvint", "", "corvint binary (default: built from ./cmd/corvint)")
	corpusMCP := flags.String("corpus-mcp", "", "corvint-corpus-mcp binary (default: built from ./cmd/corvint-corpus-mcp)")
	out := flags.String("out", "", "also write the report to FILE")
	record := flags.Bool("record-baseline", false, "write this run's measurements and host record as the budgets file's baseline")
	requireJudgement := flags.Bool("require-judgement", false, "exit 3 instead of 0 when the run is NOT_JUDGED")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "perf-budget:", err)
		return 2
	}
	budgets, err := perfbudget.LoadBudgets(*budgetsPath)
	if err != nil {
		return fail(err)
	}
	if *scaleName == "" {
		*scaleName = budgets.Scale
	}
	scale := perfbudget.Full
	switch *scaleName {
	case perfbudget.Full.Name:
	case perfbudget.Smoke.Name:
		scale = perfbudget.Smoke
	default:
		return fail(fmt.Errorf("unknown scale %q", *scaleName))
	}
	// Ceilings are derived at one scale; judging or baselining another scale against them would
	// report a meaningless verdict.
	if scale.Name != budgets.Scale {
		return fail(fmt.Errorf("budgets %s bound scale %q, not %q; use a budgets file of that scale", *budgetsPath, budgets.Scale, scale.Name))
	}
	work, err := os.MkdirTemp("", "corvint-perf-budget-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(work)
	if *fixtureDir == "" {
		*fixtureDir = filepath.Join(work, "fixture")
	} else if *fixtureDir, err = filepath.Abs(*fixtureDir); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(*fixtureDir, 0o700); err != nil {
		return fail(err)
	}
	progress := func(line string) { fmt.Fprintln(stderr, "perf-budget:", line) }
	for _, b := range []struct {
		path *string
		pkg  string
	}{{corvint, "./cmd/corvint"}, {corpusMCP, "./cmd/corvint-corpus-mcp"}} {
		if *b.path != "" {
			if *b.path, err = filepath.Abs(*b.path); err != nil {
				return fail(err)
			}
			continue
		}
		*b.path = filepath.Join(work, "bin", filepath.Base(b.pkg))
		progress("build: " + b.pkg)
		build := exec.CommandContext(ctx, "go", "build", "-o", *b.path, b.pkg)
		build.Stdout, build.Stderr = stderr, stderr
		// Owned: an interrupt retires the compiler and linker descendants too, before the
		// deferred removal of the scratch directory they write into (PRB-V0-037).
		if err := perfbudget.RunOwned(build); err != nil {
			return fail(fmt.Errorf("go build %s: %w", b.pkg, err))
		}
	}
	revision := "NOT_OBSERVED"
	if head, err := ownedOutput(ctx, "git", "rev-parse", "HEAD"); err == nil {
		revision = strings.TrimSpace(string(head))
		if status, err := ownedOutput(ctx, "git", "status", "--porcelain", "--untracked-files=no"); err == nil && len(status) > 0 {
			revision += "+dirty"
		}
	}
	report, err := perfbudget.Run(ctx, perfbudget.Options{Budgets: budgets, Corvint: *corvint, CorpusMCP: *corpusMCP, FixtureDir: *fixtureDir, Scale: scale, Revision: revision, Progress: progress})
	if err != nil {
		return fail(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fail(err)
	}
	data = append(data, '\n')
	if _, err := stdout.Write(data); err != nil {
		return fail(err)
	}
	if *out != "" {
		if err := os.WriteFile(*out, data, 0o600); err != nil {
			return fail(err)
		}
	}
	if *record {
		if err := recordBaseline(*budgetsPath, budgets, report); err != nil {
			return fail(err)
		}
		progress("baseline recorded in " + *budgetsPath)
	}
	for _, c := range report.Checks {
		progress(checkLine(c))
	}
	if report.Verdict == perfbudget.RunNotJudged {
		progress("NOT_JUDGED: " + report.NotJudgedReason)
	}
	return verdictStatus(report.Verdict, *requireJudgement)
}

// checkLine is one check's progress line. A pending or advisory target names its profile and that
// it is not enforced, so its line never reads as a passing limit (PRB-V0-056).
func checkLine(c perfbudget.Check) string {
	line := fmt.Sprintf("%s %s %s: %s", c.Scenario, c.Kind, c.Limit.Metric, c.Verdict)
	switch c.Limit.Status {
	case perfbudget.StatusPending:
		line += fmt.Sprintf(" (%s target, not enforced: pending on %s)", c.Limit.Profile, c.Limit.PendingOn)
	case perfbudget.StatusAdvisory:
		line += fmt.Sprintf(" (%s target, not enforced: advisory)", c.Limit.Profile)
	}
	return line
}

// verdictStatus is the exit status of a run verdict (PRB-V0-041): 0 PASS, 1 FAIL, and for
// NOT_JUDGED 0, or 3 under --require-judgement. NOT_JUDGED is never reported as PASS: the
// report's verdict and stderr say which it was.
func verdictStatus(verdict string, requireJudgement bool) int {
	switch verdict {
	case perfbudget.RunPass:
		return 0
	case perfbudget.RunNotJudged:
		if requireJudgement {
			return 3
		}
		return 0
	}
	return 1
}

// recordBaseline writes report as the budgets file's baseline. It refuses, leaving the file
// untouched, when a scenario measured nothing (PRB-V0-029) or the run could not be judged
// (PRB-V0-043), including a FAIL that kept a contention reason: a contended or unobserved host
// would become every later run's ceiling.
func recordBaseline(path string, budgets *perfbudget.Budgets, report *perfbudget.Report) error {
	for _, m := range report.Scenarios {
		if m.Error != "" || len(m.Samples) == 0 {
			return fmt.Errorf("baseline not recorded: scenario %s measured nothing (%s)", m.ID, m.Error)
		}
	}
	if report.Verdict == perfbudget.RunNotJudged || report.NotJudgedReason != "" {
		return fmt.Errorf("baseline not recorded: run %s on an unjudged host: %s", report.Verdict, report.NotJudgedReason)
	}
	recorded := *budgets
	recorded.Baseline = &perfbudget.Baseline{RecordedAt: report.StartedAt, Revision: report.Revision, Host: report.Host, Fixture: report.Fixture, Scenarios: report.Scenarios}
	encoded, err := json.MarshalIndent(&recorded, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// ownedOutput is exec's Output for name and args under perfbudget.RunOwned.
func ownedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	err := perfbudget.RunOwned(cmd)
	return stdout.Bytes(), err
}
