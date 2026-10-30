package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"
)

// Prefix lint (ANL-V0-009..011): flags specs that replay a navigation prefix whose edges the edge
// ledger already proves, when the prefix ends on a freshly deep-linkable screen.
const (
	PrefixesSchema    = "test-plan-prefixes/0"
	PrefixFactsSchema = "test-prefix-facts/0"
)

// PrefixFacts are caller-owned per-spec edge classifications keyed `FROM_SCREEN>TO_SCREEN`.
// Without a declaration an edge's effect is unknown and the spec abstains.
type PrefixFacts struct {
	Schema string       `json:"schema"`
	Tests  []PrefixFact `json:"tests"`
}
type PrefixFact struct {
	Path           string   `json:"path"`
	AssertionEdges []string `json:"assertion_edges"`
	MutatingEdges  []string `json:"mutating_edges"`
	ReadOnlyEdges  []string `json:"read_only_edges"`
}

func LoadPrefixFacts(name string) (*PrefixFacts, error) {
	raw, err := readReceipt(name)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "--facts must name a regular JSON file of at most 4 MiB")
	}
	var f PrefixFacts
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&f) != nil || dec.Decode(new(any)) != io.EOF || f.Schema != PrefixFactsSchema || len(f.Tests) > 4096 {
		return nil, verifyError("appmap-invalid-query", "invalid prefix facts")
	}
	seen := map[string]bool{}
	for _, t := range f.Tests {
		if len(t.AssertionEdges)+len(t.MutatingEdges)+len(t.ReadOnlyEdges) > 256 {
			return nil, verifyError("appmap-invalid-query", "at most 256 edge classifications per test")
		}
		if seen[t.Path] {
			return nil, verifyError("appmap-invalid-query", "duplicate prefix facts for one test path")
		}
		seen[t.Path] = true
	}
	return &f, nil
}

// LoadEdgeLedger reads an application-edge-ledger/0 document produced by `flows appmap edges`.
func LoadEdgeLedger(name string) (*EdgeLedger, error) {
	raw, err := readReceipt(name)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "--ledger must name a regular JSON file of at most 4 MiB")
	}
	var l EdgeLedger
	if json.Unmarshal(raw, &l) != nil || l.Schema != EdgeLedgerSchema {
		return nil, verifyError("appmap-invalid-query", "invalid edge ledger")
	}
	return &l, nil
}

