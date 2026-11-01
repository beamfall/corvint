package changee2e

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/archivepath"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	// The E2E readiness and preflight probes need the loopback HTTP client (V1-2185).
	_ "github.com/Beamfall/corvint/internal/jstestprovider/loopbackhttp"
	"github.com/Beamfall/corvint/internal/playwrightruns"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Discrimination verdicts (CCE-V0-009).
const (
	VerdictInconclusive       = "INCONCLUSIVE"
	VerdictUnstable           = "UNSTABLE"
	VerdictRegressionDetected = "REGRESSION_DETECTED"
	VerdictScenarioBroken     = "SCENARIO_BROKEN"
	VerdictControlSurvived    = "CONTROL_SURVIVED"
	VerdictDiscriminates      = "DISCRIMINATES"
	VerdictNewReach           = "NEW_REACH"
	VerdictPreserves          = "PRESERVES"
)

// Failure kinds of a repeat that fails at the outcome assertion (CCE-V0-009 rungs 7-8, CCE-V0-026).
const (
	FailureValueMismatch     = "value-mismatch"
	FailureLocatorUnresolved = "locator-unresolved"
	// FailureCleanupFailed is a failure located in the scenario's cleanup; it is never an
	// assertion result and reads INCONCLUSIVE (CCE-V0-009 rung 1).
	FailureCleanupFailed = "cleanup-failed"
	// FailureTeardownFailed is a run-level error, such as a worker fixture teardown failure, that
	// the receipt locates inside the staged test tree: the test's own failure, never an assertion
	// result and never infrastructure; it reads INCONCLUSIVE (CCE-V0-048).
	FailureTeardownFailed = "teardown-failed"
)

// ReasonOracleSelfCertified caps a CHANGED_IN_DIFF scenario's verdict (CCE-V0-016, CCE-V0-027).
const ReasonOracleSelfCertified = "oracle-self-certified"

// ReasonOracleAccepted notes a CHANGED_IN_DIFF verdict that an independent acceptance lifted
// from the cap (CCE-V0-051).
const ReasonOracleAccepted = "oracle-accepted"

// outsideTestTree is the recorded failure location of a repeat whose receipt location lies
// outside the staged test tree.
const outsideTestTree = "outside-test-tree"

// Fixed run settings (CCE-V0-008): retries and workers are never configurable.
const (
	proveRetries = 0
	proveWorkers = 1
)

// Process environment the prove runs and the BBF control hook read.
const (
	envNodeBin  = "CORVINT_E2E_NODE_BIN"
	envModules  = "CORVINT_PLAYWRIGHT_MODULES"
	envBrowsers = "PLAYWRIGHT_BROWSERS_PATH"
	envBaseURL  = "CORVINT_E2E_BASE_URL"
)

const (
	proveConfigName  = "corvint-e2e.playwright.config.cjs"
	proveProject     = "chromium"
	sideTimeout      = 15 * time.Minute
	hookRunTimeout   = 8 * time.Minute
	controlTimeout   = 600 // seconds per BBF control attempt
	controlWallClock = 1800
	markerText       = "corvint-test e2e prove disposable fixture\n"
	wrongLocator     = "corvint-wrong-locator"
	wrongValue       = "corvint-wrong-value"
	omittedEvent     = "corvint-omitted-event"
)

// proveConfig is the generated Playwright config: one worker, no retries, chromium, and the
// in-process server's URL as baseURL.
const proveConfig = `// corvint-test e2e prove: generated Playwright config.
module.exports = {
  testDir: '.',
  workers: 1,
  retries: 0,
  use: { browserName: 'chromium', baseURL: process.env.CORVINT_E2E_BASE_URL },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
};
`

// proveLimitations are carried in every proof document.
var proveLimitations = []string{
	"outcome-assertion location is the receipt's PWP-V3-008 failure location (the innermost frame in the test file); a repeat whose receipt names no location is read off the assertion",
	"failure kind is read from the failure message: a received `element(s) not found` is locator-unresolved, any other failure at the outcome assertion is value-mismatch",
	"cleanup-failed is read only for a failure located at a compiled fixture's teardown call, or in a hosted test file's own `finally` cleanup; any other fixture or hook teardown failure is a failure off the assertion at its receipt location",
	"a count assertion (toHaveCount) that matches no element reads value-mismatch, not locator-unresolved; a locator repair is proposed only for a test-id locator",
	"a fixture whose setup returns outputs has no omitted-event control: removing its setup fails the output check by construction, so it would not show the setup's effect is load-bearing",
	"a session scenario is skipped, and reads INCONCLUSIVE, when its storage-state variable is unset; prove never reads or checks the storage-state file",
	"teardown-failed is read only for a run-level error the receipt locates inside the staged test tree; an unlocated run-level error, or one located outside the tree, reads infrastructure",
	"PRESERVES is a regression-preservation check only; it does not detect the change",
	"NEW_REACH checks new behaviour against its absence; it does not detect a changed value",
}

// hostedLimitations are added to a hosted-test proof (CCE-V0-034..036).
var hostedLimitations = []string{
	"a hosted test runs under the prove config, not the host's own Playwright config: host projects, fixtures and globalSetup are not applied; runner_config lists each key whose value differs from the host config read as text (CCE-V0-049)",
	"a hosted test is selected by its literal title; a title another test in the file also matches under --grep runs both",
	"hosted controls remove the awaited action before each assertion; the assertions themselves are not perturbed",
}

func hostedGrep(sc Scenario) string {
	if !sc.Hosted {
		return ""
	}
	return grepTitle(sc.TestTitle)
}

// ProveOptions selects the revisions, scenarios and local Playwright toolchain for Prove.
type ProveOptions struct {
	Root, Base, Head string
	Repeats          int // 0 means DefaultRepeats
	// NodeModules is a node_modules directory holding @playwright/test; NodeBin is an absolute
	// path to a node binary with a sibling npx.
	NodeModules, NodeBin  string
	PackageJSON, Lockfile string // optional host files bound into the receipt identity
	Scenarios             []Scenario
	// Hosted selects an existing hosted test, read at head and run as given, in place of
	// Scenarios (CCE-V0-034).
	Hosted *HostedSelection
	// HookExecutable overrides os.Executable() as the BBF control hook and cleanup command. The
	// executable must dispatch to RunHook when run with no arguments and
	// CORVINT_BEHAVIOR_COMMAND_ROLE set.
	HookExecutable string
	// OracleAcceptance is an e2e-oracle-acceptance/0 document outside the repository that
	// records an independent acceptance of CHANGED_IN_DIFF outcomes (CCE-V0-051).
	OracleAcceptance string
	// RecordCoverage records, per compiled scenario, the application paths the in-process static
	// server serves during the head pool (CCE-V0-053).
	RecordCoverage bool
	// Tests, when set, is the aliased tests root the scenarios were compiled against: the test
	// tree is read from it at its pinned revision while the application sides stay --root's
	// (CCE-V0-136).
	Tests *TestsRoot
}

type proveReport struct {
	Schema        string `json:"schema"`
	Base          string `json:"base"`
	Head          string `json:"head"`
	Repeats       int    `json:"repeats"`
	Retries       int    `json:"retries"`
	Workers       int    `json:"workers"`
	RunnerVersion string `json:"runner_version"`
	Environment   string `json:"environment"` // "static" or the CCE-V0-021 environment digest
	// TestRoot is the aliased tests root and revision a two-tree proof read its test tree from
	// (CCE-V0-136); absent when the test tree is --root's head.
	TestRoot    *proveTestRoot  `json:"test_root,omitempty"`
	Scenarios   []scenarioProof `json:"scenarios"`
	Limitations []string        `json:"limitations"`
	Truncated   int             `json:"truncated,omitempty"` // scenarios dropped by the byte cap (CCE-V0-014)
	Digest      string          `json:"digest"`
}

type proveTestRoot struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
}

