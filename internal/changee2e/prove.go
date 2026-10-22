package changee2e

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/jstestprovider"
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
)

// ReasonOracleSelfCertified caps a CHANGED_IN_DIFF scenario's verdict (CCE-V0-016, CCE-V0-027).
const ReasonOracleSelfCertified = "oracle-self-certified"

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
	"cleanup-failed is read only for a failure located in the test file's own `finally` cleanup; a fixture or hook teardown failure reads infrastructure",
	"PRESERVES is a regression-preservation check only; it does not detect the change",
	"NEW_REACH checks new behaviour against its absence; it does not detect a changed value",
}

// hostedLimitations are added to a hosted-test proof (CCE-V0-034..036).
var hostedLimitations = []string{
	"a hosted test runs under the prove config, not the host's own Playwright config: host projects, fixtures and globalSetup are not applied",
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
}

type proveReport struct {
	Schema        string          `json:"schema"`
	Base          string          `json:"base"`
	Head          string          `json:"head"`
	Repeats       int             `json:"repeats"`
	Retries       int             `json:"retries"`
	Workers       int             `json:"workers"`
	RunnerVersion string          `json:"runner_version"`
	Environment   string          `json:"environment"` // "static" or the CCE-V0-021 environment digest
	Scenarios     []scenarioProof `json:"scenarios"`
	Limitations   []string        `json:"limitations"`
	Truncated     int             `json:"truncated,omitempty"` // scenarios dropped by the byte cap (CCE-V0-014)
	Digest        string          `json:"digest"`
}

type scenarioProof struct {
	Obligation   string `json:"obligation"`
	Placement    string `json:"placement"`
	AssertionID  string `json:"assertion_id"`
	OracleOrigin string `json:"oracle_origin"`
	Hosted       bool   `json:"hosted,omitempty"` // an existing hosted test run as given (CCE-V0-034)
	Verdict      string `json:"verdict"`
	Note         string `json:"note"`
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
}

