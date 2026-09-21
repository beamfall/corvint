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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const ExternalProfile = "corvint-playwright-external/0"
const externalOutputLimit = 4 << 20

const (
	qualifiedBundledBrowserName      = "chromium-headless-shell"
	qualifiedBundledBrowserRevision  = "1243"
	qualifiedBundledBrowserVersion   = "Google Chrome for Testing 153.0.8010.12"
	qualifiedBundledManifestVersion  = "153.0.8010.12"
	qualifiedBundledExecutableSHA256 = "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282"
	qualifiedBundledExecutableSuffix = "/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell"
	qualifiedSystemBrowserVersion    = "Google Chrome 153.0.8010.48"
	qualifiedSystemBrowserExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
)

func qualifiedPlaywrightVersion(version string) bool {
	return version == "1.60.0" || version == "1.63.0"
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
	identity, err := cfg.identity(argv)
	if err != nil {
		return Receipt{}, err
	}
	profile := ExternalProfile
	if cfg.ApplicationAttestation != nil {
		profile = AttestedExternalProfile
	}
	lifecycle := &ExternalLifecycle{ReadyURL: cfg.ServerReadyURL, DeclaredAppIdentity: cfg.AppIdentity, Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ConfigOverride: config}
	r := Receipt{Profile: profile, Kind: "e2e", Identity: identity, External: lifecycle, Tests: []TestOutcome{}}
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
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: cfg.Dir, Env: os.Environ(), Timeout: timeout, OutputLimit: externalOutputLimit, ObserveDescendants: cfg.ObserveDescendants})
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
	after, identityErr := cfg.identity(argv)
	lifecycle.InputsUnchanged = identityErr == nil && reflect.DeepEqual(identity, after)
	r.Cancelled = obs.Cancelled || ctx.Err() != nil
	if boundary := unitBoundaryFailure(obs); boundary != nil {
		r.Infrastructure = boundary
	}
	if !lifecycle.ReadyAtPublish {
		r.Infrastructure = &InfrastructureFailure{Reason: "server-unavailable-at-publish", Detail: "external server did not answer readiness after the run"}
	}
	if !lifecycle.RunnerDescendantsGone {
		r.Infrastructure = &InfrastructureFailure{Reason: "runner-cleanup-unknown", Detail: "owned runner process group cleanup was not observed"}
	}
	if !lifecycle.InputsUnchanged {
		r.Infrastructure = &InfrastructureFailure{Reason: "input-identity-changed", Detail: "bound config, test, package or declared environment changed"}
	}
	data, readErr := readBoundedReport(reportPath)
	if readErr != nil {
		if r.Infrastructure == nil {
			r.Infrastructure = &InfrastructureFailure{Reason: "report-not-written", Detail: readErr.Error()}
		}
		return r, nil
	}
	var report qualifiedReport
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		r.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: err.Error()}
		return r, nil
	}
	if _, err := decoder.Token(); err != io.EOF {
		r.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: "trailing report data"}
		return r, nil
	}
	r.Tests = report.Tests
	if err := bindQualifiedReport(&r, report); err != nil {
		r.Infrastructure = &InfrastructureFailure{Reason: "report-identity-unknown", Detail: err.Error()}
	}
	if len(report.Errors) > 0 {
		r.Infrastructure = &InfrastructureFailure{Reason: "reporter-global-error", Detail: strings.Join(report.Errors, "\n")}
	}
	if obs.ExitStatus != 0 && report.Status == "passed" {
		r.Infrastructure = &InfrastructureFailure{Reason: "exit-status-unexplained", Detail: "runner exited nonzero with a passed report"}
	}
	if report.Status == "interrupted" {
		r.Cancelled = true
	}
	if report.Status == "timedout" {
		r.Infrastructure = &InfrastructureFailure{Reason: "timeout", Detail: "Playwright global timeout"}
	}
	if obs.ExitStatus != 0 && !explainedPlaywrightFailure(r.Tests) {
		r.Infrastructure = &InfrastructureFailure{Reason: "exit-status-unexplained", Detail: "runner exited nonzero without a failing test observation"}
	}
	return r, nil
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

func explainedPlaywrightFailure(tests []TestOutcome) bool {
	for _, t := range tests {
		if t.State == StateFailed || t.State == StateTimedOut || t.State == StateInterrupted || t.State == StateInfrastructure {
			return true
		}
	}
	return false
}

