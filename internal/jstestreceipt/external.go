package jstestreceipt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const ExternalProfile = "corvint-playwright-external/0"

const ExternalOutputLimit = 4 << 20

// externalMaxConfigInputs and externalMaxConfigInputBytes bound the
// reporter-observed config-input set that is re-opened and digested at bind time
// and copied into the receipt (PWP-V0-021). The count admits the non-node_modules
// module graph of a large suite (the earlier 256 refused mid-size suites); the
// byte bound, the sum of every path and digest length, keeps the set at a quarter
// of the 4 MiB provider-document bound (testvaliditydoc.MaxInputBytes). Beyond
// either bound the report is refused as config-inputs-overflow.
const (
	ExternalMaxConfigInputs     = 4096
	ExternalMaxConfigInputBytes = 1 << 20
)

// qualifiedBundledBrowser is the exact Playwright-bundled headless-shell
// identity one Playwright release pins (PWP-V0-008). Each release pins its own
// registry revision, so the tuple is keyed by the exact runner version.
type qualifiedBundledBrowser struct {
	revision         string
	observedVersion  string
	manifestVersion  string
	executableSHA256 string
	ExecutableSuffix string
}

const qualifiedBundledBrowserName = "chromium-headless-shell"

// QualifiedBundledBrowsers names the bundled tuple of each runner version
// whose live matrix passed: 1.63.0 (issue 175) and 1.64.0 (PWP-V0-032,
// GitHub #1263). 1.60.0 predates the browser binding and has no entry.
var QualifiedBundledBrowsers = map[string]qualifiedBundledBrowser{
	"1.63.0": {
		revision:         "1243",
		observedVersion:  "Google Chrome for Testing 153.0.8010.12",
		manifestVersion:  "153.0.8010.12",
		executableSHA256: "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282",
		ExecutableSuffix: "/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell",
	},
	"1.64.0": {
		revision:         "1248",
		observedVersion:  "Google Chrome for Testing 156.0.8078.4",
		manifestVersion:  "156.0.8078.4",
		executableSHA256: "77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634",
		ExecutableSuffix: "/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell",
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

type PlaywrightUseIdentity struct {
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

// ConfigInputsWithinBounds refuses a config-input set above either bound with
// config-inputs-overflow, naming the observed value and the limit (PWP-V0-021).
func ConfigInputsWithinBounds(inputs map[string]string) error {
	if len(inputs) > ExternalMaxConfigInputs {
		return fmt.Errorf("config-inputs-overflow: observed %d config inputs, limit %d", len(inputs), ExternalMaxConfigInputs)
	}
	size := 0
	for path, digest := range inputs {
		size += len(path) + len(digest)
	}
	if size > ExternalMaxConfigInputBytes {
		return fmt.Errorf("config-inputs-overflow: observed %d config input bytes, limit %d", size, ExternalMaxConfigInputBytes)
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

// BrowserlessReason returns the closed reason a browserless test records, or "" when the test is
// not browserless or its record is outside the closed shape: runner 1.63.0 or 1.64.0, use exactly
// {"corvintBrowserless": reason}, and attempts that agree with the reason (PWP-V3-015).
func BrowserlessReason(runnerVersion string, t TestOutcome) string {
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

func KnownState(s ExecutionState) bool {
	switch s {
	case StatePassed, StateFailed, StateSkipped, StateFlaky, StateTimedOut, StateInterrupted, StateInfrastructure:
		return true
	}
	return false
}

func QualifiedTestID(identity Identity, t TestOutcome) string {
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
func QualifiedUnknown(r Receipt, t TestOutcome) bool {
	if QualifiedPrerequisitesUnknown(r, t) {
		return true
	}
	return r.External != nil && r.External.ProjectReporters != nil && !keepReportersQualified(r)
}

// QualifiedPrerequisitesUnknown is every passing prerequisite other than the
// keep-reporters qualification; the qualification command applies it to both
// of its runs (PWP-V0-015).
func QualifiedPrerequisitesUnknown(r Receipt, t TestOutcome) bool {
	x := r.External
	if !IsExternalProfile(r.Profile) {
		return false
	}
	if x == nil || x.Ownership != "external" || x.CleanupResponsibility != "external" || !x.ReadyAtStart || !x.ReadyAtPublish || !x.RunnerDescendantsGone || !x.InputsUnchanged {
		return true
	}
	if x.ConfigOverride == "" || x.ServerDescendants != "unknown" || r.ServerDescendantsGone != nil {
		return true
	}
	if QualifiedProfileShapeError(r) != nil {
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
	if (r.Profile == AttestedExternalProfile || (r.Profile == SensitiveExternalProfile && r.ApplicationAttestation != nil)) && (x.DeclaredAppIdentity != "" || ApplicationAttestationUnknown(r.ApplicationAttestation) || TestRepositoryUnknown(r.TestRepositoryAtStart, r.TestRepositoryAtPublish)) {
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
		if BrowserlessReason(r.Identity.RunnerVersion, t) != BrowserlessNoBrowserUse || ReceiptRuntimeTuple(r) != RuntimeTupleCandidate {
			return true
		}
	} else if !QualifiedPlaywrightTuple(r, t) {
		return true
	}
	for _, attempt := range t.Attempts {
		if !KnownState(attempt.State) || attempt.Retry < 0 {
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
	return t.ID != QualifiedTestID(r.Identity, t)
}

func QualifiedPlaywrightTuple(r Receipt, t TestOutcome) bool {
	if ReceiptRuntimeTuple(r) != RuntimeTupleCandidate {
		return false
	}
	if r.Identity.RunnerVersion == "1.60.0" {
		return true
	}
	var use PlaywrightUseIdentity
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
		if !ok || !RuntimeTupleAdmitted(r.Profile, r.Identity.RunnerVersion, r.Identity.NodeVersion, BrowserSourceSystem) {
			return false
		}
		// PWP-V0-037 qualified 1.64.0 system Chrome on headless /0 only.
		if r.Identity.RunnerVersion == "1.64.0" && (r.Profile != ExternalProfile || use.Headless == nil || !*use.Headless) {
			return false
		}
		return qualifiedSystemPlaywrightBrowser(qualified, use.LaunchOptions.ExecutablePath, browser.BrowserVersion, browser.ExecutablePath)
	}
	if !RuntimeTupleAdmitted(r.Profile, r.Identity.RunnerVersion, r.Identity.NodeVersion, BrowserSourceBundled) {
		return false
	}
	return qualifiedBundledPlaywrightBrowser(r.Identity.RunnerVersion, use.Headless, use.LaunchOptions.ExecutablePath, browser)
}

func qualifiedSystemPlaywrightBrowser(qualified string, configured, version, observed *string) bool {
	return configured != nil && *configured == *observed && *version == qualified && *observed == qualifiedSystemBrowserExecutable
}

func qualifiedBundledPlaywrightBrowser(runnerVersion string, headless *bool, configured *string, browser playwrightBrowserIdentity) bool {
	qualified, ok := QualifiedBundledBrowsers[runnerVersion]
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
	return filepath.IsAbs(*browser.ExecutablePath) && strings.HasSuffix(*browser.ExecutablePath, qualified.ExecutableSuffix)
}

// RunnerProcessShapeError keeps the diagnostic runner record closed.
func RunnerProcessShapeError(r Receipt) error {
	if r.External == nil || r.External.Runner == nil {
		return nil
	}
	p := r.External.Runner
	switch p.StopSignal {
	case "", "SIGINT", "SIGTERM", "SIGKILL":
	default:
		return errors.New("runner-process-invalid")
	}
	if !IsExternalProfile(r.Profile) || p.ExitStatus < -1 || (!p.ExitObserved && p.ExitStatus != -1) || (p.Signal != "" && p.ExitStatus != -1) {
		return errors.New("runner-process-invalid")
	}
	return nil
}
