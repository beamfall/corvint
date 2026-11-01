package jstestprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const ExternalProfile = "corvint-playwright-external/0"
const externalOutputLimit = 4 << 20

// externalMaxConfigInputs and externalMaxConfigInputBytes bound the
// reporter-observed config-input set that is re-opened and digested at bind time
// and copied into the receipt (PWP-V0-021). The count admits the non-node_modules
// module graph of a large suite (the earlier 256 refused mid-size suites); the
// byte bound, the sum of every path and digest length, keeps the set at a quarter
// of the 4 MiB provider-document bound (testvaliditydoc.MaxInputBytes). Beyond
// either bound the report is refused as config-inputs-overflow.
const (
	externalMaxConfigInputs     = 4096
	externalMaxConfigInputBytes = 1 << 20
)

// qualifiedBundledBrowser is the exact Playwright-bundled headless-shell
// identity one Playwright release pins (PWP-V0-008). Each release pins its own
// registry revision, so the tuple is keyed by the exact runner version.
type qualifiedBundledBrowser struct {
	revision         string
	observedVersion  string
	manifestVersion  string
	executableSHA256 string
	executableSuffix string
}

const qualifiedBundledBrowserName = "chromium-headless-shell"

// qualifiedBundledBrowsers names the bundled tuple of each runner version
// whose live matrix passed: 1.63.0 (issue 175) and 1.64.0 (PWP-V0-032,
// GitHub #1263). 1.60.0 predates the browser binding and has no entry.
var qualifiedBundledBrowsers = map[string]qualifiedBundledBrowser{
	"1.63.0": {
		revision:         "1243",
		observedVersion:  "Google Chrome for Testing 153.0.8010.12",
		manifestVersion:  "153.0.8010.12",
		executableSHA256: "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282",
		executableSuffix: "/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell",
	},
	"1.64.0": {
		revision:         "1248",
		observedVersion:  "Google Chrome for Testing 156.0.8078.4",
		manifestVersion:  "156.0.8078.4",
		executableSHA256: "77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634",
		executableSuffix: "/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell",
	},
}

// qualifiedSystemBrowsers names the exact configured system-Chrome version
// each runner release was live-qualified with (PWP-V0-008; PWP-V0-037 for
// 1.64.0). A version is never inherited across runner releases.
var qualifiedSystemBrowsers = map[string]string{
	"1.63.0": "Google Chrome 153.0.8010.48",
	"1.64.0": "Google Chrome 155.0.8059.85",
}

const qualifiedSystemBrowserExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// qualifiedPlaywrightVersions is the closed, ordered list of exact
// @playwright/test versions external mode admits (PWP-V0-032). Admission is
// not qualification: the receipt-level tuple and per-test browser identity
// still decide whether an outcome can project passing execution.
var qualifiedPlaywrightVersions = []string{"1.60.0", "1.63.0", "1.64.0"}

// QualifiedPlaywrightVersions returns a copy of the exact runner versions
// external mode admits, for refusal documents and help text.
func QualifiedPlaywrightVersions() []string {
	return slices.Clone(qualifiedPlaywrightVersions)
}

// QualifiedBundledBrowserExecutable returns the headless shell executable, relative to the
// Playwright browsers path, whose identity qualifies runner version's bundled tuple, and false
// when the version has none. A caller can check the browser is installed before running.
func QualifiedBundledBrowserExecutable(version string) (string, bool) {
	b, ok := qualifiedBundledBrowsers[version]
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(b.executableSuffix, "/"), true
}

func qualifiedPlaywrightVersion(version string) bool {
	return slices.Contains(qualifiedPlaywrightVersions, version)
}

//go:embed qualified-reporter.cjs
var qualifiedReporter []byte

type qualifiedReport struct {
	Schedule    *ExecutionSchedule `json:"schedule,omitempty"`
	ConfigFiles map[string]string  `json:"configFiles"`
	Version     string             `json:"version"`
	Files       map[string]string  `json:"files"`
	Status      string             `json:"status"`
	Tests       []TestOutcome      `json:"tests"`
	Errors      []string           `json:"errors"`
	// ErrorLocations is emitted only by a `/3` run with run-level errors: one reported location,
	// or null, per error (PWP-V3-009).
	ErrorLocations       []*Anchor             `json:"errorLocations,omitempty"`
	SensitiveInputPolicy *SensitiveInputPolicy `json:"sensitiveInputPolicy,omitempty"`
	// ProjectReporters is emitted only by a keep-reporters run (PWP-V0-011).
	ProjectReporters []reportedProjectReporter `json:"projectReporters,omitempty"`
}

type playwrightBrowserIdentity struct {
	Platform               *string `json:"platform"`
	Arch                   *string `json:"arch"`
	NodeVersion            *string `json:"nodeVersion"`
	BrowserType            *string `json:"browserType"`
	BrowserVersion         *string `json:"browserVersion"`
	Channel                *string `json:"channel"`
	ExecutableSource       *string `json:"executableSource"`
	ExecutableName         *string `json:"executableName"`
	ExecutablePath         *string `json:"executablePath"`
	ExecutableSHA256       *string `json:"executableSha256"`
	BrowserRevision        *string `json:"browserRevision"`
	ManifestBrowserVersion *string `json:"manifestBrowserVersion"`
	HeadlessShellAvailable *bool   `json:"headlessShellAvailable"`
}

type playwrightUseIdentity struct {
	BaseURL        *string                   `json:"baseURL"`
	CorvintBrowser playwrightBrowserIdentity `json:"corvintBrowser"`
	BrowserName    *string                   `json:"browserName"`
	Channel        *string                   `json:"channel"`
	ConnectOptions json.RawMessage           `json:"connectOptions"`
	Headless       *bool                     `json:"headless"`
	LaunchOptions  struct {
		ExecutablePath *string `json:"executablePath"`
	} `json:"launchOptions"`
}

