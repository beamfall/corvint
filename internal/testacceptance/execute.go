package testacceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// SafeEnvironment is a fixed local execution environment. Author/request keys
// cannot opt credentials into it. Browser caches use the trusted host default.
func SafeEnvironment() []string {
	return []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TMPDIR=/tmp"}
}

// WorkerEnvironmentSafe rejects a public direct worker invocation before any
// author/server code can observe inherited credentials or runtime injection.
func WorkerEnvironmentSafe() bool {
	allowed := map[string]string{}
	for _, entry := range SafeEnvironment() {
		parts := strings.SplitN(entry, "=", 2)
		allowed[parts[0]] = parts[1]
	}
	if len(os.Environ()) != len(allowed) {
		return false
	}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || allowed[parts[0]] != parts[1] {
			return false
		}
		delete(allowed, parts[0])
	}
	return len(allowed) == 0
}
func files(r Request) []string {
	var out []string
	for _, t := range r.Tests {
		if !slices.Contains(out, t.File) {
			out = append(out, t.File)
		}
	}
	return out
}
func cleanup(o procgroup.Observation) Cleanup {
	return Cleanup{OwnedGroup: o.OwnedProcessGroupCleanup, Status: o.DescendantCleanupStatus, Qualification: o.DescendantCleanupQualification, Descendants: o.DescendantObservation, Cancelled: o.Cancelled, TimedOut: o.TimedOut}
}
func cleanupState(c Cleanup) string {
	if c.Descendants != nil && !c.Descendants.Absent && len(c.Descendants.Failures) == 0 {
		return "survivors"
	}
	if !c.OwnedGroup || c.Descendants == nil || !c.Descendants.Absent || len(c.Descendants.Failures) > 0 {
		return "unknown"
	}
	return "observed-absent"
}

