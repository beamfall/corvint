package jstestprovider

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	defaultOutputLimit = 16 << 20
	defaultTimeout     = 5 * time.Minute
	// unitReportOutputLimit bounds the Vitest JSON report file RunUnit reads back
	// (js-live-test-provider-v0.md row report-not-written). It is its own named bound rather than
	// the external provider's 4 MiB externalOutputLimit, sized at least equal to defaultOutputLimit
	// so a multi-thousand-test suite with stack traces does not surface as report-not-written.
	unitReportOutputLimit = defaultOutputLimit
)

// Config is the shared subset of binding inputs both adapters need: the
// files whose identity must be pinned before evidence emits (AGENTS.md
// invariant 1), the tool's own argv, and the env keys a caller has declared
// worth observing (never the full process environment).
type Config struct {
	Dir             string
	TestFiles       []string
	ConfigFile      string
	PackageJSON     string
	Lockfile        string
	RunnerName      string
	RunnerVersion   string
	DeclaredEnvKeys []string
	Timeout         time.Duration
	OutputLimit     int
}

// identity collects the run's input identity under ctx (V1-3020): file
// hashing stops when ctx ends and the node --version probe is bounded by ctx,
// so a cancelled or timed-out operation returns its error instead of an
// identity collected past the deadline.
func (c Config) identity(ctx context.Context, argv []string) (Identity, error) {
	testDigests, err := digestFiles(ctx, c.TestFiles)
	if err != nil {
		return Identity{}, err
	}
	configDigest, err := digestFileContext(ctx, c.ConfigFile)
	if err != nil {
		return Identity{}, err
	}
	packageDigest, err := digestCombined(ctx, c.PackageJSON, c.Lockfile)
	if err != nil {
		return Identity{}, err
	}
	nodeVersion := nodeVersionFor(ctx, argv)
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	return Identity{
		TestFileDigests: testDigests,
		ConfigFile:      c.ConfigFile,
		ConfigDigest:    configDigest,
		PackageDigest:   packageDigest,
		NodeVersion:     nodeVersion,
		RunnerName:      c.RunnerName,
		RunnerVersion:   c.RunnerVersion,
		Environment:     declaredEnv(c.DeclaredEnvKeys),
		Argv:            argv,
	}, nil
}

func declaredEnv(keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = os.Getenv(k)
	}
	return out
}

func nodeVersion() string {
	return versionOfNode(context.Background(), "node")
}

// UnitConfig configures one Vitest unit run.
type UnitConfig struct {
	Config
	OutputFile string // where --outputFile writes, removed before the run; a temp file is used if empty.
}

// RunUnit runs `vitest run --reporter=json --outputFile=<path>` under
// procgroup containment and returns the bound receipt. A nonzero vitest exit
// status is expected on real test failures and is not itself an
// infrastructure error when the report already names the failure (a failed
// assertion, or the file-level fallback in ParseVitestJSON). But an error
// raised outside any test's own execution window (e.g. thrown from a timer
// callback after its test already passed) never touches Vitest's own
// numFailedTests/numFailedSuites counters, so it leaves every reported
// outcome "passed" while the process still exits nonzero - the report
// carries no signal at all. unexplainedNonzeroExit below catches exactly
// that gap so such a run abstains instead of publishing an all-green
// receipt (AGENTS.md invariant 2); see docs/decisions/0182 and
// js-live-test-provider-v0.md §2.1.
func RunUnit(ctx context.Context, cfg UnitConfig) (Receipt, error) {
	outputFile := cfg.OutputFile
	if outputFile == "" {
		f, err := os.CreateTemp("", "corvint-vitest-*.json")
		if err != nil {
			return Receipt{}, err
		}
		outputFile = f.Name()
		f.Close()
		defer os.Remove(outputFile)
	} else if err := os.Remove(outputFile); err != nil && !os.IsNotExist(err) {
		// A report left by an earlier run must never be read back as this run's.
		return Receipt{}, err
	}
	argv := []string{"npx", "vitest", "run", "--reporter=json", "--outputFile=" + outputFile}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	identityCtx, cancelIdentity := context.WithTimeout(ctx, timeout)
	identity, err := cfg.identity(identityCtx, argv)
	cancelIdentity()
	if err != nil {
		return Receipt{}, err
	}
	limit := cfg.OutputLimit
	if limit <= 0 {
		limit = defaultOutputLimit
	}
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: cfg.Dir, Env: os.Environ(), Timeout: timeout, OutputLimit: limit})
	receipt := Receipt{Kind: "unit", Identity: identity, AppBuildAtStart: AppBuildIdentity{Unknown: true, Reason: "vitest runs against source directly, not a served build"}}
	receipt.AppBuildAtPublish = receipt.AppBuildAtStart
	if boundary := unitBoundaryFailure(obs, "vitest", timeout); boundary != nil {
		receipt.Infrastructure = boundary
		receipt.Cancelled = obs.Cancelled
		return receipt, nil
	}
	data, err := readBoundedReport(outputFile, unitReportOutputLimit)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-not-written", Detail: err.Error()}
		return receipt, nil
	}
	tests, infra, err := ParseVitestJSON(data)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: err.Error()}
		return receipt, nil
	}
	receipt.Tests = tests
	receipt.Infrastructure = infra
	if receipt.Infrastructure == nil && unexplainedNonzeroExit(obs, tests) {
		receipt.Infrastructure = &InfrastructureFailure{
			Reason: "exit-status-unexplained",
			Detail: fmt.Sprintf("vitest exited %d but the JSON report has no failed or infrastructure outcome to account for it", obs.ExitStatus),
		}
	}
	return receipt, nil
}

