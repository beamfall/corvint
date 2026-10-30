package perfbudget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"time"
)

// BudgetSchema names the committed budgets file.
const BudgetSchema = "corvint-perf-budget/0"

// ScenarioIDs are the six measured reads GitHub #1602 names, then the unchanged-pair delta of
// GitHub #1634 (PRB-V0-050), then the code-repository index, context, lifecycle-hook and Go and
// TypeScript affected scenarios (PRB-V0-057), in report order.
var ScenarioIDs = []string{"cli-cold-read", "cli-warm-read", "mcp-first-call", "mcp-warm-call", "full-delta", "affected-path", "unchanged-delta", "index-cold-build", "index-warm-build", "context-query", "lifecycle-hook", "affected-go", "affected-ts"}

// Metrics a limit may bound.
const (
	MetricP50Wall = "p50_wall_ms"
	MetricP95Wall = "p95_wall_ms"
	MetricPeakRSS = "peak_rss_bytes"
)

// Limit statuses, the enforcement state of a limit (PRB-V0-054). An enforced limit fails the run
// when exceeded or unobserved; a pending limit is a target whose owning change has not landed; an
// advisory limit is a target no change is committed to enforce. Pending and advisory limits are
// measured and reported but never fail the run, and never pass it (PRB-V0-056).
const (
	StatusEnforced = "enforced"
	StatusPending  = "pending"
	StatusAdvisory = "advisory"
)

// Product profiles a target belongs to: the surface labels of
// docs/specs/corvint-1.0-product-and-release-v1.md (PRB-V0-055).
const (
	ProfileCore              = "core"
	ProfileRequiredCompanion = "required-companion"
	ProfileCompanion         = "companion"
	ProfileExperimental      = "experimental"
)

var profiles = []string{ProfileCore, ProfileRequiredCompanion, ProfileCompanion, ProfileExperimental}

// Budgets is the committed budgets file.
type Budgets struct {
	Schema    string           `json:"schema"`
	Scale     string           `json:"scale"`
	Scenarios []ScenarioBudget `json:"scenarios"`
	Baseline  *Baseline        `json:"baseline"`
}

// ScenarioBudget bounds one scenario. Targets are the product goals (#1598, #1601); ceilings are
// regression guards derived from the recorded baseline and always enforced.
type ScenarioBudget struct {
	ID       string  `json:"id"`
	Samples  int     `json:"samples"`
	Targets  []Limit `json:"targets"`
	Ceilings []Limit `json:"ceilings"`
}

// Limit is one bound on one metric.
type Limit struct {
	Metric    string  `json:"metric"`
	Limit     float64 `json:"limit"`
	Status    string  `json:"status"`
	Profile   string  `json:"profile,omitempty"`
	PendingOn string  `json:"pending_on,omitempty"`
	Reason    string  `json:"reason"`
}

// Baseline is a recorded measurement with the host it ran on (GitHub #1406).
type Baseline struct {
	RecordedAt string             `json:"recorded_at"`
	Revision   string             `json:"revision"`
	Host       Host               `json:"host"`
	Fixture    *Fixture           `json:"fixture"`
	Scenarios  []ScenarioMeasured `json:"scenarios"`
}

// LoadBudgets reads and validates a budgets file.
func LoadBudgets(path string) (*Budgets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Budgets
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&b); err != nil {
		return nil, fmt.Errorf("budgets %s: %w", path, err)
	}
	return &b, b.Validate()
}

// Validate refuses a budgets file that does not bound exactly the ScenarioIDs scenarios with well-formed
// limits; a pending limit must name what it waits on and no other limit may, a target names its
// product profile, and a ceiling is always enforced with no profile.
func (b *Budgets) Validate() error {
	if b.Schema != BudgetSchema {
		return fmt.Errorf("budgets: schema %q, want %q", b.Schema, BudgetSchema)
	}
	if b.Scale != Full.Name && b.Scale != Smoke.Name {
		return fmt.Errorf("budgets: unknown scale %q", b.Scale)
	}
	ids := []string{}
	for _, s := range b.Scenarios {
		ids = append(ids, s.ID)
		if s.Samples < 1 || s.Samples > 100 {
			return fmt.Errorf("budgets: %s: samples %d outside 1..100", s.ID, s.Samples)
		}
		for _, l := range append(slices.Clone(s.Targets), s.Ceilings...) {
			if l.Metric != MetricP50Wall && l.Metric != MetricP95Wall && l.Metric != MetricPeakRSS {
				return fmt.Errorf("budgets: %s: unknown metric %q", s.ID, l.Metric)
			}
			if !(l.Limit > 0) || math.IsInf(l.Limit, 0) {
				return fmt.Errorf("budgets: %s: %s limit must be positive", s.ID, l.Metric)
			}
			if l.Status != StatusEnforced && l.Status != StatusPending && l.Status != StatusAdvisory {
				return fmt.Errorf("budgets: %s: %s status %q", s.ID, l.Metric, l.Status)
			}
			if (l.Status == StatusPending) != (l.PendingOn != "") || l.Reason == "" {
				return fmt.Errorf("budgets: %s: %s needs a reason, and pending_on exactly when pending (never when enforced or advisory)", s.ID, l.Metric)
			}
		}
		for _, l := range s.Targets {
			if !slices.Contains(profiles, l.Profile) {
				return fmt.Errorf("budgets: %s: target %s profile %q, want one of %v", s.ID, l.Metric, l.Profile, profiles)
			}
		}
		ceilings := map[string]bool{}
		for _, l := range s.Ceilings {
			if l.Status != StatusEnforced || l.Profile != "" {
				return fmt.Errorf("budgets: %s: ceiling %s must be enforced and carry no product profile", s.ID, l.Metric)
			}
			ceilings[l.Metric] = true
		}
		if !ceilings[MetricP95Wall] || !ceilings[MetricPeakRSS] {
			return fmt.Errorf("budgets: %s: needs enforced p95_wall_ms and peak_rss_bytes ceilings", s.ID)
		}
	}
	if !slices.Equal(ids, ScenarioIDs) {
		return fmt.Errorf("budgets: scenarios %v, want exactly %v", ids, ScenarioIDs)
	}
	if b.Baseline != nil {
		for _, m := range b.Baseline.Scenarios {
			if p := m.Provenance; p != nil && (!timestampOK(p.RecordedAt) || !gitRevisionOK(p.ExecutableRevision) || !sha256OK(p.ExecutableSHA256) || p.Host.Cores < 1 || p.Host.Contended == nil) {
				return fmt.Errorf("budgets: baseline %s: provenance needs its run's start time, clean executable revision and SHA-256, and host observation", m.ID)
			}
		}
	}
	return nil
}