type scenarioProof struct {
	Obligation   string `json:"obligation"`
	Placement    string `json:"placement"`
	AssertionID  string `json:"assertion_id"`
	OracleOrigin string `json:"oracle_origin"`
	Hosted       bool   `json:"hosted,omitempty"` // an existing hosted test run as given (CCE-V0-034)
	// RunnerConfig lists how the prove config differs from the adopter's own config (CCE-V0-049).
	RunnerConfig *runnerConfig `json:"runner_config,omitempty"`
	// OracleDigest names a CHANGED_IN_DIFF scenario's oracle for an acceptance, and
	// OracleAcceptance is the acceptance applied to it (CCE-V0-051).
	OracleDigest     string            `json:"oracle_digest,omitempty"`
	OracleAcceptance *oracleAcceptance `json:"oracle_acceptance,omitempty"`
	// LocatorRepair proposes, and never applies, a replacement for an outcome locator that did
	// not resolve at head (CCE-V0-052).
	LocatorRepair *LocatorRepair `json:"locator_repair,omitempty"`
	// RecordedCoverage is present only when Prove records coverage (CCE-V0-053).
	RecordedCoverage *recordedCoverage `json:"recorded_coverage,omitempty"`
	Verdict          string            `json:"verdict"`
	Note             string            `json:"note"`
	// Scoring is true only for a DISCRIMINATES, NEW_REACH or PRESERVES verdict on an
	// ADMITTED_AT_BASE oracle: the scenarios CCE-V0-010 lets read above candidate.
	Scoring  bool           `json:"scoring"`
	Head     sideProof      `json:"head"`
	Base     sideProof      `json:"base"`
	Controls []controlProof `json:"controls"`
}

// sideProof is one side's pool. The application revision differs between the sides; the test
// revision and the test-tree digest are the head's on both (CCE-V0-017).
type sideProof struct {
	ApplicationRevision string          `json:"application_revision"`
	TestRevision        string          `json:"test_revision"`
	TestTreeDigest      string          `json:"test_tree_digest,omitempty"`
	Qualified           bool            `json:"qualified"`
	Reason              string          `json:"reason,omitempty"`
	Receipts            []string        `json:"receipts"`
	Outcomes            []repeatOutcome `json:"outcomes"`
	Stability           string          `json:"stability"`
	// Loss is an unqualified pool's concrete loss (CCE-V0-116); absent for a qualified pool.
	Loss *runLoss `json:"loss,omitempty"`
}

type repeatOutcome struct {
	State              string `json:"state"`
	AtOutcomeAssertion bool   `json:"at_outcome_assertion"`
	FailureKind        string `json:"failure_kind,omitempty"`
	// FailureLocation is the receipt's failure location of a failing repeat as `path:line` in the
	// test tree, or outside-test-tree; empty when the receipt names none (CCE-V0-009 step 6).
	FailureLocation string `json:"failure_location,omitempty"`
}

type controlProof struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Status     string   `json:"status"`
	PlanDigest string   `json:"plan_digest,omitempty"`
	Reasons    []string `json:"reasons"`
	// LostAttempts are the attempts BBF read infrastructure_failed, each with its concrete
	// loss (CCE-V0-116); absent when none was.
	LostAttempts []runLoss `json:"lost_attempts,omitempty"`
	// RetriedLostAttempts are the first run's losses of a control that was lost to
	// infrastructure and rerun once (CCE-V0-940); absent when the control was not rerun.
	RetriedLostAttempts []runLoss `json:"retried_lost_attempts,omitempty"`
}

// proveEnv is the validated toolchain shared by every run of one Prove call.
type proveEnv struct {
	root, nodeModules, runnerVersion string
	packageJSON, lockfile            string
	environment                      *Environment // nil serves the tree statically
	serveBin, servePath              string       // a bare serve[0] resolved once, and the PATH serve runs with
	hook                             behaviorfalsify.Command
	acceptances                      *oracleAcceptances // nil without an acceptance document
	recordCoverage                   bool
	// tests is the aliased tests root of a two-tree proof (CCE-V0-136); a zero value reads the
	// test tree from root at head.
	tests testsRoot
	// side runs one staged pool and returns its receipt, either the qualified receipt digest
	// or the reason it has none, and the provider's run error; controls applies the CCE-V0-018 controls at head,
	// all of them when only is nil and otherwise the controls only names. Prove uses
	// runQualified and runControls; the offline ladder tests substitute fakes (CCE-V0-028).
	side     func(ctx context.Context, in runInput) (jstestprovider.Receipt, string, string, error)
	controls func(ctx context.Context, head string, sc Scenario, tree testTree, headReceipt jstestprovider.Receipt, only []string) []controlProof
}

// Prove runs each compiled scenario at head and then at base through the Playwright provider,
// applies the two BBF controls to the outcome assertion at head, and returns the sealed
// `e2e-prove/0` document (CCE-V0-008/009). It sets process environment for the duration of the
// call and is not safe for concurrent use.
func Prove(ctx context.Context, o ProveOptions) ([]byte, error) {
	if o.Root == "" {
		return nil, errNoRoot
	}
	repeats := o.Repeats
	if repeats == 0 {
		repeats = DefaultRepeats
	}
	if repeats < 1 || repeats > MaxRepeats {
		return nil, refuse("e2e-repeats-over-limit", "repeats %d outside 1..%d", o.Repeats, MaxRepeats)
	}
	if o.Hosted != nil && len(o.Scenarios) > 0 {
		return nil, refuse("e2e-hosted-test-invalid", "a hosted test is proved alone, without compiled scenarios")
	}
	if o.Hosted == nil && len(o.Scenarios) == 0 {
		return nil, refuse("e2e-scenarios-required", "no compiled scenario to prove")
	}
	if len(o.Scenarios) > MaxScenarios {
		return nil, refuse("e2e-scenarios-over-limit", "%d scenarios exceed %d", len(o.Scenarios), MaxScenarios)
	}
	if err := validateScenarios(o.Scenarios); err != nil {
		return nil, err
	}
	root, err := resolveExisting(o.Root)
	if err != nil {
		return nil, err
	}
	base, err := resolveRev(ctx, root, o.Base)
	if err != nil {
		return nil, err
	}
	head, err := resolveRev(ctx, root, o.Head)
	if err != nil {
		return nil, err
	}
	tests, err := proveTestsRoot(ctx, o)
	if err != nil {
		return nil, err
	}
	scenarios := o.Scenarios
	if o.Hosted != nil {
		sc, err := hostedScenario(ctx, root, base, head, *o.Hosted)
		if err != nil {
			return nil, err
		}
		scenarios = []Scenario{sc}
	}
	envRoot, envRev := root, head
	if tests.repo != "" {
		envRoot, envRev = tests.dir, tests.rev
	}
	environment, err := readEnvironment(ctx, envRoot, envRev)
	if err != nil {
		return nil, err
	}
	acceptances, err := readOracleAcceptance(ctx, root, base, head, o.OracleAcceptance)
	if err != nil {
		return nil, err
	}
	if tests.repo != "" && o.OracleAcceptance != "" {
		if _, err := CheckOut(tests.dir, o.OracleAcceptance); err != nil {
			return nil, refuse("e2e-oracle-acceptance-invalid", "%s: the acceptance must be recorded outside the tests repository", o.OracleAcceptance)
		}
	}
	env, err := proveToolchain(root, o)
	if err != nil {
		return nil, err
	}
	env.environment = environment
	env.acceptances = acceptances
	env.recordCoverage = o.RecordCoverage
	env.tests = tests
	restore := setEnv(proveEnvironment(o))
	defer restore()
	// A bare serve name is resolved once on Prove's PATH, so the sides and the BBF hook, which runs
	// with a fixed PATH, start the same executable with the same PATH.
	env.serveBin, env.servePath = lookServe(environment), os.Getenv("PATH")

	report := proveReport{Schema: ProveSchema, Base: base, Head: head, Repeats: repeats, Retries: proveRetries,
		Workers: proveWorkers, RunnerVersion: env.runnerVersion, Environment: environment.digest(), Limitations: proveLimitations}
	if environment != nil {
		report.Limitations = append(slices.Clone(proveLimitations), environmentLimitations...)
	}
	if tests.repo != "" {
		report.TestRoot = &proveTestRoot{Repo: tests.repo, Revision: tests.rev}
	}
	if o.Hosted != nil {
		report.Limitations = append(slices.Clone(report.Limitations), hostedLimitations...)
	}
	for _, sc := range scenarios {
		report.Scenarios = append(report.Scenarios, env.proveScenario(ctx, base, head, sc, repeats))
	}
	return capList(&report.Scenarios, &report.Truncated, func() ([]byte, error) { return seal(&report, &report.Digest) })
}