// unexplainedNonzeroExit reports whether vitest exited nonzero while every
// parsed outcome says passed/skipped - the signature of an error raised
// outside any test's own run (see RunUnit's doc comment). A report that
// already carries a failed or infrastructure outcome fully explains a
// nonzero exit on its own and is left alone.
func unexplainedNonzeroExit(obs procgroup.Observation, tests []TestOutcome) bool {
	if !obs.ExitObserved || obs.ExitStatus == 0 {
		return false
	}
	for _, t := range tests {
		if t.State == StateFailed || t.State == StateInfrastructure {
			return false
		}
	}
	return true
}

func unitBoundaryFailure(obs procgroup.Observation, runner string, timeout time.Duration) *InfrastructureFailure {
	switch {
	case obs.TimedOut:
		return &InfrastructureFailure{Reason: "timeout", Detail: boundaryTimeoutDetail(runner, timeout)}
	case obs.OutputOverflow || obs.StdoutOverflow || obs.StderrOverflow:
		return &InfrastructureFailure{Reason: "output-overflow", Detail: "stdout or stderr exceeded its collection bound"}
	case obs.Cancelled:
		return &InfrastructureFailure{Reason: "cancelled", Detail: "run was cancelled before completion"}
	case !obs.Started:
		return &InfrastructureFailure{Reason: "start-failed", Detail: errString(obs.Err)}
	case !obs.WaitCompleted:
		return &InfrastructureFailure{Reason: "wait-not-completed", Detail: errString(obs.Err)}
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// E2EConfig configures one Playwright E2E run. Corvint starts and owns the app
// server itself under procgroup rather than relying on Playwright's
// `webServer` teardown, per the memo's named lifecycle risk (
// evidence/ipr-08-runner-selection.md section 6, item 1) and the roadmap
// acceptance line requiring proven descendant cleanup.
type E2EConfig struct {
	// ScopedTempOutput keeps native generated output in provider-owned scratch.
	// It does not admit caller-controlled runner options or paths.
	ScopedTempOutput bool
	Config
	Freshness          *FreshnessConfig
	ObserveDescendants bool
	ExternalServer     bool
	// CallerOwnedServer runs against a server the caller started and owns: the provider
	// never starts or stops it and only polls ServerReadyURL (NEA-V0-012).
	CallerOwnedServer      bool
	AppIdentity            string
	ServerArgv             []string
	ServerReadyURL         string
	ServerReadyLimit       time.Duration
	AppBuildDir            string // "" => unknown app build identity.
	TestArgv               []string
	ApplicationAttestation *ApplicationAttestationProvider
	SensitiveInputPolicy   *SensitiveInputPolicy
	RetainAttemptDetails   bool
	// KeepReporters appends the provider reporter to the project's reporter
	// list instead of replacing it (PWP-V0-010); external mode only.
	KeepReporters bool
	// negation is set only by RunNegate (LPCV-V0-058): trace recording, the
	// fault injection module and the scratch output directory.
	negation *negationRun
	// KeepReportersQualification is a decoded qualified record carried into
	// the keep-reporters receipt (PWP-V0-016); keep-reporters mode only.
	KeepReportersQualification *KeepReportersQualification
	// RunnerOutput, when set, receives every console byte of the external
	// runner, untruncated, from a bounded queue; a sink implementing
	// procgroup.IncompleteTee learns when its copy ended early (PWP-V0-028);
	// external mode only.
	RunnerOutput io.Writer
	// InterruptGrace, when positive, sends SIGINT on timeout and waits this
	// long before SIGTERM (PWP-V0-030); external mode only.
	InterruptGrace time.Duration
	// Preflight is a declared preflight run after readiness and before the
	// runner (PWP-V0-090); external mode only.
	Preflight *Preflight
}

// RunE2E starts the app server, waits for it to answer ServerReadyURL, runs
// the Playwright test command, then cancels the server's context and
// confirms procgroup's owned-process-group cleanup fired before returning.
// The app build identity is captured once before the readiness check passes
// and again after the test command completes; a mismatch is reported as
// StaleAppBuild rather than silently trusted.
// AdmitE2E returns the refusal RunE2E would give before anything starts, so
// watch can refuse before it reports a run (PWP-V0-038). Freshness runs keep
// their own admission inside RunFreshE2E.
func AdmitE2E(cfg E2EConfig) error {
	if cfg.Preflight != nil && (!cfg.ExternalServer || cfg.CallerOwnedServer || cfg.Freshness != nil) {
		return refuseE2E("preflight-requires-external-server")
	}
	if cfg.CallerOwnedServer {
		return admitCallerOwned(cfg)
	}
	if cfg.KeepReporters && (!cfg.ExternalServer || cfg.Freshness != nil) {
		return refuseE2E("keep-reporters-unsupported-mode")
	}
	if q := cfg.KeepReportersQualification; q != nil {
		if !cfg.KeepReporters {
			return refuseE2E("keep-reporters-qualification-requires-keep-reporters")
		}
		if q.Verdict != KeepReportersQualified || keepReportersQualificationError(*q) != nil {
			return refuseE2E(keepReportersQualificationInvalid)
		}
	}
	if cfg.Freshness != nil {
		// The freshness runner admits from the shared tuple table (PWP-V0-040).
		if m := AdmittedRuntimeTupleMismatch(FreshnessProfile, cfg.RunnerVersion, "", ""); m != nil {
			return &E2ERefusal{Code: "freshness-runtime-tuple-unqualified", Detail: m.detail(), Tuple: m}
		}
		return nil
	}
	if cfg.RetainAttemptDetails && (!cfg.ExternalServer || cfg.ApplicationAttestation != nil || cfg.SensitiveInputPolicy != nil) {
		return refuseE2E("external-attempt-details-composition-unsupported")
	}
	if cfg.ApplicationAttestation != nil && !cfg.ExternalServer {
		return refuseE2E("application-attestation-requires-external-server")
	}
	if cfg.SensitiveInputPolicy != nil && !cfg.ExternalServer {
		return refuseE2E("sensitive-input-redaction-requires-external-server")
	}
	if cfg.ExternalServer {
		return admitExternal(cfg)
	}
	return nil
}

func RunE2E(ctx context.Context, cfg E2EConfig) (Receipt, error) {
	if err := AdmitE2E(cfg); err != nil {
		return Receipt{}, err
	}
	if cfg.Freshness != nil {
		return RunFreshE2E(ctx, cfg)
	}
	if cfg.ExternalServer {
		return runExternal(ctx, cfg)
	}
	if cfg.CallerOwnedServer {
		return runCallerOwned(ctx, cfg)
	}
	argv := append(append([]string{}, cfg.ServerArgv...), cfg.TestArgv...)
	identity, err := cfg.identity(ctx, argv)
	if err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{Kind: "e2e", Identity: identity}

	atStart, err := DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		return Receipt{}, err
	}
	receipt.AppBuildAtStart = atStart

	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	serverDone := make(chan procgroup.Observation, 1)
	go func() {
		serverDone <- procgroup.Run(serverCtx, procgroup.Spec{
			Argv:        resolveArgv(cfg.ServerArgv),
			Dir:         cfg.Dir,
			Env:         os.Environ(),
			Timeout:     serverBudget(cfg.Timeout),
			OutputLimit: outputLimitOrDefault(cfg.OutputLimit),
		})
	}()

	readyLimit := cfg.ServerReadyLimit
	if readyLimit <= 0 {
		readyLimit = 15 * time.Second
	}
	if err := waitReady(ctx, cfg.ServerReadyURL, readyLimit); err != nil {
		cancelServer()
		serverObs := <-serverDone
		descendantsGone := serverObs.OwnedProcessGroupCleanup
		receipt.ServerDescendantsGone = &descendantsGone
		receipt.AppBuildAtPublish = AppBuildIdentity{Unknown: true, Reason: "no test run reached publish"}
		receipt.Infrastructure = &InfrastructureFailure{Reason: "server-not-ready", Detail: err.Error()}
		if ctx.Err() != nil {
			receipt.Cancelled = true
			receipt.Infrastructure = &InfrastructureFailure{Reason: "cancelled", Detail: "run was cancelled before completion"}
		}
		return receipt, nil
	}

	testTimeout := cfg.Timeout
	if testTimeout <= 0 {
		testTimeout = defaultTimeout
	}
	testObs := procgroup.Run(ctx, procgroup.Spec{
		Argv:        resolveArgv(cfg.TestArgv),
		Dir:         cfg.Dir,
		Env:         os.Environ(),
		Timeout:     testTimeout,
		OutputLimit: outputLimitOrDefault(cfg.OutputLimit),
	})

	cancelServer()
	serverObs := <-serverDone
	descendantsGone := serverObs.OwnedProcessGroupCleanup
	receipt.ServerDescendantsGone = &descendantsGone

	atPublish, err := DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		return Receipt{}, err
	}
	receipt.AppBuildAtPublish = atPublish
	receipt.StaleAppBuild = isStaleAppBuild(atStart, atPublish)

	if testObs.Cancelled || ctx.Err() != nil {
		receipt.Cancelled = true
		receipt.Infrastructure = &InfrastructureFailure{Reason: "cancelled", Detail: "run was cancelled before completion"}
		return receipt, nil
	}
	if boundary := e2eBoundaryFailure(testObs, testTimeout); boundary != nil {
		receipt.Infrastructure = boundary
		return receipt, nil
	}

	tests, infra, err := ParsePlaywrightJSON(testObs.Stdout)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: err.Error() + "; stderr: " + string(testObs.Stderr)}
		return receipt, nil
	}
	if infra != nil {
		receipt.Infrastructure = infra
		return receipt, nil
	}
	receipt.Tests = tests
	return receipt, nil
}