func runExternal(ctx context.Context, cfg E2EConfig) (Receipt, error) {
	if err := admitExternal(cfg); err != nil {
		return Receipt{}, err
	}
	scratch, err := os.MkdirTemp("", "corvint-playwright-")
	if err != nil {
		return Receipt{}, err
	}
	defer os.RemoveAll(scratch)
	config, argv, reportPath, err := externalCommand(cfg, scratch)
	if err != nil {
		return Receipt{}, err
	}
	identity, err := cfg.identity(ctx, argv)
	if err != nil {
		return Receipt{}, err
	}
	profile := externalRequestProfile(cfg)
	lifecycle := &ExternalLifecycle{ReadyURL: cfg.ServerReadyURL, DeclaredAppIdentity: cfg.AppIdentity, Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ConfigOverride: config}
	if cfg.KeepReporters {
		// Entries stay unobserved (nil) until the reporter's closed list binds.
		lifecycle.ProjectReporters = &ProjectReporters{Effects: "unknown", Qualification: cfg.KeepReportersQualification}
	}
	r := Receipt{Profile: profile, Kind: "e2e", Identity: identity, External: lifecycle, SensitiveInputPolicy: cfg.SensitiveInputPolicy, Tests: []TestOutcome{}}
	var provider *preparedApplicationAttestationProvider
	if cfg.ApplicationAttestation != nil {
		provider, err = prepareApplicationAttestationProvider(*cfg.ApplicationAttestation, cfg.Dir, scratch, identity.Environment)
		if err != nil {
			r.ApplicationAttestation = &ApplicationAttestationReceipt{Failures: []string{err.Error()}}
			r.Infrastructure = &InfrastructureFailure{Reason: err.Error(), Detail: "application attestation provider could not be prepared"}
			return r, nil
		}
		r.ApplicationAttestation = &provider.receipt
		var repositoryFailure string
		r.TestRepositoryAtStart, repositoryFailure = observeTestRepository(ctx, cfg.Dir)
		if repositoryFailure != "" {
			setApplicationAttestationFailure(&r, repositoryFailure)
			return r, nil
		}
	}
	r.AppBuildAtStart, err = DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		return r, err
	}
	r.AppBuildAtPublish = AppBuildIdentity{Unknown: true, Reason: "not-observed"}
	readyLimit := cfg.ServerReadyLimit
	if readyLimit <= 0 {
		readyLimit = 15 * time.Second
	}
	if err := externalReady(ctx, cfg.ServerReadyURL, readyLimit); err != nil {
		r.Infrastructure = &InfrastructureFailure{Reason: "server-not-ready", Detail: err.Error()}
		r.Cancelled = ctx.Err() != nil
		return r, nil
	}
	lifecycle.ReadyAtStart = true
	if cfg.Preflight != nil {
		// A failed preflight is infrastructure before tests: the runner never
		// starts, so the receipt carries no test verdicts (PWP-V0-091).
		var failure *InfrastructureFailure
		r.Preflight, failure = runPreflight(ctx, cfg.Preflight, cfg.Dir)
		if failure != nil {
			r.Infrastructure = failure
			r.Cancelled = ctx.Err() != nil
			return r, nil
		}
	}
	if provider != nil {
		before, failure := provider.observe(ctx)
		r.ApplicationAttestation.Before = before
		if failure != "" {
			setApplicationAttestationFailure(&r, failure)
			return r, nil
		}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if cfg.negation != nil {
		cfg.negation.started = true
	}
	spec := procgroup.Spec{Argv: resolveArgv(argv), Dir: cfg.Dir, Env: os.Environ(), Timeout: timeout, OutputLimit: externalOutputLimit,
		OverflowPolicy: procgroup.OverflowTruncate, InterruptGrace: cfg.InterruptGrace, ObserveDescendants: cfg.ObserveDescendants, AfterStart: externalRunnerAfterStart}
	if cfg.RunnerOutput != nil {
		// One lock keeps a stdout and a stderr write whole; only the tee
		// forwarders wait on it, never the drains (PWP-V0-028).
		tee := &lockedWriter{w: cfg.RunnerOutput}
		spec.StdoutTee, spec.StderrTee = tee, tee
	}
	obs := procgroup.Run(ctx, spec)
	lifecycle.Runner = runnerProcess(obs)
	r.DescendantObservation = obs.DescendantObservation
	r.RunnerResources = obs.Usage
	lifecycle.RunnerDescendantsGone = obs.OwnedProcessGroupCleanup
	// The caller may already be cancelled. A separate bounded observation checks
	// survival without extending ownership to the external service.
	postCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	lifecycle.ReadyAtPublish = externalReady(postCtx, cfg.ServerReadyURL, time.Second) == nil
	cancel()
	r.AppBuildAtPublish, err = DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		r.AppBuildAtPublish = AppBuildIdentity{Unknown: true, Reason: err.Error()}
	}
	r.StaleAppBuild = isStaleAppBuild(r.AppBuildAtStart, r.AppBuildAtPublish)
	if provider != nil {
		if !provider.unchanged() {
			setApplicationAttestationFailure(&r, "application-attestation-provider-drift")
		}
		postCtx, postCancel := context.WithTimeout(context.Background(), provider.timeout)
		after, failure := provider.observe(postCtx)
		postCancel()
		r.ApplicationAttestation.After = after
		if failure != "" {
			setApplicationAttestationFailure(&r, failure)
		} else if r.ApplicationAttestation.Before != nil {
			for _, drift := range compareApplicationAttestations(r.ApplicationAttestation.Before.Attestation, after.Attestation) {
				setApplicationAttestationFailure(&r, drift)
			}
		}
		repositoryCtx, repositoryCancel := context.WithTimeout(context.Background(), 15*time.Second)
		r.TestRepositoryAtPublish, failure = observeTestRepository(repositoryCtx, cfg.Dir)
		repositoryCancel()
		if failure != "" {
			setApplicationAttestationFailure(&r, failure)
		} else if r.TestRepositoryAtStart == nil || *r.TestRepositoryAtStart != *r.TestRepositoryAtPublish {
			setApplicationAttestationFailure(&r, "test-repository-drift")
		}
	}
	after, identityErr := cfg.identity(ctx, argv)
	lifecycle.InputsUnchanged = identityErr == nil && reflect.DeepEqual(identity, after)
	r.Cancelled = obs.Cancelled || ctx.Err() != nil
	if boundary := externalBoundaryFailure(obs, timeout); boundary != nil {
		r.Infrastructure = boundary
	}
	if !lifecycle.ReadyAtPublish {
		r.Infrastructure = &InfrastructureFailure{Reason: "server-unavailable-at-publish", Detail: "external server did not answer readiness after the run"}
	}
	if !lifecycle.RunnerDescendantsGone {
		r.Infrastructure = &InfrastructureFailure{Reason: "runner-cleanup-unknown", Detail: "owned runner process group cleanup was not observed"}
	}
	if cfg.Preflight != nil && digestFileOrEmpty(cfg.Preflight.File) != cfg.Preflight.Digest {
		lifecycle.InputsUnchanged = false
	}
	if !lifecycle.InputsUnchanged {
		r.Infrastructure = &InfrastructureFailure{Reason: "input-identity-changed", Detail: "bound config, test, package, preflight declaration or declared environment changed"}
	}
	data, readErr := readBoundedReport(reportPath, externalOutputLimit)
	if readErr != nil {
		if r.Infrastructure == nil {
			r.Infrastructure = &InfrastructureFailure{Reason: "report-not-written", Detail: readErr.Error()}
		}
		return r, nil
	}
	var report qualifiedReport
	report, decodeFailure := decodeQualifiedReport(data, profile)
	if decodeFailure == nil && !cfg.KeepReporters && report.ProjectReporters != nil {
		decodeFailure = externalReportFailure(profile, "report-unparseable", "unknown field \"projectReporters\"")
	}
	if decodeFailure != nil {
		r.Infrastructure = decodeFailure
		return r, nil
	}
	if profile == SensitiveExternalProfile {
		if !reflect.DeepEqual(report.SensitiveInputPolicy, cfg.SensitiveInputPolicy) {
			r.Infrastructure = &InfrastructureFailure{Reason: "sensitive-input-policy-drift", Detail: "reporter policy differs from the caller-bound additive policy"}
			return r, nil
		}
		r.Tests = report.Tests
		if findings := ValidateSensitiveInputEvidence(r); len(findings) != 0 {
			r.Infrastructure = &InfrastructureFailure{Reason: SensitiveInputUnredacted, Detail: findings[0].Path}
			return r, nil
		}
	} else {
		r.Tests = report.Tests
		externalHygiene(r.Tests)
	}
	if r.Identity.NodeVersion == "" {
		r.Identity.NodeVersion = recordedNodeVersion(r.Tests)
	}
	if err := bindQualifiedReport(&r, report); err != nil {
		r.Infrastructure = externalReportFailure(profile, "report-identity-unknown", err.Error())
	}
	if cfg.KeepReporters {
		if err := bindProjectReporters(&r, report.ProjectReporters); err != nil && r.Infrastructure == nil {
			r.Infrastructure = externalReportFailure(profile, err.Error(), "kept project reporter entries were not observed in the closed shape")
		}
	}
	if len(report.Errors) > 0 {
		r.Infrastructure = externalReportFailure(profile, ReasonReporterGlobalError, strings.Join(report.Errors, "\n"))
		r.Infrastructure.Location = firstErrorLocation(report)
	}
	// A timed-out runner's exit status is explained by the timeout, including
	// an interrupted report written during the interrupt grace (PWP-V0-030).
	if obs.ExitStatus != 0 && report.Status == "passed" && !obs.TimedOut {
		r.Infrastructure = &InfrastructureFailure{Reason: "exit-status-unexplained", Detail: "runner exited nonzero with a passed report"}
	}
	applyReportStatus(&r, report.Status, obs.TimedOut)
	// A located run-level error of a `/3` run explains a nonzero exit (PWP-V3-009).
	if obs.ExitStatus != 0 && !obs.TimedOut && !explainedPlaywrightFailure(r.Tests) && !locatedGlobalError(r) {
		r.Infrastructure = &InfrastructureFailure{Reason: "exit-status-unexplained", Detail: "runner exited nonzero without a failing test observation"}
	}
	if profile == SensitiveExternalProfile {
		redacted, err := RedactSensitiveInputEvidence(r)
		if err != nil {
			return Receipt{}, err
		}
		r = redacted
	}
	return r, nil
}

