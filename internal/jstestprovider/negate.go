package jstestprovider

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/stepnegation"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// negationInjection is the Corvint-owned fault injection module whose
// SHA-256 the document binds (LPCV-V0-060).
//
//go:embed negation-injection.cjs
var negationInjection []byte

//go:embed negation-list-reporter.cjs
var negationListReporter []byte

// Typed negate refusals (LPCV-V0-057, LPCV-V0-058, LPCV-V0-066). Each is
// returned before any test run except an unqualified browser tuple, which
// the first baseline is the earliest observation of.
const (
	NegateModeUnsupported   = "negate-mode-unsupported"
	NegateTupleUnqualified  = "runtime-tuple-unqualified"
	NegateTestAmbiguous     = "negate-test-ambiguous"
	NegateDirtyRepository   = "negate-dirty-test-repository"
	NegateInvalidArguments  = "invalid-arguments"
	negateTraceLimit        = 256 << 20
	negateListLimit         = 16 << 20
	negateResultsLimit      = 1 << 20
	defaultStepMaxRuns      = 3
	defaultAllStepsMaxRuns  = 2
	maximumBaselineRepeat   = 5
	negateListTimeout       = 2 * time.Minute
	negateObservationBudget = 15 * time.Second
)

// NegateRefusal is a typed refusal: the command emits no document.
type NegateRefusal struct {
	Code    string
	Message string
}

func (r *NegateRefusal) Error() string { return r.Code + ": " + r.Message }

func refuse(code, format string, args ...any) error {
	return &NegateRefusal{Code: code, Message: fmt.Sprintf(format, args...)}
}

// NegateConfig selects one test and the steps to control (LPCV-V0-057).
type NegateConfig struct {
	// E2E is the external-server configuration; its TestArgv must be empty
	// because the selection below is the only one a negate run uses.
	E2E E2EConfig
	// Root is the test repository worktree root; Spec is worktree-relative.
	Root           string
	Spec           string
	Test           string
	Project        string
	Step           string
	AllSteps       bool
	MaxRuns        int
	BaselineRepeat int
	// allowUnqualifiedTuple is a test-only seam for live diagnostics on a
	// host without a qualified tuple. Its documents are never qualification
	// evidence (LPCV-V0-070).
	allowUnqualifiedTuple bool
}

// negationRun is the controlled-config overlay of one negate run.
type negationRun struct {
	outputDir    string
	module       string
	plan         string
	results      string
	listReporter string
	listOutput   string
	// started records that the runner process group was started, so its
	// cleanup observation applies (LPCV-V0-069).
	started bool
}

func (n *negationRun) prelude(quoted func(string) string) string {
	if n.module == "" {
		return ""
	}
	return "require(" + quoted(n.module) + ").install(" + quoted(n.plan) + ", " + quoted(n.results) + ");\n"
}

// override forces trace recording into the private scratch directory, no
// retries, one repeat and one worker, whatever the project declares.
func (n *negationRun) override(quoted func(string) string) string {
	reporter := ""
	if n.listReporter != "" {
		reporter = ", reporter: [[" + quoted(n.listReporter) + ", {output:" + quoted(n.listOutput) + "}]]"
	}
	out := quoted(n.outputDir)
	return "const negationUse = use => ({...(use || {}), trace: {mode: 'on', screenshots: false, snapshots: true, sources: false}});\n" +
		"module.exports = {...module.exports, outputDir: " + out + ", retries: 0, repeatEach: 1, workers: 1, use: negationUse(module.exports.use), projects: module.exports.projects?.map(project => ({...project, outputDir: " + out + ", retries: 0, repeatEach: 1, use: negationUse(project.use)}))" + reporter + "};\n"
}

// negation is one negate invocation's private state.
type negation struct {
	cfg      NegateConfig
	base     E2EConfig
	scratch  string
	spec     string
	key      stepnegation.TestKey
	origin   string
	budget   int
	used     int
	sequence int
	// attestation is the first /1 observation every later run must equal.
	attestation       *ApplicationAttestation
	drift             bool
	cleanupIncomplete bool
}

// observation is one run's receipt and parsed trace.
type observation struct {
	receipt Receipt
	test    *TestOutcome
	trace   *stepnegation.Trace
	applied map[string]int
	hidden  map[string]bool
}