// proveEnv is the validated toolchain shared by every run of one Prove call.
type proveEnv struct {
	root, nodeModules, runnerVersion string
	packageJSON, lockfile            string
	environment                      *Environment // nil serves the tree statically
	serveBin, servePath              string       // a bare serve[0] resolved once, and the PATH serve runs with
	hook                             behaviorfalsify.Command
	// side runs one staged pool and returns its receipt and either the qualified receipt digest
	// or the reason it has none; controls applies the CCE-V0-018 controls at head. Prove uses
	// runQualified and runControls; the offline ladder tests substitute fakes (CCE-V0-028).
	side     func(ctx context.Context, in runInput) (jstestprovider.Receipt, string, string)
	controls func(ctx context.Context, head string, sc Scenario, tree testTree, headReceipt jstestprovider.Receipt) []controlProof
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
	scenarios := o.Scenarios
	if o.Hosted != nil {
		sc, err := hostedScenario(ctx, root, base, head, *o.Hosted)
		if err != nil {
			return nil, err
		}
		scenarios = []Scenario{sc}
	}
	environment, err := readEnvironment(ctx, root, head)
	if err != nil {
		return nil, err
	}
	env, err := proveToolchain(root, o)
	if err != nil {
		return nil, err
	}
	env.environment = environment
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
	tree, reason := e.headTestTree(ctx, head, sc)
	if reason != "" {
		p.Head, p.Base = unrunSide(head, head, reason), unrunSide(base, head, reason)
		p.Controls = notRunControls(sc, "verdict-decided-before-controls")
		p.Verdict, p.Note = VerdictInconclusive, reason
		return p
	}
	var headReceipt jstestprovider.Receipt
	p.Head, headReceipt = e.proveSide(ctx, head, head, tree, sc, repeats)
	p.Base, _ = e.proveSide(ctx, base, head, tree, sc, repeats)
	if controlsReachable(p.Head, p.Base) {
		p.Controls = e.controls(ctx, head, sc, tree, headReceipt)
	} else {
		p.Controls = notRunControls(sc, "verdict-decided-before-controls")
	}
	p.Verdict, p.Note = verdict(p.Head, p.Base, p.Controls)
	p.Verdict, p.Note = capOracle(sc.OracleOrigin, p.Verdict, p.Note)
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

// headTestTree extracts the head revision once and reads the scenario's test tree from it
// (CCE-V0-017).
func (e *proveEnv) headTestTree(ctx context.Context, head string, sc Scenario) (testTree, string) {
	dir, err := os.MkdirTemp("", "corvint-e2e-testtree-")
	if err != nil {
		return testTree{}, "workspace-unavailable"
	}
	defer os.RemoveAll(dir)
	if err := extractRev(ctx, e.root, head, dir); err != nil {
		return testTree{}, "workspace-unavailable"
	}
	return buildTestTree(dir, sc)
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
		return nil, bad("node binary must be an absolute path named node")
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
func runQualified(ctx context.Context, in runInput) (jstestprovider.Receipt, string, string) {
	receipt, runErr := runPlaywright(ctx, in)
	digest, reason := qualify(receipt, runErr, in.Repeats)
	return receipt, digest, reason
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
		if cache, err := os.UserCacheDir(); err == nil {
			vals[envBrowsers] = filepath.Join(cache, "ms-playwright")
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
func (e *proveEnv) proveSide(ctx context.Context, app, test string, tt testTree, sc Scenario, repeats int) (sideProof, jstestprovider.Receipt) {
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
	receipt, digest, reason := e.side(ctx, runInput{Tree: tree, AppIdentity: app, Placement: sc.Placement, Repeats: repeats,
		NodeModules: e.nodeModules, RunnerVersion: e.runnerVersion, PackageJSON: e.packageJSON, Lockfile: e.lockfile, Timeout: sideTimeout,
		Environment: e.environment, ServeBin: e.serveBin, ServePath: e.servePath, Grep: hostedGrep(sc)})
	side.Qualified, side.Reason = reason == "", reason
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
		target := filepath.Join(dest, filepath.FromSlash(name))
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
		srv := &http.Server{Handler: http.FileServer(http.Dir(tree)), ReadHeaderTimeout: 10 * time.Second}
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
			DeclaredEnvKeys: []string{envBrowsers, envBaseURL},
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

// qualify returns the native receipt digest of a qualified run, or a reason it has none.
func qualify(r jstestprovider.Receipt, runErr error, repeats int) (digest, reason string) {
	if code, ok := asEnvError(runErr); ok {
		return "", code
	}
	switch {
	case runErr != nil:
		return "", "provider-error"
	case r.Cancelled:
		return "", "cancelled"
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
	if filepath.IsAbs(l.File) {
		for _, root := range roots {
			rel, err := filepath.Rel(root, l.File)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				return filepath.ToSlash(rel), l.Line
			}
		}
	}
	return outsideTestTree, l.Line
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
	if slices.Contains(cleanupLines(sc.Text), line) {
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
	for i, line := range setupLines(sc) {
		out = append(out, controlSpec{"control:omitted-event:setup-" + strconv.Itoa(i+1), behaviorfalsify.OmittedEvent, line, omittedEvent})
	}
	return out
}

// setupLines are the 1-based lines of the fixture setup calls: the awaited statements between the
// test declaration and the scenario's `try`.
func setupLines(sc Scenario) []int {
	var out []int
	inTest := false
	for i, l := range strings.Split(string(sc.Text), "\n") {
		switch {
		case strings.HasPrefix(l, "test("):
			inTest = true
		case !inTest:
		case l == "  try {":
			return out
		case strings.HasPrefix(l, "  await "):
			out = append(out, i+1)
		}
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
func (e *proveEnv) runControls(ctx context.Context, head string, sc Scenario, tree testTree, headReceipt jstestprovider.Receipt) []controlProof {
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
	request := e.controlRequest(sc, rev, repo, disposable, headReceipt)
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
	out := []controlProof{}
	for _, spec := range controlSpecs(sc) {
		c := controlProof{ID: spec.id, Kind: string(spec.kind), Status: string(behaviorfalsify.StatusNotRun),
			PlanDigest: plan.Digest, Reasons: []string{"control-result-missing"}}
		for _, r := range report.Results {
			if r.ID == spec.id && r.Kind == spec.kind {
				c.Status, c.Reasons = string(r.Status), append([]string{}, r.Reasons...)
			}
		}
		out = append(out, c)
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

func (e *proveEnv) controlRequest(sc Scenario, rev, repo, disposable string, headReceipt jstestprovider.Receipt) behaviorfalsify.Request {
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
		for k, v := range map[string]string{"locator": sc.Locator, "matcher": sc.Matcher, "marker": sc.Marker, "value": sc.Value} {
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
	for _, spec := range controlSpecs(sc) {
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