// proveScenario proves one scenario: it reads the head test tree, runs it over the head and then
// the base application tree, applies the controls when the ladder reaches them, and reads the
// verdict with the oracle_origin cap.
func (e *proveEnv) proveScenario(ctx context.Context, base, head string, sc Scenario, repeats int) scenarioProof {
	p := scenarioProof{Obligation: sc.Obligation, Placement: sc.Placement, AssertionID: sc.AssertionID, OracleOrigin: sc.OracleOrigin, Hosted: sc.Hosted}
	if sc.Hosted {
		p.RunnerConfig = adopterRunnerConfig(ctx, e.root, head, sc.Placement)
	}
	if sc.OracleOrigin == OriginChangedInDiff {
		p.OracleDigest = oracleDigest(sc)
	}
	testRev := e.testRevision(head)
	tree, reason := e.headTestTree(ctx, base, head, sc)
	if reason != "" {
		p.Head, p.Base = unrunSide(head, testRev, reason), unrunSide(base, testRev, reason)
		p.Controls = notRunControls(sc, "verdict-decided-before-controls")
		p.Verdict, p.Note = VerdictInconclusive, reason
		return p
	}
	var headReceipt jstestprovider.Receipt
	var recorder *coverageRecorder
	if e.recordCoverage && sc.Screen != "" && e.environment == nil {
		recorder = newCoverageRecorder()
	}
	p.Head, headReceipt = e.proveSide(ctx, head, testRev, tree, sc, repeats, recorder)
	p.Base, _ = e.proveSide(ctx, base, testRev, tree, sc, repeats, nil)
	if e.recordCoverage {
		p.RecordedCoverage = e.recorded(ctx, head, tree, sc, p.Head, recorder)
	}
	if controlsReachable(p.Head, p.Base) {
		p.Controls = e.rerunLostControls(ctx, head, sc, tree, headReceipt, e.controls(ctx, head, sc, tree, headReceipt, nil))
	} else {
		p.Controls = notRunControls(sc, "verdict-decided-before-controls")
	}
	p.Verdict, p.Note = verdict(p.Head, p.Base, p.Controls)
	if sc.Locator != "" && sc.LocatorForm == "" && !sc.Hosted && headFailsUnresolved(p.Head) {
		p.LocatorRepair = proposeLocatorRepair(ctx, e.root, base, head, sc.Locator)
	}
	p.OracleAcceptance = e.acceptances.lookup(sc)
	if a := p.OracleAcceptance; a != nil && a.Status == "accepted" && p.Verdict != VerdictInconclusive {
		p.Note += "; " + ReasonOracleAccepted + " by " + a.AcceptedBy
	} else {
		p.Verdict, p.Note = capOracle(sc.OracleOrigin, p.Verdict, p.Note)
		if a != nil && p.Verdict == VerdictInconclusive && strings.HasPrefix(p.Note, ReasonOracleSelfCertified) {
			p.Note += "; acceptance not applied: " + a.Status
		}
	}
	switch p.Verdict {
	case VerdictDiscriminates, VerdictNewReach, VerdictPreserves:
		p.Scoring = sc.OracleOrigin == OriginAdmittedAtBase
	}
	return p
}

// capOracle reads a CHANGED_IN_DIFF scenario's positive verdict INCONCLUSIVE with
// oracle-self-certified: the change edited the outcome it is proved against (CCE-V0-016/027).
func capOracle(origin, verdict, note string) (string, string) {
	if origin != OriginChangedInDiff {
		return verdict, note
	}
	switch verdict {
	case VerdictRegressionDetected, VerdictDiscriminates, VerdictNewReach, VerdictPreserves:
		return VerdictInconclusive, ReasonOracleSelfCertified + ": the ladder read " + verdict + " against an outcome the change edited"
	}
	return verdict, note
}

// testRevision is the revision the test tree is read at: head, or the tests root's pin.
func (e *proveEnv) testRevision(head string) string {
	if e.tests.repo != "" {
		return e.tests.rev
	}
	return head
}

// headTestTree extracts the test revision once and reads the scenario's test tree from it
// (CCE-V0-017). In a two-tree proof the test revision is the tests root's pin, and a test-tree
// file that would replace different application bytes at base or head, or that a head
// application file imports once staged, reads ReasonTestTreeOverlaps (CCE-V0-136).
func (e *proveEnv) headTestTree(ctx context.Context, base, head string, sc Scenario) (testTree, string) {
	dir, err := os.MkdirTemp("", "corvint-e2e-testtree-")
	if err != nil {
		return testTree{}, "workspace-unavailable"
	}
	defer os.RemoveAll(dir)
	src, rev := e.root, head
	if e.tests.repo != "" {
		src, rev = e.tests.dir, e.tests.rev
	}
	if err := extractRev(ctx, src, rev, filepath.Join(dir, "tests")); err != nil {
		return testTree{}, "workspace-unavailable"
	}
	tt, reason := buildTestTree(filepath.Join(dir, "tests"), sc)
	if reason != "" || e.tests.repo == "" {
		return tt, reason
	}
	return tt, e.crossTreeOverlap(ctx, base, head, tt, filepath.Join(dir, "tests"), filepath.Join(dir, "app"))
}

// crossTreeOverlap checks a two-tree test tree against the application at base and head
// (CCE-V0-136). The generated runner configuration is written by Prove and never compared. A
// relative import that resolves to a different file once the tree is staged over the application
// (an application `helper.ts` shadowing the tests' `helper/index.ts`) also overlaps.
func (e *proveEnv) crossTreeOverlap(ctx context.Context, base, head string, tt testTree, testsDir, appDir string) string {
	for rel, data := range tt.files {
		if rel == proveConfigName {
			continue
		}
		for _, rev := range []string{base, head} {
			app, _, ok, err := blobAt(ctx, e.root, rev, rel)
			if err != nil {
				return ReasonTestTreeUnresolved
			}
			if ok && !bytes.Equal(app, data) {
				return ReasonTestTreeOverlaps
			}
		}
	}
	if err := stageTree(ctx, e.root, head, appDir, tt); err != nil {
		return "workspace-unavailable"
	}
	for rel, data := range tt.files {
		for _, spec := range relativeImports(data) {
			want, _ := resolveModule(testsDir, path.Dir(rel), spec)
			if got, _ := resolveModule(appDir, path.Dir(rel), spec); got != want {
				return ReasonTestTreeOverlaps
			}
		}
	}
	overlaps, err := overlapsApplication(appDir, tt.files)
	switch {
	case err != nil:
		return ReasonTestTreeUnresolved
	case overlaps:
		return ReasonTestTreeOverlaps
	}
	return ""
}

