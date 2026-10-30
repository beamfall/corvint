package jstestprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// PreflightSchema is the declared-preflight document an external e2e run may
// carry (PWP-V0-090). The receipt member keeps the same schema name.
const PreflightSchema = "corvint-js-preflight/0"

// Preflight probe kinds, states and the infrastructure phase (PWP-V0-090..093).
const (
	PreflightHTTP         = "http"
	PreflightCommand      = "command"
	PreflightStorageState = "storage-state"

	PreflightPassed = "passed"
	PreflightFailed = "failed"
	PreflightNotRun = "not-run"

	// InfrastructurePhasePreflight marks an infrastructure failure observed
	// before any test ran (PWP-V0-092).
	InfrastructurePhasePreflight = "preflight"
	ReasonPreflightFailed        = "preflight-failed"
)

// Preflight declaration bounds (PWP-V0-090).
const (
	preflightMaxBytes        = 64 << 10
	preflightMaxProbes       = 32
	preflightMaxProbeTimeout = 10 * time.Minute
	preflightMaxTotalTimeout = time.Hour
	preflightMaxArgv         = 64
	preflightMaxNames        = 32
	preflightStateMaxBytes   = 16 << 20
	preflightCommandOutput   = 64 << 10
)

var preflightProbeName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// preflightNow is the storage-state validity clock; tests replace it.
var preflightNow = time.Now

// PreflightDeclaration is the caller-declared preflight document (PWP-V0-090).
type PreflightDeclaration struct {
	Schema         string           `json:"schema"`
	BaseURL        string           `json:"baseUrl"`
	TotalTimeoutMS int64            `json:"totalTimeoutMs"`
	Probes         []PreflightProbe `json:"probes"`
}

// PreflightProbe is one declared probe. Only the members of its kind may be set.
type PreflightProbe struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	TimeoutMS int64  `json:"timeoutMs"`
	// http: a same-origin path under baseUrl and the admitted statuses (default 2xx).
	Path         string `json:"path,omitempty"`
	ExpectStatus []int  `json:"expectStatus,omitempty"`
	// command: a setup or seed command run in the test directory; exit 0 passes.
	Argv []string `json:"argv,omitempty"`
	// storage-state: a Playwright storage-state file relative to the test directory.
	File            string   `json:"file,omitempty"`
	RequireCookies  []string `json:"requireCookies,omitempty"`
	RequireOrigins  []string `json:"requireOrigins,omitempty"`
	MinValidSeconds int64    `json:"minValidSeconds,omitempty"`
}

// Preflight is a loaded, validated declaration bound to its file digest.
type Preflight struct {
	File        string
	Digest      string
	Declaration PreflightDeclaration
}

// PreflightReceipt is the receipt member a preflight run writes (PWP-V0-091).
type PreflightReceipt struct {
	Schema            string                 `json:"schema"`
	DeclarationFile   string                 `json:"declarationFile"`
	DeclarationDigest string                 `json:"declarationDigest"`
	BaseURL           string                 `json:"baseUrl"`
	Outcome           string                 `json:"outcome"`
	Probes            []PreflightProbeResult `json:"probes"`
}

// PreflightProbeResult is one probe's observation. Reason is closed; Observed
// is a short provider-written fact (a status, an exit code, a cookie name),
// never command output or a storage-state value.
type PreflightProbeResult struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	Reason         string `json:"reason,omitempty"`
	Artefact       string `json:"artefact"`
	ArtefactDigest string `json:"artefactDigest,omitempty"`
	Observed       string `json:"observed,omitempty"`
	DurationMS     int64  `json:"durationMs"`
}

// preflightReasons is the closed reason vocabulary (PWP-V0-091).
var preflightReasons = map[string]bool{
	"http-unreachable": true, "http-status-unexpected": true,
	"command-start-failed": true, "command-exit-nonzero": true, "command-timeout": true, "command-cleanup-unknown": true,
	"storage-state-missing": true, "storage-state-unreadable": true, "storage-state-invalid": true,
	"storage-state-cookie-missing": true, "storage-state-cookie-expiring": true, "storage-state-origin-missing": true,
	"earlier-probe-failed": true, "preflight-timeout": true, "cancelled": true,
}

func preflightRefusal(detail string) error {
	return &E2ERefusal{Code: "preflight-declaration-invalid", Detail: detail}
}