// RunWorker is invoked only in a clean self-worker. It never trusts supplied
// outcomes; each result comes from an actual provider/executor invocation.
func RunWorker(ctx context.Context, j Job) WorkerResult {
	if !WorkerEnvironmentSafe() {
		return WorkerResult{Error: "worker-environment-unsafe"}
	}
	if e := approval(j.Request, j.Approved); e != nil {
		return WorkerResult{Error: e.Error()}
	}
	r := j.Request
	if j.Kind == "control" {
		if j.TestIndex < 0 || j.TestIndex >= len(r.Tests) {
			return WorkerResult{Error: "control-index-invalid"}
		}
		t := r.Tests[j.TestIndex]
		c := Control{ID: t.ID, Status: "missing", Unsupported: []string{"baseline-qualified-test-id", "provider-per-test-freshness"}}
		if t.Control == nil {
			return WorkerResult{Control: &c}
		}
		c.PlanDigest = t.Control.Digest
		input, _ := json.Marshal(t.Control)
		observation := procgroup.Run(ctx, procgroup.Spec{Argv: []string{t.ControlCommand.Argv[0], "execute-receipt", "--experimental", "--approve-plan", t.Control.Digest}, Dir: r.TestRepository.Root, Env: SafeEnvironment(), Stdin: input, InputLimit: InputLimit, OutputLimit: ReportLimit, Timeout: 125 * time.Second, ObserveDescendants: true})
		var receipt behaviorfalsify.EvidenceReceipt
		if observation.Err != nil || observation.ExitStatus != 0 || observation.Cancelled || observation.TimedOut || observation.OutputOverflow || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup {
			c.Status = "invalid"
			c.Unsupported = append(c.Unsupported, "control-process-incomplete")
			return WorkerResult{Control: &c}
		}
		if behaviorfalsify.Decode(observation.Stdout, &receipt) != nil {
			c.Status = "invalid"
			c.Unsupported = append(c.Unsupported, "control-output-invalid")
			return WorkerResult{Control: &c}
		}

		c.ReceiptDigest = receipt.Digest
		c.Unsupported = append(c.Unsupported, receipt.Unsupported...)
		if receipt.Tool != *t.Tool {
			c.Status = "invalid"
			c.Unsupported = append(c.Unsupported, "control-tool-identity-mismatch")
			return WorkerResult{Control: &c}
		}
		if behaviorfalsify.VerifyReceipt(receipt, t.Control.Digest, t.Tool.Executable) != nil {
			c.Unsupported = append(c.Unsupported, "control-verification-failed")
			c.Status = "invalid"
			return WorkerResult{Control: &c}
		}
		c.Status = "blocked"
		if receipt.Report.Counts[behaviorfalsify.StatusSurvived] > 0 {
			c.Status = "survived"
		} else if receipt.Report.Executed > 0 && receipt.Report.MutationScore.Defined && receipt.Report.MutationScore.Killed == receipt.Report.MutationScore.Denominator && receipt.Report.Executed == receipt.Report.Requested && len(receipt.Report.CoverageGaps) == 0 {
			c.Status = "killed"
		}
		if e := Validate(r); e != nil {
			return WorkerResult{Error: e.Error()}
		}
		return WorkerResult{Control: &c}
	}
	if j.Kind != "repeat" && j.Kind != "probe-original" && j.Kind != "probe-reversed" && j.Kind != "probe-isolated" {
		return WorkerResult{Error: "worker-kind-invalid"}
	}
	expected := files(r)
	target := ""
	if j.Kind == "probe-isolated" {
		if j.TestIndex < 0 || j.TestIndex >= len(r.Tests) {
			return WorkerResult{Error: "isolation-index-invalid"}
		}
		target = r.Tests[j.TestIndex].ID
		expected = []string{r.Tests[j.TestIndex].File}
	}
	if len(j.Files) != len(expected) {
		return WorkerResult{Error: "worker-files-invalid"}
	}
	seenFiles := map[string]bool{}
	for _, f := range j.Files {
		if seenFiles[f] || !slices.Contains(expected, f) {
			return WorkerResult{Error: "worker-files-invalid"}
		}
		seenFiles[f] = true
	}
	out, e := os.MkdirTemp("", "corvint-new-test-output-")
	if e != nil {
		return WorkerResult{Error: "output-allocation-failed"}
	}
	defer os.RemoveAll(out)
	argv := append([]string{}, r.Runner.Argv...)
	argv = append(argv, "test", "--config", r.Config, "--reporter=json", "--retries=0", "--workers=1", "--forbid-only", "--trace=off", "--output", out)
	if target != "" {
		argv = append(argv, "--grep="+isolationPattern(r.Tests[j.TestIndex].Title))
	}
	for _, f := range j.Files {
		argv = append(argv, regexp.QuoteMeta(f))
	}
	receipt, e := jstestprovider.RunE2E(ctx, jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: r.TestRepository.Root, TestFiles: files(r), ConfigFile: r.Config, PackageJSON: r.Package, Lockfile: r.Lockfile, RunnerName: "playwright", RunnerVersion: r.RunnerVersion, Timeout: time.Duration(r.TimeoutSeconds) * time.Second, OutputLimit: 4 << 20}, ServerArgv: r.Server.Argv, ServerReadyURL: r.ReadyURL, ServerReadyLimit: 5 * time.Second, TestArgv: argv, AppBuildDir: r.AppBuildDir})
	if e != nil {
		return WorkerResult{Error: "provider-execution-error"}
	}
	b, _ := json.Marshal(receipt)
	run := Run{Kind: j.Kind, TestID: target, RequestedFiles: j.Files, ReceiptSHA256: Hash(b), ObservedSchedule: receipt.Schedule, Rows: []Row{}, Reasons: []string{}, NodeVersion: receipt.Identity.NodeVersion}
	if receipt.Infrastructure != nil {
		run.Reasons = append(run.Reasons, "provider-infrastructure")
	}
	if receipt.Cancelled {
		run.Reasons = append(run.Reasons, "provider-cancelled")
	}
	if receipt.StaleAppBuild || receipt.AppBuildAtStart.Unknown || receipt.AppBuildAtPublish.Unknown {
		run.Reasons = append(run.Reasons, "provider-build-unknown-or-stale")
	}
	if receipt.ServerDescendantsGone == nil || !*receipt.ServerDescendantsGone {
		run.Reasons = append(run.Reasons, "provider-server-cleanup-unknown")
	}
	seen := map[string]bool{}
	for _, observed := range receipt.Tests {
		matches := []Test{}
		anchorFile := ""
		if observed.Anchor != nil {
			anchorFile = observed.Anchor.File
			if !filepath.IsAbs(anchorFile) {
				anchorFile = filepath.Join(r.TestRepository.Root, anchorFile)
			}
		}
		for _, t := range r.Tests {
			if observed.Name == t.Title && observed.Anchor != nil && filepath.Clean(anchorFile) == t.File {
				matches = append(matches, t)
			}
		}
		if len(matches) != 1 {
			run.Reasons = append(run.Reasons, "extra-or-unmatched-test")
			continue
		}
		t := matches[0]
		if target != "" && t.ID != target {
			run.Reasons = append(run.Reasons, "isolation-not-observed")
			continue
		}
		if seen[t.ID] {
			run.Reasons = append(run.Reasons, "duplicate-test")
		}
		seen[t.ID] = true
		row := Row{ID: t.ID, ObservedID: observed.ID, State: observed.State, DurationMS: observed.DurationMS, Retries: observed.Retries, Attempts: len(observed.AttemptDetails), Validity: jstestprovider.ReceiptTestProjection(receipt, observed), IdentityUnknown: []string{}}
		if observed.ID == "" || observed.ID != t.ID {
			row.IdentityUnknown = append(row.IdentityUnknown, "test-id")
		}
		if observed.Project == nil || observed.Project.Name != t.Project {
			row.IdentityUnknown = append(row.IdentityUnknown, "project-browser")
		}
		if observed.Anchor == nil || observed.Anchor.Line != t.Line {
			row.IdentityUnknown = append(row.IdentityUnknown, "declaration-line")
		}
		if row.Attempts != 1 || row.Retries != 0 {
			run.Reasons = append(run.Reasons, "attempt-count-or-retry")
		}
		for _, a := range observed.AttemptDetails {
			if a.Retry != 0 {
				run.Reasons = append(run.Reasons, "observed-retry")
			}
		}
		run.Rows = append(run.Rows, row)
	}
	for _, t := range r.Tests {
		if !seen[t.ID] && (target == "" || t.ID == target) {
			run.Reasons = append(run.Reasons, "missing-test")
		}
	}
	if e := Validate(r); e != nil {
		run.Reasons = append(run.Reasons, "inputs-drifted")
	}
	return WorkerResult{Run: &run}
}