// RunNegate executes one negate invocation (LPCV-V0-057 to LPCV-V0-066) and
// returns the document. A *NegateRefusal means no document is emitted. The
// scratch directory is removed on every path; a removal or owned process
// group cleanup that is not observed sets CleanupIncomplete.
func RunNegate(ctx context.Context, cfg NegateConfig) (stepnegation.Document, error) {
	n, err := admitNegate(cfg)
	if err != nil {
		return stepnegation.Document{}, err
	}
	// Every later read uses the admitted, defaulted configuration.
	cfg = n.cfg
	startRepository, failure := observeTestRepository(ctx, cfg.Root)
	if failure != "" {
		return stepnegation.Document{}, refuse(NegateDirtyRepository, "test repository is %s", strings.TrimPrefix(failure, "test-repository-"))
	}
	n.scratch, err = os.MkdirTemp("", "corvint-negate-")
	if err != nil {
		return stepnegation.Document{}, err
	}
	defer func() {
		if n.scratch != "" {
			_ = os.RemoveAll(n.scratch)
		}
	}()
	module := filepath.Join(n.scratch, "injection.cjs")
	lister := filepath.Join(n.scratch, "list-reporter.cjs")
	if err := os.WriteFile(module, negationInjection, 0o600); err != nil {
		return stepnegation.Document{}, err
	}
	if err := os.WriteFile(lister, negationListReporter, 0o600); err != nil {
		return stepnegation.Document{}, err
	}
	selected, err := n.list(ctx, lister)
	if err != nil {
		return stepnegation.Document{}, err
	}
	// --no-deps keeps dependency projects' tests from running alongside the
	// one selected test (LPCV-V0-058).
	n.base.TestArgv = []string{n.spec + ":" + strconv.Itoa(selected.line), "--retries=0", "--workers=1", "--repeat-each=1", "--no-deps"}
	if selected.project != "" {
		n.base.TestArgv = append(n.base.TestArgv, "--project="+selected.project)
	}
	n.key = stepnegation.TestKey{File: n.cfg.Spec, FullTitle: n.cfg.Test, Project: selected.project}

	document := stepnegation.Document{Schema: stepnegation.Schema, Mode: stepnegation.ModeStep, Steps: []stepnegation.Step{}, Inventory: stepnegation.Inventory{Steps: []stepnegation.InventoryStep{}}}
	if cfg.AllSteps {
		document.Mode = stepnegation.ModeAllSteps
	}
	document.Runs = stepnegation.Runs{Budget: n.budget, BaselineRepeat: cfg.BaselineRepeat}

	var first *observation
	baselineReason := ""
	for repeat := 0; repeat < cfg.BaselineRepeat; repeat++ {
		run, err := n.run(ctx, module, nil)
		if err != nil {
			return stepnegation.Document{}, err
		}
		if repeat == 0 {
			first = &run
			if run.test != nil && !n.cfg.allowUnqualifiedTuple && !qualifiedPlaywrightTuple(run.receipt, *run.test) {
				return stepnegation.Document{}, refuse(NegateTupleUnqualified, "the baseline observed an unqualified runner, Node or browser tuple")
			}
		}
		if reason := n.baselineFailure(run); reason != "" {
			baselineReason = reason
			break
		}
		document.Runs.BaselinePassed++
	}
	if first.trace != nil {
		document.Inventory = first.trace.Inventory()
	}
	document.Binding = n.binding(startRepository, first)

	if baselineReason != "" {
		document.Steps = n.unresolved(first.trace, baselineReason)
	} else if cfg.AllSteps {
		document.Steps = n.allSteps(ctx, module, first.trace)
	} else {
		document.Steps = n.singleStep(ctx, module, first.trace, document.Runs.BaselinePassed)
	}
	document.Runs.Used = n.used

	endCtx, cancel := context.WithTimeout(context.Background(), negateObservationBudget)
	endRepository, failure := observeTestRepository(endCtx, cfg.Root)
	cancel()
	if failure != "" || endRepository == nil || *endRepository != *startRepository || n.drift {
		for index := range document.Steps {
			step := &document.Steps[index]
			step.Witness, step.Requires = stepnegation.WitnessNone, ""
			step.Strength = testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: stepnegation.ReasonIdentityDrift, Anchors: []string{}}
			if step.Ordinal == 0 {
				step.PlanDigest = ""
			}
		}
	}
	if err := os.RemoveAll(n.scratch); err != nil {
		n.cleanupIncomplete = true
	}
	n.scratch = ""
	document.CleanupIncomplete = n.cleanupIncomplete
	return document, stepnegation.Validate(document)
}