// LoadPreflight reads and validates a declaration file. Every refusal is the
// typed preflight-declaration-invalid E2E refusal (PWP-V0-090).
func LoadPreflight(path string) (*Preflight, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, preflightRefusal(err.Error())
	}
	data, err := readBoundedReport(abs, preflightMaxBytes)
	if err != nil {
		return nil, preflightRefusal(err.Error())
	}
	if secretscreen.MatchString(string(data)) {
		return nil, preflightRefusal("declaration is secret-shaped")
	}
	var declaration PreflightDeclaration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declaration); err != nil {
		return nil, preflightRefusal(err.Error())
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, preflightRefusal("trailing declaration data")
	}
	if err := validatePreflight(declaration); err != nil {
		return nil, preflightRefusal(err.Error())
	}
	sum := sha256.Sum256(data)
	return &Preflight{File: abs, Digest: hex.EncodeToString(sum[:]), Declaration: declaration}, nil
}

func validatePreflight(d PreflightDeclaration) error {
	if d.Schema != PreflightSchema {
		return fmt.Errorf("schema must be %s", PreflightSchema)
	}
	if err := preflightBaseURL(d.BaseURL); err != nil {
		return err
	}
	if d.TotalTimeoutMS <= 0 || time.Duration(d.TotalTimeoutMS)*time.Millisecond > preflightMaxTotalTimeout {
		return errors.New("totalTimeoutMs must be in 1..3600000")
	}
	if len(d.Probes) == 0 || len(d.Probes) > preflightMaxProbes {
		return fmt.Errorf("probes must number 1..%d", preflightMaxProbes)
	}
	seen := map[string]bool{}
	for i, p := range d.Probes {
		if !preflightProbeName.MatchString(p.Name) || seen[p.Name] {
			return fmt.Errorf("probes[%d]: name must be unique and match %s", i, preflightProbeName)
		}
		seen[p.Name] = true
		if p.TimeoutMS <= 0 || time.Duration(p.TimeoutMS)*time.Millisecond > preflightMaxProbeTimeout {
			return fmt.Errorf("probe %s: timeoutMs must be in 1..600000", p.Name)
		}
		if err := validatePreflightProbe(p); err != nil {
			return fmt.Errorf("probe %s: %w", p.Name, err)
		}
	}
	return nil
}

func preflightBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(raw) > 4096 {
		return errors.New("baseUrl must be an http(s) URL without credentials, query or fragment")
	}
	return nil
}

func validatePreflightProbe(p PreflightProbe) error {
	http := p.Path != "" || len(p.ExpectStatus) != 0
	command := len(p.Argv) != 0
	state := p.File != "" || len(p.RequireCookies) != 0 || len(p.RequireOrigins) != 0 || p.MinValidSeconds != 0
	switch p.Kind {
	case PreflightHTTP:
		if command || state {
			return errors.New("http probe carries members of another kind")
		}
		if !strings.HasPrefix(p.Path, "/") || strings.HasPrefix(p.Path, "//") || strings.Contains(p.Path, "#") || len(p.Path) > 2048 {
			return errors.New("http probe path must be a same-origin absolute path")
		}
		for _, status := range p.ExpectStatus {
			if status < 100 || status > 599 {
				return errors.New("expectStatus entries must be in 100..599")
			}
		}
	case PreflightCommand:
		if http || state {
			return errors.New("command probe carries members of another kind")
		}
		if len(p.Argv) == 0 || len(p.Argv) > preflightMaxArgv || p.Argv[0] == "" {
			return fmt.Errorf("command probe argv must have 1..%d tokens", preflightMaxArgv)
		}
	case PreflightStorageState:
		if http || command {
			return errors.New("storage-state probe carries members of another kind")
		}
		clean := filepath.Clean(p.File)
		if p.File == "" || filepath.IsAbs(p.File) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("storage-state file must be a path inside the test directory")
		}
		if len(p.RequireCookies) > preflightMaxNames || len(p.RequireOrigins) > preflightMaxNames || p.MinValidSeconds < 0 {
			return errors.New("storage-state requirements out of bounds")
		}
	default:
		return errors.New("kind must be http, command or storage-state")
	}
	return nil
}