func invoke(ctx context.Context, exe string, j Job) (WorkerResult, Cleanup) {
	b, _ := json.Marshal(j)
	// Controls have their own bounded executor wall-clock limit; the outer bound
	// includes cleanup and cannot be extended by request-controlled argv.
	timeout := time.Duration(j.Request.TimeoutSeconds+15) * time.Second
	if j.Kind == "control" {
		timeout = 135 * time.Second
	}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{exe, "worker"}, Dir: j.Request.TestRepository.Root, Env: SafeEnvironment(), Stdin: b, InputLimit: InputLimit, OutputLimit: ReportLimit, Timeout: timeout, ObserveDescendants: true})
	c := cleanup(o)
	var w WorkerResult
	if o.Err != nil || !o.Started || !o.WaitCompleted || o.ExitStatus != 0 || o.OutputOverflow || o.Cancelled || o.TimedOut {
		w.Error = "worker-incomplete"
		return w, c
	}
	d := json.NewDecoder(bytes.NewReader(o.Stdout))
	d.DisallowUnknownFields()
	if e := d.Decode(&w); e != nil {
		w = WorkerResult{Error: "worker-report-invalid"}
	}
	return w, c
}

// Execute requires the independently reviewed request digest. Its executable
// argument is the trusted companion binary, never a request-selected program.
func Execute(ctx context.Context, r Request, approved, exe, build string) (Report, error) {
	if e := approval(r, approved); e != nil {
		return Report{}, e
	}
	if !cleanAbsolute(exe) || regular(exe) != nil {
		return Report{}, errors.New("self-worker-invalid")
	}
	report := Report{Schema: Schema, RequestDigest: approved, Product: r.Product, TestRepository: r.TestRepository, Environment: r.Environment, Build: build, Runs: []Run{}, Controls: []Control{}, Unknowns: []string{"provider-per-test-freshness", "qualified-baseline-test-and-browser-identity", "authenticated-operator-and-hook-semantics", "full-descendant-containment", "observed-order-with-owned-provider", "runner-and-hook-import-closure-unqualified", "runner-version-claim-only"}}
	executableBytes, e := os.ReadFile(exe)
	if e != nil {
		return Report{}, errors.New("self-worker-unreadable")
	}
	report.ExecutableSHA256 = Hash(executableBytes)
	order := files(r)
	for i := 0; i < r.Repeat; i++ {
		w, c := invoke(ctx, exe, Job{Request: r, Approved: approved, Kind: "repeat", Files: order})
		run := Run{Kind: "repeat", Ordinal: i + 1, RequestedFiles: order, Reasons: []string{}, Rows: []Row{}}
		if w.Run != nil {
			run = *w.Run
			run.Ordinal = i + 1
		}
		run.Cleanup = c
		if w.Error != "" {
			run.Reasons = append(run.Reasons, w.Error)
		}
		report.Runs = append(report.Runs, run)
		if ctx.Err() != nil {
			break
		}
	}
	mixed := false
	for _, t := range r.Tests {
		mixed = mixed || mixedOutcomes(report.Runs, t.ID)
	}
	if mixed && ctx.Err() == nil {
		for i, kind := range []string{"probe-original", "probe-reversed"} {
			selected := slices.Clone(order)
			if i == 1 {
				slices.Reverse(selected)
			}
			w, c := invoke(ctx, exe, Job{Request: r, Approved: approved, Kind: kind, Files: selected})
			run := Run{Kind: kind, RequestedFiles: selected, Rows: []Row{}, Reasons: []string{}}
			if w.Run != nil {
				run = *w.Run
			}
			run.Cleanup = c
			if w.Error != "" {
				run.Reasons = append(run.Reasons, w.Error)
			}
			report.Runs = append(report.Runs, run)
		}
	}
	// A disagreeing test is also run alone, so order dependence can be told
	// apart from nondeterminism. Isolation is requested, never an observed schedule.
	for i, t := range r.Tests {
		if !mixedOutcomes(report.Runs, t.ID) {
			continue
		}
		for n := 1; n <= IsolationRepeats && ctx.Err() == nil; n++ {
			w, c := invoke(ctx, exe, Job{Request: r, Approved: approved, Kind: "probe-isolated", Files: []string{t.File}, TestIndex: i})
			run := Run{Kind: "probe-isolated", RequestedFiles: []string{t.File}, Rows: []Row{}, Reasons: []string{}}
			if w.Run != nil {
				run = *w.Run
			}
			run.Kind, run.Ordinal, run.TestID, run.Cleanup = "probe-isolated", n, t.ID, c
			if w.Error != "" {
				run.Reasons = append(run.Reasons, w.Error)
			}
			report.Runs = append(report.Runs, run)
		}
	}
	for i, t := range r.Tests {
		c := Control{ID: t.ID, Status: "not-run", Unsupported: []string{"control-not-run"}}
		if ctx.Err() == nil {
			w, life := invoke(ctx, exe, Job{Request: r, Approved: approved, Kind: "control", TestIndex: i})
			if w.Control != nil {
				c = *w.Control
			}
			c.Cleanup = life
			if w.Error != "" {
				c.Status = "invalid"
			}
		}
		report.Controls = append(report.Controls, c)
	}
	classify(&report, r)
	b, e := json.Marshal(report)
	if e != nil || len(b) > ReportLimit {
		return Report{}, errors.New("report-bound")
	}
	return report, nil
}

// IsolationRepeats is the fixed number of single-test runs for each test whose
// repeats disagree. It is not caller-selectable.
const IsolationRepeats = 2

// mixedOutcomes reports passed and failed repeat rows for one test.
func mixedOutcomes(runs []Run, id string) bool {
	pass, fail := false, false
	for _, run := range runs {
		if run.Kind != "repeat" {
			continue
		}
		for _, row := range run.Rows {
			if row.ID == id {
				pass = pass || row.State == jstestprovider.StatePassed
				fail = fail || row.State == jstestprovider.StateFailed
			}
		}
	}
	return pass && fail
}

// isolationPattern selects one title in Playwright's space-joined grep path,
// allowing trailing tags. Ambiguous matches are retained as isolation-not-observed.
func isolationPattern(title string) string {
	return `(?:^| )` + regexp.QuoteMeta(title) + `(?: @\S+)*$`
}