// proveTestsRoot validates a two-tree proof's tests root (CCE-V0-136). A hosted test is read
// from --root at head, so a hosted proof with an aliased tests root refuses.
func proveTestsRoot(ctx context.Context, o ProveOptions) (testsRoot, error) {
	if o.Tests == nil {
		return testsRoot{}, nil
	}
	if o.Hosted != nil {
		return testsRoot{}, refuse("e2e-roots-unsupported", "a hosted test is read from --root; aliased tests roots (--repo) prove compiled scenarios only")
	}
	if o.Tests.Repo == "" || o.Tests.Revision == "" {
		return testsRoot{}, refuse("e2e-roots-unsupported", "an aliased tests root needs its alias and pinned revision")
	}
	dir, err := resolveExisting(o.Tests.Dir)
	if err != nil {
		return testsRoot{}, err
	}
	rev, err := resolveRev(ctx, dir, o.Tests.Revision)
	if err != nil {
		return testsRoot{}, err
	}
	return testsRoot{dir: dir, rev: rev, repo: o.Tests.Repo}, nil
}

func unrunSide(app, test, reason string) sideProof {
	return sideProof{ApplicationRevision: app, TestRevision: test, Reason: reason, Receipts: []string{}, Outcomes: []repeatOutcome{},
		Stability: playwrightruns.Insufficient}
}

func validateScenarios(scs []Scenario) error {
	seen := map[string]bool{}
	for i, sc := range scs {
		p := sc.Placement
		lines := strings.Count(string(sc.Text), "\n") + 1
		switch {
		case sc.Obligation == "" || sc.AssertionID == "" || sc.TestTitle == "" || len(sc.Text) == 0:
			return refuse("e2e-scenario-invalid", "scenario %d lacks identity or text", i)
		case sc.Hosted && !hostedPlacement(p),
			!sc.Hosted && (p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") ||
				!strings.HasSuffix(p, ".spec.ts") || strings.Contains(p, "node_modules")):
			return refuse("e2e-scenario-invalid", "scenario %d placement %q is not a clean relative spec path", i, p)
		case seen[p]:
			return refuse("e2e-scenario-invalid", "duplicate placement %q", p)
		case sc.AssertionLine < 1 || sc.AssertionLine > lines || !sc.Hosted && (sc.Matcher == "" || sc.Locator == "" || sc.Marker == ""):
			return refuse("e2e-scenario-invalid", "scenario %d outcome assertion is unbound", i)
		case sc.Hosted && (len(sc.AssertionLines) == 0 || len(sc.EventLines) == 0 || sc.DeclLine < 1):
			return refuse("e2e-scenario-invalid", "hosted scenario %d lacks its assertion, event or declaration lines", i)
		}
		seen[p] = true
	}
	return nil
}

func proveToolchain(root string, o ProveOptions) (*proveEnv, error) {
	bad := func(format string, args ...any) error { return refuse("e2e-toolchain-unavailable", format, args...) }
	if !filepath.IsAbs(o.NodeBin) || filepath.Base(o.NodeBin) != "node" {
		return nil, bad("node binary must be an absolute path named node (--node-bin takes the node file, with npx beside it, not its directory)")
	}
	for _, p := range []string{o.NodeBin, filepath.Join(filepath.Dir(o.NodeBin), "npx")} {
		if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return nil, bad("%s is unavailable", filepath.Base(p))
		}
	}
	modules, err := filepath.EvalSymlinks(o.NodeModules)
	if err != nil || !filepath.IsAbs(o.NodeModules) {
		return nil, bad("node_modules directory is unavailable")
	}
	version, err := playwrightVersion(modules)
	if err != nil {
		return nil, bad("@playwright/test is unavailable: %v", err)
	}
	if why := bundledBrowserRefusal(runtime.GOOS, runtime.GOARCH, modules, version); why != "" {
		return nil, bad("%s", why)
	}
	hookPath := o.HookExecutable
	if hookPath == "" {
		if hookPath, err = os.Executable(); err != nil {
			return nil, bad("hook executable is unavailable")
		}
	}
	hookPath, err = filepath.EvalSymlinks(hookPath)
	if err != nil {
		return nil, bad("hook executable is unavailable")
	}
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return nil, bad("hook executable is unreadable")
	}
	e := &proveEnv{root: root, nodeModules: modules, runnerVersion: version, packageJSON: o.PackageJSON, lockfile: o.Lockfile,
		hook: behaviorfalsify.Command{Path: hookPath, ExecutableSHA256: "sha256:" + sha256Hex(data)}, side: runQualified}
	e.controls = e.runControls
	return e, nil
}

// runQualified runs one pool through the Playwright provider and qualifies its receipt.
func runQualified(ctx context.Context, in runInput) (jstestprovider.Receipt, string, string, error) {
	receipt, runErr := runPlaywright(ctx, in)
	digest, reason := qualify(receipt, runErr, in.Repeats, treeRoots(in.Tree))
	return receipt, digest, reason, runErr
}

func playwrightVersion(modules string) (string, error) {
	data, err := os.ReadFile(filepath.Join(modules, "@playwright", "test", "package.json"))
	if err != nil {
		return "", errors.New("package.json missing")
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Version == "" {
		return "", errors.New("version unreadable")
	}
	return pkg.Version, nil
}

// bundledBrowserRefusal says why the qualified headless shell runner version needs cannot be
// found under the browsers path Prove runs with (CCE-V0-117), and returns "" when it is installed
// or no bundled tuple applies. Every qualified 1.63/1.64 identity requires it, so without it each
// repeat would read report-identity-unknown:project-location-unknown. A relative browsers path is
// refused because each side's runner would resolve it against its own working directory. Only
// darwin/arm64 has a bundled tuple.
func bundledBrowserRefusal(goos, goarch, modules, version string) string {
	if goos != "darwin" || goarch != "arm64" {
		return ""
	}
	rel, ok := jstestprovider.QualifiedBundledBrowserExecutable(version)
	if !ok {
		return ""
	}
	dir := browsersPath(modules)
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		return fmt.Sprintf("%s=%q is relative, so each side's runner would resolve it against its own working directory; set it to an absolute directory or 0", envBrowsers, dir)
	}
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
		return ""
	}
	return fmt.Sprintf("playwright %s's qualified headless shell is not installed at %s; run `npx playwright install chromium-headless-shell` with that runner, or set %s to the directory holding it", version, p, envBrowsers)
}

// browsersPath is the directory Playwright reads browsers from: PLAYWRIGHT_BROWSERS_PATH, its "0"
// form inside playwright-core, or the default cache proveEnvironment pins.
func browsersPath(modules string) string {
	switch v := os.Getenv(envBrowsers); v {
	case "":
		return defaultBrowsersPath()
	case "0":
		return filepath.Join(modules, "playwright-core", ".local-browsers")
	default:
		return v
	}
}

func defaultBrowsersPath() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, "ms-playwright")
}

// proveEnvironment is the process environment Prove sets: the node directory first on PATH and the
// three values BBF declares for the hook. A missing browsers path is pinned to Playwright's default
// cache, because the hook runs with HOME set to the disposable root.
func proveEnvironment(o ProveOptions) map[string]string {
	vals := map[string]string{
		"PATH":     filepath.Dir(o.NodeBin) + string(os.PathListSeparator) + os.Getenv("PATH"),
		envNodeBin: o.NodeBin,
		envModules: o.NodeModules,
	}
	if os.Getenv(envBrowsers) == "" {
		if dir := defaultBrowsersPath(); dir != "" {
			vals[envBrowsers] = dir
		}
	}
	return vals
}