// runPreflight runs every probe in declared order inside the total budget and
// stops at the first failure; later probes are not-run (PWP-V0-091). It never
// starts the test runner and returns the infrastructure failure to record, or
// nil when every probe passed.
func runPreflight(ctx context.Context, p *Preflight, dir string) (*PreflightReceipt, *InfrastructureFailure) {
	d := p.Declaration
	receipt := &PreflightReceipt{Schema: PreflightSchema, DeclarationFile: p.File, DeclarationDigest: p.Digest, BaseURL: d.BaseURL, Outcome: PreflightPassed, Probes: make([]PreflightProbeResult, 0, len(d.Probes))}
	total, cancel := context.WithTimeout(ctx, time.Duration(d.TotalTimeoutMS)*time.Millisecond)
	defer cancel()
	var failure *InfrastructureFailure
	for _, probe := range d.Probes {
		result := PreflightProbeResult{Name: probe.Name, Kind: probe.Kind, State: PreflightNotRun, Artefact: preflightArtefact(d.BaseURL, probe)}
		switch {
		case failure != nil:
			result.Reason = "earlier-probe-failed"
		case ctx.Err() != nil:
			result.Reason = "cancelled"
		case total.Err() != nil:
			result.Reason = "preflight-timeout"
		default:
			started := time.Now()
			runProbe(total, probe, dir, &result)
			result.DurationMS = time.Since(started).Milliseconds()
			if result.State == PreflightFailed && ctx.Err() == nil && total.Err() != nil && result.Reason != "command-timeout" {
				// The total budget, not the probe's own bound, ended it.
				result.State, result.Reason, result.Observed = PreflightNotRun, "preflight-timeout", ""
			}
		}
		if failure == nil && result.State != PreflightPassed {
			receipt.Outcome = PreflightFailed
			failure = &InfrastructureFailure{Reason: ReasonPreflightFailed, Detail: result.Name + ": " + result.Reason, Phase: InfrastructurePhasePreflight, Probe: result.Name, Artefact: result.Artefact}
		}
		receipt.Probes = append(receipt.Probes, result)
	}
	return receipt, failure
}

func preflightArtefact(base string, p PreflightProbe) string {
	switch p.Kind {
	case PreflightHTTP:
		return strings.TrimSuffix(base, "/") + p.Path
	case PreflightCommand:
		return p.Argv[0]
	default:
		return filepath.ToSlash(filepath.Clean(p.File))
	}
}

func runProbe(ctx context.Context, p PreflightProbe, dir string, result *PreflightProbeResult) {
	limit := time.Duration(p.TimeoutMS) * time.Millisecond
	reason, observed := "", ""
	switch p.Kind {
	case PreflightHTTP:
		reason, observed = probeHTTP(ctx, result.Artefact, p.ExpectStatus, limit)
	case PreflightCommand:
		reason, observed = probeCommand(ctx, p.Argv, dir, limit)
	case PreflightStorageState:
		var digest string
		reason, observed, digest = probeStorageState(filepath.Join(dir, p.File), p)
		result.ArtefactDigest = digest
	}
	result.State = PreflightPassed
	if reason != "" {
		result.State = PreflightFailed
	}
	result.Reason, result.Observed = reason, observed
}

// probeHTTP polls one GET until an admitted status answers or the probe bound
// ends. Redirects are never followed.
func probeHTTP(ctx context.Context, address string, expect []int, limit time.Duration) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	last := 0
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return "http-unreachable", ""
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			last = resp.StatusCode
			if admittedStatus(last, expect) {
				return "", fmt.Sprintf("status %d", last)
			}
		}
		select {
		case <-ctx.Done():
			if last != 0 {
				return "http-status-unexpected", fmt.Sprintf("status %d", last)
			}
			return "http-unreachable", ""
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func admittedStatus(status int, expect []int) bool {
	if len(expect) == 0 {
		return status >= 200 && status < 300
	}
	return slices.Contains(expect, status)
}

// probeCommand runs a setup or seed command as its own process group. Its
// output is bounded and never retained.
func probeCommand(ctx context.Context, argv []string, dir string, limit time.Duration) (string, string) {
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: limit, OutputLimit: preflightCommandOutput, OverflowPolicy: procgroup.OverflowTruncate})
	switch {
	case !obs.Started:
		return "command-start-failed", ""
	case obs.TimedOut:
		return "command-timeout", ""
	case !obs.OwnedProcessGroupCleanup:
		return "command-cleanup-unknown", ""
	case !obs.ExitObserved:
		return "command-exit-nonzero", "signal " + obs.Signal
	case obs.ExitStatus != 0:
		return "command-exit-nonzero", fmt.Sprintf("exit %d", obs.ExitStatus)
	}
	return "", "exit 0"
}

type storageState struct {
	Cookies *[]struct {
		Name    string  `json:"name"`
		Expires float64 `json:"expires"`
	} `json:"cookies"`
	Origins *[]struct {
		Origin string `json:"origin"`
	} `json:"origins"`
}

