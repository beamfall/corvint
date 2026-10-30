package jstestprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/stepnegation"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// negateFixturePage is the synthetic external application of the
// LPCV-V0-070 matrix: one application-origin JSON value, one value rendered
// only by script, one value fetched from a second (third-party) origin, a
// counter-driven flaky value and an audit that reports any tampering with
// the total.
const negateFixturePage = `<!doctype html><html><body>
<div id="total">loading</div><div id="static"></div><div id="partner"></div><div id="flaky"></div><div id="audit">audit pending</div><button id="go">go</button>
<script>
document.getElementById('static').textContent = ['Static', 'Title'].join(' ');
const total = document.getElementById('total');
fetch('/api/total').then(r => r.json()).then(j => {
  total.textContent = j.total;
  const audit = document.getElementById('audit');
  audit.textContent = /^\d+ apples$/.test(total.textContent) ? 'audit ok' : 'audit failed';
  new MutationObserver(() => { if (!/^\d+ apples$/.test(total.textContent)) audit.textContent = 'audit failed'; }).observe(total, {childList: true, characterData: true, subtree: true});
});
fetch('%s/api/partner').then(r => r.json()).then(j => { document.getElementById('partner').textContent = j.label; });
fetch('/api/flaky').then(r => r.text()).then(t => { document.getElementById('flaky').textContent = t; });
</script></body></html>`