func admitExternal(c E2EConfig) error {
	if !qualifiedPlaywrightVersion(c.RunnerVersion) {
		return errors.New("external-playwright-version-unqualified")
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
		return errors.New("external-input-bound-or-secret")
	}
	if len(c.ServerArgv) != 0 {
		return errors.New("external-server-command-forbidden")
	}
	u, err := url.Parse(c.ServerReadyURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return errors.New("external-readiness-url-required")
	}
	if c.ApplicationAttestation == nil {
		if strings.TrimSpace(c.AppIdentity) == "" {
			return errors.New("external-app-identity-required")
		}
	} else {
		if strings.TrimSpace(c.AppIdentity) != "" || c.AppBuildDir != "" {
			return errors.New("external-attestation-conflicts-with-caller-identity")
		}
		if len(c.ApplicationAttestation.Argv) == 0 || c.ApplicationAttestation.ConfigFile == "" {
			return errors.New("application-attestation-provider-required")
		}
	}
	if c.ConfigFile == "" || c.RunnerVersion == "" || len(c.TestFiles) == 0 {
		return errors.New("external-config-version-test-files-required")
	}
	// External mode constructs the executable/config/reporter. Extra argv consists
	// only of selectors and bounded execution controls; lifecycle overrides cannot pass.
	for _, arg := range c.TestArgv {
		if strings.HasPrefix(arg, "-") && !allowedExternalOption(arg) {
			return fmt.Errorf("external-unsupported-option: %s", arg)
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
	config := "const imported = require(" + quoted(c.ConfigFile) + ");\nconst original = imported.default || imported;\nconst base = " + quoted(filepath.Dir(c.ConfigFile)) + ";\nconst resolve = value => require('node:path').resolve(base, value);\nconst modulePath = value => Array.isArray(value) ? value.map(modulePath) : typeof value === 'string' ? require.resolve(value, {paths:[base]}) : value;\nconst paths = object => { const result = {...object}; for (const key of ['testDir', 'outputDir', 'snapshotDir', 'tsconfig']) if (typeof result[key] === 'string') result[key] = resolve(result[key]); return result; };\nmodule.exports = {...paths(original), testDir: original.testDir ? resolve(original.testDir) : base, globalSetup: modulePath(original.globalSetup), globalTeardown: modulePath(original.globalTeardown), projects: original.projects?.map(paths), webServer: undefined, reporter: [[" + quoted(reporterPath) + ", {output:" + quoted(reportPath) + "}]]};\n"
	if err := os.WriteFile(reporterPath, qualifiedReporter, 0600); err != nil {
		return "", nil, "", err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return "", nil, "", err
	}
	argv := append([]string{"npx", "--no-install", "playwright", "test", "--config=" + configPath}, c.TestArgv...)
	return config, argv, reportPath, nil
}

func externalReady(ctx context.Context, address string, limit time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func readBoundedReport(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, externalOutputLimit+1))
	if len(data) > externalOutputLimit {
		return nil, errors.New("report-output-overflow")
	}
	return data, err
}

func bindQualifiedReport(r *Receipt, report qualifiedReport) error {
	r.Schedule = report.Schedule
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
		if t.Project == nil || t.Project.Name == "" || t.Project.Browser == "" || t.Anchor == nil || t.Anchor.Line <= 0 {
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
		if report.Files[t.Anchor.File] == "" || report.Files[t.Anchor.File] != r.Identity.TestFileDigests[t.Anchor.File] {
			return errors.New("test-file-unbound")
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
func qualifiedUnknown(r Receipt, t TestOutcome) bool {
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
	if r.Profile == ExternalProfile && strings.TrimSpace(x.DeclaredAppIdentity) == "" {
		return true
	}
	if r.Profile == AttestedExternalProfile && (x.DeclaredAppIdentity != "" || applicationAttestationUnknown(r.ApplicationAttestation) || testRepositoryUnknown(r.TestRepositoryAtStart, r.TestRepositoryAtPublish)) {
		return true
	}
	u, err := url.Parse(x.ReadyURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return true
	}
	if len(x.DeclaredAppIdentity) > 4096 || len(x.ReadyURL) > 4096 || len(x.ConfigOverride) > 64<<10 {
		return true
	}
	if t.Project == nil || t.Project.Name == "" || t.Project.Browser == "" || t.Project.ConfigDigest == "" || t.Project.ConfigDigest != r.Identity.ConfigDigest || t.ID == "" {
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
	if !qualifiedPlaywrightTuple(r, t) {
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
	if r.Identity.RunnerVersion == "1.60.0" {
		return true
	}
	if r.Identity.RunnerVersion != "1.63.0" || r.Identity.NodeVersion != "v22.23.2" {
		return false
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
		return qualifiedSystemPlaywrightBrowser(use.LaunchOptions.ExecutablePath, browser.BrowserVersion, browser.ExecutablePath)
	}
	return qualifiedBundledPlaywrightBrowser(use.Headless, use.LaunchOptions.ExecutablePath, browser)
}

func qualifiedSystemPlaywrightBrowser(configured, version, observed *string) bool {
	return configured != nil && *configured == *observed && *version == qualifiedSystemBrowserVersion && *observed == qualifiedSystemBrowserExecutable
}

func qualifiedBundledPlaywrightBrowser(headless *bool, configured *string, browser playwrightBrowserIdentity) bool {
	if headless == nil || !*headless {
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
	if *browser.BrowserVersion != qualifiedBundledBrowserVersion || *browser.ManifestBrowserVersion != qualifiedBundledManifestVersion {
		return false
	}
	if *browser.BrowserRevision != qualifiedBundledBrowserRevision || *browser.ExecutableSHA256 != qualifiedBundledExecutableSHA256 {
		return false
	}
	return filepath.IsAbs(*browser.ExecutablePath) && strings.HasSuffix(*browser.ExecutablePath, qualifiedBundledExecutableSuffix)
}
