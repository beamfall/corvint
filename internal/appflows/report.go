package appflows

import (
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func ValidateEvidence(in Input, e Evidence) error {
	bad := errors.New("invalid or unsupported flow evidence")
	if e.Profile != EvidenceProfile || e.Authority != "CALLER_REPORTED" {
		return bad
	}
	if e.CleanupScope != "owned-process-group-and-owned-browser" {
		return bad
	}
	if !wire.IsSha256(e.ProviderDigest) || e.Parser != "typescript-5.9.3" || len(e.NodeVersion) > 64 {
		return bad
	}
	if e.Mode == "observe" && e.PlaywrightVersion != "1.63.0" {
		return bad
	}
	if e.Mode != "scan" && e.Mode != "observe" {
		return bad
	}
	if !wire.IsSha256(e.Binding.ManifestDigest) || !wire.IsGitOid(e.Binding.Tree) {
		return bad
	}
	if len(e.Tests) > 256 || len(e.Controls) > 100 || len(e.Runs) > 25 || len(e.Gaps) > 128 {
		return bad
	}
	if len(e.Binding.Sources) > MaxSources+1 || len(e.Parser) > 64 || len(e.Browser) > 64 {
		return bad
	}
	if !wire.IsSha256(e.Binding.FrontendDigest) || !wire.IsSha256(e.Binding.BackendDigest) {
		return bad
	}
	if !identifier.MatchString(e.Binding.Fixture) {
		return bad
	}
	for p, digest := range e.Binding.Sources {
		if !safePath(p) || !wire.IsSha256(digest) {
			return bad
		}
	}
	for _, gap := range e.Gaps {
		if len(gap) > 256 {
			return bad
		}
	}
	seen := map[string]bool{}
	for _, t := range e.Tests {
		if seen[t.ID] || !wire.IsSha256(t.ID) {
			return bad
		}
		seen[t.ID] = true
		if t.ID != Digest([]byte(t.Anchor.Path+"\x00"+strconv.Itoa(t.Anchor.Line))) {
			return bad
		}
		if err := validateTest(in, e, t); err != nil {
			return err
		}
	}
	if e.Mode == "scan" && (len(e.Runs) != 0 || len(e.Controls) != 0) {
		return bad
	}
	if e.Mode == "observe" && !wire.IsSha256(e.RunID) {
		return bad
	}
	seen = map[string]bool{}
	for _, c := range e.Controls {
		if !wire.IsSha256(c.ID) || !wire.IsSha256(c.State) || seen[c.ID] {
			return bad
		}
		seen[c.ID] = true
		if c.Selector != "" && !selector.MatchString(c.Selector) {
			return bad
		}
		if !slices.Contains([]string{"link", "form", "input", "button", "unsupported"}, c.Kind) {
			return bad
		}
	}
	seen = map[string]bool{}
	for _, r := range e.Runs {
		if seen[r.Scenario] {
			return bad
		}
		seen[r.Scenario] = true
		if err := validateRun(in.Manifest, r); err != nil {
			return err
		}
	}
	return nil
}

func validateTest(in Input, e Evidence, t Test) error {
	bad := errors.New("invalid test evidence")
	if !slices.Contains(in.Manifest.Tests, t.Anchor.Path) {
		return bad
	}
	if t.Anchor.Digest != e.Binding.Sources[t.Anchor.Path] || t.Anchor.Line < 1 {
		return bad
	}
	if t.Complete && !localPath(t.Route) {
		return bad
	}
	if len(t.Route) > 256 || len(t.Actions) > 50 || len(t.Assertions) > 50 {
		return bad
	}
	for _, a := range t.Actions {
		if a.Kind == "fill" && !wire.IsSha256(a.Value) {
			return bad
		}
		if err := validateAction(a); err != nil {
			return err
		}
	}
	for _, a := range t.Assertions {
		if a.Anchor.Path != t.Anchor.Path || a.Anchor.Digest != t.Anchor.Digest || a.Anchor.Line < t.Anchor.Line {
			return bad
		}
		if !selector.MatchString(a.Selector) || !wire.IsSha256(a.WantDigest) {
			return bad
		}
		if !slices.Contains([]string{"text", "contains", "count", "visible"}, a.Kind) {
			return bad
		}
	}
	return nil
}

func validateRun(m Manifest, r Run) error {
	bad := errors.New("invalid runtime evidence")
	i := slices.IndexFunc(m.Scenarios, func(s Scenario) bool { return s.ID == r.Scenario })
	if i < 0 || len(r.States) > 50 || len(r.Checks) > 25 {
		return bad
	}
	if !slices.Contains([]string{"matched", "contradicted", "inconclusive"}, r.Outcome) {
		return bad
	}
	for _, state := range r.States {
		if !wire.IsSha256(state) {
			return bad
		}
	}
	seen := map[string]bool{}
	failed := false
	for _, c := range r.Checks {
		if seen[c.ID] {
			return bad
		}
		seen[c.ID] = true
		ci := slices.IndexFunc(m.Scenarios[i].Checks, func(w Check) bool { return w.ID == c.ID })
		if ci < 0 || !slices.Contains([]string{"matched", "contradicted", "unknown"}, c.Outcome) {
			return bad
		}
		layer := "ui"
		if m.Scenarios[i].Checks[ci].Kind == "json" {
			layer = "backend"
		}
		if c.Layer != layer {
			return bad
		}
		if c.Outcome != "matched" {
			failed = true
		}
	}
	if r.Outcome == "matched" {
		if failed || !r.IdentityMatched || len(seen) != len(m.Scenarios[i].Checks) || len(r.States) == 0 {
			return bad
		}
	}
	return nil
}

func ReportFor(in Input, e *Evidence) (Report, error) {
	r := Report{Profile: ReportProfile, Authority: "CALLER_REPORTED", Application: in.Manifest.Application,
		Binding: in.Binding, Freshness: "unobserved", Flows: []Flow{}, Discovered: []Control{},
		Candidates: []Candidate{},
		Frontier:   []string{"application-completeness-unknown", "declared-test-inventory-only"}}
	if e != nil {
		if err := ValidateEvidence(in, *e); err != nil {
			return r, err
		}
		r.Freshness = "current"
		r.RunID, r.ProviderDigest = e.RunID, e.ProviderDigest
		if !reflect.DeepEqual(in.Binding, e.Binding) {
			r.Freshness = "stale"
		}
		r.Frontier = append(r.Frontier, e.Gaps...)
		r.Discovered = e.Controls
		for _, t := range e.Tests {
			if !t.Complete {
				continue
			}
			r.Candidates = append(r.Candidates, Candidate{ID: t.ID, Basis: "test-inferred", Route: t.Route, Actions: t.Actions, Source: t.Anchor, Freshness: r.Freshness})
		}
		for _, c := range e.Controls {
			if !c.Exercised {
				r.Frontier = append(r.Frontier, "unexplored-control:"+c.ID)
			}
		}
	}
	for _, s := range in.Manifest.Scenarios {
		r.Flows = append(r.Flows, reportFlow(s, e, r.Freshness))
	}
	slices.Sort(r.Frontier)
	r.Frontier = slices.Compact(r.Frontier)
	return r, nil
}

func reportFlow(s Scenario, e *Evidence, freshness string) Flow {
	f := Flow{ID: s.ID, Role: s.Role, Basis: s.Basis, Coverage: []Coverage{}, Runtime: "unobserved", Checks: []CheckResult{}, Gaps: []string{}, Next: "observe this declared scenario"}
	f.RoleIdentity = "caller-declared-unverified"
	for _, c := range s.Checks {
		f.Coverage = append(f.Coverage, coverage(s, c, e, freshness))
	}
	if e == nil {
		return f
	}
	if freshness != "current" {
		f.Runtime = "stale"
		f.Gaps = append(f.Gaps, "reobserve-changed-inputs")
		return f
	}
	for _, r := range e.Runs {
		if r.Scenario != s.ID {
			continue
		}
		f.Runtime, f.Checks = r.Outcome, r.Checks
		if !e.Cleanup || !e.BrowserClosed || !e.ServerExited || !r.IdentityMatched {
			f.Runtime = "inconclusive"
		}
		if f.Runtime == "matched" && s.Basis == "inferred" {
			f.Runtime = "hypothesis-agreement"
		}
		if f.Runtime == "contradicted" {
			f.Next = "inspect the contradicted expected outcome"
		}
	}
	for _, c := range f.Coverage {
		if c.State == "assertion-candidate" {
			continue
		}
		f.Gaps = append(f.Gaps, c.Check+":"+c.State)
	}
	if len(f.Gaps) > 0 && f.Runtime != "contradicted" {
		f.Next = "add or resolve tests for the listed check gaps"
	}
	return f
}

func coverage(s Scenario, c Check, e *Evidence, freshness string) Coverage {
	r := Coverage{Check: c.ID, State: "mapping-unknown", Tests: []string{}, Assertions: []Anchor{}}
	if e == nil || freshness != "current" {
		return r
	}
	if c.Kind == "json" {
		r.State = "backend-test-mapping-unsupported"
		return r
	}
	for _, t := range e.Tests {
		if !t.Complete || t.Route != s.Path || !sameActions(t.Actions, s.Actions) {
			continue
		}
		r.Tests = append(r.Tests, t.ID)
		for _, a := range t.Assertions {
			if a.Kind == c.Kind && a.Selector == c.Selector && a.WantDigest == ValueDigest(c.Want) {
				r.Assertions = append(r.Assertions, a.Anchor)
			}
		}
	}
	if len(r.Assertions) > 0 {
		r.State = "assertion-candidate"
		return r
	}
	if !e.InventoryComplete {
		return r
	}
	r.State = "no-test-in-declared-inventory"
	if len(r.Tests) > 0 {
		r.State = "action-candidate-without-assertion"
	}
	return r
}

func sameActions(observed, expected []Action) bool {
	want := slices.Clone(expected)
	for i := range want {
		if want[i].Kind == "fill" {
			want[i].Value = Digest([]byte(want[i].Value))
		}
	}
	return slices.Equal(observed, want)
}

// Record is explicit opt-in learning: a new private artifact, never an implicit index update.
func Record(in Input, raw []byte, filename string) error {
	var e Evidence
	if err := Decode(raw, &e); err != nil {
		return err
	}
	r, err := ReportFor(in, &e)
	if err != nil {
		return err
	}
	if r.Freshness != "current" {
		return errors.New("cannot record stale flow evidence")
	}
	if strings.TrimSpace(filename) == "" {
		return errors.New("record needs an output file")
	}
	return WriteConfined(in.Root, filename, raw)
}
