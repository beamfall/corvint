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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// PreflightSchema is jstestreceipt.PreflightSchema, kept under its old name.
const PreflightSchema = jstestreceipt.PreflightSchema

// PreflightHTTP is jstestreceipt.PreflightHTTP, kept under its old name.
const PreflightHTTP = jstestreceipt.PreflightHTTP

// PreflightCommand is jstestreceipt.PreflightCommand, kept under its old name.
const PreflightCommand = jstestreceipt.PreflightCommand

// PreflightStorageState is jstestreceipt.PreflightStorageState, kept under its old name.
const PreflightStorageState = jstestreceipt.PreflightStorageState

// PreflightPassed is jstestreceipt.PreflightPassed, kept under its old name.
const PreflightPassed = jstestreceipt.PreflightPassed

// PreflightFailed is jstestreceipt.PreflightFailed, kept under its old name.
const PreflightFailed = jstestreceipt.PreflightFailed

// PreflightNotRun is jstestreceipt.PreflightNotRun, kept under its old name.
const PreflightNotRun = jstestreceipt.PreflightNotRun

// InfrastructurePhasePreflight is jstestreceipt.InfrastructurePhasePreflight, kept under its old name.
const InfrastructurePhasePreflight = jstestreceipt.InfrastructurePhasePreflight

// ReasonPreflightFailed is jstestreceipt.ReasonPreflightFailed, kept under its old name.
const ReasonPreflightFailed = jstestreceipt.ReasonPreflightFailed

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

// PreflightReceipt is jstestreceipt.PreflightReceipt, kept under its old name.
type PreflightReceipt = jstestreceipt.PreflightReceipt

// PreflightProbeResult is jstestreceipt.PreflightProbeResult, kept under its old name.
type PreflightProbeResult = jstestreceipt.PreflightProbeResult

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
	data, err := readBoundedReport(abs, jstestreceipt.PreflightMaxBytes)
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
	if err := jstestreceipt.PreflightBaseURL(d.BaseURL); err != nil {
		return err
	}
	if d.TotalTimeoutMS <= 0 || time.Duration(d.TotalTimeoutMS)*time.Millisecond > jstestreceipt.PreflightMaxTotalTimeout {
		return errors.New("totalTimeoutMs must be in 1..3600000")
	}
	if len(d.Probes) == 0 || len(d.Probes) > jstestreceipt.PreflightMaxProbes {
		return fmt.Errorf("probes must number 1..%d", jstestreceipt.PreflightMaxProbes)
	}
	seen := map[string]bool{}
	for i, p := range d.Probes {
		if !jstestreceipt.PreflightProbeName.MatchString(p.Name) || seen[p.Name] {
			return fmt.Errorf("probes[%d]: name must be unique and match %s", i, jstestreceipt.PreflightProbeName)
		}
		seen[p.Name] = true
		if p.TimeoutMS <= 0 || time.Duration(p.TimeoutMS)*time.Millisecond > jstestreceipt.PreflightMaxProbeTimeout {
			return fmt.Errorf("probe %s: timeoutMs must be in 1..600000", p.Name)
		}
		if err := validatePreflightProbe(p); err != nil {
			return fmt.Errorf("probe %s: %w", p.Name, err)
		}
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
		if len(p.Argv) == 0 || len(p.Argv) > jstestreceipt.PreflightMaxArgv || p.Argv[0] == "" {
			return fmt.Errorf("command probe argv must have 1..%d tokens", jstestreceipt.PreflightMaxArgv)
		}
	case PreflightStorageState:
		if http || command {
			return errors.New("storage-state probe carries members of another kind")
		}
		clean := filepath.Clean(p.File)
		if p.File == "" || filepath.IsAbs(p.File) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("storage-state file must be a path inside the test directory")
		}
		if len(p.RequireCookies) > jstestreceipt.PreflightMaxNames || len(p.RequireOrigins) > jstestreceipt.PreflightMaxNames || p.MinValidSeconds < 0 {
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
	if probeAddress(address) != nil {
		return "http-unreachable", ""
	}
	last := 0
	for {
		if status, err := loopbackGet(ctx, address, false); err == nil {
			last = status
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
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: limit, OutputLimit: jstestreceipt.PreflightCommandOutput, OverflowPolicy: procgroup.OverflowTruncate})
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
	data, err := readBoundedReport(path, jstestreceipt.PreflightStateMaxBytes)
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

// digestFileOrEmpty is the file's SHA-256, or "" when it cannot be read, so a
// vanished declaration reads as changed.
func digestFileOrEmpty(path string) string {
	data, err := readBoundedReport(path, jstestreceipt.PreflightMaxBytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
