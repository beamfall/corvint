package jstestprovider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	defaultOutputLimit = 16 << 20
	defaultTimeout     = 5 * time.Minute
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

func (c Config) identity(argv []string) (Identity, error) {
	testDigests, err := digestFiles(c.TestFiles)
	if err != nil {
		return Identity{}, err
	}
	configDigest, err := digestFile(c.ConfigFile)
	if err != nil {
		return Identity{}, err
	}
	packageDigest, err := digestCombined(c.PackageJSON, c.Lockfile)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		TestFileDigests: testDigests,
		ConfigFile:      c.ConfigFile,
		ConfigDigest:    configDigest,
		PackageDigest:   packageDigest,
		NodeVersion:     nodeVersion(),
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
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
	identity, err := cfg.identity(argv)
	if err != nil {
		return Receipt{}, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	limit := cfg.OutputLimit
	if limit <= 0 {
		limit = defaultOutputLimit
	}
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: cfg.Dir, Env: os.Environ(), Timeout: timeout, OutputLimit: limit})
	receipt := Receipt{Kind: "unit", Identity: identity, AppBuildAtStart: AppBuildIdentity{Unknown: true, Reason: "vitest runs against source directly, not a served build"}}
	receipt.AppBuildAtPublish = receipt.AppBuildAtStart
	if boundary := unitBoundaryFailure(obs); boundary != nil {
		receipt.Infrastructure = boundary
		receipt.Cancelled = obs.Cancelled
		return receipt, nil
	}
	data, err := os.ReadFile(outputFile)
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

func unitBoundaryFailure(obs procgroup.Observation) *InfrastructureFailure {
	switch {
	case obs.TimedOut:
		return &InfrastructureFailure{Reason: "timeout", Detail: "vitest did not complete within the bound"}
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
	Config
	ObserveDescendants     bool
	ExternalServer         bool
	AppIdentity            string
	ServerArgv             []string
	ServerReadyURL         string
	ServerReadyLimit       time.Duration
	AppBuildDir            string // "" => unknown app build identity.
	TestArgv               []string
	ApplicationAttestation *ApplicationAttestationProvider
	SensitiveInputPolicy   *SensitiveInputPolicy
}

// RunE2E starts the app server, waits for it to answer ServerReadyURL, runs
// the Playwright test command, then cancels the server's context and
// confirms procgroup's owned-process-group cleanup fired before returning.
// The app build identity is captured once before the readiness check passes
// and again after the test command completes; a mismatch is reported as
// StaleAppBuild rather than silently trusted.
func RunE2E(ctx context.Context, cfg E2EConfig) (Receipt, error) {
	if cfg.ApplicationAttestation != nil && !cfg.ExternalServer {
		return Receipt{}, errors.New("application-attestation-requires-external-server")
	}
	if cfg.SensitiveInputPolicy != nil && !cfg.ExternalServer {
		return Receipt{}, errors.New("sensitive-input-redaction-requires-external-server")
	}
	if cfg.ExternalServer {
		return runExternal(ctx, cfg)
	}
	argv := append(append([]string{}, cfg.ServerArgv...), cfg.TestArgv...)
	identity, err := cfg.identity(argv)
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
	if boundary := e2eBoundaryFailure(testObs); boundary != nil {
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
func e2eBoundaryFailure(obs procgroup.Observation) *InfrastructureFailure {
	switch {
	case obs.TimedOut:
		return &InfrastructureFailure{Reason: "timeout", Detail: "playwright test did not complete within the bound"}
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
	deadline := time.Now().Add(limit)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return &net.OpError{Op: "wait-ready", Err: context.DeadlineExceeded}
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