func admitNegate(cfg NegateConfig) (*negation, error) {
	e := cfg.E2E
	switch {
	case !e.ExternalServer:
		return nil, refuse(NegateModeUnsupported, "negate runs only in external-server mode")
	case e.Freshness != nil:
		return nil, refuse(NegateModeUnsupported, "per-test freshness mode is not composed with negate")
	case e.SensitiveInputPolicy != nil:
		return nil, refuse(NegateModeUnsupported, "profile /2 is not composed with negate")
	case e.RetainAttemptDetails:
		return nil, refuse(NegateModeUnsupported, "profile /3 is not composed with negate")
	case e.KeepReporters:
		return nil, refuse(NegateModeUnsupported, "keep-reporters is not composed with negate")
	case len(e.TestArgv) != 0:
		return nil, refuse(NegateInvalidArguments, "negate selects its test by spec, title and project only")
	case cfg.AllSteps == (cfg.Step != ""):
		return nil, refuse(NegateInvalidArguments, "exactly one of --step and --all-steps is required")
	case cfg.Test == "":
		return nil, refuse(NegateInvalidArguments, "--test is required")
	}
	if cfg.BaselineRepeat == 0 {
		cfg.BaselineRepeat = 1
	}
	if cfg.MaxRuns == 0 {
		cfg.MaxRuns = defaultStepMaxRuns
		if cfg.AllSteps {
			cfg.MaxRuns = defaultAllStepsMaxRuns
		}
	}
	if cfg.BaselineRepeat < 1 || cfg.BaselineRepeat > maximumBaselineRepeat || cfg.MaxRuns < cfg.BaselineRepeat {
		return nil, refuse(NegateInvalidArguments, "--baseline-repeat is 1 to 5 and --max-runs is at least the baseline repeat")
	}
	if !filepath.IsAbs(cfg.Root) || cfg.Spec == "" || filepath.IsAbs(cfg.Spec) || filepath.Clean(cfg.Spec) != cfg.Spec || cfg.Spec == ".." || strings.HasPrefix(cfg.Spec, ".."+string(filepath.Separator)) {
		return nil, refuse(NegateInvalidArguments, "--spec must be a clean worktree-relative path")
	}
	spec := filepath.Join(cfg.Root, cfg.Spec)
	if info, err := os.Stat(spec); err != nil || !info.Mode().IsRegular() {
		return nil, refuse(NegateInvalidArguments, "--spec does not name a regular file")
	}
	base := e
	base.TestFiles = append([]string{}, e.TestFiles...)
	if !containsString(base.TestFiles, spec) {
		base.TestFiles = append(base.TestFiles, spec)
	}
	base.TestArgv = []string{spec}
	if err := admitExternal(base); err != nil {
		code, _, _ := strings.Cut(err.Error(), ":")
		if code == "external-playwright-version-unqualified" {
			code = NegateTupleUnqualified
		}
		return nil, refuse(code, "%s", err.Error())
	}
	profile := ExternalProfile
	if base.ApplicationAttestation != nil {
		profile = AttestedExternalProfile
	}
	tuple := ReceiptRuntimeTuple(Receipt{Profile: profile, Identity: Identity{RunnerVersion: base.RunnerVersion, NodeVersion: nodeVersion()}})
	if tuple != RuntimeTupleCandidate && !cfg.allowUnqualifiedTuple {
		return nil, refuse(NegateTupleUnqualified, "runner or Node version is not a qualified tuple (%s)", tuple)
	}
	return &negation{cfg: cfg, base: base, spec: spec, origin: stepnegation.Origin(base.ServerReadyURL), budget: cfg.MaxRuns}, nil
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

type listedTest struct {
	File      string   `json:"file"`
	Line      int      `json:"line"`
	TitlePath []string `json:"titlePath"`
	Project   string   `json:"project"`
}

type selectedTest struct {
	line    int
	project string
}

// list collects the configured tests with `playwright test --list`, which
// runs no test, and selects exactly one by file, full title and project.
func (n *negation) list(ctx context.Context, lister string) (selectedTest, error) {
	output := filepath.Join(n.scratch, "list.json")
	cfg := n.base
	cfg.TestArgv = []string{n.spec}
	if n.cfg.Project != "" {
		cfg.TestArgv = append(cfg.TestArgv, "--project="+n.cfg.Project)
	}
	cfg.negation = &negationRun{outputDir: filepath.Join(n.scratch, "list-output"), listReporter: lister, listOutput: output}
	listScratch := filepath.Join(n.scratch, "list")
	if err := os.Mkdir(listScratch, 0o700); err != nil {
		return selectedTest{}, err
	}
	_, argv, _, err := externalCommand(cfg, listScratch)
	if err != nil {
		return selectedTest{}, err
	}
	argv = append(argv, "--list")
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: cfg.Dir, Env: os.Environ(), Timeout: negateListTimeout, OutputLimit: externalOutputLimit})
	if !obs.OwnedProcessGroupCleanup {
		n.cleanupIncomplete = true
	}
	data, readErr := readBoundedReport(output, negateListLimit)
	if obs.ExitStatus != 0 || readErr != nil {
		return selectedTest{}, refuse(NegateTestAmbiguous, "the test list could not be collected")
	}
	var tests []listedTest
	if err := json.Unmarshal(data, &tests); err != nil {
		return selectedTest{}, refuse(NegateTestAmbiguous, "the test list does not decode")
	}
	return selectTest(tests, n.spec, n.cfg.Test, n.cfg.Project)
}