func timestampOK(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func gitRevisionOK(s string) bool {
	return len(s) == 40 && hexOK(s)
}

func sha256OK(s string) bool {
	return len(s) == 64 && hexOK(s)
}

func hexOK(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Verdicts of one limit. Only an enforced limit can pass; a pending or advisory target within its
// limit is "within", never "pass" (PRB-V0-056).
const (
	VerdictPass             = "pass"
	VerdictExceeded         = "exceeded"
	VerdictPendingWithin    = "pending-within"
	VerdictPendingExceeded  = "pending-exceeded"
	VerdictUnknown          = "unknown"
	VerdictPendingUnknown   = "pending-unknown"
	VerdictAdvisoryWithin   = "advisory-within"
	VerdictAdvisoryExceeded = "advisory-exceeded"
	VerdictAdvisoryUnknown  = "advisory-unknown"
	VerdictNotJudged        = "not-judged"
)

// Run verdicts (PRB-V0-038).
const (
	RunPass      = "PASS"
	RunFail      = "FAIL"
	RunNotJudged = "NOT_JUDGED"
)

// Judge evaluates the budgets like Evaluate and then withholds judgement when the host record
// cannot support one (PRB-V0-039, PRB-V0-040): every enforced limit with an observed value
// becomes not-judged with the reason, keeping that value. A scenario that failed or was not
// measured, or an enforced metric that was not observed, still fails the run, because neither is
// a timing effect.
func Judge(b *Budgets, host Host, measured []ScenarioMeasured) ([]Check, string, string) {
	checks, ok := Evaluate(b, measured)
	reason := host.NotJudgedReason()
	if reason == "" {
		if ok {
			return checks, RunPass, ""
		}
		return checks, RunFail, ""
	}
	verdict := RunNotJudged
	for i, c := range checks {
		switch {
		case c.Verdict == VerdictUnknown:
			verdict = RunFail
		case c.Limit.Status == StatusEnforced:
			checks[i].Verdict, checks[i].Reason = VerdictNotJudged, reason
		}
	}
	return checks, verdict, reason
}

// Check is one evaluated limit.
type Check struct {
	Scenario string   `json:"scenario"`
	Kind     string   `json:"kind"`
	Limit    Limit    `json:"limit"`
	Observed *float64 `json:"observed"`
	Verdict  string   `json:"verdict"`
	Reason   string   `json:"reason,omitempty"`
}

// Evaluate checks every target and ceiling against the measurements. It fails (ok false) on an
// exceeded enforced limit, on an enforced limit whose metric was not observed, and on a scenario
// that was not measured or errored; pending and advisory limits never fail it.
func Evaluate(b *Budgets, measured []ScenarioMeasured) ([]Check, bool) {
	byID := map[string]ScenarioMeasured{}
	for _, m := range measured {
		byID[m.ID] = m
	}
	checks, ok := []Check{}, true
	for _, s := range b.Scenarios {
		m, found := byID[s.ID]
		for _, group := range []struct {
			kind   string
			limits []Limit
		}{{"target", s.Targets}, {"ceiling", s.Ceilings}} {
			for _, l := range group.limits {
				c := Check{Scenario: s.ID, Kind: group.kind, Limit: l}
				var observed *float64
				switch {
				case !found:
					c.Reason = "scenario not measured"
				case m.Error != "":
					c.Reason = "scenario failed: " + m.Error
				default:
					observed = m.metric(l.Metric)
					if observed == nil {
						c.Reason = l.Metric + " NOT_OBSERVED on this host"
					}
				}
				c.Observed = observed
				switch {
				case l.Status == StatusPending:
					c.Verdict = unenforcedVerdict(observed, l.Limit, VerdictPendingUnknown, VerdictPendingWithin, VerdictPendingExceeded)
				case l.Status == StatusAdvisory:
					c.Verdict = unenforcedVerdict(observed, l.Limit, VerdictAdvisoryUnknown, VerdictAdvisoryWithin, VerdictAdvisoryExceeded)
				case observed == nil:
					c.Verdict, ok = VerdictUnknown, false
				case *observed <= l.Limit:
					c.Verdict = VerdictPass
				default:
					c.Verdict, ok = VerdictExceeded, false
				}
				checks = append(checks, c)
			}
		}
		if !found || m.Error != "" {
			ok = false
		}
	}
	return checks, ok
}

func unenforcedVerdict(observed *float64, limit float64, unknown, within, exceeded string) string {
	switch {
	case observed == nil:
		return unknown
	case *observed <= limit:
		return within
	default:
		return exceeded
	}
}