// probeStorageState checks a Playwright storage-state file exists, parses,
// holds every required cookie and origin, and that each required cookie with
// an expiry stays valid for minValidSeconds. It returns the file digest.
func probeStorageState(path string, p PreflightProbe) (string, string, string) {
	data, err := readBoundedReport(path, preflightStateMaxBytes)
	if errors.Is(err, os.ErrNotExist) {
		return "storage-state-missing", "", ""
	}
	if err != nil {
		return "storage-state-unreadable", "", ""
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	var state storageState
	if json.Unmarshal(data, &state) != nil || state.Cookies == nil || state.Origins == nil {
		return "storage-state-invalid", "", digest
	}
	deadline := float64(preflightNow().Unix() + p.MinValidSeconds)
	for _, name := range p.RequireCookies {
		found, valid := false, false
		for _, cookie := range *state.Cookies {
			if cookie.Name == name {
				found = true
				valid = valid || cookie.Expires == -1 || cookie.Expires > deadline
			}
		}
		if !found {
			return "storage-state-cookie-missing", "cookie " + name, digest
		}
		if !valid {
			return "storage-state-cookie-expiring", "cookie " + name, digest
		}
	}
	for _, origin := range p.RequireOrigins {
		if !slices.ContainsFunc(*state.Origins, func(o struct {
			Origin string `json:"origin"`
		}) bool {
			return o.Origin == origin
		}) {
			return "storage-state-origin-missing", "origin " + origin, digest
		}
	}
	return "", "", digest
}

// preflightShapeError checks a receipt's preflight member against its
// infrastructure and tests (PWP-V0-093). A violation removes passing authority.
func preflightShapeError(r Receipt) error {
	phase := r.Infrastructure != nil && r.Infrastructure.Phase != ""
	if r.Preflight == nil {
		if phase {
			return errors.New("preflight-phase-without-preflight")
		}
		return nil
	}
	p := r.Preflight
	if r.Profile == "" || r.Profile == FreshnessProfile || r.External == nil {
		return errors.New("preflight-requires-external-profile")
	}
	if p.Schema != PreflightSchema || len(p.DeclarationDigest) != 64 || preflightBaseURL(p.BaseURL) != nil || len(p.Probes) == 0 || len(p.Probes) > preflightMaxProbes {
		return errors.New("preflight-invalid")
	}
	failed := -1
	for i, probe := range p.Probes {
		if !preflightProbeName.MatchString(probe.Name) || (probe.Kind != PreflightHTTP && probe.Kind != PreflightCommand && probe.Kind != PreflightStorageState) {
			return errors.New("preflight-invalid")
		}
		switch probe.State {
		case PreflightPassed:
			if failed >= 0 || probe.Reason != "" {
				return errors.New("preflight-invalid")
			}
		case PreflightFailed, PreflightNotRun:
			if !preflightReasons[probe.Reason] {
				return errors.New("preflight-invalid")
			}
			if failed < 0 {
				failed = i
			} else if probe.State != PreflightNotRun || probe.Reason != "earlier-probe-failed" {
				return errors.New("preflight-invalid")
			}
		default:
			return errors.New("preflight-invalid")
		}
	}
	if failed < 0 {
		if p.Outcome != PreflightPassed || phase {
			return errors.New("preflight-invalid")
		}
		return nil
	}
	first := p.Probes[failed]
	x := r.Infrastructure
	if p.Outcome != PreflightFailed || x == nil || x.Reason != ReasonPreflightFailed || x.Phase != InfrastructurePhasePreflight || x.Probe != first.Name || x.Artefact != first.Artefact {
		return errors.New("preflight-invalid")
	}
	if len(r.Tests) != 0 {
		return errors.New("preflight-failure-has-test-verdicts")
	}
	return nil
}

// preflightAnchors names the probe and artefact behind a preflight
// infrastructure projection (PWP-V0-092).
func preflightAnchors(x *InfrastructureFailure) []string {
	if x == nil || x.Phase != InfrastructurePhasePreflight {
		return nil
	}
	anchors := []string{"preflight-probe:" + x.Probe}
	if x.Artefact != "" {
		anchors = append(anchors, "preflight-artefact:"+x.Artefact)
	}
	return anchors
}

// digestFileOrEmpty is the file's SHA-256, or "" when it cannot be read, so a
// vanished declaration reads as changed.
func digestFileOrEmpty(path string) string {
	data, err := readBoundedReport(path, preflightMaxBytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