func setEnv(vals map[string]string) func() {
	type prior struct {
		value string
		ok    bool
	}
	saved := map[string]prior{}
	for k, v := range vals {
		old, ok := os.LookupEnv(k)
		saved[k] = prior{old, ok}
		_ = os.Setenv(k, v)
	}
	return func() {
		for k, p := range saved {
			if p.ok {
				_ = os.Setenv(k, p.value)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}

// proveSide runs the head test tree over a disposable extraction of the application revision app
// (CCE-V0-017).
func (e *proveEnv) proveSide(ctx context.Context, app, test string, tt testTree, sc Scenario, repeats int, recorder *coverageRecorder) (sideProof, jstestprovider.Receipt) {
	side := sideProof{ApplicationRevision: app, TestRevision: test, TestTreeDigest: tt.digest, Receipts: []string{}, Outcomes: []repeatOutcome{}}
	dir, err := os.MkdirTemp("", "corvint-e2e-prove-")
	if err != nil {
		side.Reason = "workspace-unavailable"
		side.Stability = playwrightruns.Insufficient
		return side, jstestprovider.Receipt{}
	}
	defer os.RemoveAll(dir)
	tree := filepath.Join(dir, "tree")
	if err := stageTree(ctx, e.root, app, tree, tt); err != nil {
		side.Reason = "workspace-unavailable"
		side.Stability = playwrightruns.Insufficient
		return side, jstestprovider.Receipt{}
	}
	roots := treeRoots(tree)
	receipt, digest, reason, runErr := e.side(ctx, runInput{Tree: tree, AppIdentity: app, Placement: sc.Placement, Repeats: repeats,
		NodeModules: e.nodeModules, RunnerVersion: e.runnerVersion, PackageJSON: e.packageJSON, Lockfile: e.lockfile, Timeout: sideTimeout,
		Environment: e.environment, ServeBin: e.serveBin, ServePath: e.servePath, Grep: hostedGrep(sc), Record: recorder.recordFunc(), SessionEnv: sc.SessionEnv})
	side.Qualified, side.Reason = reason == "", reason
	side.Loss = sideLoss(reason, receipt, runErr, roots)
	if digest != "" {
		side.Receipts = append(side.Receipts, digest)
	}
	for _, t := range receipt.Tests {
		file, line := receiptLocation(roots, t)
		kind := assertionFailure(string(t.State), t.FailureMessage, file, line, sc)
		o := repeatOutcome{State: string(t.State), AtOutcomeAssertion: kind != "" && kind != FailureCleanupFailed, FailureKind: kind}
		if failing(o.State) {
			o.FailureLocation = locationText(file, line)
		}
		side.Outcomes = append(side.Outcomes, o)
	}
	side.Stability = stability(side.Outcomes)
	return side, receipt
}

// stageTree extracts the application revision rev into dest and writes the test tree over it.
func stageTree(ctx context.Context, repo, rev, dest string, tt testTree) error {
	if err := extractRev(ctx, repo, rev, dest); err != nil {
		return err
	}
	for rel, data := range tt.files {
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// extractRev writes `git archive rev` of repo into dest, a new directory.
func extractRev(ctx context.Context, repo, rev, dest string) error {
	data, err := git(ctx, repo, "archive", "--format=tar", rev)
	if err != nil {
		return refuse("e2e-tree-unavailable", "git archive %s: %v", rev, err)
	}
	return untar(data, dest)
}

func untar(data []byte, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := path.Clean(h.Name)
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return refuse("e2e-tree-unavailable", "archive entry %q escapes the tree", h.Name)
		}
		if name == "." {
			continue
		}
		target, err := archivepath.Join(dest, name)
		if err != nil {
			return refuse("e2e-tree-unavailable", "archive entry %q escapes the tree", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				mode = 0o755
			}
			body, err := io.ReadAll(io.LimitReader(tr, maxGitBytes))
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, body, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Links are not reproduced: a chain of individually contained links (a -> ., e2e -> a/..)
			// can resolve outside the disposable tree, and later writes would follow it.
			continue
		}
	}
}

func testArgv(in runInput) []string {
	argv := []string{in.Placement, "--repeat-each=" + strconv.Itoa(in.Repeats),
		"--retries=" + strconv.Itoa(proveRetries), "--workers=" + strconv.Itoa(proveWorkers), "--project=" + proveProject}
	if in.Grep != "" {
		argv = append(argv, "--grep="+in.Grep)
	}
	return argv
}

type runInput struct {
	Tree, AppIdentity, Placement string
	Repeats                      int
	NodeModules, RunnerVersion   string
	PackageJSON, Lockfile        string
	Timeout                      time.Duration
	Environment                  *Environment
	ServeBin, ServePath          string
	Grep                         string // a --grep title pattern selecting one hosted test (CCE-V0-034)
	// Record, when set, receives each path the in-process static server answers (CCE-V0-053).
	Record func(path string)
	// SessionEnv is the scenario's storage-state variable, passed to the run when set (CCE-V0-107).
	SessionEnv string
}

// runPlaywright serves tree, through the declared environment's serve argv when there is one and
// otherwise in-process and statically on 127.0.0.1, and runs the placement through the external
// Playwright provider with retained attempt details (PWP).
func runPlaywright(ctx context.Context, in runInput) (jstestprovider.Receipt, error) {
	tree, err := filepath.EvalSymlinks(in.Tree)
	if err != nil {
		return jstestprovider.Receipt{}, err
	}
	if err := os.Symlink(in.NodeModules, filepath.Join(tree, "node_modules")); err != nil {
		return jstestprovider.Receipt{}, err
	}
	var url, ready string
	var served *serving
	if in.Environment != nil {
		served, err = serveTree(ctx, in.Environment, tree, in.ServeBin, in.ServePath)
		if err != nil {
			return jstestprovider.Receipt{}, err
		}
		defer served.stop()
		url, ready = in.Environment.origin(), in.Environment.readyURL()
	} else {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return jstestprovider.Receipt{}, err
		}
		var handler http.Handler = http.FileServer(http.Dir(tree))
		if in.Record != nil {
			handler = recordingHandler(handler, in.Record)
		}
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
		go func() { _ = srv.Serve(ln) }()
		defer srv.Close()
		url = "http://" + ln.Addr().String() + "/"
		ready = url
	}
	restore := setEnv(map[string]string{envBaseURL: url})
	defer restore()
	receipt, err := jstestprovider.RunE2E(ctx, jstestprovider.E2EConfig{
		ScopedTempOutput: true,
		Config: jstestprovider.Config{
			Dir:             tree,
			TestFiles:       []string{filepath.Join(tree, filepath.FromSlash(in.Placement))},
			ConfigFile:      filepath.Join(tree, proveConfigName),
			PackageJSON:     in.PackageJSON,
			Lockfile:        in.Lockfile,
			RunnerName:      "playwright",
			RunnerVersion:   in.RunnerVersion,
			DeclaredEnvKeys: runEnvKeys(in.SessionEnv),
			Timeout:         in.Timeout,
		},
		ExternalServer:       true,
		AppIdentity:          in.AppIdentity,
		ServerReadyURL:       ready,
		RetainAttemptDetails: true,
		TestArgv:             testArgv(in),
	})
	if served != nil {
		// A serve that ended during the run leaves no proof of what answered the run's requests.
		if aliveErr := served.alive(); aliveErr != nil {
			return jstestprovider.Receipt{}, aliveErr
		}
	}
	return receipt, err
}

// runEnvKeys are the variables a Playwright run reads: the browsers path, the base URL and, for a
// session scenario whose variable is set, its storage-state path (CCE-V0-107).
func runEnvKeys(session string) []string {
	keys := []string{envBrowsers, envBaseURL}
	if _, ok := os.LookupEnv(session); ok && session != "" {
		keys = append(keys, session)
	}
	return keys
}

// qualify returns the native receipt digest of a qualified run, or a reason it has none. roots are
// the staged test tree's paths, which place a run-level error (CCE-V0-048).
func qualify(r jstestprovider.Receipt, runErr error, repeats int, roots []string) (digest, reason string) {
	if code, ok := asEnvError(runErr); ok {
		return "", code
	}
	switch {
	case runErr != nil:
		return "", "provider-error"
	case r.Cancelled:
		return "", "cancelled"
	case teardownFailure(r, roots) != "":
		return "", teardownFailure(r, roots)
	case r.Infrastructure != nil:
		// Only a bare provider reason code is kept; free-text reasons can carry absolute paths.
		if reasonCode.MatchString(r.Infrastructure.Reason) {
			if reasonCode.MatchString(r.Infrastructure.Detail) {
				return "", "infrastructure:" + r.Infrastructure.Reason + ":" + r.Infrastructure.Detail
			}
			return "", "infrastructure:" + r.Infrastructure.Reason
		}
		return "", "infrastructure"
	case len(r.Tests) != repeats:
		return "", "repeat-count-mismatch"
	}
	for _, t := range r.Tests {
		if !jstestprovider.QualifiedReceiptBindingReady(r, t) {
			return "", "receipt-not-qualified"
		}
		// A test timeout is the test's own outcome, not lost infrastructure: an omitted
		// fixture step that leaves an action waiting must still kill its control (CCE-V0-039).
		if t.State == jstestprovider.StateTimedOut {
			t.State = jstestprovider.StateFailed
		}
		if jstestprovider.ReceiptTestProjection(r, t).Execution.State == testvalidity.ExecutionInfrastructure {
			return "", "infrastructure"
		}
	}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		// The encoder's own refusal code (for example `qualified-document-secret-shaped`) is the
		// loss reason CCE-V0-116 retains; a code outside the reason grammar reads as the generic
		// refusal so no encoder text reaches the report.
		if code := err.Error(); reasonCode.MatchString(code) {
			return "", code
		}
		return "", "qualified-encoding-refused"
	}
	return "sha256:" + sha256Hex(data), ""
}

var (
	ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")
	reasonCode = regexp.MustCompile(`^[a-z][a-z-]{0,63}$`)
)

// treeRoots are the staged tree's path and its symlink-resolved path: the runner reports
// locations under the resolved tree.
func treeRoots(tree string) []string {
	roots := []string{tree}
	if resolved, err := filepath.EvalSymlinks(tree); err == nil && resolved != tree {
		roots = append(roots, resolved)
	}
	return roots
}

// receiptLocation is a repeat's PWP-V3-008 failure location from its last attempt, as a
// test-tree-relative slash path, or outsideTestTree; the file is empty when the receipt names none.
func receiptLocation(roots []string, t jstestprovider.TestOutcome) (string, int) {
	n := len(t.AttemptDetails)
	if n == 0 || t.AttemptDetails[n-1].FailureLocation == nil {
		return "", 0
	}
	l := t.AttemptDetails[n-1].FailureLocation
	return treeLocation(roots, *l), l.Line
}

// treeLocation is a reported location's test-tree-relative slash path, or outsideTestTree.
func treeLocation(roots []string, l jstestprovider.Anchor) string {
	if filepath.IsAbs(l.File) {
		for _, root := range roots {
			rel, err := filepath.Rel(root, l.File)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				return filepath.ToSlash(rel)
			}
		}
	}
	return outsideTestTree
}

