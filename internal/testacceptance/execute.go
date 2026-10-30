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

// SafeEnvironment preserves the default four-key execution environment.
func SafeEnvironment() []string {
	return []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TMPDIR=/tmp"}
}

// safeTempRoot admits only existing canonical directories, never ambient aliases.
func safeTempRoot(root string, private bool) bool {
	if !cleanAbsolute(root) || len(root) > 4096 || strings.ContainsAny(root, "\x00\r\n") {
		return false
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return false
	}
	info, err := os.Stat(root)
	return err == nil && info.IsDir() && (!private || info.Mode().Perm() == 0700)
}

func workerEnvironment() []string {
	env := SafeEnvironment()
	env[3] = "TMPDIR=" + os.Getenv("TMPDIR")
	return env
}

// WorkerEnvironmentSafe retains exact fixed keys/values; scoped TMPDIR must be
// a canonical private directory. /tmp is the backward-compatible default.
func WorkerEnvironmentSafe() bool { return WorkerEnvironmentSafeFor(Request{}) }

// WorkerEnvironmentSafeFor also admits exactly the request's declared external_app pairs
// (NEA-V0-013); a request without external_app admits only the fixed keys.
func WorkerEnvironmentSafeFor(r Request) bool {
	root := os.Getenv("TMPDIR")
	if root != "/tmp" && !safeTempRoot(root, true) {
		return false
	}
	allowed := map[string]string{}
	for _, entry := range append(workerEnvironment(), externalEnvironment(r)...) {
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
	if !WorkerEnvironmentSafeFor(j.Request) {
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
		if r.Schema == FreshRequestSchema {
			c.Unsupported = []string{}
		}
		if t.Control == nil {
			return WorkerResult{Control: &c}
		}
		c.PlanDigest = t.Control.Digest
		input, _ := json.Marshal(t.Control)
		observation := procgroup.Run(ctx, procgroup.Spec{Argv: []string{t.ControlCommand.Argv[0], "execute-receipt", "--experimental", "--approve-plan", t.Control.Digest}, Dir: r.TestRepository.Root, Env: workerEnvironment(), Stdin: input, InputLimit: InputLimit, OutputLimit: ReportLimit, Timeout: 125 * time.Second, ObserveDescendants: true})
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
		if r.Schema == FreshRequestSchema {
			c.Evidence = &receipt
		}
		c.NotApplied, c.ApplicationUnobserved = controlApplication(*t.Control, receipt)
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
	var hooks []HookResult
	if r.ExternalApp != nil {
		reset := externalHook(ctx, r, "reset", j.Kind, j.Ordinal)
		hooks = append(hooks, reset)
		if reset.Status != "passed" {
			run := Run{Kind: j.Kind, TestID: target, RequestedFiles: j.Files, Rows: []Row{}, Reasons: []string{"external-reset-failed"}, ExternalHooks: hooks}
			externalRecheck(&run, r)
			return WorkerResult{Run: &run}
		}
		// The reset hook ran with the caller's privileges, so the pins are rechecked before the
		// runner reads any storage-state file (NEA-V0-014).
		drift := Run{Kind: j.Kind, TestID: target, RequestedFiles: j.Files, Rows: []Row{}, Reasons: []string{}, ExternalHooks: hooks}
		if externalRecheck(&drift, r); len(drift.Reasons) != 0 {
			return WorkerResult{Run: &drift}
		}
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
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: r.TestRepository.Root, TestFiles: files(r), ConfigFile: r.Config, PackageJSON: r.Package, Lockfile: r.Lockfile, RunnerName: "playwright", RunnerVersion: r.RunnerVersion, Timeout: time.Duration(r.TimeoutSeconds) * time.Second, OutputLimit: 4 << 20}, ServerArgv: r.Server.Argv, ServerReadyURL: r.ReadyURL, ServerReadyLimit: 5 * time.Second, TestArgv: argv, AppBuildDir: r.AppBuildDir}
	if r.ExternalApp != nil {
		cfg.CallerOwnedServer, cfg.ServerArgv = true, nil
	}
	if r.Schema == FreshRequestSchema {
		cfg.Freshness = &r.Freshness.Provider
		cfg.DeclaredEnvKeys = []string{"PATH", "LANG", "LC_ALL", "TMPDIR"}
		cfg.ApplicationAttestation = &jstestprovider.ApplicationAttestationProvider{Argv: r.Freshness.Attestation.Argv, ConfigFile: r.Freshness.AttestationConfiguration.Path, Timeout: 5 * time.Second}
		cfg.TestArgv = []string{"--retries=0", "--workers=1", "--forbid-only"}
		if os.Getenv("TMPDIR") != "/tmp" {
			cfg.ScopedTempOutput = true
		}
		if target != "" {
			cfg.TestArgv = append(cfg.TestArgv, "--grep="+isolationPattern(r.Tests[j.TestIndex].Title))
		}
		for _, file := range j.Files {
			cfg.TestArgv = append(cfg.TestArgv, regexp.QuoteMeta(file))
		}
	}
	receipt, e := jstestprovider.RunE2E(ctx, cfg)
	if e != nil {
		return WorkerResult{Error: "provider-execution-error"}
	}
	b, _ := json.Marshal(receipt)
	run := Run{Kind: j.Kind, TestID: target, RequestedFiles: j.Files, ReceiptSHA256: Hash(b), ObservedSchedule: receipt.Schedule, Rows: []Row{}, Reasons: []string{}, NodeVersion: receipt.Identity.NodeVersion}
	if r.Schema == FreshRequestSchema {
		native, err := jstestprovider.EncodeFreshness(receipt)
		if err != nil {
			return WorkerResult{Error: "freshness-codec-failed"}
		}
		run.NativeReceipt = native
		run.ReceiptSHA256 = Hash(native)
	}
	if r.ExternalApp != nil {
		health := externalHook(ctx, r, "health", j.Kind, j.Ordinal)
		run.ExternalHooks = append(hooks, health)
		if health.Status != "passed" {
			run.Reasons = append(run.Reasons, "external-health-failed")
		}
	}
	markInfrastructure(&run, receipt.Infrastructure)
	if receipt.Cancelled {
		run.Reasons = append(run.Reasons, "provider-cancelled")
	}
	if r.Schema == RequestSchema && (receipt.StaleAppBuild || receipt.AppBuildAtStart.Unknown || receipt.AppBuildAtPublish.Unknown) {
		run.Reasons = append(run.Reasons, "provider-build-unknown-or-stale")
	}
	if (r.Schema == RequestSchema && r.ExternalApp == nil && (receipt.ServerDescendantsGone == nil || !*receipt.ServerDescendantsGone)) || (r.Schema == FreshRequestSchema && (receipt.Freshness == nil || receipt.Freshness.ServerGone == nil || !*receipt.Freshness.ServerGone)) {
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
		if r.Schema == FreshRequestSchema {
			row.Attempts = len(observed.Attempts)
		}
		if observed.ID == "" || (r.Schema == RequestSchema && observed.ID != t.ID) {
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
	if r.ExternalApp != nil {
		externalRecheck(&run, r)
	} else if e := Validate(r); e != nil {
		run.Reasons = append(run.Reasons, "inputs-drifted")
	}
	return WorkerResult{Run: &run}
}

// hookOrdinal is the run ordinal the health hook reads; it is omitted from jobs without
// external_app, so their worker input keeps its prior bytes.
func hookOrdinal(r Request, n int) int {
	if r.ExternalApp == nil {
		return 0
	}
	return n
}

// externalRecheck revalidates the request after a run; a storage-state file that no longer
// matches its pin adds auth-file-drift beside inputs-drifted (NEA-V0-014).
func externalRecheck(run *Run, r Request) {
	if e := Validate(r); e != nil {
		if e.Error() == "auth-file-drift" {
			run.Reasons = append(run.Reasons, "auth-file-drift")
		}
		run.Reasons = append(run.Reasons, "inputs-drifted")
	}
}

func invoke(ctx context.Context, exe string, j Job) (WorkerResult, Cleanup) {
	return invokeEnvironment(ctx, []string{exe, "worker"}, j, SafeEnvironment())
}

// invokeEnvironment re-enters the companion: worker is its executable followed by the fixed
// arguments that select the worker (CTF-V0-006).
func invokeEnvironment(ctx context.Context, worker []string, j Job, env []string) (WorkerResult, Cleanup) {
	b, _ := json.Marshal(j)
	// Controls have their own bounded executor wall-clock limit; the outer bound
	// includes cleanup and cannot be extended by request-controlled argv.
	timeout := time.Duration(j.Request.TimeoutSeconds+15) * time.Second
	if j.Request.ExternalApp != nil {
		timeout += externalHookBudget * time.Second
	}
	if j.Kind == "control" {
		timeout = 135 * time.Second
	}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: slices.Clone(worker), Dir: j.Request.TestRepository.Root, Env: env, Stdin: b, InputLimit: InputLimit, OutputLimit: ReportLimit, Timeout: timeout, ObserveDescendants: true})
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
	return ExecuteWithTempRoot(ctx, r, approved, exe, build, "")
}

// ExecuteWithTempRoot selects an operator-owned temporary output scope. Empty
// preserves /tmp; ambient parent TMPDIR never silently changes worker policy.
func ExecuteWithTempRoot(ctx context.Context, r Request, approved, exe, build, root string) (Report, error) {
	return ExecuteWithWorker(ctx, r, approved, exe, []string{"worker"}, build, root)
}

// ExecuteWithWorker re-enters exe with reentry, the fixed arguments that select its worker:
// `worker` for corvint-tests-accept, `accept worker` for corvint-test (CTF-V0-006).
func ExecuteWithWorker(ctx context.Context, r Request, approved, exe string, reentry []string, build, root string) (report Report, err error) {
	if e := approval(r, approved); e != nil {
		return Report{}, e
	}
	if !cleanAbsolute(exe) || regular(exe) != nil {
		return Report{}, errors.New("self-worker-invalid")
	}
	env := append(SafeEnvironment(), externalEnvironment(r)...)
	if root != "" {
		if !safeTempRoot(root, false) {
			return Report{}, errors.New("temporary-root-invalid")
		}
		scope, allocationErr := os.MkdirTemp(root, "corvint-acceptance-run-")
		if allocationErr != nil {
			return Report{}, errors.New("temporary-root-allocation-failed")
		}
		// Parent cleanup also retires output left by forcefully stopped workers.
		defer func() {
			if os.RemoveAll(scope) != nil {
				err = errors.New("temporary-root-cleanup-failed")
			}
		}()
		env[3] = "TMPDIR=" + scope
	}
	runWorker := func(job Job) (WorkerResult, Cleanup) {
		return invokeEnvironment(ctx, append([]string{exe}, reentry...), job, env)
	}
	report = Report{Schema: Schema, RequestDigest: approved, Product: r.Product, TestRepository: r.TestRepository, Environment: r.Environment, Build: build, Runs: []Run{}, Controls: []Control{}, Unknowns: []string{"provider-per-test-freshness", "qualified-baseline-test-and-browser-identity", "authenticated-operator-and-hook-semantics", "full-descendant-containment", "observed-order-with-owned-provider", "runner-and-hook-import-closure-unqualified", "runner-version-claim-only"}}
	if r.Schema == FreshRequestSchema {
		report.Schema = FreshSchema
		report.Unknowns = []string{"authenticated-operator-and-hook-semantics", "full-descendant-containment", "general-browser-and-host-qualification", "compiled-source-provenance", "unsupported-core-mcp-flows-consumers"}
	}
	executableBytes, e := os.ReadFile(exe)
	if e != nil {
		return Report{}, errors.New("self-worker-unreadable")
	}
	report.ExecutableSHA256 = Hash(executableBytes)
	order := files(r)
	for i := 0; i < r.Repeat; i++ {
		w, c := runWorker(Job{Request: r, Approved: approved, Kind: "repeat", Files: order, Ordinal: hookOrdinal(r, i+1)})
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
			w, c := runWorker(Job{Request: r, Approved: approved, Kind: kind, Files: selected})
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
			w, c := runWorker(Job{Request: r, Approved: approved, Kind: "probe-isolated", Files: []string{t.File}, TestIndex: i, Ordinal: hookOrdinal(r, n)})
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
			w, life := runWorker(Job{Request: r, Approved: approved, Kind: "control", TestIndex: i})
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
	if r.ExternalApp != nil {
		report.External = externalReport(r)
		report.Unknowns = append(report.Unknowns, "external-server-state-caller-owned", "disposable-account-caller-declared")
	}
	classify(&report, r)
	b, e := json.Marshal(report)
	if e != nil || len(b) > ReportLimit {
		return Report{}, errors.New("report-bound")
	}
	if e := refuseAuthMaterial(r, report); e != nil {
		return Report{}, e
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

// infrastructureCode admits only a closed code-shaped token, so no stderr, path or other provider
// free text reaches the report.
var infrastructureCode = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// markInfrastructure records a provider infrastructure failure on run: the provider-infrastructure
// reason and, with it, the named cause (NEA-V0-011). A nil failure leaves run unchanged.
func markInfrastructure(run *Run, f *jstestprovider.InfrastructureFailure) {
	if f == nil {
		return
	}
	run.Reasons = append(run.Reasons, "provider-infrastructure")
	run.Infrastructure = infrastructureCause(f)
}

// infrastructureCause names which provider infrastructure failure a run hit (NEA-V0-011), for
// example report-identity-unknown:project-location-unknown when the runner's qualified browser
// is not installed. A reason that is not a code reads "unrecognized".
func infrastructureCause(f *jstestprovider.InfrastructureFailure) string {
	if !infrastructureCode.MatchString(f.Reason) {
		return "unrecognized"
	}
	if infrastructureCode.MatchString(f.Detail) {
		return f.Reason + ":" + f.Detail
	}
	return f.Reason
}
