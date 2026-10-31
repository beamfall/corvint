package behaviorfalsify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Shared-server isolation profile (BBF-V0-019..023).
const (
	IsolationSharedServer      = "shared-server"
	ExternalStateSharedServer  = "declared-shared-server"
	IsolationCheckSchema       = "corvint-browser-behavior-isolation-check/0"
	IsolationObservationSchema = "corvint-browser-behavior-isolation-observation/0"
	isolationPhaseReset        = "reset"
	isolationPhaseVerify       = "verify"
	isolationOutputLimit       = 64 << 10
	isolationVerifyTimeout     = 5 * time.Second
	// isolationReserve keeps the post-cleanup identity check inside the wall-clock budget.
	isolationReserve = isolationVerifyTimeout + processShutdownAllowance
)

// Runner-neutral provider profile (BBF-V0-024..028) and its RSpec binding (BBF-V0-029..033).
const (
	ProviderGoTest        = "go-test"
	ProviderRSpec         = "rspec"
	rspecReportName       = ".corvint-behavior-rspec.json"
	PerturbationSchema    = "corvint-behavior-control-perturbation/0"
	perturbationLimit     = 1 << 20
	providerOutputLimit   = 16 << 20
	maxProviderExamples   = 256
	maxPerturbationKeys   = 64
	providerRunnerBrowser = "none"
)