// teardownFailure attributes a receipt's run-level reporter error by its PWP-V3-009 location
// (CCE-V0-048): `teardown-failed at <path>:<line>` when it lies in the staged test tree outside
// the linked node_modules, otherwise empty, so the error keeps reading infrastructure.
func teardownFailure(r jstestprovider.Receipt, roots []string) string {
	inf := r.Infrastructure
	if inf == nil || inf.Reason != jstestprovider.ReasonReporterGlobalError || inf.Location == nil || inf.Location.Line < 1 {
		return ""
	}
	file := treeLocation(roots, *inf.Location)
	if file == outsideTestTree || file == "node_modules" || strings.HasPrefix(file, "node_modules/") {
		return ""
	}
	return FailureTeardownFailed + " at " + locationText(file, inf.Location.Line)
}

func locationText(file string, line int) string {
	switch file {
	case "":
		return ""
	case outsideTestTree:
		return outsideTestTree
	}
	return file + ":" + strconv.Itoa(line)
}

// assertionFailure reads a failed repeat at its receipt location (CCE-V0-009 step 6, CCE-V0-035):
// empty unless it failed in the scenario's file at one of its outcome-assertion lines or in its
// cleanup; cleanup-failed in a cleanup line; otherwise the CCE-V0-026 failure kind, read from the
// message: Playwright reports a locator that matched nothing as a received `<element(s) not
// found>`, and any other failure at the assertion compared a different value.
func assertionFailure(state, message, file string, line int, sc Scenario) string {
	if state != string(jstestprovider.StateFailed) || file != sc.Placement {
		return ""
	}
	if slices.Contains(sc.CleanupLines, line) || slices.Contains(cleanupLines(sc.Text), line) {
		return FailureCleanupFailed
	}
	if !slices.Contains(assertionLines(sc), line) {
		return ""
	}
	if strings.Contains(ansiEscape.ReplaceAllString(message, ""), "element(s) not found") {
		return FailureLocatorUnresolved
	}
	return FailureValueMismatch
}

// assertionLines are the outcome-assertion lines: the compiled scenario's one assertion, or each
// assertion of a hosted test (CCE-V0-035).
func assertionLines(sc Scenario) []int {
	if sc.Hosted {
		return sc.AssertionLines
	}
	return []int{sc.AssertionLine}
}

// cleanupLines are the 1-based lines inside each `finally` block of a test file: the block opens
// on a line ending in `} finally {` and closes on the next line at the same indentation that
// starts with `}`.
func cleanupLines(text []byte) []int {
	var out []int
	indent := -1
	for i, l := range strings.Split(string(text), "\n") {
		body := strings.TrimLeft(l, " \t")
		depth := len(l) - len(body)
		switch {
		case indent < 0 && strings.HasSuffix(strings.TrimRight(body, " \t"), "finally {"):
			indent = depth
		case indent >= 0 && depth == indent && strings.HasPrefix(body, "}"):
			indent = -1
		case indent >= 0:
			out = append(out, i+1)
		}
	}
	return out
}

// stability classifies the head pool in the PWR-V0-003 vocabulary: differing outcomes, or any
// flaky repeat, are INTERMITTENT at any count.
func stability(outs []repeatOutcome) string {
	if len(outs) == 0 {
		return playwrightruns.Insufficient
	}
	for _, o := range outs {
		if o.State == string(jstestprovider.StateFlaky) || o.State != outs[0].State {
			return playwrightruns.Intermittent
		}
	}
	return playwrightruns.Deterministic
}

func failing(state string) bool {
	switch jstestprovider.ExecutionState(state) {
	case jstestprovider.StateFailed, jstestprovider.StateTimedOut, jstestprovider.StateInterrupted, jstestprovider.StateInfrastructure:
		return true
	}
	return false
}

// controlsReachable reports whether the verdict can reach the control rungs (CCE-V0-009 rung 5).
func controlsReachable(head, base sideProof) bool {
	if !head.Qualified || !base.Qualified || incompletePool(head, base) != "" || head.Stability != playwrightruns.Deterministic {
		return false
	}
	for _, o := range head.Outcomes {
		if o.State != string(jstestprovider.StatePassed) {
			return false
		}
	}
	return true
}

// controlSpec is one required control (CCE-V0-018): the two BBF template controls on the outcome
// assertion, omitted-event on the step expected to produce the outcome, and omitted-event on each
// fixture setup.
type controlSpec struct {
	id          string
	kind        behaviorfalsify.ControlKind
	line        int
	replacement string
}

func controlSpecs(sc Scenario) []controlSpec {
	if sc.Hosted {
		// A hosted test's outcome assertions are its own; only removing the behaviour each one
		// follows can show that the assertion depends on it (CCE-V0-036).
		var out []controlSpec
		for i, line := range sc.EventLines {
			out = append(out, controlSpec{"control:omitted-event:step-" + strconv.Itoa(i+1), behaviorfalsify.OmittedEvent, line, omittedEvent})
		}
		return out
	}
	out := []controlSpec{
		{"control:" + string(behaviorfalsify.WrongLocator), behaviorfalsify.WrongLocator, sc.AssertionLine, wrongLocator},
		{"control:" + string(behaviorfalsify.WrongExpectedValue), behaviorfalsify.WrongExpectedValue, sc.AssertionLine, wrongValue},
		{"control:omitted-event:step", behaviorfalsify.OmittedEvent, sc.AssertionLine - 1, omittedEvent},
	}
	if sc.Matcher == MatcherCount {
		// A count is perturbed to the next integer: no quoted wrong value is a count (CCE-V0-110).
		if n, err := strconv.Atoi(sc.Value); err == nil {
			out[1].replacement = strconv.Itoa(n + 1)
		}
	}
	for i, line := range sc.SetupLines {
		out = append(out, controlSpec{"control:omitted-event:setup-" + strconv.Itoa(i+1), behaviorfalsify.OmittedEvent, line, omittedEvent})
	}
	return out
}

