package jstestprovider

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/stepnegation"
)

// negateFixture returns an admissible /0 negate config over a synthetic
// worktree with one spec file.
func negateFixture(t *testing.T) NegateConfig {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cart.spec.cjs", "playwright.config.cjs"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("// synthetic\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return NegateConfig{
		E2E: E2EConfig{
			Config:         Config{Dir: root, ConfigFile: filepath.Join(root, "playwright.config.cjs"), RunnerName: "playwright", RunnerVersion: "1.63.0"},
			ExternalServer: true, AppIdentity: "fixture-app", ServerReadyURL: "http://127.0.0.1:4000/health",
		},
		Root: root, Spec: "cart.spec.cjs", Test: "cart > total", Project: "chromium", Step: "total",
	}
}

// LPCV-V0-057, LPCV-V0-058: every mode and argument refusal is typed and
// precedes any run, and the run budget defaults by mode.
func TestAdmitNegateRefusesBeforeAnyRun(t *testing.T) {
	for _, test := range []struct {
		name, code string
		mutate     func(*NegateConfig)
	}{
		{"owned server", NegateModeUnsupported, func(c *NegateConfig) { c.E2E.ExternalServer = false }},
		{"per-test freshness", NegateModeUnsupported, func(c *NegateConfig) { c.E2E.Freshness = &FreshnessConfig{} }},
		{"profile /2", NegateModeUnsupported, func(c *NegateConfig) { c.E2E.SensitiveInputPolicy = &SensitiveInputPolicy{} }},
		{"profile /3", NegateModeUnsupported, func(c *NegateConfig) { c.E2E.RetainAttemptDetails = true }},
		{"keep reporters", NegateModeUnsupported, func(c *NegateConfig) { c.E2E.KeepReporters = true }},
		{"extra test argv", NegateInvalidArguments, func(c *NegateConfig) { c.E2E.TestArgv = []string{"--grep=x"} }},
		{"neither step nor all-steps", NegateInvalidArguments, func(c *NegateConfig) { c.Step = "" }},
		{"both step and all-steps", NegateInvalidArguments, func(c *NegateConfig) { c.AllSteps = true }},
		{"no test title", NegateInvalidArguments, func(c *NegateConfig) { c.Test = "" }},
		{"baseline repeat too high", NegateInvalidArguments, func(c *NegateConfig) { c.BaselineRepeat, c.MaxRuns = 6, 9 }},
		{"budget below baselines", NegateInvalidArguments, func(c *NegateConfig) { c.BaselineRepeat, c.MaxRuns = 3, 2 }},
		{"absolute spec", NegateInvalidArguments, func(c *NegateConfig) { c.Spec = filepath.Join(c.Root, c.Spec) }},
		{"escaping spec", NegateInvalidArguments, func(c *NegateConfig) { c.Spec = "../cart.spec.cjs" }},
		{"missing spec", NegateInvalidArguments, func(c *NegateConfig) { c.Spec = "missing.spec.cjs" }},
		{"unqualified runner version", NegateTupleUnqualified, func(c *NegateConfig) { c.E2E.RunnerVersion = "1.50.0" }},
		{"no application identity", "external-app-identity-required", func(c *NegateConfig) { c.E2E.AppIdentity = "" }},
		{"no readiness url", "external-readiness-url-required", func(c *NegateConfig) { c.E2E.ServerReadyURL = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := negateFixture(t)
			cfg.allowUnqualifiedTuple = true
			test.mutate(&cfg)
			_, err := admitNegate(cfg)
			var refusal *NegateRefusal
			if !errors.As(err, &refusal) || refusal.Code != test.code {
				t.Fatalf("admitNegate = %v, want %s", err, test.code)
			}
		})
	}
	t.Run("defaults and origin", func(t *testing.T) {
		cfg := negateFixture(t)
		cfg.allowUnqualifiedTuple = true
		n, err := admitNegate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		// RunNegate reads the admitted configuration, so the zero baseline
		// repeat a library caller passes is already the default of one.
		if n.cfg.BaselineRepeat != 1 || n.cfg.MaxRuns != defaultStepMaxRuns {
			t.Fatalf("admitted config = %+v", n.cfg)
		}
		if n.budget != defaultStepMaxRuns || n.origin != "http://127.0.0.1:4000" || n.spec != filepath.Join(cfg.Root, "cart.spec.cjs") || !containsString(n.base.TestFiles, n.spec) {
			t.Fatalf("negation = %+v", n)
		}
		cfg.Step, cfg.AllSteps = "", true
		if n, err = admitNegate(cfg); err != nil || n.budget != defaultAllStepsMaxRuns {
			t.Fatalf("all-steps budget = %v, %v", n, err)
		}
	})
	t.Run("host tuple decides without the seam", func(t *testing.T) {
		cfg := negateFixture(t)
		_, err := admitNegate(cfg)
		tuple := ReceiptRuntimeTuple(Receipt{Profile: ExternalProfile, Identity: Identity{RunnerVersion: "1.63.0", NodeVersion: nodeVersion()}})
		var refusal *NegateRefusal
		if tuple == RuntimeTupleCandidate {
			if err != nil {
				t.Fatalf("qualified host refused: %v", err)
			}
		} else if !errors.As(err, &refusal) || refusal.Code != NegateTupleUnqualified {
			t.Fatalf("unqualified host (%s) admitted: %v", tuple, err)
		}
	})
}

