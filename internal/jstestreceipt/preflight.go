package jstestreceipt

import (
	"errors"
	"net/url"
	"regexp"
	"time"
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
	PreflightMaxBytes        = 64 << 10
	PreflightMaxProbes       = 32
	PreflightMaxProbeTimeout = 10 * time.Minute
	PreflightMaxTotalTimeout = time.Hour
	PreflightMaxArgv         = 64
	PreflightMaxNames        = 32
	PreflightStateMaxBytes   = 16 << 20
	PreflightCommandOutput   = 64 << 10
)

var PreflightProbeName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

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

func PreflightBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(raw) > 4096 {
		return errors.New("baseUrl must be an http(s) URL without credentials, query or fragment")
	}
	return nil
}

// PreflightShapeError checks a receipt's preflight member against its
// infrastructure and tests (PWP-V0-093). A violation removes passing authority.
func PreflightShapeError(r Receipt) error {
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
	if p.Schema != PreflightSchema || len(p.DeclarationDigest) != 64 || PreflightBaseURL(p.BaseURL) != nil || len(p.Probes) == 0 || len(p.Probes) > PreflightMaxProbes {
		return errors.New("preflight-invalid")
	}
	failed := -1
	for i, probe := range p.Probes {
		if !PreflightProbeName.MatchString(probe.Name) || (probe.Kind != PreflightHTTP && probe.Kind != PreflightCommand && probe.Kind != PreflightStorageState) {
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