func notRunControls(sc Scenario, reason string) []controlProof {
	out := []controlProof{}
	for _, c := range controlSpecs(sc) {
		out = append(out, controlProof{ID: c.id, Kind: string(c.kind), Status: string(behaviorfalsify.StatusNotRun), Reasons: []string{reason}})
	}
	return out
}

// incompletePool is the CCE-V0-009 rung-1 reason a pool is incomplete: empty, or a repeat that was
// skipped or did not finish.
func incompletePool(head, base sideProof) string {
	for _, s := range []sideProof{head, base} {
		if len(s.Outcomes) == 0 {
			return "pool-empty"
		}
		for _, o := range s.Outcomes {
			switch jstestprovider.ExecutionState(o.State) {
			case jstestprovider.StateSkipped, jstestprovider.StateInterrupted:
				return "repeat-incomplete"
			}
			if o.FailureKind == FailureCleanupFailed {
				return FailureCleanupFailed + " at " + o.FailureLocation
			}
		}
	}
	return ""
}

// verdict is the CCE-V0-009 discrimination verdict: the first rung that holds.
func verdict(head, base sideProof, controls []controlProof) (string, string) {
	if !head.Qualified || !base.Qualified {
		for _, s := range []sideProof{head, base} {
			if strings.HasPrefix(s.Reason, FailureTeardownFailed+" at ") {
				return VerdictInconclusive, s.Reason
			}
		}
		return VerdictInconclusive, "run-without-qualified-receipt"
	}
	if reason := incompletePool(head, base); reason != "" {
		return VerdictInconclusive, reason
	}
	if head.Stability == playwrightruns.Intermittent {
		return VerdictUnstable, "head-pool-intermittent"
	}
	if every(head.Outcomes, func(o repeatOutcome) bool { return o.AtOutcomeAssertion }) &&
		every(base.Outcomes, func(o repeatOutcome) bool { return o.State == string(jstestprovider.StatePassed) }) {
		return VerdictRegressionDetected, "every head repeat fails at the outcome assertion and every base repeat passes: the change broke a declared outcome"
	}
	for _, o := range head.Outcomes {
		if failing(o.State) {
			return VerdictScenarioBroken, "head-repeat-failed" + at(o)
		}
	}
	if len(controls) == 0 {
		return VerdictInconclusive, "controls-missing"
	}
	for _, c := range controls {
		if c.Status == string(behaviorfalsify.StatusSurvived) {
			return VerdictControlSurvived, "control-survived:" + c.ID
		}
	}
	for _, c := range controls {
		if c.Status != string(behaviorfalsify.StatusInfrastructureFailed) {
			continue
		}
		for _, r := range c.Reasons {
			if strings.HasPrefix(r, FailureTeardownFailed+" at ") {
				return VerdictInconclusive, r + " (control " + c.ID + ")"
			}
		}
	}
	for _, c := range controls {
		if c.Status != string(behaviorfalsify.StatusKilled) {
			return VerdictInconclusive, "control-not-killed:" + c.ID
		}
	}
	kind := func(k string) func(repeatOutcome) bool {
		return func(o repeatOutcome) bool { return o.AtOutcomeAssertion && o.FailureKind == k }
	}
	switch {
	case every(base.Outcomes, kind(FailureValueMismatch)):
		return VerdictDiscriminates, "every base repeat fails at the outcome assertion with a value mismatch"
	case every(base.Outcomes, kind(FailureLocatorUnresolved)):
		return VerdictNewReach, "every base repeat fails at the outcome assertion because the locator did not resolve: checks new behaviour against its absence, not a changed value"
	case every(base.Outcomes, func(o repeatOutcome) bool { return o.State == string(jstestprovider.StatePassed) }):
		return VerdictPreserves, "regression-preservation check only; not evidence that the scenario detects the change"
	}
	for _, o := range base.Outcomes {
		if failing(o.State) && !o.AtOutcomeAssertion {
			return VerdictInconclusive, "base-mixed-or-failed-off-assertion" + at(o)
		}
	}
	return VerdictInconclusive, "base-mixed-or-failed-off-assertion"
}

// at names a failing repeat's receipt location in a verdict note.
func at(o repeatOutcome) string {
	if o.FailureLocation == "" {
		return " (receipt names no failure location)"
	}
	return " at " + o.FailureLocation
}

func every(outs []repeatOutcome, f func(repeatOutcome) bool) bool {
	for _, o := range outs {
		if !f(o) {
			return false
		}
	}
	return len(outs) > 0
}

// runControls applies the CCE-V0-018 controls at head (CCE-V0-009 rungs 5-6). The proof
// repository holds the head tree with the test tree over it, and serves as application, test and
// documentation repository at one revision; the scenario file is the contract file.
func (e *proveEnv) runControls(ctx context.Context, head string, sc Scenario, tree testTree, headReceipt jstestprovider.Receipt, only []string) []controlProof {
	specs := controlSpecs(sc)
	if only != nil {
		specs = slices.DeleteFunc(specs, func(s controlSpec) bool { return !slices.Contains(only, s.id) })
	}
	work, err := os.MkdirTemp("", "corvint-e2e-controls-")
	if err != nil {
		return notRunControls(sc, "control-workspace-unavailable")
	}
	defer os.RemoveAll(work)
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return notRunControls(sc, "control-workspace-unavailable")
	}
	repo, disposable := filepath.Join(work, "proof"), filepath.Join(work, "disposable")
	rev, err := e.proofRepo(ctx, head, repo, tree)
	if err != nil {
		return notRunControls(sc, "proof-repository-unavailable")
	}
	if err := os.MkdirAll(disposable, 0o755); err != nil {
		return notRunControls(sc, "control-workspace-unavailable")
	}
	if err := os.WriteFile(filepath.Join(disposable, behaviorfalsify.MarkerName), []byte(markerText), 0o644); err != nil {
		return notRunControls(sc, "control-workspace-unavailable")
	}
	request := e.controlRequest(sc, specs, rev, repo, disposable, headReceipt)
	plan, err := behaviorfalsify.BuildPlan(request)
	if err != nil {
		reason := "plan-refused"
		if !strings.Contains(err.Error(), "/") {
			reason += ": " + err.Error()
		}
		return notRunControls(sc, reason)
	}
	report, err := behaviorfalsify.Execute(ctx, plan, plan.Digest)
	if err != nil {
		out := notRunControls(sc, "execute-refused")
		for i := range out {
			out[i].PlanDigest = plan.Digest
		}
		return out
	}
	return controlResults(specs, plan.Digest, report)
}

// rerunLostControls reruns once, alone, each control that read infrastructure_failed without a
// located teardown failure, and keeps the first run's losses beside the rerun's result
// (CCE-V0-940). A killed, survived, invalid, not-run or teardown-attributed control never reruns.
func (e *proveEnv) rerunLostControls(ctx context.Context, head string, sc Scenario, tree testTree, headReceipt jstestprovider.Receipt, controls []controlProof) []controlProof {
	var lost []string
	for _, c := range controls {
		if c.Status == string(behaviorfalsify.StatusInfrastructureFailed) &&
			!slices.ContainsFunc(c.Reasons, func(r string) bool { return strings.HasPrefix(r, FailureTeardownFailed+" at ") }) {
			lost = append(lost, c.ID)
		}
	}
	if len(lost) == 0 {
		return controls
	}
	rerun := e.controls(ctx, head, sc, tree, headReceipt, lost)
	for i, c := range controls {
		if !slices.Contains(lost, c.ID) {
			continue
		}
		for _, r := range rerun {
			if r.ID != c.ID || r.Kind != c.Kind {
				continue
			}
			r.RetriedLostAttempts = c.LostAttempts
			if len(r.RetriedLostAttempts) == 0 { // keep a trace of the rerun even without attempt detail
				r.RetriedLostAttempts = []runLoss{{AttemptReasons: append([]string{}, c.Reasons...)}}
			}
			controls[i] = r
		}
	}
	return controls
}