// externalHygiene applies the native row's run-evidence hygiene (AFU-V1-038) to a reporter's
// failure messages and artifacts, identically on the last-attempt fields and every `/3` attempt
// entry (PWP-V3-010). The reporter joins every Playwright error, and the "Browser logs" tail of an
// action still pending when the test timed out carries the browser launch command line, whose
// `--password-store=basic` switch is secret-shaped and would otherwise refuse the whole qualified
// document. The sensitive-input profile is validated as the reporter wrote it: its redaction
// placeholder is the one this screen writes, so a screened `/2` report would read as already
// redacted and fail closed.
func externalHygiene(tests []TestOutcome) {
	for i := range tests {
		scrubPlaywrightOutcome(&tests[i])
	}
}

func externalReportFailure(profile, reason, detail string) *InfrastructureFailure {
	if profile == SensitiveExternalProfile {
		return &InfrastructureFailure{Reason: reason, Detail: "untrusted reporter output rejected"}
	}
	return &InfrastructureFailure{Reason: reason, Detail: detail}
}

func decodeQualifiedReport(data []byte, profile string) (qualifiedReport, *InfrastructureFailure) {
	var report qualifiedReport
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return qualifiedReport{}, externalReportFailure(profile, "report-unparseable", err.Error())
	}
	if _, err := decoder.Token(); err != io.EOF {
		return qualifiedReport{}, externalReportFailure(profile, "report-unparseable", "trailing report data")
	}
	if profile != AttemptExternalProfile && hasAttemptDetails(Receipt{Tests: report.Tests}) {
		return qualifiedReport{}, externalReportFailure(profile, "report-unparseable", "unknown field \"attemptDetails\"")
	}
	if profile != AttemptExternalProfile && report.ErrorLocations != nil {
		return qualifiedReport{}, externalReportFailure(profile, "report-unparseable", "unknown field \"errorLocations\"")
	}
	if profile == AttemptExternalProfile {
		if err := ValidateAttemptDetails(report.Tests); err != nil {
			return qualifiedReport{}, externalReportFailure(profile, "external-attempt-details-invalid", "attempt inventory disagrees with retained details")
		}
	}
	return report, nil
}

