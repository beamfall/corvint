// Command perf-budget measures the read budgets of performance-profiling-and-read-budget-v0.md
// (PRB-V0-020..036) over a generated realistic-size fixture: `make perf-budget`.
//
//	perf-budget [--budgets FILE] [--scale full|smoke] [--fixture DIR] [--corvint BIN]
//	            [--corpus-mcp BIN] [--out FILE] [--scenario ID]... [--record-baseline]
//	            [--require-judgement]
//	perf-budget [--budgets FILE] --record-baseline-from REPORT
//
// --scenario measures and judges only the named scenarios; with --record-baseline it replaces
// only their baseline measurements, each with its run's provenance (PRB-V0-970).
// --record-baseline-from records the baseline from a report another run wrote with --out, the
// path when the local host is contended: run with --out on a quiet host, copy the report back,
// and record it here (PRB-V0-971).
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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/jsonend"
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
	var selected []string
	flags.Func("scenario", "measure and judge only scenario ID (repeatable; PRB-V0-970)", func(id string) error {
		if !slices.Contains(perfbudget.ScenarioIDs, id) {
			return fmt.Errorf("unknown scenario %q, want one of %v", id, perfbudget.ScenarioIDs)
		}
		if slices.Contains(selected, id) {
			return fmt.Errorf("scenario %q named twice", id)
		}
		selected = append(selected, id)
		return nil
	})
	recordFrom := flags.String("record-baseline-from", "", "record the baseline from REPORT, a report another run wrote with --out (PRB-V0-971)")
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
	if *recordFrom != "" {
		if *record || len(selected) != 0 {
			return fail(errors.New("--record-baseline-from records a finished report; it takes no --record-baseline or --scenario"))
		}
		report, err := readReport(*recordFrom)
		if err != nil {
			return fail(err)
		}
		if report.Fixture == nil || report.Fixture.Scale.Name != budgets.Scale {
			return fail(fmt.Errorf("baseline not recorded: report %s is not a run at the budgets scale %q", *recordFrom, budgets.Scale))
		}
		if err := recordBaseline(*budgetsPath, budgets, report); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stderr, "perf-budget: baseline recorded in "+*budgetsPath+" from "+*recordFrom)
		return 0
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
	report, err := perfbudget.Run(ctx, perfbudget.Options{Budgets: budgets, Corvint: *corvint, CorpusMCP: *corpusMCP, FixtureDir: *fixtureDir, Scale: scale, Revision: revision, Progress: progress, Selected: selected})
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
	if len(report.Selected) == 0 {
		recorded.Baseline = &perfbudget.Baseline{RecordedAt: report.StartedAt, Revision: report.Revision, Host: report.Host, Fixture: report.Fixture, Scenarios: report.Scenarios}
	} else {
		merged, err := mergeBaseline(budgets.Baseline, report)
		if err != nil {
			return err
		}
		recorded.Baseline = merged
		// The provenance fields are checked where the budgets file is loaded (PRB-V0-053).
		if err := recorded.Validate(); err != nil {
			return fmt.Errorf("baseline not recorded: %w", err)
		}
	}
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

// mergeBaseline replaces the selected scenarios' measurements of a filtered run in a copy of
// baseline, each attributed to its run by PRB-V0-053 provenance; the other measurements, and the
// baseline's own revision, host and fixture, are kept (PRB-V0-970).
func mergeBaseline(baseline *perfbudget.Baseline, report *perfbudget.Report) (*perfbudget.Baseline, error) {
	if baseline == nil {
		return nil, errors.New("baseline not recorded: a --scenario run replaces measurements of an existing baseline, and the budgets file has none; record a whole run first")
	}
	if report.Fixture == nil {
		return nil, errors.New("baseline not recorded: the run names no fixture, so its executable is unknown")
	}
	merged := *baseline
	merged.Scenarios = slices.Clone(baseline.Scenarios)
	for _, m := range report.Scenarios {
		m.Provenance = &perfbudget.Provenance{RecordedAt: report.StartedAt, ExecutableRevision: report.Revision, ExecutableSHA256: report.Fixture.BuilderSHA256, Host: report.Host}
		index := slices.IndexFunc(merged.Scenarios, func(b perfbudget.ScenarioMeasured) bool { return b.ID == m.ID })
		if index < 0 {
			return nil, fmt.Errorf("baseline not recorded: the baseline has no scenario %s to replace", m.ID)
		}
		merged.Scenarios[index] = m
	}
	return &merged, nil
}

// readReport decodes one report a run wrote with --out, refusing unknown members and any content
// after the one JSON value (PRB-V0-971).
func readReport(path string) (*perfbudget.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report perfbudget.Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil || !jsonend.Done(decoder) {
		return nil, fmt.Errorf("report %s is not one perf-budget report", path)
	}
	if report.Schema != perfbudget.ReportSchema {
		return nil, fmt.Errorf("report %s has schema %q, want %q", path, report.Schema, perfbudget.ReportSchema)
	}
	if err := recordableReport(&report); err != nil {
		return nil, fmt.Errorf("baseline not recorded: report %s %w", path, err)
	}
	return &report, nil
}

// recordableReport refuses a report another run wrote unless it carries everything a baseline
// attributes and judges by: a clean revision and start time, the fixture and the executable that
// built it, a judged uncontended host, a PASS or FAIL verdict, and exactly the ScenarioIDs (or,
// for a filtered run, exactly its distinct selected ones in report order), each measured
// (PRB-V0-971).
func recordableReport(report *perfbudget.Report) error {
	switch {
	case !lowerHex(report.Revision, 40):
		return fmt.Errorf("names no clean 40-hex revision (%q)", report.Revision)
	case !rfc3339(report.StartedAt):
		return fmt.Errorf("names no RFC 3339 start time (%q)", report.StartedAt)
	case report.Fixture == nil || !lowerHex(report.Fixture.BuilderSHA256, 64):
		return errors.New("names no fixture built by a known executable SHA-256")
	case report.Host.Cores < 1 || report.Host.Contended == nil || report.Host.NotJudgedReason() != "":
		return fmt.Errorf("was not run on a judged, uncontended host (%s)", report.Host.NotJudgedReason())
	case report.Verdict != perfbudget.RunPass && report.Verdict != perfbudget.RunFail || report.NotJudgedReason != "":
		return fmt.Errorf("has verdict %q, not a judged PASS or FAIL", report.Verdict)
	}
	want := perfbudget.ScenarioIDs
	if len(report.Selected) != 0 {
		want = nil
		for _, id := range perfbudget.ScenarioIDs {
			if slices.Contains(report.Selected, id) {
				want = append(want, id)
			}
		}
		if len(want) != len(report.Selected) {
			return fmt.Errorf("selects unknown or repeated scenarios %v", report.Selected)
		}
	}
	got := make([]string, 0, len(report.Scenarios))
	for _, m := range report.Scenarios {
		got = append(got, m.ID)
		if m.Error != "" || len(m.Samples) == 0 || m.P50WallMS == nil || m.P95WallMS == nil {
			return fmt.Errorf("did not measure scenario %s", m.ID)
		}
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("measured scenarios %v, want exactly %v", got, want)
	}
	return nil
}

func lowerHex(s string, length int) bool {
	if len(s) != length {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func rfc3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