func controlResults(specs []controlSpec, digest string, report behaviorfalsify.Report) []controlProof {
	out := []controlProof{}
	for _, spec := range specs {
		c := controlProof{ID: spec.id, Kind: string(spec.kind), Status: string(behaviorfalsify.StatusNotRun),
			PlanDigest: digest, Reasons: []string{"control-result-missing"}}
		for _, r := range report.Results {
			if r.ID == spec.id && r.Kind == spec.kind {
				c.Status, c.Reasons = string(r.Status), append([]string{}, r.Reasons...)
				if r.Status == behaviorfalsify.StatusInfrastructureFailed {
					c.Reasons = append(c.Reasons, controlTeardown(r)...)
				}
				c.LostAttempts = controlLosses(r)
			}
		}
		out = append(out, c)
	}
	return out
}

func controlTeardown(r behaviorfalsify.ControlResult) []string {
	var out []string
	for _, a := range r.Attempts {
		if a.Receipt == nil || a.Receipt.Infrastructure == nil {
			continue
		}
		if d, _, _ := parseHookDetail(a.Receipt.Infrastructure.Detail); strings.HasPrefix(d, FailureTeardownFailed+" at ") && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

func (e *proveEnv) proofRepo(ctx context.Context, head, repo string, tree testTree) (string, error) {
	if err := stageTree(ctx, e.root, head, repo, tree); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"-c", "init.defaultBranch=main", "init", "-q"},
		{"add", "-A", "-f", "."},
		{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-q", "--no-verify", "-m", "corvint-test e2e prove"},
	} {
		if _, err := gitWrite(ctx, repo, args...); err != nil {
			return "", err
		}
	}
	out, err := git(ctx, repo, "rev-parse", "HEAD")
	return strings.TrimSpace(string(out)), err
}

// gitWrite runs one git write in the disposable proof repository with an isolated environment and
// a fixed identity and date.
func gitWrite(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return gitrun.Run(ctx, gitrun.NewBudget(1, 120*time.Second), gitrun.Options{
		Dir: dir, Env: []string{"HOME=" + os.TempDir(), "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C", "TZ=UTC",
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=corvint-test", "GIT_AUTHOR_EMAIL=corvint-test@localhost", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=corvint-test", "GIT_COMMITTER_EMAIL=corvint-test@localhost", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"},
		StdoutLimit: 1 << 20, PerOpTimeout: 120 * time.Second,
	}, args...)
}

func (e *proveEnv) controlRequest(sc Scenario, specs []controlSpec, rev, repo, disposable string, headReceipt jstestprovider.Receipt) behaviorfalsify.Request {
	keys := []string{envNodeBin, envModules, envBrowsers}
	if e.environment != nil {
		// The hook's serve runs with the same declared names as the prove sides; an unset name is
		// left out, since BBF would pass it to the hook as an empty value the sides never see.
		for _, n := range e.environment.Env {
			if _, ok := os.LookupEnv(n); ok {
				keys = append(keys, n)
			}
		}
	}
	if _, ok := os.LookupEnv(sc.SessionEnv); ok && sc.SessionEnv != "" && !slices.Contains(keys, sc.SessionEnv) {
		keys = append(keys, sc.SessionEnv) // the control runs under the same session (CCE-V0-107)
	}
	slices.Sort(keys)
	envValues := map[string]string{}
	for _, k := range keys {
		envValues[k] = os.Getenv(k)
	}
	target := behaviorfalsify.TargetIdentity{
		ContractID: sc.Obligation, ContractSHA256: "sha256:" + sha256Hex(sc.Text), CriterionID: sc.Obligation,
		AssertionID: sc.AssertionID, ApplicationRevision: rev, TestRevision: rev, DocumentationRevision: rev,
		ContractFile: sc.Placement, TestID: sc.Placement + "::" + sc.TestTitle, TestFile: sc.Placement,
		TestLine: testLine(sc), TestTitle: sc.TestTitle, Project: proveProject,
	}
	runner := behaviorfalsify.RunnerIdentity{Runner: "playwright", RunnerVersion: e.runnerVersion, Browser: proveProject,
		BrowserVersion: browserVersion(headReceipt), ConfigFile: proveConfigName,
		ConfigSHA256: "sha256:" + sha256Hex([]byte(proveConfig)), EnvironmentSHA256: digestCanonical(envValues)}
	def := func(spec controlSpec) map[string]string {
		d := map[string]string{"tree": repo, "file": sc.Placement, "line": strconv.Itoa(spec.line),
			"replacement": spec.replacement, "kind": string(spec.kind)}
		for k, v := range map[string]string{"locator": sc.Locator, "matcher": sc.Matcher, "marker": sc.Marker, "value": sc.Value, "locator_form": sc.LocatorForm, "session_env": sc.SessionEnv} {
			if v != "" { // BBF refuses an empty definition value; a hosted test has none of these
				d[k] = v
			}
		}
		if sc.Hosted {
			d["grep"] = grepTitle(sc.TestTitle)
		}
		if e.packageJSON != "" {
			d["package_json"] = e.packageJSON
		}
		if e.lockfile != "" {
			d["lockfile"] = e.lockfile
		}
		if e.environment != nil {
			raw, _ := json.Marshal(e.environment)
			d["environment"] = string(raw)
			d["serve_path"] = e.servePath
			if e.serveBin != "" { // BBF refuses an empty definition value
				d["serve_bin"] = e.serveBin
			}
		}
		return d
	}
	hook := e.hook
	var controls []behaviorfalsify.ControlSpec
	for _, spec := range specs {
		controls = append(controls, behaviorfalsify.ControlSpec{ID: spec.id, Kind: spec.kind, Disposition: "run", Definition: def(spec), Hook: &hook})
	}
	return behaviorfalsify.Request{
		Target: target, Runner: runner, Controls: controls,
		Cleanup: e.hook, DisposableRoot: disposable,
		Repositories:    behaviorfalsify.RepositoryRoots{Application: repo, Test: repo, Documentation: repo},
		MarkerSHA256:    "sha256:" + sha256Hex([]byte(markerText)),
		DeclaredEnvKeys: keys, Attempts: 1, TimeoutSeconds: controlTimeout, WallClockSeconds: controlWallClock,
		ExternalState: "none",
	}
}

// digestCanonical matches BBF's environment digest: SHA-256 over JSON without HTML escaping or
// the trailing newline.
func digestCanonical(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	sum := sha256.Sum256(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func testLine(sc Scenario) int {
	if sc.Hosted {
		return sc.DeclLine
	}
	for i, l := range strings.Split(string(sc.Text), "\n") {
		if strings.HasPrefix(l, "test(") {
			return i + 1
		}
	}
	return sc.AssertionLine
}

func browserVersion(r jstestprovider.Receipt) string {
	for _, t := range r.Tests {
		if t.Project == nil {
			continue
		}
		var use struct {
			CorvintBrowser struct {
				BrowserVersion string `json:"browserVersion"`
			} `json:"corvintBrowser"`
		}
		if json.Unmarshal(t.Project.Use, &use) == nil && use.CorvintBrowser.BrowserVersion != "" {
			return use.CorvintBrowser.BrowserVersion
		}
	}
	return "unknown"
}