// LPCV-V0-057: exactly one listed test by file, full title and project.
func TestSelectTestRequiresExactlyOne(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(root, "cart.spec.cjs")
	listed := func(title, project string, line int) listedTest {
		return listedTest{File: spec, Line: line, TitlePath: append([]string{"", project, "cart.spec.cjs"}, strings.Split(title, " > ")...), Project: project}
	}
	tests := []listedTest{listed("cart > total", "chromium", 10), listed("cart > total", "webkit", 10), listed("cart > total > nested", "chromium", 20), listed("other", "chromium", 30)}
	if selected, err := selectTest(tests, spec, "cart > total", "chromium"); err != nil || selected.line != 10 || selected.project != "chromium" {
		t.Fatalf("select = %+v, %v", selected, err)
	}
	for name, test := range map[string][2]string{
		"two projects match": {"cart > total", ""},
		"no title match":     {"cart", "chromium"},
		"no project match":   {"other", "webkit"},
	} {
		var refusal *NegateRefusal
		if _, err := selectTest(tests, spec, test[0], test[1]); !errors.As(err, &refusal) || refusal.Code != NegateTestAmbiguous {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := selectTest(tests, filepath.Join(root, "other.spec.cjs"), "cart > total", "chromium"); err == nil {
		t.Error("another file matched")
	}
	// A parameterized loop declares several tests on one line; a file:line
	// run would execute all of them, so the selection refuses.
	looped := []listedTest{listed("cart > total 1", "chromium", 40), listed("cart > total 2", "chromium", 40), listed("cart > total 1", "webkit", 40)}
	var refusal *NegateRefusal
	if _, err := selectTest(looped, spec, "cart > total 1", "chromium"); !errors.As(err, &refusal) || refusal.Code != NegateTestAmbiguous {
		t.Errorf("shared declaration line: %v", err)
	}
	if selected, err := selectTest(looped, spec, "cart > total 1", "webkit"); err != nil || selected.line != 40 {
		t.Errorf("a line shared only across projects is selectable: %+v, %v", selected, err)
	}
}

// LPCV-V0-058, LPCV-V0-060: the controlled-config overlay forces trace
// recording into scratch, no retries, one repeat and one worker, and loads
// the injection module only when faults exist.
func TestNegationOverlayForcesControlledConfig(t *testing.T) {
	quoted := strconv.Quote
	overlay := &negationRun{outputDir: "/scratch/run-1/output"}
	if overlay.prelude(quoted) != "" {
		t.Fatal("a baseline run loads the injection module")
	}
	text := overlay.override(quoted)
	for _, want := range []string{`outputDir: "/scratch/run-1/output"`, "retries: 0", "repeatEach: 1", "workers: 1", "trace: {mode: 'on'", "projects: module.exports.projects?.map"} {
		if !strings.Contains(text, want) {
			t.Errorf("override lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "reporter:") {
		t.Error("a run overlay replaced the qualified reporter")
	}
	faulted := &negationRun{outputDir: "/o", module: "/scratch/injection.cjs", plan: "/scratch/run-2/plan.json", results: "/scratch/run-2/results.jsonl"}
	if got := faulted.prelude(quoted); got != `require("/scratch/injection.cjs").install("/scratch/run-2/plan.json", "/scratch/run-2/results.jsonl");`+"\n" {
		t.Fatalf("prelude = %q", got)
	}
	lister := &negationRun{outputDir: "/o", listReporter: "/scratch/list-reporter.cjs", listOutput: "/scratch/list.json"}
	if !strings.Contains(lister.override(quoted), `reporter: [["/scratch/list-reporter.cjs", {output:"/scratch/list.json"}]]`) {
		t.Fatal("the list overlay does not install the list reporter")
	}
}

// LPCV-V0-059: baseline classification.
func TestBaselineFailureClassification(t *testing.T) {
	cfg := negateFixture(t)
	n := &negation{cfg: cfg, spec: filepath.Join(cfg.Root, cfg.Spec), key: stepnegation.TestKey{File: cfg.Spec, FullTitle: cfg.Test, Project: "chromium"}}
	passing := func() observation {
		return observation{
			receipt: Receipt{},
			test: &TestOutcome{FullName: " > chromium > cart.spec.cjs > cart > total", State: StatePassed, Attempts: []Attempt{{State: StatePassed, FailureKind: "none"}},
				Anchor: &Anchor{File: n.spec, Line: 3}, Project: &ProjectIdentity{Name: "chromium"}},
			trace: &stepnegation.Trace{Steps: []stepnegation.TraceStep{{Title: "total", Ordinal: 1}}},
		}
	}
	if reason := n.baselineFailure(passing()); reason != "" {
		t.Fatalf("passing baseline = %q", reason)
	}
	for _, test := range []struct {
		name, reason string
		mutate       func(*observation)
	}{
		{"infrastructure", stepnegation.ReasonBaselineInfrastructure, func(o *observation) {
			o.receipt.Infrastructure = &InfrastructureFailure{Reason: "runner-crashed"}
		}},
		{"cancelled", stepnegation.ReasonBaselineInfrastructure, func(o *observation) { o.receipt.Cancelled = true }},
		{"no trace", stepnegation.ReasonBaselineInfrastructure, func(o *observation) { o.trace = nil }},
		{"fixture failure", stepnegation.ReasonBaselineInfrastructure, func(o *observation) {
			o.test.State, o.test.Attempts[0].State, o.test.Attempts[0].FailureKind = StateFailed, StateFailed, "browser-or-fixture"
		}},
		{"interrupted", stepnegation.ReasonBaselineInfrastructure, func(o *observation) { o.test.State = StateInterrupted }},
		{"failed", stepnegation.ReasonBaselineNotPassing, func(o *observation) { o.test.State = StateFailed }},
		{"retried pass", stepnegation.ReasonBaselineNotPassing, func(o *observation) { o.test.Attempts = append(o.test.Attempts, Attempt{State: StatePassed}) }},
		{"other test", stepnegation.ReasonBaselineNotPassing, func(o *observation) { o.test.FullName = " > chromium > cart.spec.cjs > cart > other" }},
		{"other project", stepnegation.ReasonBaselineNotPassing, func(o *observation) { o.test.Project.Name = "webkit" }},
		{"failed step", stepnegation.ReasonBaselineNotPassing, func(o *observation) { o.trace.Steps[0].Failed = true }},
		{"failed hook", stepnegation.ReasonBaselineNotPassing, func(o *observation) {
			o.trace.Failures = []stepnegation.Failure{{Method: "hook", Error: "x"}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			o := passing()
			test.mutate(&o)
			if reason := n.baselineFailure(o); reason != test.reason {
				t.Fatalf("reason = %q, want %q", reason, test.reason)
			}
		})
	}
	t.Run("unqualified identity is tolerated only under the seam", func(t *testing.T) {
		o := passing()
		o.receipt.Infrastructure = &InfrastructureFailure{Reason: "report-identity-unknown", Detail: "project-location-unknown"}
		if reason := n.baselineFailure(o); reason != stepnegation.ReasonBaselineInfrastructure {
			t.Fatalf("without the seam: %q", reason)
		}
		n.cfg.allowUnqualifiedTuple = true
		if reason := n.baselineFailure(o); reason != "" {
			t.Fatalf("under the seam: %q", reason)
		}
	})
}