type FlaggedPrefix struct {
	Test          string   `json:"test"`
	PrefixEdges   []string `json:"prefix_edges"`
	ProvenBy      []string `json:"proven_by"`
	EntryScreen   string   `json:"entry_screen"`
	EntryTemplate string   `json:"entry_template"`
	Params        []string `json:"params"`
	Suggestion    string   `json:"suggestion"`
	SavedSteps    int      `json:"saved_steps"`
	SharedWith    []string `json:"shared_with"`
}
type PrefixAbstention struct {
	Test   string `json:"test"`
	Reason string `json:"reason"`
	// Edge, Ref and Call name the step and the call that decided a derived-effect abstention
	// (ANL-V0-036); absent for the other reasons.
	Edge string `json:"edge,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Call string `json:"call,omitempty"`
}
type PrefixReport struct {
	Schema       string             `json:"schema"`
	App          string             `json:"app"`
	Revision     string             `json:"revision"`
	MapDigest    string             `json:"map_digest"`
	LedgerDigest string             `json:"ledger_digest,omitempty"`
	Authority    string             `json:"authority"`
	Flagged      []FlaggedPrefix    `json:"flagged"`
	Abstained    []PrefixAbstention `json:"abstained"`
	Omitted      map[string]int     `json:"omitted"`
	Digest       string             `json:"digest"`
}

func prefixEdgeID(from, to string) string { return from + ">" + to }

// Prefixes lints every spec's test-sequence chain. A nil ledger proves nothing, so nothing flags.
func Prefixes(ctx context.Context, m *Map, l *EdgeLedger, f *PrefixFacts, o Options) ([]byte, error) {
	cap, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	p := newProjection(ctx, m, o)
	for i := range m.Screens {
		for _, a := range m.lineage(&m.Screens[i]) {
			p.cite(a)
		}
	}
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" && e.Effect != "" {
			p.cite(e.Anchor)
			for _, a := range e.EffectFrom {
				p.cite(a)
			}
		}
	}
	p.check()
	if err := checkLedgerBinding(m, l, p.fresh.evaluated); err != nil {
		return nil, err
	}
	proven := map[string]string{}
	rep := PrefixReport{Schema: PrefixesSchema, App: m.App, Revision: p.fresh.evaluated, MapDigest: m.Digest, Authority: AuthorityLearned, Flagged: []FlaggedPrefix{}, Abstained: []PrefixAbstention{}, Omitted: map[string]int{"flagged": 0, "abstained": 0}}
	if l != nil {
		rep.LedgerDigest = l.Digest
		for _, r := range l.Edges {
			if test, ok := provenBy(r); ok {
				k := prefixEdgeID(r.From, r.To)
				if cur, seen := proven[k]; !seen || test < cur {
					proven[k] = test
				}
			}
		}
	}
	facts := map[string]PrefixFact{}
	if f != nil {
		for _, t := range f.Tests {
			facts[t.Path] = t
		}
	}
	bySource := map[string][]Edge{}
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" {
			bySource[e.Source] = append(bySource[e.Source], e)
		}
	}
	files := append([]TestFile{}, m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, tf := range files {
		if tf.Role != roleSpec {
			continue
		}
		edges := bySource[tf.ID]
		if len(edges) == 0 {
			continue
		}
		abstain := func(reason string) {
			rep.Abstained = append(rep.Abstained, PrefixAbstention{Test: tf.Path, Reason: reason})
		}
		if tf.Join != StatusResolved {
			abstain("join-unknown")
			continue
		}
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].Anchor.Start < edges[j].Anchor.Start })
		ids := make([]string, len(edges))
		continuous := true
		for i, e := range edges {
			ids[i] = prefixEdgeID(e.From, e.To)
			continuous = continuous && (i == 0 || edges[i-1].To == e.From)
		}
		if !continuous {
			abstain("sequence-discontinuous")
			continue
		}
		fact := facts[tf.Path]
		in := func(list []string, id string) bool {
			for _, x := range list {
				if x == id {
					return true
				}
			}
			return false
		}
		reason, step := "", PrefixAbstention{}
		by := make([]string, len(ids))
		for i, id := range ids {
			// A derived effect (ANL-V0-035) governs its step: the sidecar may add an assertion or
			// mutation but never upgrades a state-effect or unknown step to read-only (ANL-V0-036).
			e := edges[i]
			derived := func(r string) {
				if reason == "" {
					reason, step = r, PrefixAbstention{Edge: id, Ref: e.EffectRef, Call: e.EffectCall}
				}
			}
			switch {
			case in(fact.AssertionEdges, id):
				reason, step = "assertion-target", PrefixAbstention{}
			case in(fact.MutatingEdges, id):
				reason, step = "state-change", PrefixAbstention{}
			case e.Effect == EffectState:
				derived("step-state-effect")
			case e.Effect == EffectUnknown:
				derived("step-effect-unknown")
			case e.Effect != "" && e.Effect != EffectNavigation:
				derived("step-effect-unknown")
			case e.Effect == EffectNavigation && p.worst(append([]Anchor{e.Anchor}, e.EffectFrom...)) != Fresh:
				derived("step-effect-not-fresh")
			case e.Effect == "" && !in(fact.ReadOnlyEdges, id) && reason == "":
				reason = "effects-unknown"
			case proven[id] == "" && reason == "":
				reason = "edge-not-proven"
			}
			by[i] = proven[id]
		}
		last := m.screen(edges[len(edges)-1].To)
		if reason == "" {
			switch {
			case last == nil || last.Status != StatusResolved || last.Abstract || last.Template == "":
				reason = "entry-not-deep-linkable"
			case p.worst(m.lineage(last)) != Fresh:
				reason = "entry-template-not-fresh"
			}
		}
		if reason == "" && len(ids) < 2 {
			reason = "nothing-to-save"
		}
		if reason != "" {
			step.Test, step.Reason = tf.Path, reason
			rep.Abstained = append(rep.Abstained, step)
			continue
		}
		fp := FlaggedPrefix{Test: tf.Path, PrefixEdges: ids, ProvenBy: by, EntryScreen: last.ID, EntryTemplate: m.url(last), Params: append([]string{}, last.Params...), SavedSteps: len(ids) - 1, SharedWith: []string{}}
		fp.Suggestion = "goto(" + fp.EntryTemplate + ") or a session fixture"
		if len(fp.Params) > 0 {
			fp.Suggestion += " with params " + strings.Join(fp.Params, ",")
		}
		rep.Flagged = append(rep.Flagged, fp)
	}
	for i := range rep.Flagged {
		for j := range rep.Flagged {
			if i != j && rep.Flagged[i].PrefixEdges[0] == rep.Flagged[j].PrefixEdges[0] {
				rep.Flagged[i].SharedWith = append(rep.Flagged[i].SharedWith, rep.Flagged[j].Test)
			}
		}
	}
	complete, _ := json.Marshal(rep)
	rep.Digest = digest(complete)
	for {
		raw, err := json.Marshal(rep)
		if err != nil {
			return nil, err
		}
		raw = append(raw, '\n')
		if len(raw) <= cap {
			return raw, nil
		}
		switch {
		case len(rep.Abstained) > 0:
			rep.Abstained = rep.Abstained[:len(rep.Abstained)-1]
			rep.Omitted["abstained"]++
		case len(rep.Flagged) > 0:
			rep.Flagged = rep.Flagged[:len(rep.Flagged)-1]
			rep.Omitted["flagged"]++
		default:
			return nil, budgetError("prefix report header exceeds --budget")
		}
	}
}

// LoadPrefixReport reads an earlier test-plan-prefixes/0 output used as a --check baseline.
func LoadPrefixReport(name string) (*PrefixReport, error) {
	raw, err := readReceipt(name)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "--baseline must name a regular JSON file of at most 4 MiB")
	}
	var r PrefixReport
	if json.Unmarshal(raw, &r) != nil || r.Schema != PrefixesSchema {
		return nil, verifyError("appmap-invalid-query", "invalid prefix baseline")
	}
	return &r, nil
}
