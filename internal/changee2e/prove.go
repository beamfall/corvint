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
	VerdictInconclusive    = "INCONCLUSIVE"
	VerdictUnstable        = "UNSTABLE"
	VerdictFailsAtHead     = "FAILS_AT_HEAD"
	VerdictControlSurvived = "CONTROL_SURVIVED"
	VerdictDiscriminates   = "DISCRIMINATES"
	VerdictPreserves       = "PRESERVES"
)

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
	"outcome-assertion location is matched from the failure message (matcher and locator); the provider receipt does not retain the error line",
	"PRESERVES is a regression-preservation check only; it does not detect the change",
	"document byte cap (CCE-V0-014) is not applied by this slice",
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
	Scenarios     []scenarioProof `json:"scenarios"`
	Limitations   []string        `json:"limitations"`
	Digest        string          `json:"digest"`
}

type scenarioProof struct {
	Obligation  string         `json:"obligation"`
	Placement   string         `json:"placement"`
	AssertionID string         `json:"assertion_id"`
	Verdict     string         `json:"verdict"`
	Note        string         `json:"note"`
	Head        sideProof      `json:"head"`
	Base        sideProof      `json:"base"`
	Controls    []controlProof `json:"controls"`
}

type sideProof struct {
	Revision  string          `json:"revision"`
	Qualified bool            `json:"qualified"`
	Reason    string          `json:"reason,omitempty"`
	Receipts  []string        `json:"receipts"`
	Outcomes  []repeatOutcome `json:"outcomes"`
	Stability string          `json:"stability"`
}