func setApplicationAttestationFailure(receipt *Receipt, reason string) {
	if receipt.ApplicationAttestation == nil {
		receipt.ApplicationAttestation = &ApplicationAttestationReceipt{Failures: []string{}}
	}
	for _, existing := range receipt.ApplicationAttestation.Failures {
		if existing == reason {
			return
		}
	}
	receipt.ApplicationAttestation.Failures = append(receipt.ApplicationAttestation.Failures, reason)
	if receipt.Infrastructure == nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: reason, Detail: "external application attestation did not qualify"}
	}
}

// ReasonReporterGlobalError is the infrastructure reason of a run whose reporter recorded a
// run-level error, such as a worker fixture teardown failure.
const ReasonReporterGlobalError = "reporter-global-error"

// firstErrorLocation is the reported location of the first located run-level error of a `/3`
// report (PWP-V3-009), or nil when the locations do not pair one to one with the errors.
func firstErrorLocation(report qualifiedReport) *Anchor {
	if len(report.ErrorLocations) != len(report.Errors) {
		return nil
	}
	for _, l := range report.ErrorLocations {
		if l != nil && l.File != "" && l.Line > 0 {
			return &Anchor{File: l.File, Line: l.Line}
		}
	}
	return nil
}

func locatedGlobalError(r Receipt) bool {
	return r.Profile == AttemptExternalProfile && r.Infrastructure != nil && r.Infrastructure.Reason == ReasonReporterGlobalError && r.Infrastructure.Location != nil
}

func explainedPlaywrightFailure(tests []TestOutcome) bool {
	for _, t := range tests {
		if t.State == StateFailed || t.State == StateTimedOut || t.State == StateInterrupted || t.State == StateInfrastructure {
			return true
		}
	}
	return false
}

// externalRequestProfile is the receipt profile an external request writes.
func externalRequestProfile(cfg E2EConfig) string {
	profile := ExternalProfile
	if cfg.RetainAttemptDetails {
		profile = AttemptExternalProfile
	}
	if cfg.ApplicationAttestation != nil {
		profile = AttestedExternalProfile
	}
	if cfg.SensitiveInputPolicy != nil {
		profile = SensitiveExternalProfile
	}
	if cfg.Freshness != nil {
		profile = FreshnessProfile
	}
	return profile
}

// admitExternal refuses an external request before anything starts. Every
// refusal is an *E2ERefusal so the CLI can write a typed document (PWP-V0-034).
func admitExternal(c E2EConfig) error {
	if !qualifiedPlaywrightVersion(c.RunnerVersion) {
		return &E2ERefusal{Code: "external-playwright-version-unqualified", Detail: unqualifiedVersionDetail(), Tuple: &RuntimeTupleMismatch{Profile: externalRequestProfile(c), Field: TupleFieldRunner, Admitted: QualifiedPlaywrightVersions()}}
	}
	if _, err := sensitivePolicy(c.SensitiveInputPolicy); err != nil {
		return refuseE2E(err.Error())
	}
	var providerArgs []string
	var providerConfig string
	if c.ApplicationAttestation != nil {
		providerArgs = c.ApplicationAttestation.Argv
		providerConfig = c.ApplicationAttestation.ConfigFile
	}
	bound, _ := json.Marshal(struct {
		App            string
		URL            string
		Args           []string
		Env            map[string]string
		ProviderArgs   []string
		ProviderConfig string
	}{c.AppIdentity, c.ServerReadyURL, c.TestArgv, declaredEnv(c.DeclaredEnvKeys), providerArgs, providerConfig})
	if len(bound) > 64<<10 || secretscreen.MatchString(string(bound)) {
		return refuseE2E("external-input-bound-or-secret")
	}
	if len(c.ServerArgv) != 0 {
		return refuseE2E("external-server-command-forbidden")
	}
	u, err := url.Parse(c.ServerReadyURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return refuseE2E("external-readiness-url-required")
	}
	if c.ApplicationAttestation == nil {
		if strings.TrimSpace(c.AppIdentity) == "" {
			return refuseE2E("external-app-identity-required")
		}
	} else {
		if strings.TrimSpace(c.AppIdentity) != "" || c.AppBuildDir != "" {
			return refuseE2E("external-attestation-conflicts-with-caller-identity")
		}
		if len(c.ApplicationAttestation.Argv) == 0 || c.ApplicationAttestation.ConfigFile == "" {
			return refuseE2E("application-attestation-provider-required")
		}
	}
	if c.ConfigFile == "" || c.RunnerVersion == "" || len(c.TestFiles) == 0 {
		return refuseE2E("external-config-version-test-files-required")
	}
	// External mode constructs the executable/config/reporter. Extra argv consists
	// only of selectors and bounded execution controls; lifecycle overrides cannot pass.
	for _, arg := range c.TestArgv {
		if strings.HasPrefix(arg, "-") && !allowedExternalOption(arg) {
			return &E2ERefusal{Code: "external-unsupported-option", Detail: arg}
		}
	}
	return nil
}