// selectTest picks the one listed test with the given file, full title and
// project; zero or several matches refuse negate-test-ambiguous.
func selectTest(tests []listedTest, spec, fullTitle, project string) (selectedTest, error) {
	want := canonicalPath(spec)
	var matches []selectedTest
	for _, test := range tests {
		if len(test.TitlePath) < 4 || canonicalPath(test.File) != want || strings.Join(test.TitlePath[3:], " > ") != fullTitle {
			continue
		}
		if project != "" && test.Project != project {
			continue
		}
		matches = append(matches, selectedTest{line: test.Line, project: test.Project})
	}
	if len(matches) != 1 {
		return selectedTest{}, refuse(NegateTestAmbiguous, "the selection matches %d tests", len(matches))
	}
	// A run selects by file:line and project, so a declaration line shared
	// with another test (a parameterized loop) would run both (LPCV-V0-057).
	shared := 0
	for _, test := range tests {
		if canonicalPath(test.File) == want && test.Line == matches[0].line && test.Project == matches[0].project {
			shared++
		}
	}
	if shared != 1 {
		return selectedTest{}, refuse(NegateTestAmbiguous, "the selected test shares its declaration line with %d other tests, so a run cannot select only it", shared-1)
	}
	return matches[0], nil
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// run executes one run of the selected test with the given faults installed.
// It spends one unit of the run budget.
func (n *negation) run(ctx context.Context, module string, installs []stepnegation.Install) (observation, error) {
	n.used++
	n.sequence++
	dir := filepath.Join(n.scratch, "run-"+strconv.Itoa(n.sequence))
	if err := os.Mkdir(dir, 0o700); err != nil {
		return observation{}, err
	}
	defer func() {
		if os.RemoveAll(dir) != nil {
			n.cleanupIncomplete = true
		}
	}()
	overlay := &negationRun{outputDir: filepath.Join(dir, "output")}
	if len(installs) != 0 {
		plan, err := json.Marshal(installs)
		if err != nil {
			return observation{}, err
		}
		overlay.module, overlay.plan, overlay.results = module, filepath.Join(dir, "plan.json"), filepath.Join(dir, "results.jsonl")
		if err := os.WriteFile(overlay.plan, plan, 0o600); err != nil {
			return observation{}, err
		}
	}
	cfg := n.base
	cfg.negation = overlay
	receipt, err := runExternal(ctx, cfg)
	if err != nil {
		return observation{}, err
	}
	result := observation{receipt: receipt}
	if overlay.started && (receipt.External == nil || !receipt.External.RunnerDescendantsGone) {
		n.cleanupIncomplete = true
	}
	n.observeAttestation(receipt)
	if len(receipt.Tests) == 1 {
		result.test = &receipt.Tests[0]
	}
	if data, ok := singleTrace(overlay.outputDir); ok {
		if trace, err := stepnegation.ParseTrace(data, n.cfg.Root); err == nil {
			result.trace = trace
		}
	}
	if overlay.results != "" {
		result.applied, result.hidden = readApplications(overlay.results)
	}
	return result, nil
}

// observeAttestation records /1 drift: every run's before and after
// attestation must equal the first one observed (LPCV-V0-066).
func (n *negation) observeAttestation(receipt Receipt) {
	attestation := receipt.ApplicationAttestation
	if n.base.ApplicationAttestation == nil {
		return
	}
	if attestation == nil || attestation.Before == nil || attestation.After == nil || len(attestation.Failures) != 0 {
		n.drift = true
		return
	}
	if n.attestation == nil {
		first := attestation.Before.Attestation
		n.attestation = &first
	}
	if !reflect.DeepEqual(*n.attestation, attestation.Before.Attestation) || !reflect.DeepEqual(*n.attestation, attestation.After.Attestation) {
		n.drift = true
	}
}

// singleTrace reads the one trace archive a single-test run records.
func singleTrace(outputDir string) ([]byte, bool) {
	var found []string
	_ = filepath.WalkDir(outputDir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() && entry.Name() == "trace.zip" {
			found = append(found, path)
		}
		return nil
	})
	if len(found) != 1 {
		return nil, false
	}
	data, err := readBoundedReport(found[0], negateTraceLimit)
	return data, err == nil
}