// e2eBoundaryFailure classifies the Playwright test process observation after
// cancellation has been ruled out, in the same order unitBoundaryFailure uses.
func e2eBoundaryFailure(obs procgroup.Observation, timeout time.Duration) *InfrastructureFailure {
	switch {
	case obs.TimedOut:
		return &InfrastructureFailure{Reason: "timeout", Detail: boundaryTimeoutDetail("playwright test", timeout)}
	case obs.OutputOverflow || obs.StdoutOverflow || obs.StderrOverflow:
		return &InfrastructureFailure{Reason: "output-overflow", Detail: "stdout or stderr exceeded its collection bound"}
	case !obs.Started:
		return &InfrastructureFailure{Reason: "start-failed", Detail: errString(obs.Err)}
	case !obs.WaitCompleted:
		return &InfrastructureFailure{Reason: "wait-not-completed", Detail: errString(obs.Err)}
	}
	return nil
}

// isStaleAppBuild reports a stale-app-build state only when both digests
// are actually known and differ. Either side being "unknown" (no app build
// directory configured) is a separate UNKNOWN freshness case, never
// silently read as fresh nor as stale.
func isStaleAppBuild(atStart, atPublish AppBuildIdentity) bool {
	return !atStart.Unknown && !atPublish.Unknown && atStart.Digest != atPublish.Digest
}

func serverBudget(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return timeout + time.Minute
}

func outputLimitOrDefault(limit int) int {
	if limit <= 0 {
		return defaultOutputLimit
	}
	return limit
}

func waitReady(ctx context.Context, url string, limit time.Duration) error {
	if url == "" {
		return nil
	}
	if err := probeAddress(url); err != nil {
		return err
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if status, err := loopbackGet(ctx, url, true); err == nil && status < 500 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return fmt.Errorf("wait-ready: %w", context.DeadlineExceeded)
}

// resolveArgv resolves argv[0] to an absolute path via PATH lookup, matching
// the containment runner's own expectation (see internal/console/process.go)
// that a spawned leader is identified unambiguously.
func resolveArgv(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return argv
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return argv
	}
	out := append([]string{abs}, argv[1:]...)
	return out
}

// boundaryTimeoutDetail names the runner that timed out and the effective
// --timeout value that bounded it, so an operator knows which bound to raise
// (PWP-V0-020).
func boundaryTimeoutDetail(runner string, timeout time.Duration) string {
	return runner + " did not complete within the --timeout " + timeout.String() + " bound"
}