func allowedExternalOption(arg string) bool {
	for _, prefix := range []string{"--project=", "--grep=", "--grep-invert=", "--workers=", "--timeout=", "--retries=", "--repeat-each=", "--max-failures="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return arg == "--no-deps" || arg == "--forbid-only"
}

func externalCommand(c E2EConfig, scratch string) (string, []string, string, error) {
	configPath := filepath.Join(scratch, "playwright.config.cjs")
	reporterPath := filepath.Join(scratch, "reporter.cjs")
	reportPath := filepath.Join(scratch, "report.json")
	quoted := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	policy, _ := json.Marshal(c.SensitiveInputPolicy)
	attemptOption := ""
	if c.RetainAttemptDetails {
		attemptOption = ", retainAttemptDetails:true"
	}
	if c.Freshness != nil {
		tuples, _ := json.Marshal(freshnessReporterTuples())
		attemptOption = ", freshnessProfile:true, freshnessTuples:" + string(tuples)
	}
	prelude, kept := "", ""
	if c.KeepReporters {
		prelude, kept, attemptOption = keptReportersConfig(), keptReportersAfterProvider, attemptOption+keepReportersOption
	}
	if c.negation != nil {
		prelude += c.negation.prelude(quoted)
	}
	config := "const imported = require(" + quoted(c.ConfigFile) + ");\nconst original = imported.default || imported;\nconst base = " + quoted(filepath.Dir(c.ConfigFile)) + ";\nconst resolve = value => require('node:path').resolve(base, value);\nconst modulePath = value => Array.isArray(value) ? value.map(modulePath) : typeof value === 'string' ? require.resolve(value, {paths:[base]}) : value;\nconst paths = object => { const result = {...object}; for (const key of ['testDir', 'outputDir', 'snapshotDir', 'tsconfig']) if (typeof result[key] === 'string') result[key] = resolve(result[key]); return result; };\n" + prelude + "module.exports = {...paths(original), testDir: original.testDir ? resolve(original.testDir) : base, globalSetup: modulePath(original.globalSetup), globalTeardown: modulePath(original.globalTeardown), projects: original.projects?.map(paths), webServer: undefined, reporter: [[" + quoted(reporterPath) + ", {output:" + quoted(reportPath) + ", sensitiveInputPolicy:" + string(policy) + attemptOption + "}]" + kept + "]};\n"
	if c.ScopedTempOutput {
		output := quoted(filepath.Join(scratch, "output"))
		config += "module.exports.outputDir = " + output + ";\nfor (const project of module.exports.projects || []) project.outputDir = " + output + ";\n"
	}
	if c.negation != nil {
		config += c.negation.override(quoted)
	}
	if err := os.WriteFile(reporterPath, qualifiedReporter, 0600); err != nil {
		return "", nil, "", err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return "", nil, "", err
	}
	argv := append([]string{"npx", "--no-install", "playwright", "test", "--config=" + configPath}, c.TestArgv...)
	if c.Freshness != nil {
		argv = append(append([]string{}, c.Freshness.Runner.Argv...), "test", "--config="+configPath)
		argv = append(argv, c.TestArgv...)
	}
	return config, argv, reportPath, nil
}

func externalReady(ctx context.Context, address string, limit time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if err := probeAddress(address); err != nil {
		return err
	}
	for {
		if status, err := loopbackGet(ctx, address, false); err == nil && status >= 200 && status < 300 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func readBoundedReport(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(data) > limit {
		return nil, errors.New("report-output-overflow")
	}
	return data, err
}

// configInputsWithinBounds refuses a config-input set above either bound with
// config-inputs-overflow, naming the observed value and the limit (PWP-V0-021).
func configInputsWithinBounds(inputs map[string]string) error {
	if len(inputs) > externalMaxConfigInputs {
		return fmt.Errorf("config-inputs-overflow: observed %d config inputs, limit %d", len(inputs), externalMaxConfigInputs)
	}
	size := 0
	for path, digest := range inputs {
		size += len(path) + len(digest)
	}
	if size > externalMaxConfigInputBytes {
		return fmt.Errorf("config-inputs-overflow: observed %d config input bytes, limit %d", size, externalMaxConfigInputBytes)
	}
	return nil
}

func bindQualifiedReport(r *Receipt, report qualifiedReport) error {
	r.Schedule = report.Schedule
	if err := configInputsWithinBounds(report.ConfigFiles); err != nil {
		return err
	}
	if report.ConfigFiles[r.Identity.ConfigFile] != r.Identity.ConfigDigest {
		return errors.New("config-inputs-unobserved")
	}
	r.Identity.ConfigInputDigests = report.ConfigFiles
	for path, digest := range report.ConfigFiles {
		current, err := digestFile(path)
		if err != nil || current != digest {
			r.External.InputsUnchanged = false
			return errors.New("config-input-drift")
		}
	}
	if report.Version != r.Identity.RunnerVersion {
		return errors.New("runner-version-mismatch")
	}
	if len(report.Tests) == 0 {
		return errors.New("no-tests-observed")
	}
	if report.Status != "passed" && report.Status != "failed" && report.Status != "timedout" && report.Status != "interrupted" {
		return errors.New("run-status-unknown")
	}
	seen := map[string]bool{}
	for i := range r.Tests {
		t := &r.Tests[i]
		if t.Project == nil || (t.Project.Name == "" && r.Profile != AttemptExternalProfile) || t.Project.Browser == "" || t.Anchor == nil || t.Anchor.Line <= 0 {
			return errors.New("project-location-unknown")
		}
		if len(t.Attempts) == 0 || t.FullName == "" {
			return errors.New("test-attempt-identity-unknown")
		}
		for _, attempt := range t.Attempts {
			if !knownState(attempt.State) || attempt.Retry < 0 {
				return errors.New("test-attempt-state-unknown")
			}
		}
		if !knownState(t.State) {
			return errors.New("test-state-unknown")
		}
		if t.Project.Browser == BrowserlessProject && browserlessReason(report.Version, *t) == "" {
			return errors.New("project-location-unknown")
		}
		if !anchorFileBound(r.Identity, report, t.Anchor.File) {
			return unboundAnchorFiles(r.Identity, report, r.Tests)
		}
		t.Project.ConfigDigest = r.Identity.ConfigDigest
		var use any
		if err := json.Unmarshal(t.Project.Use, &use); err != nil {
			return err
		}
		t.Project.Use, _ = json.Marshal(use)
		t.ID = qualifiedTestID(r.Identity, *t)
		if seen[t.ID] {
			return errors.New("test-identity-ambiguous")
		}
		seen[t.ID] = true
	}
	return nil
}

// BrowserlessProject is the project browser the qualified reporter records for a test that used
// no browser; its use is exactly {"corvintBrowserless": reason} (PWP-V3-014).
const BrowserlessProject = "none"

// Closed browserless reasons (PWP-V3-014).
const (
	BrowserlessNoBrowserUse        = "no-browser-use"
	BrowserlessNotRun              = "not-run"
	BrowserlessFixtureBeforeLaunch = "fixture-failed-before-browser"
)

// browserlessReason returns the closed reason a browserless test records, or "" when the test is
// not browserless or its record is outside the closed shape: runner 1.63.0 or 1.64.0, use exactly
// {"corvintBrowserless": reason}, and attempts that agree with the reason (PWP-V3-015).
func browserlessReason(runnerVersion string, t TestOutcome) string {
	if t.Project == nil || t.Project.Browser != BrowserlessProject || (runnerVersion != "1.63.0" && runnerVersion != "1.64.0") || len(t.Attempts) == 0 {
		return ""
	}
	var use map[string]json.RawMessage
	if json.Unmarshal(t.Project.Use, &use) != nil || len(use) != 1 {
		return ""
	}
	var reason string
	if json.Unmarshal(use["corvintBrowserless"], &reason) != nil {
		return ""
	}
	var every ExecutionState
	switch reason {
	case BrowserlessNoBrowserUse:
		return reason
	case BrowserlessNotRun:
		every = StateSkipped
	case BrowserlessFixtureBeforeLaunch:
		every = StateInfrastructure
	default:
		return ""
	}
	for _, attempt := range t.Attempts {
		if attempt.State != every {
			return ""
		}
	}
	return reason
}

// ProjectLocationUnknownTests names, as JSON pointers into the emitted qualified document, the
// tests a project-location-unknown refusal rejects: no project, no project name where the profile
// needs one, no browser identity, a browserless record outside the closed shape, or no anchor. It
// names at most externalMaxUnboundNamed tests and carries no reported text (PWP-V3-016).
func ProjectLocationUnknownTests(r Receipt) []string {
	var named []string
	extra := 0
	for i := range r.Tests {
		t := r.Tests[i]
		unknown := t.Project == nil || (t.Project.Name == "" && r.Profile != AttemptExternalProfile) || t.Project.Browser == "" || t.Anchor == nil || t.Anchor.Line <= 0
		if !unknown && t.Project.Browser == BrowserlessProject {
			unknown = browserlessReason(r.Identity.RunnerVersion, t) == ""
		}
		if !unknown {
			continue
		}
		if len(named) == externalMaxUnboundNamed {
			extra++
			continue
		}
		named = append(named, fmt.Sprintf("/receipt/tests/%d", i))
	}
	if extra > 0 {
		named = append(named, fmt.Sprintf("+%d more", extra))
	}
	return named
}

// externalMaxUnboundNamed bounds how many unbound anchor files the
// test-file-unbound detail names (PWP-V0-019).
const externalMaxUnboundNamed = 8

func anchorFileBound(identity Identity, report qualifiedReport, file string) bool {
	return report.Files[file] != "" && report.Files[file] == identity.TestFileDigests[file]
}

// unboundAnchorFiles names every distinct reported anchor file that is not
// bound, with its cause, so a run whose Playwright project dependencies
// (setup projects) report results for files the caller did not pass with
// --test-file says which files to bind (PWP-V0-019). It never binds them.
func unboundAnchorFiles(identity Identity, report qualifiedReport, tests []TestOutcome) error {
	causes := map[string]string{}
	for _, t := range tests {
		if t.Anchor == nil || anchorFileBound(identity, report, t.Anchor.File) {
			continue
		}
		file := t.Anchor.File
		switch {
		case identity.TestFileDigests[file] == "":
			causes[file] = "not passed with --test-file"
		case report.Files[file] == "":
			causes[file] = "not digested by the reporter"
		default:
			causes[file] = "reporter digest differs from the --test-file digest"
		}
	}
	files := make([]string, 0, len(causes))
	for file := range causes {
		files = append(files, file)
	}
	slices.Sort(files)
	named := make([]string, 0, externalMaxUnboundNamed+1)
	for i, file := range files {
		if i == externalMaxUnboundNamed {
			named = append(named, fmt.Sprintf("+%d more", len(files)-i))
			break
		}
		named = append(named, fmt.Sprintf("%s (%s)", file, causes[file]))
	}
	return fmt.Errorf("test-file-unbound: %s", strings.Join(named, "; "))
}

func knownState(s ExecutionState) bool {
	switch s {
	case StatePassed, StateFailed, StateSkipped, StateFlaky, StateTimedOut, StateInterrupted, StateInfrastructure:
		return true
	}
	return false
}

func qualifiedTestID(identity Identity, t TestOutcome) string {
	identity.Argv = append([]string{}, identity.Argv...)
	for i, arg := range identity.Argv {
		if strings.HasPrefix(arg, "--config=") {
			identity.Argv[i] = "--config=" + identity.ConfigFile
		}
	}
	data, _ := json.Marshal(struct {
		Identity Identity
		Project  *ProjectIdentity
		Anchor   *Anchor
		FullName string
	}{identity, t.Project, t.Anchor, t.FullName})
	d := sha256.Sum256(data)
	return hex.EncodeToString(d[:])
}

// QualifiedTestProjection checks receipt-wide prerequisites before projecting a
// passed test. Consumers must not trust a carried projection or an isolated row.
// Keep-reporters mode runs project reporters beside the provider reporter and
// projects passing execution only with a matching retained qualification
// (PWP-V0-012, PWP-V0-016). The retained receipt stays readable either way.
func qualifiedUnknown(r Receipt, t TestOutcome) bool {
	if qualifiedPrerequisitesUnknown(r, t) {
		return true
	}
	return r.External != nil && r.External.ProjectReporters != nil && !keepReportersQualified(r)
}

// qualifiedPrerequisitesUnknown is every passing prerequisite other than the
// keep-reporters qualification; the qualification command applies it to both
// of its runs (PWP-V0-015).
func qualifiedPrerequisitesUnknown(r Receipt, t TestOutcome) bool {
	x := r.External
	if !isExternalProfile(r.Profile) {
		return false
	}
	if x == nil || x.Ownership != "external" || x.CleanupResponsibility != "external" || !x.ReadyAtStart || !x.ReadyAtPublish || !x.RunnerDescendantsGone || !x.InputsUnchanged {
		return true
	}
	if x.ConfigOverride == "" || x.ServerDescendants != "unknown" || r.ServerDescendantsGone != nil {
		return true
	}
	if qualifiedProfileShapeError(r) != nil {
		return true
	}
	// /2 changes the reporter and must complete its own live matrix before it
	// can project passing execution. Its redacted trace remains readable.
	if r.Profile == SensitiveExternalProfile {
		return true
	}
	if (r.Profile == ExternalProfile || r.Profile == AttemptExternalProfile || (r.Profile == SensitiveExternalProfile && r.ApplicationAttestation == nil)) && strings.TrimSpace(x.DeclaredAppIdentity) == "" {
		return true
	}
	if (r.Profile == AttestedExternalProfile || (r.Profile == SensitiveExternalProfile && r.ApplicationAttestation != nil)) && (x.DeclaredAppIdentity != "" || applicationAttestationUnknown(r.ApplicationAttestation) || testRepositoryUnknown(r.TestRepositoryAtStart, r.TestRepositoryAtPublish)) {
		return true
	}
	u, err := url.Parse(x.ReadyURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return true
	}
	if len(x.DeclaredAppIdentity) > 4096 || len(x.ReadyURL) > 4096 || len(x.ConfigOverride) > 64<<10 {
		return true
	}
	if t.Project == nil || (t.Project.Name == "" && r.Profile != AttemptExternalProfile) || t.Project.Browser == "" || t.Project.ConfigDigest == "" || t.Project.ConfigDigest != r.Identity.ConfigDigest || t.ID == "" {
		return true
	}
	if t.Anchor == nil || t.Anchor.Line <= 0 || t.FullName == "" || len(t.Attempts) == 0 || r.Identity.TestFileDigests[t.Anchor.File] == "" || r.Identity.RunnerVersion == "" || r.Identity.NodeVersion == "" {
		return true
	}
	if r.Identity.ConfigInputDigests[r.Identity.ConfigFile] != r.Identity.ConfigDigest {
		return true
	}
	var use map[string]json.RawMessage
	if json.Unmarshal(t.Project.Use, &use) != nil || use == nil {
		return true
	}
	if t.Project.Device == "" || len(t.Project.Name) > 4096 || len(t.Project.Browser) > 256 || len(t.Project.Use) > 64<<10 {
		return true
	}
	if !filepath.IsAbs(r.Identity.ConfigFile) || len(r.Identity.Argv) == 0 || r.Identity.RunnerName != "playwright" {
		return true
	}
	if t.Project.Browser == BrowserlessProject {
		// PWP-V3-015: a test that used no browser qualifies on the runner and node tuple alone.
		// A test that never ran, or whose fixture failed before any browser, makes no claim.
		if browserlessReason(r.Identity.RunnerVersion, t) != BrowserlessNoBrowserUse || ReceiptRuntimeTuple(r) != RuntimeTupleCandidate {
			return true
		}
	} else if !qualifiedPlaywrightTuple(r, t) {
		return true
	}
	for _, attempt := range t.Attempts {
		if !knownState(attempt.State) || attempt.Retry < 0 {
			return true
		}
		if attempt.State == StateInfrastructure {
			return true
		}
	}
	last := t.Attempts[len(t.Attempts)-1]
	if last.Retry != t.Retries {
		return true
	}
	if t.State == StatePassed || t.State == StateFlaky {
		if last.State != StatePassed {
			return true
		}
	}
	if t.State == StatePassed {
		for _, attempt := range t.Attempts {
			if attempt.State != StatePassed {
				return true
			}
		}
	}
	return t.ID != qualifiedTestID(r.Identity, t)
}

func qualifiedPlaywrightTuple(r Receipt, t TestOutcome) bool {
	if ReceiptRuntimeTuple(r) != RuntimeTupleCandidate {
		return false
	}
	if r.Identity.RunnerVersion == "1.60.0" {
		return true
	}
	var use playwrightUseIdentity
	if json.Unmarshal(t.Project.Use, &use) != nil {
		return false
	}
	if len(use.ConnectOptions) != 0 {
		return false
	}
	browser := use.CorvintBrowser
	if use.BrowserName == nil || use.Channel == nil || browser.Platform == nil || browser.Arch == nil || browser.NodeVersion == nil || browser.BrowserType == nil || browser.BrowserVersion == nil || browser.Channel == nil || browser.ExecutablePath == nil || browser.HeadlessShellAvailable == nil {
		return false
	}
	if *use.BrowserName != t.Project.Browser || *use.BrowserName != *browser.BrowserType || *use.Channel != *browser.Channel || *browser.Platform != "darwin" || *browser.Arch != "arm64" || *browser.NodeVersion != r.Identity.NodeVersion || *browser.BrowserType != "chromium" || *browser.Channel != "" || !*browser.HeadlessShellAvailable {
		return false
	}
	if browser.ExecutableSource == nil {
		qualified, ok := qualifiedSystemBrowsers[r.Identity.RunnerVersion]
		if !ok || !runtimeTupleAdmitted(r.Profile, r.Identity.RunnerVersion, r.Identity.NodeVersion, BrowserSourceSystem) {
			return false
		}
		// PWP-V0-037 qualified 1.64.0 system Chrome on headless /0 only.
		if r.Identity.RunnerVersion == "1.64.0" && (r.Profile != ExternalProfile || use.Headless == nil || !*use.Headless) {
			return false
		}
		return qualifiedSystemPlaywrightBrowser(qualified, use.LaunchOptions.ExecutablePath, browser.BrowserVersion, browser.ExecutablePath)
	}
	if !runtimeTupleAdmitted(r.Profile, r.Identity.RunnerVersion, r.Identity.NodeVersion, BrowserSourceBundled) {
		return false
	}
	return qualifiedBundledPlaywrightBrowser(r.Identity.RunnerVersion, use.Headless, use.LaunchOptions.ExecutablePath, browser)
}

func qualifiedSystemPlaywrightBrowser(qualified string, configured, version, observed *string) bool {
	return configured != nil && *configured == *observed && *version == qualified && *observed == qualifiedSystemBrowserExecutable
}

func qualifiedBundledPlaywrightBrowser(runnerVersion string, headless *bool, configured *string, browser playwrightBrowserIdentity) bool {
	qualified, ok := qualifiedBundledBrowsers[runnerVersion]
	if !ok || headless == nil || !*headless {
		return false
	}
	if configured != nil && *configured != "" {
		return false
	}
	if browser.ExecutableName == nil || browser.ExecutableSHA256 == nil || browser.BrowserRevision == nil || browser.ManifestBrowserVersion == nil {
		return false
	}
	if *browser.ExecutableSource != "playwright-bundled" || *browser.ExecutableName != qualifiedBundledBrowserName {
		return false
	}
	if *browser.BrowserVersion != qualified.observedVersion || *browser.ManifestBrowserVersion != qualified.manifestVersion {
		return false
	}
	if *browser.BrowserRevision != qualified.revision || *browser.ExecutableSHA256 != qualified.executableSHA256 {
		return false
	}
	return filepath.IsAbs(*browser.ExecutablePath) && strings.HasSuffix(*browser.ExecutablePath, qualified.executableSuffix)
}

// externalBoundaryFailure classifies the external-server Playwright process
// observation in the vitest classifier's order, cancellation included, and
// names the Playwright runner and its --timeout value on timeout (PWP-V0-020).
// externalBoundaryFailure ignores console overflow: external console capture
// truncates and the report file carries the observation (PWP-V0-028).
// applyReportStatus maps a run-level Playwright report status onto the receipt.
// An interrupted report from a run Corvint timed out (the PWP-V0-030 SIGINT)
// stays a timeout interruption: the boundary already records the timeout
// infrastructure failure, and only an interruption Corvint did not cause is a
// cancellation (V1-1812).
func applyReportStatus(r *Receipt, status string, timedOut bool) {
	switch status {
	case "interrupted":
		if timedOut {
			if r.Infrastructure == nil {
				r.Infrastructure = &InfrastructureFailure{Reason: "timeout", Detail: "runner interrupted by the provider timeout"}
			}
			return
		}
		r.Cancelled = true
	case "timedout":
		r.Infrastructure = &InfrastructureFailure{Reason: "timeout", Detail: "Playwright global timeout"}
	}
}

func externalBoundaryFailure(obs procgroup.Observation, timeout time.Duration) *InfrastructureFailure {
	obs.OutputOverflow, obs.StdoutOverflow, obs.StderrOverflow = false, false, false
	return unitBoundaryFailure(obs, "playwright test", timeout)
}

func runnerProcess(obs procgroup.Observation) *RunnerProcess {
	return &RunnerProcess{ExitObserved: obs.ExitObserved, ExitStatus: obs.ExitStatus, Signal: obs.Signal, StopSignal: obs.StopSignal,
		OutputTruncated: obs.OutputOverflow || obs.StdoutOverflow || obs.StderrOverflow}
}

// runnerProcessShapeError keeps the diagnostic runner record closed.
func runnerProcessShapeError(r Receipt) error {
	if r.External == nil || r.External.Runner == nil {
		return nil
	}
	p := r.External.Runner
	switch p.StopSignal {
	case "", "SIGINT", "SIGTERM", "SIGKILL":
	default:
		return errors.New("runner-process-invalid")
	}
	if !isExternalProfile(r.Profile) || p.ExitStatus < -1 || (!p.ExitObserved && p.ExitStatus != -1) || (p.Signal != "" && p.ExitStatus != -1) {
		return errors.New("runner-process-invalid")
	}
	return nil
}

// lockedWriter serialises the runner's two console streams onto one sink.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(value []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(value)
}

// TeeIncomplete passes procgroup's early-end notice to a sink that wants it.
func (l *lockedWriter) TeeIncomplete() {
	if sink, ok := l.w.(procgroup.IncompleteTee); ok {
		sink.TeeIncomplete()
	}
}

// externalRunnerAfterStart, when set, is the external runner's procgroup
// AfterStart hook, which runs before the runner's Timeout clock starts. It is
// a test-only readiness handshake with a stand-in runner (V1-1942); it is
// appended here so existing line citations into this file keep their lines.
var externalRunnerAfterStart func(context.Context, int) error