type repeatOutcome struct {
	State              string `json:"state"`
	AtOutcomeAssertion bool   `json:"at_outcome_assertion"`
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
	hook                             behaviorfalsify.Command
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
	if len(o.Scenarios) == 0 {
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
	env, err := proveToolchain(root, o)
	if err != nil {
		return nil, err
	}
	restore := setEnv(proveEnvironment(o))
	defer restore()

	report := proveReport{Schema: ProveSchema, Base: base, Head: head, Repeats: repeats, Retries: proveRetries,
		Workers: proveWorkers, RunnerVersion: env.runnerVersion, Limitations: proveLimitations}
	for _, sc := range o.Scenarios {
		p := scenarioProof{Obligation: sc.Obligation, Placement: sc.Placement, AssertionID: sc.AssertionID}
		var headReceipt jstestprovider.Receipt
		p.Head, headReceipt = env.proveSide(ctx, head, sc, repeats)
		p.Base, _ = env.proveSide(ctx, base, sc, repeats)
		if controlsReachable(p.Head, p.Base) {
			p.Controls = env.runControls(ctx, head, sc, headReceipt)
		} else {
			p.Controls = notRunControls("verdict-decided-before-controls")
		}
		p.Verdict, p.Note = verdict(p.Head, p.Base, p.Controls)
		report.Scenarios = append(report.Scenarios, p)
	}
	return seal(&report, &report.Digest)
}

func validateScenarios(scs []Scenario) error {
	seen := map[string]bool{}
	for i, sc := range scs {
		p := sc.Placement
		lines := strings.Count(string(sc.Text), "\n") + 1
		switch {
		case sc.Obligation == "" || sc.AssertionID == "" || sc.TestTitle == "" || len(sc.Text) == 0:
			return refuse("e2e-scenario-invalid", "scenario %d lacks identity or text", i)
		case p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") ||
			!strings.HasSuffix(p, ".spec.ts") || strings.Contains(p, "node_modules"):
			return refuse("e2e-scenario-invalid", "scenario %d placement %q is not a clean relative spec path", i, p)
		case seen[p]:
			return refuse("e2e-scenario-invalid", "duplicate placement %q", p)
		case sc.AssertionLine < 1 || sc.AssertionLine > lines || sc.Matcher == "" || sc.Locator == "":
			return refuse("e2e-scenario-invalid", "scenario %d outcome assertion is unbound", i)
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
	return &proveEnv{root: root, nodeModules: modules, runnerVersion: version, packageJSON: o.PackageJSON, lockfile: o.Lockfile,
		hook: behaviorfalsify.Command{Path: hookPath, ExecutableSHA256: "sha256:" + sha256Hex(data)}}, nil
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

// proveSide runs one scenario at rev in a disposable extraction of that revision.
func (e *proveEnv) proveSide(ctx context.Context, rev string, sc Scenario, repeats int) (sideProof, jstestprovider.Receipt) {
	side := sideProof{Revision: rev, Receipts: []string{}, Outcomes: []repeatOutcome{}}
	dir, err := os.MkdirTemp("", "corvint-e2e-prove-")
	if err != nil {
		side.Reason = "workspace-unavailable"
		side.Stability = playwrightruns.Insufficient
		return side, jstestprovider.Receipt{}
	}
	defer os.RemoveAll(dir)
	tree := filepath.Join(dir, "tree")
	if err := e.stageTree(ctx, e.root, rev, tree, sc); err != nil {
		side.Reason = "workspace-unavailable"
		side.Stability = playwrightruns.Insufficient
		return side, jstestprovider.Receipt{}
	}
	receipt, runErr := runPlaywright(ctx, runInput{Tree: tree, AppIdentity: rev, Placement: sc.Placement, Repeats: repeats,
		NodeModules: e.nodeModules, RunnerVersion: e.runnerVersion, PackageJSON: e.packageJSON, Lockfile: e.lockfile, Timeout: sideTimeout})
	digest, reason := qualify(receipt, runErr, repeats)
	side.Qualified, side.Reason = reason == "", reason
	if digest != "" {
		side.Receipts = append(side.Receipts, digest)
	}
	for _, t := range receipt.Tests {
		side.Outcomes = append(side.Outcomes, repeatOutcome{State: string(t.State),
			AtOutcomeAssertion: failsAtAssertion(string(t.State), t.FailureMessage, sc.Matcher, sc.Locator)})
	}
	side.Stability = stability(side.Outcomes)
	return side, receipt
}

// stageTree extracts rev into dest and writes the scenario and the generated config.
func (e *proveEnv) stageTree(ctx context.Context, repo, rev, dest string, sc Scenario) error {
	if err := extractRev(ctx, repo, rev, dest); err != nil {
		return err
	}
	target := filepath.Join(dest, filepath.FromSlash(sc.Placement))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, sc.Text, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, proveConfigName), []byte(proveConfig), 0o644)
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
			resolved := path.Join(path.Dir(name), h.Linkname)
			if path.IsAbs(h.Linkname) || resolved == ".." || strings.HasPrefix(resolved, "../") {
				continue // an escaping link is not reproduced in the disposable tree
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		}
	}
}

type runInput struct {
	Tree, AppIdentity, Placement string
	Repeats                      int
	NodeModules, RunnerVersion   string
	PackageJSON, Lockfile        string
	Timeout                      time.Duration
}

// runPlaywright serves tree in-process on 127.0.0.1 and runs the placement through the external
// Playwright provider with retained attempt details (PWP).
func runPlaywright(ctx context.Context, in runInput) (jstestprovider.Receipt, error) {
	tree, err := filepath.EvalSymlinks(in.Tree)
	if err != nil {
		return jstestprovider.Receipt{}, err
	}
	if err := os.Symlink(in.NodeModules, filepath.Join(tree, "node_modules")); err != nil {
		return jstestprovider.Receipt{}, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return jstestprovider.Receipt{}, err
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(tree)), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	url := "http://" + ln.Addr().String() + "/"
	restore := setEnv(map[string]string{envBaseURL: url})
	defer restore()
	return jstestprovider.RunE2E(ctx, jstestprovider.E2EConfig{
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
		ServerReadyURL:       url,
		RetainAttemptDetails: true,
		TestArgv: []string{in.Placement, "--repeat-each=" + strconv.Itoa(in.Repeats),
			"--retries=" + strconv.Itoa(proveRetries), "--workers=" + strconv.Itoa(proveWorkers), "--project=" + proveProject},
	})
}

// qualify returns the native receipt digest of a qualified run, or a reason it has none.
func qualify(r jstestprovider.Receipt, runErr error, repeats int) (digest, reason string) {
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

// failsAtAssertion reports whether a failed repeat failed at the outcome assertion. PWP does not
// retain the failing error's line, so the location is matched from the failure message: the
// Playwright `expect(locator).<matcher>` header and the `getByTestId('<locator>')` it names.
func failsAtAssertion(state, message, matcher, locator string) bool {
	if state != string(jstestprovider.StateFailed) {
		return false
	}
	m := ansiEscape.ReplaceAllString(message, "")
	if !strings.Contains(m, "expect(locator)."+matcher) {
		return false
	}
	return strings.Contains(m, "getByTestId('"+locator+"')") || strings.Contains(m, `getByTestId("`+locator+`")`)
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

// controlsReachable reports whether the verdict can reach the control steps (CCE-V0-009 step 4).
func controlsReachable(head, base sideProof) bool {
	if !head.Qualified || !base.Qualified || head.Stability != playwrightruns.Deterministic {
		return false
	}
	for _, o := range head.Outcomes {
		if o.State != string(jstestprovider.StatePassed) {
			return false
		}
	}
	return true
}

var controlKinds = []behaviorfalsify.ControlKind{behaviorfalsify.WrongLocator, behaviorfalsify.WrongExpectedValue}

func notRunControls(reason string) []controlProof {
	out := []controlProof{}
	for _, k := range controlKinds {
		out = append(out, controlProof{ID: "control:" + string(k), Kind: string(k), Status: string(behaviorfalsify.StatusNotRun), Reasons: []string{reason}})
	}
	return out
}

// verdict is the CCE-V0-009 discrimination verdict: the first rule that holds.
func verdict(head, base sideProof, controls []controlProof) (string, string) {
	if !head.Qualified || !base.Qualified {
		return VerdictInconclusive, "run-without-qualified-receipt"
	}
	if head.Stability == playwrightruns.Intermittent {
		return VerdictUnstable, "head-pool-intermittent"
	}
	for _, o := range head.Outcomes {
		if failing(o.State) {
			return VerdictFailsAtHead, "head-repeat-failed"
		}
	}
	status := map[string]string{}
	for _, c := range controls {
		status[c.Kind] = c.Status
	}
	for _, k := range controlKinds {
		if status[string(k)] == string(behaviorfalsify.StatusSurvived) {
			return VerdictControlSurvived, "control-survived:" + string(k)
		}
	}
	for _, k := range controlKinds {
		if status[string(k)] != string(behaviorfalsify.StatusKilled) {
			return VerdictInconclusive, "control-not-killed:" + string(k)
		}
	}
	allAt, allPass := len(base.Outcomes) > 0, len(base.Outcomes) > 0
	for _, o := range base.Outcomes {
		allAt = allAt && o.AtOutcomeAssertion
		allPass = allPass && o.State == string(jstestprovider.StatePassed)
	}
	switch {
	case allAt:
		return VerdictDiscriminates, "every base repeat fails at the outcome assertion"
	case allPass:
		return VerdictPreserves, "regression-preservation check only; not evidence that the scenario detects the change"
	}
	return VerdictInconclusive, "base-mixed-or-failed-off-assertion"
}

// runControls applies the BBF wrong-locator and wrong-expected-value controls to the outcome
// assertion at head (CCE-V0-009 steps 4-5). The proof repository holds the head tree, the
// scenario and the config, and serves as application, test and documentation repository at one
// revision; the scenario file is the contract file.
func (e *proveEnv) runControls(ctx context.Context, head string, sc Scenario, headReceipt jstestprovider.Receipt) []controlProof {
	work, err := os.MkdirTemp("", "corvint-e2e-controls-")
	if err != nil {
		return notRunControls("control-workspace-unavailable")
	}
	defer os.RemoveAll(work)
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return notRunControls("control-workspace-unavailable")
	}
	repo, disposable := filepath.Join(work, "proof"), filepath.Join(work, "disposable")
	rev, err := e.proofRepo(ctx, head, repo, sc)
	if err != nil {
		return notRunControls("proof-repository-unavailable")
	}
	if err := os.MkdirAll(disposable, 0o755); err != nil {
		return notRunControls("control-workspace-unavailable")
	}
	if err := os.WriteFile(filepath.Join(disposable, behaviorfalsify.MarkerName), []byte(markerText), 0o644); err != nil {
		return notRunControls("control-workspace-unavailable")
	}
	request := e.controlRequest(sc, rev, repo, disposable, headReceipt)
	plan, err := behaviorfalsify.BuildPlan(request)
	if err != nil {
		reason := "plan-refused"
		if !strings.Contains(err.Error(), "/") {
			reason += ": " + err.Error()
		}
		return notRunControls(reason)
	}
	report, err := behaviorfalsify.Execute(ctx, plan, plan.Digest)
	if err != nil {
		out := notRunControls("execute-refused")
		for i := range out {
			out[i].PlanDigest = plan.Digest
		}
		return out
	}
	out := []controlProof{}
	for _, k := range controlKinds {
		c := controlProof{ID: "control:" + string(k), Kind: string(k), Status: string(behaviorfalsify.StatusNotRun),
			PlanDigest: plan.Digest, Reasons: []string{"control-result-missing"}}
		for _, r := range report.Results {
			if r.Kind == k {
				c.ID, c.Status, c.Reasons = r.ID, string(r.Status), append([]string{}, r.Reasons...)
			}
		}
		out = append(out, c)
	}
	return out
}

func (e *proveEnv) proofRepo(ctx context.Context, head, repo string, sc Scenario) (string, error) {
	if err := e.stageTree(ctx, e.root, head, repo, sc); err != nil {
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
	def := func(kind behaviorfalsify.ControlKind, replacement string) map[string]string {
		d := map[string]string{"tree": repo, "file": sc.Placement, "line": strconv.Itoa(sc.AssertionLine),
			"locator": sc.Locator, "matcher": sc.Matcher, "replacement": replacement, "kind": string(kind)}
		if sc.Value != "" {
			d["value"] = sc.Value
		}
		if e.packageJSON != "" {
			d["package_json"] = e.packageJSON
		}
		if e.lockfile != "" {
			d["lockfile"] = e.lockfile
		}
		return d
	}
	hook := e.hook
	return behaviorfalsify.Request{
		Target: target, Runner: runner,
		Controls: []behaviorfalsify.ControlSpec{
			{ID: "control:wrong-locator", Kind: behaviorfalsify.WrongLocator, Disposition: "run", Definition: def(behaviorfalsify.WrongLocator, wrongLocator), Hook: &hook},
			{ID: "control:wrong-expected-value", Kind: behaviorfalsify.WrongExpectedValue, Disposition: "run", Definition: def(behaviorfalsify.WrongExpectedValue, wrongValue), Hook: &hook},
		},
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
