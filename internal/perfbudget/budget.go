package perfbudget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
)

// BudgetSchema names the committed budgets file.
const BudgetSchema = "corvint-perf-budget/0"

// ScenarioIDs are the six measured reads GitHub #1602 names, then the unchanged-pair delta of
// GitHub #1634 (PRB-V0-050), in report order.
var ScenarioIDs = []string{"cli-cold-read", "cli-warm-read", "mcp-first-call", "mcp-warm-call", "full-delta", "affected-path", "unchanged-delta"}

// Metrics a limit may bound.
const (
	MetricP50Wall = "p50_wall_ms"
	MetricP95Wall = "p95_wall_ms"
	MetricPeakRSS = "peak_rss_bytes"
)

// Limit statuses. An enforced limit fails the run when exceeded or unobserved; a pending limit
// is a target whose owning change has not landed, measured and reported but never failing.
const (
	StatusEnforced = "enforced"
	StatusPending  = "pending"
)

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
// limits; a pending limit must name what it waits on, and a ceiling can never be pending.
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
			if l.Status != StatusEnforced && l.Status != StatusPending {
				return fmt.Errorf("budgets: %s: %s status %q", s.ID, l.Metric, l.Status)
			}
			if (l.Status == StatusPending) != (l.PendingOn != "") || l.Reason == "" {
				return fmt.Errorf("budgets: %s: %s needs a reason, and pending_on exactly when pending", s.ID, l.Metric)
			}
		}
		ceilings := map[string]bool{}
		for _, l := range s.Ceilings {
			if l.Status != StatusEnforced {
				return fmt.Errorf("budgets: %s: ceiling %s must be enforced", s.ID, l.Metric)
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
	return nil
}

// Verdicts of one limit.
const (
	VerdictPass            = "pass"
	VerdictExceeded        = "exceeded"
	VerdictPendingPass     = "pending-pass"
	VerdictPendingExceeded = "pending-exceeded"
	VerdictUnknown         = "unknown"
	VerdictPendingUnknown  = "pending-unknown"
)

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
// that was not measured or errored; pending limits never fail it.
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
				pending := l.Status == StatusPending
				switch {
				case observed == nil && pending:
					c.Verdict = VerdictPendingUnknown
				case observed == nil:
					c.Verdict, ok = VerdictUnknown, false
				case *observed <= l.Limit && pending:
					c.Verdict = VerdictPendingPass
				case *observed <= l.Limit:
					c.Verdict = VerdictPass
				case pending:
					c.Verdict = VerdictPendingExceeded
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