// readApplications sums the injection module's application counts.
func readApplications(path string) (map[string]int, map[string]bool) {
	applied, hidden := map[string]int{}, map[string]bool{}
	data, err := readBoundedReport(path, negateResultsLimit)
	if err != nil {
		return applied, hidden
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var entry struct {
			ID      string `json:"id"`
			Applied int    `json:"applied"`
			Hidden  bool   `json:"hidden"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil || entry.Applied < 0 {
			continue
		}
		applied[entry.ID] += entry.Applied
		hidden[entry.ID] = hidden[entry.ID] || entry.Hidden
	}
	return applied, hidden
}

// infrastructure reports a run-level infrastructure failure. Under the
// test-only unqualified-tuple seam the reporter cannot identify an
// unqualified browser, so only that one identity failure is tolerated there;
// a qualified invocation never takes this branch.
func (n *negation) infrastructure(r Receipt) bool {
	if r.Infrastructure == nil {
		return false
	}
	return !(n.cfg.allowUnqualifiedTuple && r.Infrastructure.Reason == "report-identity-unknown" && r.Infrastructure.Detail == "project-location-unknown")
}

// baselineFailure classifies one baseline run (LPCV-V0-059); "" is a pass.
func (n *negation) baselineFailure(run observation) string {
	r := run.receipt
	if n.infrastructure(r) || r.Cancelled || run.trace == nil || run.test == nil {
		return stepnegation.ReasonBaselineInfrastructure
	}
	t := run.test
	for _, attempt := range t.Attempts {
		if attempt.FailureKind == "browser-or-fixture" {
			return stepnegation.ReasonBaselineInfrastructure
		}
	}
	if t.State == StateInfrastructure || t.State == StateInterrupted {
		return stepnegation.ReasonBaselineInfrastructure
	}
	if t.State != StatePassed || len(t.Attempts) != 1 || !n.sameTest(*t) {
		return stepnegation.ReasonBaselineNotPassing
	}
	if len(run.trace.Failures) != 0 {
		return stepnegation.ReasonBaselineNotPassing
	}
	for _, step := range run.trace.Steps {
		if step.Failed {
			return stepnegation.ReasonBaselineNotPassing
		}
	}
	for _, assertion := range run.trace.Assertions {
		if assertion.Failed {
			return stepnegation.ReasonBaselineNotPassing
		}
	}
	return ""
}

// sameTest checks the reported test is the requested identity.
func (n *negation) sameTest(t TestOutcome) bool {
	parts := strings.SplitN(t.FullName, " > ", 4)
	if len(parts) != 4 || parts[3] != n.key.FullTitle || t.Anchor == nil || canonicalPath(t.Anchor.File) != canonicalPath(n.spec) {
		return false
	}
	return t.Project != nil && t.Project.Name == n.key.Project
}

func (n *negation) binding(repository *ApplicationRepositoryIdentity, first *observation) stepnegation.Binding {
	r := first.receipt
	binding := stepnegation.Binding{
		TestRepository:        stepnegation.Repository{RootCommit: repository.RootCommit, Revision: repository.Revision, Tree: repository.Tree},
		ConfigFile:            relativeTo(n.cfg.Root, n.base.ConfigFile),
		ConfigDigest:          r.Identity.ConfigDigest,
		SpecFile:              n.cfg.Spec,
		SpecDigest:            r.Identity.TestFileDigests[n.spec],
		Test:                  stepnegation.TestBinding{File: n.key.File, FullTitle: stepnegation.Screen(n.key.FullTitle), Project: n.key.Project},
		Runner:                stepnegation.Runner{Name: "playwright", Version: n.base.RunnerVersion, NodeVersion: r.Identity.NodeVersion, Tuple: ReceiptRuntimeTuple(r)},
		ReadinessOrigin:       n.origin,
		InjectionModuleDigest: sha256Hex(negationInjection),
	}
	if first.test != nil && first.test.Project != nil {
		binding.Test.Browser, binding.Test.Device = first.test.Project.Browser, first.test.Project.Device
	}
	if binding.Runner.Tuple == RuntimeTupleCandidate && first.test != nil && !qualifiedPlaywrightTuple(r, *first.test) {
		binding.Runner.Tuple = RuntimeTupleUnqualified
	}
	if n.base.ApplicationAttestation == nil {
		binding.Application = stepnegation.Application{Profile: ExternalProfile, Label: stepnegation.Screen(n.base.AppIdentity)}
	} else {
		binding.Application = stepnegation.Application{Profile: AttestedExternalProfile}
		if n.attestation != nil {
			a := n.attestation
			binding.Application.RootCommit, binding.Application.Revision, binding.Application.Tree = a.Repository.RootCommit, a.Repository.Revision, a.Repository.Tree
			binding.Application.InstanceKind, binding.Application.InstanceID, binding.Application.StartGeneration = a.Instance.Kind, a.Instance.ID, a.Instance.StartGeneration
		}
	}
	return binding
}

func relativeTo(root, path string) string {
	if relative, err := filepath.Rel(canonicalPath(root), canonicalPath(path)); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return "sha256:" + sha256Hex([]byte(path))
}

// unresolved gives every requested step the baseline reason.
func (n *negation) unresolved(trace *stepnegation.Trace, reason string) []stepnegation.Step {
	entry := func(ordinal int, title string) stepnegation.Step {
		return stepnegation.Step{Title: title, Ordinal: ordinal, Witness: stepnegation.WitnessNone, Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: reason, Anchors: []string{}}}
	}
	var inventory []stepnegation.InventoryStep
	if trace != nil {
		inventory = trace.Inventory().Steps
	}
	if !n.cfg.AllSteps {
		ordinals := stepOrdinals(trace, n.cfg.Step)
		if len(ordinals) == 1 {
			return []stepnegation.Step{entry(ordinals[0], inventory[ordinals[0]-1].Title)}
		}
		return []stepnegation.Step{entry(0, stepnegation.Screen(n.cfg.Step))}
	}
	if len(inventory) == 0 {
		return []stepnegation.Step{entry(0, "")}
	}
	steps := make([]stepnegation.Step, 0, len(inventory))
	for _, step := range inventory {
		steps = append(steps, entry(step.Ordinal, step.Title))
	}
	return steps
}

func stepOrdinals(trace *stepnegation.Trace, title string) []int {
	var ordinals []int
	if trace == nil {
		return nil
	}
	for _, step := range trace.Steps {
		if step.Title == title {
			ordinals = append(ordinals, step.Ordinal)
		}
	}
	return ordinals
}

// singleStep measures one named step: the network fault first, then the
// DOM fault unless the network fault killed it (LPCV-V0-063).
func (n *negation) singleStep(ctx context.Context, module string, baseline *stepnegation.Trace, baselinePasses int) []stepnegation.Step {
	ordinals := stepOrdinals(baseline, n.cfg.Step)
	unresolved := func(reason string) []stepnegation.Step {
		return []stepnegation.Step{{Title: stepnegation.Screen(n.cfg.Step), Witness: stepnegation.WitnessNone, Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: reason, Anchors: []string{}}}}
	}
	switch len(ordinals) {
	case 0:
		return unresolved(stepnegation.ReasonStepNotFound)
	case 1:
	default:
		return unresolved(stepnegation.ReasonStepTitleAmbiguous)
	}
	derivation := stepnegation.Derive(baseline, n.key, ordinals[0], n.origin)
	if derivation.Network == nil && derivation.DOM == nil {
		return []stepnegation.Step{stepnegation.Underivable(derivation)}
	}
	var network, dom *testvalidity.Axis
	if derivation.Network != nil && n.used < n.budget {
		axis := n.faulted(ctx, module, baseline, derivation, derivation.Network, baselinePasses)
		network = &axis
	}
	if derivation.DOM != nil && (network == nil || network.State != testvalidity.StrengthKilled) && (derivation.Network == nil || network != nil) && n.used < n.budget {
		axis := n.faulted(ctx, module, baseline, derivation, derivation.DOM, baselinePasses)
		dom = &axis
	}
	return []stepnegation.Step{stepnegation.ResolveSingle(derivation, network, dom)}
}

func (n *negation) faulted(ctx context.Context, module string, baseline *stepnegation.Trace, derivation stepnegation.Derivation, fault *stepnegation.Fault, baselinePasses int) testvalidity.Axis {
	run := n.faultedRun(ctx, module, []stepnegation.Install{fault.Install})
	return stepnegation.EvaluateSingle(baseline, derivation, fault, run, baselinePasses)
}

// faultedRun executes one faulted run and maps it onto the pass rule's
// observation. An interrupted invocation starts no further run.
func (n *negation) faultedRun(ctx context.Context, module string, installs []stepnegation.Install) stepnegation.Run {
	if ctx.Err() != nil {
		return stepnegation.Run{Status: "interrupted"}
	}
	result, err := n.run(ctx, module, installs)
	if err != nil {
		return stepnegation.Run{Infrastructure: true}
	}
	run := stepnegation.Run{Trace: result.trace, Applied: result.applied, Hidden: result.hidden}
	r := result.receipt
	if result.test == nil || n.infrastructure(r) || !n.sameTest(*result.test) {
		run.Infrastructure = true
		return run
	}
	if r.Cancelled {
		run.Status = "interrupted"
		return run
	}
	t := result.test
	run.Status, run.Attempts = string(t.State), len(t.Attempts)
	for _, attempt := range t.Attempts {
		if attempt.FailureKind == "browser-or-fixture" {
			run.Infrastructure = true
		}
	}
	if t.State == StateInfrastructure {
		run.Infrastructure = true
	}
	return run
}

// allSteps measures every derivable step in one joint run (LPCV-V0-064).
func (n *negation) allSteps(ctx context.Context, module string, baseline *stepnegation.Trace) []stepnegation.Step {
	inventory := baseline.Inventory()
	derivations := make([]stepnegation.Derivation, 0, len(inventory.Steps))
	for _, step := range inventory.Steps {
		derivations = append(derivations, stepnegation.Derive(baseline, n.key, step.Ordinal, n.origin))
	}
	faults, entries := stepnegation.PlanJoint(derivations)
	if len(faults) != 0 {
		if n.used < n.budget {
			installs := make([]stepnegation.Install, 0, len(faults))
			for _, joint := range faults {
				installs = append(installs, joint.Fault.Install)
			}
			for ordinal, entry := range stepnegation.EvaluateJoint(faults, n.faultedRun(ctx, module, installs)) {
				entries[ordinal] = entry
			}
		} else {
			for _, joint := range faults {
				d := joint.Derivation
				entries[d.Ordinal] = stepnegation.Step{PlanDigest: joint.Fault.Digest, Title: stepnegation.Screen(d.Title), Ordinal: d.Ordinal, Witness: stepnegation.WitnessNone,
					Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: stepnegation.ReasonRunBudgetExhausted, Anchors: []string{"plan:" + joint.Fault.Digest}}}
			}
		}
	}
	steps := make([]stepnegation.Step, 0, len(entries))
	for _, step := range inventory.Steps {
		if entry, ok := entries[step.Ordinal]; ok {
			steps = append(steps, entry)
		}
	}
	return steps
}