var (
	goTestNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]{0,251}$`)
	goPackagePattern  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*(/[A-Za-z0-9_][A-Za-z0-9._-]*)*$`)
	// rspecExamplePattern is one project-relative spec file and one RSpec scoped example id, the
	// form RSpec's JSON report gives as an example's id without its leading "./" (BBF-V0-029).
	rspecExamplePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*(/[A-Za-z0-9_][A-Za-z0-9._-]*)*\.rb\[[1-9][0-9]{0,8}(:[1-9][0-9]{0,8}){0,31}\]$`)
)

// rspecForcedEnvironment is the closed environment the RSpec binding always sets: Bundler never
// rewrites the lockfile and SPEC_OPTS cannot inject formatter or filter options (BBF-V0-030).
var rspecForcedEnvironment = map[string]string{
	"BUNDLE_FROZEN": "true",
	"SPEC_OPTS":     "",
}

// forcedEnvironment is the provider's forced environment.
func forcedEnvironment(name string) map[string]string {
	if name == ProviderRSpec {
		return rspecForcedEnvironment
	}
	return providerForcedEnvironment
}

// providerForcedEnvironment is the closed Go environment the provider always sets; neither the
// declared nor the perturbation environment may name these keys (BBF-V0-026).
var providerForcedEnvironment = map[string]string{
	"CGO_ENABLED": "0",
	"GOENV":       "off",
	"GOFLAGS":     "-mod=readonly",
	"GOPROXY":     "off",
	"GOSUMDB":     "off",
	"GOTELEMETRY": "off",
	"GOTOOLCHAIN": "local",
	"GOWORK":      "off",
}

// DeclaredIsolation is the caller's explicit declaration that controls run against one shared,
// long-lived local server. It is an observation boundary, not a sandbox (BBF-V0-019).
type DeclaredIsolation struct {
	Profile             string  `json:"profile"`
	ServerURL           string  `json:"server_url"`
	ApplicationIdentity string  `json:"application_identity"`
	Reset               Command `json:"reset"`
}

// ServerIdentity is a server URL and application identity with the source that established it
// (BBF-V0-216).
type ServerIdentity struct {
	ServerURL           string `json:"server_url"`
	ApplicationIdentity string `json:"application_identity"`
	Source              string `json:"source"`
}

// Server identity sources (BBF-V0-216). The runner never contacts the server (BBF-V0-020), so every
// identity it records is IdentityHookReported; IdentityObserved is reserved for an independent
// probe of the server, which no profile performs yet.
const (
	IdentityObserved     = "observed"
	IdentityHookReported = "hook-reported"
	// IsolationHookReportedGap is the report-level coverage gap that keeps a declared isolation
	// resting on any hook-reported identity below complete. It contains '/', which no control ID
	// may contain, so it cannot collide with a criterion-level gap.
	IsolationHookReportedGap = "isolation/hook-reported-identity"
)

// IsolationObservation records the reset before and the identity check after one attempt.
type IsolationObservation struct {
	ResetProcess  ProcessEvidence  `json:"reset_process"`
	Before        *ServerIdentity  `json:"before,omitempty"`
	VerifyProcess *ProcessEvidence `json:"verify_process,omitempty"`
	After         *ServerIdentity  `json:"after,omitempty"`
	// resetRaw and verifyRaw are the phases' stdout, held in memory so an evidence receipt can
	// retain them; they are never part of the report wire (BBF-V0-022).
	resetRaw, verifyRaw []byte
}

// IsolationCheck is the bounded JSON request a reset/health hook receives on stdin.
type IsolationCheck struct {
	Schema              string `json:"schema"`
	PlanDigest          string `json:"plan_digest"`
	Phase               string `json:"phase"`
	ControlID           string `json:"control_id"`
	Attempt             int    `json:"attempt"`
	ServerURL           string `json:"server_url"`
	ApplicationIdentity string `json:"application_identity"`
}

// IsolationReport is the closed JSON a reset/health hook prints.
type IsolationReport struct {
	Schema              string `json:"schema"`
	PlanDigest          string `json:"plan_digest"`
	Phase               string `json:"phase"`
	ControlID           string `json:"control_id"`
	Attempt             int    `json:"attempt"`
	ServerURL           string `json:"server_url"`
	ApplicationIdentity string `json:"application_identity"`
}

// ProviderSpec selects the runner-neutral provider, its pinned toolchain and the examples it
// reruns under each control (BBF-V0-024).
type ProviderSpec struct {
	Name                string   `json:"name"`
	Toolchain           Command  `json:"toolchain"`
	Package             string   `json:"package"`
	Examples            []string `json:"examples"`
	PerturbationEnvKeys []string `json:"perturbation_env_keys"`
}

// Perturbation is the closed JSON a runner-neutral control hook prints: the declared
// environment values that perturb the rerun examples (BBF-V0-025).
type Perturbation struct {
	Schema             string            `json:"schema"`
	PlanDigest         string            `json:"plan_digest"`
	PerturbationSHA256 string            `json:"perturbation_sha256"`
	Attempt            int               `json:"attempt"`
	Environment        map[string]string `json:"environment"`
}

func canonicalProfiles(request Request) Request {
	if isolation := request.Isolation; isolation != nil {
		clone := *isolation
		request.Isolation = &clone
	}
	if provider := request.Provider; provider != nil {
		clone := *provider
		clone.Examples = append([]string(nil), provider.Examples...)
		clone.PerturbationEnvKeys = append([]string(nil), provider.PerturbationEnvKeys...)
		slices.Sort(clone.Examples)
		slices.Sort(clone.PerturbationEnvKeys)
		request.Provider = &clone
	}
	if auth := request.Authentication; auth != nil {
		clone := *auth
		clone.StorageState = append([]StorageStateFile(nil), auth.StorageState...)
		slices.SortFunc(clone.StorageState, func(a, b StorageStateFile) int { return strings.Compare(a.Path, b.Path) })
		request.Authentication = &clone
	}
	return request
}

func validateIsolation(request Request) error {
	isolation := request.Isolation
	if isolation == nil {
		if request.ExternalState != "none" {
			return errors.New("persistent external state is outside the admitted boundary")
		}
		return nil
	}
	if isolation.Profile != IsolationSharedServer || request.ExternalState != ExternalStateSharedServer {
		return errors.New("declared isolation requires the shared-server profile and external state")
	}
	if !loopbackServerURL(isolation.ServerURL) {
		return errors.New("declared server URL must be a credential-free loopback http(s) URL")
	}
	if !digestPattern.MatchString(isolation.ApplicationIdentity) && !gitOIDPattern.MatchString(isolation.ApplicationIdentity) {
		return errors.New("declared application identity must be a digest or Git revision")
	}
	if err := validateCommand(isolation.Reset); err != nil {
		return errors.New("reset hook: " + err.Error())
	}
	if time.Duration(request.WallClockSeconds)*time.Second <= cleanupReserve+isolationReserve {
		return errors.New("wall clock budget does not exceed cleanup and isolation reserve")
	}
	return nil
}

func loopbackServerURL(raw string) bool {
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "\x00\r\n\t ") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// validateProvider admits the runner identity of either the default Playwright profile or the
// runner-neutral provider profile.
func validateProvider(request Request) error {
	provider := request.Provider
	if provider == nil {
		if request.Runner.Runner != "playwright" {
			return errors.New("invalid runner identity")
		}
		return nil
	}
	runner := request.Runner
	exampleValid := goTestNamePattern.MatchString
	switch {
	case runner.Browser != providerRunnerBrowser || runner.BrowserVersion != providerRunnerBrowser || runner.Runner != provider.Name:
		return errors.New("runner-neutral provider identity mismatch")
	case provider.Name == ProviderGoTest && filepath.Base(runner.ConfigFile) == "go.mod":
	case provider.Name == ProviderRSpec && (filepath.Base(runner.ConfigFile) == ".rspec" || filepath.Base(runner.ConfigFile) == "Gemfile"):
		exampleValid = rspecExample
	default:
		return errors.New("runner-neutral provider identity mismatch")
	}
	if err := validateCommand(provider.Toolchain); err != nil {
		return errors.New("provider toolchain: " + err.Error())
	}
	// A "..." anywhere would make go test expand the one approved package into many (BBF-V0-024).
	if strings.Contains(provider.Package, "...") || provider.Package != "." && (!repositoryRelative(provider.Package) || !goPackagePattern.MatchString(filepath.ToSlash(provider.Package))) {
		return errors.New("provider package must be one repository-relative directory")
	}
	if request.Target.Project != provider.Package {
		return errors.New("target project must name the provider package")
	}
	if len(provider.Examples) == 0 || len(provider.Examples) > maxProviderExamples || !uniqueStrings(provider.Examples, exampleValid) {
		return errors.New("provider examples must be 1-256 unique top-level Go test names or RSpec example ids")
	}
	if !slices.Contains(provider.Examples, request.Target.AssertionID) {
		return errors.New("target assertion must be a selected example")
	}
	for _, control := range request.Controls {
		if control.ApplicationMarker != nil {
			return errors.New("application markers are not supported by the runner-neutral profile")
		}
		for _, name := range append(slices.Clone(control.UnrelatedCriteria), control.RequiredSetup...) {
			if name == request.Target.AssertionID || !slices.Contains(provider.Examples, name) {
				return errors.New("unrelated and setup observations must be other selected examples")
			}
		}
	}
	if len(provider.PerturbationEnvKeys) == 0 || len(provider.PerturbationEnvKeys) > maxPerturbationKeys || !uniqueStrings(provider.PerturbationEnvKeys, envPattern.MatchString) {
		return errors.New("provider perturbation environment keys invalid")
	}
	for _, key := range append(slices.Clone(provider.PerturbationEnvKeys), request.DeclaredEnvKeys...) {
		if _, forced := forcedEnvironment(provider.Name)[key]; forced || reservedEnvironmentKeys[key] || (provider.Name == ProviderGoTest && key == "GOROOT") {
			return errors.New("environment key overrides the provider boundary")
		}
	}
	for _, key := range provider.PerturbationEnvKeys {
		if slices.Contains(request.DeclaredEnvKeys, key) {
			return errors.New("perturbation key is also a declared environment key")
		}
	}
	if !repositoryClean(request.Repositories.Test) {
		return errors.New("provider test repository has uncommitted changes")
	}
	return nil
}

// rspecExample admits one RSpec example selector: a local spec file path, whose components never
// start with a dot, with one scoped id, at most 1024 bytes (BBF-V0-029).
func rspecExample(name string) bool {
	return len(name) <= 1024 && rspecExamplePattern.MatchString(name)
}

// runIsolationPhase runs the pinned reset/health hook for one phase and returns its stdout and
// the identity it reported, or a nil identity when the hook failed or its report does not bind
// this attempt.
func runIsolationPhase(ctx context.Context, plan Plan, control PlannedControl, attempt int, phase string, timeout time.Duration) (ProcessEvidence, *ServerIdentity, []byte) {
	isolation := plan.Request.Isolation
	input, _ := marshalJSON(IsolationCheck{Schema: IsolationCheckSchema, PlanDigest: plan.Digest, Phase: phase, ControlID: control.ID, Attempt: attempt, ServerURL: isolation.ServerURL, ApplicationIdentity: isolation.ApplicationIdentity})
	raw, process := runPinned(ctx, isolation.Reset, plan.Request.DisposableRoot, plan.Request.DeclaredEnvKeys, "isolation-"+phase, input, timeout, isolationOutputLimit)
	return process, isolationIdentity(plan, control.ID, attempt, phase, process, raw), raw
}

// isolationIdentity derives the identity a phase's stdout reports for this attempt, or nil when
// the hook failed or its report does not bind the attempt. The executor and the offline receipt
// verifier share it, so a recorded identity always re-derives from retained bytes (BBF-V0-022).
func isolationIdentity(plan Plan, controlID string, attempt int, phase string, process ProcessEvidence, raw []byte) *ServerIdentity {
	if !processSucceeded(process) {
		return nil
	}
	var report IsolationReport
	if Decode(raw, &report) != nil || report.Schema != IsolationObservationSchema || report.PlanDigest != plan.Digest || report.Phase != phase || report.ControlID != controlID || report.Attempt != attempt || len(report.ServerURL) > 4096 || len(report.ApplicationIdentity) > 4096 {
		return nil
	}
	return &ServerIdentity{ServerURL: report.ServerURL, ApplicationIdentity: report.ApplicationIdentity, Source: IdentityHookReported}
}

// hookReportedIsolation reports whether a plan's declared isolation rests on any identity that was
// not independently observed, or on none at all; such a report carries IsolationHookReportedGap so
// a forged hook identity can never yield a complete verdict (BBF-V0-216).
func hookReportedIsolation(plan Plan, results []ControlResult) bool {
	if plan.Request.Isolation == nil {
		return false
	}
	observed := false
	for _, result := range results {
		for _, attempt := range result.Attempts {
			if attempt.Isolation == nil {
				continue
			}
			for _, identity := range []*ServerIdentity{attempt.Isolation.Before, attempt.Isolation.After} {
				if identity == nil {
					continue
				}
				if identity.Source != IdentityObserved {
					return true
				}
				observed = true
			}
		}
	}
	return !observed
}

// declaredServer reports whether identity names the declared server, whatever its source.
func declaredServer(identity ServerIdentity, isolation *DeclaredIsolation) bool {
	return identity.ServerURL == isolation.ServerURL && identity.ApplicationIdentity == isolation.ApplicationIdentity
}

// isolationBytesVerify reports whether a retained phase stdout matches its recorded process digest
// and re-derives exactly the recorded identity.
func isolationBytesVerify(plan Plan, controlID string, attempt int, phase string, process ProcessEvidence, raw []byte, identity *ServerIdentity) bool {
	if process.StdoutSHA256 == "" {
		if len(raw) != 0 {
			return false
		}
	} else if digestBytes(raw) != process.StdoutSHA256 {
		return false
	}
	derived := isolationIdentity(plan, controlID, attempt, phase, process, raw)
	return derived == nil && identity == nil || derived != nil && identity != nil && *derived == *identity
}

// isolationFailure is the infrastructure reason an attempt's isolation observation yields, or
// nil when the declared server identity held before and after the control (BBF-V0-021).
func isolationFailure(plan Plan, result AttemptResult) string {
	isolation := plan.Request.Isolation
	if isolation == nil {
		if result.Isolation != nil {
			return "isolation-observation-unexpected"
		}
		return ""
	}
	observed := result.Isolation
	switch {
	case observed == nil || !processSucceeded(observed.ResetProcess) || observed.Before == nil:
		return "isolation-reset-failed"
	case !declaredServer(*observed.Before, isolation):
		return "isolation-identity-drift"
	case observed.VerifyProcess == nil || !processSucceeded(*observed.VerifyProcess) || observed.After == nil:
		return "isolation-health-failed"
	case !declaredServer(*observed.After, isolation):
		return "isolation-identity-drift"
	}
	return ""
}

// runProviderAttempt decodes the hook's perturbation, reruns the selected examples through the
// pinned provider and returns the receipt synthesized from the provider's native events.
func runProviderAttempt(ctx context.Context, plan Plan, control PlannedControl, attempt int, hookRaw []byte, timeout time.Duration, result *AttemptResult) {
	provider := plan.Request.Provider
	var perturbation Perturbation
	if Decode(hookRaw, &perturbation) != nil || perturbation.Schema != PerturbationSchema || perturbation.PlanDigest != plan.Digest || perturbation.PerturbationSHA256 != control.PerturbationSHA256 || perturbation.Attempt != attempt || !validPerturbation(provider, perturbation.Environment) {
		result.Reasons = append(result.Reasons, "perturbation-output-invalid")
		return
	}
	if !repositoryAtRevision(plan.Request.Repositories.Test, plan.Request.Target.TestRevision) {
		result.Reasons = append(result.Reasons, "test-repository-drift")
		return
	}
	end := time.Now().Add(timeout)
	scratch, err := os.MkdirTemp(acceptanceTempRoot(), "corvint-behavior-provider-")
	if err != nil {
		result.Reasons = append(result.Reasons, "provider-staging-unavailable")
		return
	}
	defer os.RemoveAll(scratch)
	// The examples run in a disposable clone at the approved revision, never in the protected
	// test repository, so a test that writes a tracked file cannot modify it (BBF-V0-026).
	checkout := filepath.Join(scratch, "repository")
	if stageTestRepository(ctx, plan.Request.Repositories.Test, plan.Request.Target.TestRevision, checkout, timeout) != nil {
		result.Reasons = append(result.Reasons, "provider-staging-unavailable")
		return
	}
	native, process := runProvider(ctx, plan, perturbation.Environment, scratch, checkout, time.Until(end))
	result.ProviderProcess = &process
	if !repositoryAtRevision(plan.Request.Repositories.Test, plan.Request.Target.TestRevision) {
		result.Reasons = append(result.Reasons, "test-repository-mutated")
		return
	}
	if !providerProcessSucceeded(process) {
		return
	}
	receipt, reason := providerReceipt(plan, control, attempt, native)
	if reason != "" {
		result.Reasons = append(result.Reasons, reason)
		return
	}
	result.Receipt = &receipt
}

func validPerturbation(provider *ProviderSpec, environment map[string]string) bool {
	for key, value := range environment {
		if !slices.Contains(provider.PerturbationEnvKeys, key) || len(value) > 4096 || strings.ContainsRune(value, 0) {
			return false
		}
	}
	return true
}

// stageTestRepository clones the test repository's committed objects into checkout and detaches
// it at revision; the clone copies objects without hardlinks and runs no template hooks.
func stageTestRepository(ctx context.Context, repository, revision, checkout string, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("provider staging budget exhausted")
	}
	budget := gitrun.NewBudget(2, timeout)
	parent := filepath.Dir(checkout)
	if _, err := gitrun.Run(ctx, budget, gitrun.Options{Dir: parent, Env: isolatedGitEnvironment(parent), StdoutLimit: 1 << 20}, "clone", "-q", "--no-hardlinks", "--no-checkout", "--template=", "--", repository, checkout); err != nil {
		return err
	}
	_, err := gitrun.Run(ctx, budget, gitrun.Options{Dir: checkout, Env: isolatedGitEnvironment(checkout), StdoutLimit: 1 << 20}, "checkout", "-q", "--detach", revision)
	return err
}

func runProvider(ctx context.Context, plan Plan, perturbation map[string]string, scratch, checkout string, timeout time.Duration) ([]byte, ProcessEvidence) {
	provider := plan.Request.Provider
	if timeout <= 0 {
		return nil, ProcessEvidence{Error: "command budget exhausted"}
	}
	if err := validateCommand(provider.Toolchain); err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	// The GOROOT members a go test run reads are digested at the driver's check, so the private
	// copy staged below runs only those bytes (BBF-V0-820).
	goroot, gorootWant := filepath.Dir(filepath.Dir(provider.Toolchain.Path)), ""
	if provider.Name != ProviderRSpec {
		if info, err := os.Stat(filepath.Join(goroot, "pkg", "tool")); err != nil || !info.IsDir() {
			return nil, ProcessEvidence{Error: "provider toolchain GOROOT unavailable"}
		}
		var err error
		if gorootWant, err = gorootDigest(goroot, ""); errors.Is(err, errGOROOTUnsupported) {
			return nil, ProcessEvidence{Error: err.Error()}
		} else if err != nil {
			return nil, ProcessEvidence{Error: "provider toolchain GOROOT unavailable"}
		}
	}
	afterToolchainChecked(provider.Toolchain)
	if provider.Name == ProviderRSpec {
		return runRSpecProvider(ctx, plan, perturbation, scratch, filepath.Join(checkout, filepath.FromSlash(provider.Package)), timeout)
	}
	pattern := "."
	if provider.Package != "." {
		pattern = "./" + filepath.ToSlash(provider.Package)
	}
	values := map[string]string{
		"HOME": scratch, "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "CORVINT_BEHAVIOR_COMMAND_ROLE": "provider",
		"GOCACHE": filepath.Join(scratch, "cache"), "GOPATH": filepath.Join(scratch, "gopath"), "TMPDIR": scratch,
	}
	for key, value := range declaredEnvironment(plan.Request.DeclaredEnvKeys) {
		values[key] = value
	}
	for key, value := range perturbation {
		values[key] = value
	}
	for key, value := range providerForcedEnvironment {
		values[key] = value
	}
	// The verified driver bytes run from a private GOROOT holding the digested copy of its tools
	// and standard library, so a swap of any of them after the check cannot run (BBF-V0-214,
	// BBF-V0-820).
	staged, cleanup, err := stageVerified(provider.Toolchain)
	if err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	defer cleanup()
	private := filepath.Join(scratch, "goroot")
	defer os.RemoveAll(private)
	if err := stageGOROOT(goroot, gorootWant, private); err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	driver, err := placeVerifiedDriver(staged, private, goroot, provider.Toolchain.Path)
	if err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	afterToolchainStaged(provider.Toolchain)
	values["GOROOT"] = private
	argv := []string{driver, "test", "-json", "-count=1", "-run", "^(" + strings.Join(provider.Examples, "|") + ")$", pattern}
	return observeProcess(ctx, argv, checkout, environmentList(values), nil, timeout, providerOutputLimit)
}

// afterToolchainStaged is a test seam between staging and execution (BBF-V0-214).
var afterToolchainStaged = func(Command) {}

// afterToolchainChecked is a test seam between the toolchain digest check and anything that runs
// it, which exercises the BBF-V0-214 limit that the RSpec toolchain still runs in place (V1-2215).
var afterToolchainChecked = func(Command) {}

// runRSpecProvider runs the pinned RSpec command in the staged project directory with RSpec's
// stock JSON formatter writing to a fixed project-relative report, so a container wrapper that
// shares the directory writes it where the host reads it, and returns the report bytes, which are
// the native receipt (BBF-V0-030). A missing, irregular or over-bound report is nil.
func runRSpecProvider(ctx context.Context, plan Plan, perturbation map[string]string, scratch, project string, timeout time.Duration) ([]byte, ProcessEvidence) {
	provider := plan.Request.Provider
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, ProcessEvidence{Error: "provider project directory unavailable"}
	}
	defer root.Close()
	if _, err := root.Lstat(rspecReportName); !errors.Is(err, os.ErrNotExist) {
		return nil, ProcessEvidence{Error: "provider report path is occupied"}
	}
	values := map[string]string{"HOME": scratch, "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "CORVINT_BEHAVIOR_COMMAND_ROLE": "provider", "TMPDIR": scratch}
	for key, value := range declaredEnvironment(plan.Request.DeclaredEnvKeys) {
		values[key] = value
	}
	for key, value := range perturbation {
		values[key] = value
	}
	for key, value := range rspecForcedEnvironment {
		values[key] = value
	}
	// The verified toolchain bytes run from a private copy, so a swap after the check cannot run
	// (BBF-V0-820).
	staged, cleanup, err := stageVerified(provider.Toolchain)
	if err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	defer cleanup()
	argv := append([]string{staged, "--format", "json", "--out", rspecReportName}, provider.Examples...)
	_, process := observeProcess(ctx, argv, project, environmentList(values), nil, timeout, providerOutputLimit)
	return readRSpecReport(root), process
}

func readRSpecReport(root *os.Root) []byte {
	info, err := root.Lstat(rspecReportName)
	if err != nil || !info.Mode().IsRegular() || info.Size() > providerOutputLimit {
		return nil
	}
	file, err := root.Open(rspecReportName)
	if err != nil {
		return nil
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !os.SameFile(info, opened) {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(file, providerOutputLimit+1))
	if err != nil || len(data) > providerOutputLimit {
		return nil
	}
	return data
}

// providerProcessSucceeded admits go test's and RSpec's documented exit statuses: 0 when every
// selected example passed and 1 when one failed. Every other boundary loss is infrastructure.
func providerProcessSucceeded(evidence ProcessEvidence) bool {
	exit := evidence.Exit
	evidence.Exit = 0
	return (exit == 0 || exit == 1) && processSucceeded(evidence)
}

type goTestEvent struct {
	Action      string `json:"Action"`
	Package     string `json:"Package"`
	Test        string `json:"Test"`
	Output      string `json:"Output"`
	FailedBuild string `json:"FailedBuild"`
}

// providerReceipt synthesizes the hook receipt from go test -json events: each selected example
// maps to its own terminal action, and a failure whose output reports a panic is not an
// assertion failure (BBF-V0-027).
func providerReceipt(plan Plan, control PlannedControl, attempt int, stdout []byte) (HookReceipt, string) {
	if plan.Request.Provider.Name == ProviderRSpec {
		return rspecReceipt(plan, control, attempt, stdout)
	}
	// Outcomes are keyed by (package, test); the approved pattern names exactly one package, so
	// events from a second package are contradictory, never attributed to an example (BBF-V0-027).
	type example struct{ pkg, test string }
	states, panicked, packagePanic := map[example]string{}, map[example]bool{}, false
	pkg, pkgSeen := "", false
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var event goTestEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return HookReceipt{}, "provider-output-invalid"
		}
		if event.Action == "build-fail" || event.FailedBuild != "" {
			return HookReceipt{}, "provider-build-failed"
		}
		if event.Package != "" || event.Test != "" {
			if pkgSeen && event.Package != pkg {
				return HookReceipt{}, "provider-output-contradictory"
			}
			pkg, pkgSeen = event.Package, true
		}
		if event.Action == "output" && strings.Contains(event.Output, "panic: ") {
			// A panic attributed to no top-level example taints every failure in the run.
			if name, _, _ := strings.Cut(event.Test, "/"); name != "" {
				panicked[example{event.Package, name}] = true
			} else {
				packagePanic = true
			}
		}
		if event.Test == "" || strings.Contains(event.Test, "/") {
			continue
		}
		switch event.Action {
		case "pass", "fail", "skip":
			key := example{event.Package, event.Test}
			if _, seen := states[key]; seen {
				return HookReceipt{}, "provider-output-contradictory"
			}
			states[key] = map[string]string{"pass": "passed", "fail": "failed", "skip": "skipped"}[event.Action]
		}
	}
	observe := func(name string) (string, string) {
		key := example{pkg, name}
		state, ok := states[key]
		if !ok {
			return "not_run", ""
		}
		if state == "failed" {
			if panicked[key] || packagePanic {
				return state, "panic"
			}
			return state, "assertion"
		}
		return state, ""
	}
	return synthesizedReceipt(plan, control, attempt, stdout, observe), ""
}

// synthesizedReceipt builds the hook receipt from per-example observations of the native report.
func synthesizedReceipt(plan Plan, control PlannedControl, attempt int, native []byte, observe func(string) (string, string)) HookReceipt {
	target := plan.Request.Target
	state, kind := observe(target.AssertionID)
	receipt := HookReceipt{
		Schema: HookSchema, PlanDigest: plan.Digest, PerturbationSHA256: control.PerturbationSHA256,
		Target: target, Runner: plan.Request.Runner, Attempt: attempt, NativeReceiptSHA256: digestBytes(native),
		TestOutcome:       state,
		TargetObservation: CriterionObservation{CriterionID: target.CriterionID, AssertionID: target.AssertionID, State: state, FailureKind: kind},
		Unrelated:         []CriterionObservation{}, Setup: []SetupObservation{}, Artifacts: []Artifact{},
	}
	for _, name := range control.UnrelatedCriteria {
		state, kind := observe(name)
		receipt.Unrelated = append(receipt.Unrelated, CriterionObservation{CriterionID: name, AssertionID: name, State: state, FailureKind: kind})
	}
	for _, name := range control.RequiredSetup {
		state, _ := observe(name)
		receipt.Setup = append(receipt.Setup, SetupObservation{ID: name, State: state})
	}
	return receipt
}

// rspecAssertionErrors are the exception classes RSpec raises for a failed expectation; any other
// exception is an error outside the assertion (BBF-V0-031).
var rspecAssertionErrors = map[string]bool{
	"RSpec::Expectations::ExpectationNotMetError":          true,
	"RSpec::Expectations::MultipleExpectationsNotMetError": true,
	"RSpec::Mocks::MockExpectationError":                   true,
}

type rspecReportDocument struct {
	Examples *[]struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Exception *struct {
			Class string `json:"class"`
		} `json:"exception"`
	} `json:"examples"`
	Summary *struct {
		ExampleCount            *int `json:"example_count"`
		ErrorsOutsideOfExamples *int `json:"errors_outside_of_examples_count"`
	} `json:"summary"`
}

// rspecReceipt synthesizes the hook receipt from RSpec's JSON report: each selected example takes
// the status of the report example whose id, without its leading "./", equals it (BBF-V0-031).
func rspecReceipt(plan Plan, control PlannedControl, attempt int, report []byte) (HookReceipt, string) {
	var document rspecReportDocument
	if report == nil || json.Unmarshal(report, &document) != nil || document.Examples == nil || document.Summary == nil || document.Summary.ExampleCount == nil || document.Summary.ErrorsOutsideOfExamples == nil {
		return HookReceipt{}, "provider-output-invalid"
	}
	if *document.Summary.ErrorsOutsideOfExamples != 0 {
		return HookReceipt{}, "provider-build-failed"
	}
	if *document.Summary.ExampleCount != len(*document.Examples) {
		return HookReceipt{}, "provider-output-contradictory"
	}
	type outcome struct{ state, kind string }
	outcomes := map[string]outcome{}
	for _, example := range *document.Examples {
		id := strings.TrimPrefix(example.ID, "./")
		if _, seen := outcomes[id]; seen || !slices.Contains(plan.Request.Provider.Examples, id) {
			return HookReceipt{}, "provider-output-contradictory"
		}
		switch example.Status {
		case "passed":
			outcomes[id] = outcome{state: "passed"}
		case "pending":
			outcomes[id] = outcome{state: "skipped"}
		case "failed":
			kind := "panic"
			if example.Exception != nil && rspecAssertionErrors[example.Exception.Class] {
				kind = "assertion"
			}
			outcomes[id] = outcome{state: "failed", kind: kind}
		default:
			return HookReceipt{}, "provider-output-invalid"
		}
	}
	observe := func(name string) (string, string) {
		if observed, ok := outcomes[name]; ok {
			return observed.state, observed.kind
		}
		return "not_run", ""
	}
	return synthesizedReceipt(plan, control, attempt, report, observe), ""
}

// providerInfrastructureFailure reports whether a runner-neutral attempt lost its provider
// execution boundary: the provider process failed or its package did not build.
func providerInfrastructureFailure(plan Plan, result AttemptResult) bool {
	if plan.Request.Provider == nil {
		return false
	}
	if result.ProviderProcess != nil && !providerProcessSucceeded(*result.ProviderProcess) {
		return true
	}
	return slices.Contains(result.Reasons, "provider-build-failed")
}

func repositoryClean(root string) bool {
	status, err := gitBytes(root, "status", "--porcelain=v1", "--untracked-files=all")
	return err == nil && len(status) == 0
}

func repositoryAtRevision(root, revision string) bool {
	head, err := gitBytes(root, "rev-parse", "HEAD")
	return err == nil && strings.TrimSpace(string(head)) == revision && repositoryClean(root)
}