// TestNegateLiveDiagnostic runs the LPCV-V0-070 matrix shape against the
// checked-in synthetic fixtures. It needs installed Playwright modules and a
// browser. Without CORVINT_NEGATE_QUALIFIED_NODE it runs through the
// unqualified-tuple seam: every document is diagnostic, never qualification.
// With it (LPCV-V0-126), that absolute Node binary is placed first on PATH,
// the seam is off, and every negate must observe a PWP-V0-008 candidate tuple
// or refuse negate-tuple-unqualified.
func TestNegateLiveDiagnostic(t *testing.T) {
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES to installed node_modules for the step-negation live matrix")
	}
	qualified := negateLiveNode(t, os.Getenv("CORVINT_NEGATE_QUALIFIED_NODE"))
	envKeys := []string{"CORVINT_NEGATE_URL"}
	if os.Getenv("PLAYWRIGHT_BROWSERS_PATH") != "" {
		envKeys = append(envKeys, "PLAYWRIGHT_BROWSERS_PATH")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"playwright.config.cjs", "negate.spec.cjs"} {
		data, err := os.ReadFile(filepath.Join("testdata", "negate", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(modules, "@playwright/test/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}

	partner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"label":"partner data"}`))
	}))
	defer partner.Close()
	var flaky atomic.Int32
	// slowSeen closes when the fixture reaches /api/slow; slowCancelled closes
	// when that request's server context ends, which is the cancellation the
	// interruption case must observe (LPCV-V0-127).
	slowSeen, slowCancelled := make(chan struct{}), make(chan struct{})
	var slowOnce, slowCancelOnce sync.Once
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, negateFixturePage, partner.URL)
		case "/api/total":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"total":"42 apples","unit":"fruit"}`))
		case "/api/flaky":
			w.Header().Set("Content-Type", "text/plain")
			if flaky.Add(1) == 1 {
				_, _ = w.Write([]byte("steady"))
			} else {
				_, _ = w.Write([]byte("wobbly"))
			}
		case "/api/slow":
			slowOnce.Do(func() { close(slowSeen) })
			<-r.Context().Done()
			slowCancelOnce.Do(func() { close(slowCancelled) })
		default:
			http.NotFound(w, r)
		}
	}))
	defer app.Close()
	t.Setenv("CORVINT_NEGATE_URL", app.URL)

	negate := func(t *testing.T, ctx context.Context, title, step string, maxRuns, repeat int) stepnegation.Document {
		t.Helper()
		cfg := NegateConfig{
			E2E: E2EConfig{
				Config:         Config{Dir: root, ConfigFile: filepath.Join(root, "playwright.config.cjs"), RunnerName: "playwright", RunnerVersion: pkg.Version, DeclaredEnvKeys: envKeys, Timeout: 90 * time.Second},
				ExternalServer: true, AppIdentity: "negate-fixture-v1", ServerReadyURL: app.URL,
			},
			Root: root, Spec: "negate.spec.cjs", Test: "negate > " + title, Project: "chromium",
			Step: step, AllSteps: step == "", MaxRuns: maxRuns, BaselineRepeat: repeat,
			allowUnqualifiedTuple: !qualified,
		}
		if strings.HasPrefix(title, "!") {
			cfg.Test = title[1:]
		}
		document, err := RunNegate(ctx, cfg)
		if err != nil {
			t.Fatalf("RunNegate(%s): %v", title, err)
		}
		if _, err := stepnegation.Encode(document); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(document.Steps)
		runner, _ := json.Marshal(document.Binding.Runner)
		t.Logf("%s: qualified=%v runner %s runs %+v steps %s cleanupIncomplete=%v", title, qualified, runner, document.Runs, encoded, document.CleanupIncomplete)
		if document.CleanupIncomplete {
			t.Errorf("%s: cleanup incomplete", title)
		}
		if qualified && document.Binding.Runner.Tuple != RuntimeTupleCandidate {
			t.Errorf("%s: runner tuple %q, want %q on a qualified run", title, document.Binding.Runner.Tuple, RuntimeTupleCandidate)
		}
		return document
	}
	byTitle := func(document stepnegation.Document) map[string]stepnegation.Step {
		steps := map[string]stepnegation.Step{}
		for _, step := range document.Steps {
			steps[step.Title] = step
		}
		return steps
	}
	expect := func(t *testing.T, step stepnegation.Step, state, reason, witness string) {
		t.Helper()
		if step.Strength.State != state || step.Strength.Reason != reason || step.Witness != witness {
			t.Errorf("step %q = %s/%s witness %s, want %s/%s witness %s", step.Title, step.Strength.State, step.Strength.Reason, step.Witness, state, reason, witness)
		}
	}
	ctx := context.Background()

	t.Run("network-text-killed", func(t *testing.T) {
		document := negate(t, ctx, "network text", "total", 0, 1)
		expect(t, byTitle(document)["total"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessNetwork)
	})
	t.Run("static-element-killed-by-dom", func(t *testing.T) {
		document := negate(t, ctx, "static text", "static", 0, 1)
		expect(t, byTitle(document)["static"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessDOM)
	})
	t.Run("caught-assertion-survived", func(t *testing.T) {
		document := negate(t, ctx, "caught assertion", "caught", 0, 1)
		// The network fault survives, so the DOM fault runs and its survival is final.
		expect(t, byTitle(document)["caught"], testvalidity.StrengthSurvived, stepnegation.ReasonFaultSurvived, stepnegation.WitnessDOM)
		if document.Runs.Used != 3 {
			t.Errorf("runs %+v, want baseline, network and DOM runs", document.Runs)
		}
	})
	t.Run("pattern-and-assertion-free-unproven", func(t *testing.T) {
		document := negate(t, ctx, "unproven steps", "", 0, 1)
		steps := byTitle(document)
		expect(t, steps["pattern"], testvalidity.StrengthNotMeasured, stepnegation.ReasonUnderivable, stepnegation.WitnessNone)
		expect(t, steps["click"], testvalidity.StrengthNotMeasured, stepnegation.ReasonStepHasNoAssertion, stepnegation.WitnessNone)
		if steps["pattern"].Requires != stepnegation.RequiresManualControl || document.Runs.Used != 1 {
			t.Errorf("pattern requires %q, runs %+v", steps["pattern"].Requires, document.Runs)
		}
	})
	t.Run("soft-steps-one-joint-run", func(t *testing.T) {
		document := negate(t, ctx, "soft steps", "", 0, 1)
		steps := byTitle(document)
		expect(t, steps["total"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessJointNetwork)
		expect(t, steps["static"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessJointDOM)
		if document.Runs.Used != 2 {
			t.Errorf("runs %+v, want baseline plus one joint run", document.Runs)
		}
		// The open step has no assertion, so it is outside the denominator.
		if axis := stepnegation.Aggregate(document); axis.State != testvalidity.StrengthKilled || axis.Reason != stepnegation.ReasonStepControlsKilled {
			t.Errorf("aggregate %+v, want KILLED step-controls-killed", axis)
		}
	})
	t.Run("hard-steps-later-not-reached", func(t *testing.T) {
		document := negate(t, ctx, "hard steps", "", 0, 1)
		steps := byTitle(document)
		expect(t, steps["total"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessJointNetwork)
		expect(t, steps["static"], testvalidity.StrengthNotMeasured, stepnegation.ReasonStepNotReached, stepnegation.WitnessNone)
	})
	t.Run("collateral-not-isolated", func(t *testing.T) {
		document := negate(t, ctx, "collateral", "total", 0, 1)
		expect(t, byTitle(document)["total"], testvalidity.StrengthNotMeasured, stepnegation.ReasonFaultNotStepIsolated, stepnegation.WitnessNone)
	})
	t.Run("third-party-dependency-unfaulted", func(t *testing.T) {
		document := negate(t, ctx, "third party", "partner", 0, 1)
		expect(t, byTitle(document)["partner"], testvalidity.StrengthKilled, stepnegation.ReasonKilled, stepnegation.WitnessDOM)
	})
	t.Run("failing-fixture-baseline-infrastructure", func(t *testing.T) {
		document := negate(t, ctx, "!failing fixture", "open", 0, 1)
		expect(t, document.Steps[0], testvalidity.StrengthNotMeasured, stepnegation.ReasonBaselineInfrastructure, stepnegation.WitnessNone)
	})
	t.Run("flaky-baseline", func(t *testing.T) {
		flaky.Store(0)
		document := negate(t, ctx, "flaky", "flaky", 0, 2)
		expect(t, document.Steps[0], testvalidity.StrengthNotMeasured, stepnegation.ReasonBaselineNotPassing, stepnegation.WitnessNone)
		if document.Runs.BaselinePassed != 1 || document.Runs.Used != 2 {
			t.Errorf("runs %+v, want one passing baseline of two", document.Runs)
		}
	})
	t.Run("interruption-cleanup", func(t *testing.T) {
		interrupted, cancel := context.WithCancel(ctx)
		defer cancel()
		var cancelledOnSlow atomic.Bool
		go func() {
			select {
			case <-slowSeen:
				cancelledOnSlow.Store(true)
				cancel()
			case <-time.After(60 * time.Second):
			}
		}()
		document := negate(t, interrupted, "slow", "slow", 0, 1)
		expect(t, document.Steps[0], testvalidity.StrengthNotMeasured, stepnegation.ReasonBaselineInfrastructure, stepnegation.WitnessNone)
		// A browser or fixture failure before /api/slow also yields
		// baseline-infrastructure with clean cleanup, so the case counts only
		// when the slow request was reached, the run was cancelled on it, and
		// the server saw that request end.
		requireInterruptionObserved(t, slowSeen, slowCancelled, cancelledOnSlow.Load())
	})
	t.Run("ambiguous-selection-refused", func(t *testing.T) {
		_, err := RunNegate(ctx, NegateConfig{E2E: E2EConfig{Config: Config{Dir: root, ConfigFile: filepath.Join(root, "playwright.config.cjs"), RunnerName: "playwright", RunnerVersion: pkg.Version, Timeout: 90 * time.Second}, ExternalServer: true, AppIdentity: "negate-fixture-v1", ServerReadyURL: app.URL},
			Root: root, Spec: "negate.spec.cjs", Test: "negate > no such test", Step: "x", allowUnqualifiedTuple: !qualified})
		if refusal, ok := err.(*NegateRefusal); !ok || refusal.Code != NegateTestAmbiguous {
			t.Fatalf("err = %v, want %s", err, NegateTestAmbiguous)
		}
	})
}

// requireInterruptionObserved fails the interruption case unless the fixture
// reached the slow request, the negate run was cancelled on that signal, and
// the server observed the request end (LPCV-V0-127).
func requireInterruptionObserved(t *testing.T, seen, cancelled <-chan struct{}, cancelledOnSlow bool) {
	t.Helper()
	select {
	case <-seen:
	default:
		t.Fatal("interruption not exercised: the fixture never reached /api/slow")
	}
	if !cancelledOnSlow {
		t.Fatal("interruption not exercised: the run was not cancelled on the slow request")
	}
	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("interruption not observed: the /api/slow request did not end after cancellation")
	}
}

// negateLiveNode places an explicitly named Node first on this test's PATH
// (as PWP-V0-009 does for the provider matrix) and reports whether the run is
// a qualified-tuple run. An empty name keeps the host PATH and the diagnostic
// seam.
func negateLiveNode(t *testing.T, node string) bool {
	t.Helper()
	if node == "" {
		return false
	}
	if !filepath.IsAbs(node) || filepath.Base(node) != "node" {
		t.Fatalf("CORVINT_NEGATE_QUALIFIED_NODE must be an absolute path to a node binary, got %q", node)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(node), "npx")); err != nil {
		t.Fatalf("CORVINT_NEGATE_QUALIFIED_NODE has no sibling npx: %v", err)
	}
	t.Setenv("PATH", filepath.Dir(node)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if resolved, err := exec.LookPath("node"); err != nil || resolved != node {
		t.Fatalf("PATH resolves node to %q, want %q: %v", resolved, node, err)
	}
	return true
}
