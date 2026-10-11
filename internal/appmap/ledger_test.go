package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	js "github.com/Beamfall/corvint/internal/jstestprovider"
)

func readLedger(t *testing.T, m *Map, v *Verification, d *EdgeDeclarations, o Options, require ...string) (EdgeLedger, []byte, bool) {
	t.Helper()
	raw, ok, err := ProjectEdges(context.Background(), m, v, d, require, o)
	if err != nil {
		t.Fatal(err)
	}
	var l EdgeLedger
	if err = json.Unmarshal(raw, &l); err != nil {
		t.Fatal(err)
	}
	return l, raw, ok
}
func ledgerFixture(t *testing.T) (string, string, *Map, Flow, LedgerEdge) {
	t.Helper()
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	f := *m.flow("book-tee-time")
	return root, rev, m, f, ledgerIdentity(f.Steps[0], f.Steps[1])
}
func edgeRow(t *testing.T, l EdgeLedger, id string) LedgerEdge {
	t.Helper()
	for _, r := range l.Edges {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("missing %s", id)
	return LedgerEdge{}
}
func edgeBindings(id string, keys ...string) *EdgeDeclarations {
	d := &EdgeDeclarations{Schema: EdgeDeclarationsSchema}
	for _, k := range keys {
		d.Tests = append(d.Tests, EdgeDeclaration{TestKey: k, Proves: []string{id}})
	}
	return d
}

// ANL-V0-001: identity is independent of flow/test/revision, includes the trigger and selector.
func TestANLV0001Identity(t *testing.T) {
	root, _, m, f, row := ledgerFixture(t)
	l, _, _ := readLedger(t, m, nil, nil, Options{Root: root, Full: true})
	if len(l.Edges) == 0 || edgeRow(t, l, row.ID).Status != EdgeUnproven || row.Trigger != StatusUnknown {
		t.Fatal(l)
	}
	a, b := f.Steps[0], f.Steps[1]
	a.ID = "step:other/a"
	b.ID = "step:other/b"
	if ledgerIdentity(a, b).ID != row.ID {
		t.Fatal("step ID changed identity")
	}
	b.Action = "click"
	if ledgerIdentity(a, b).ID == row.ID {
		t.Fatal("trigger not in identity")
	}
}

// ANL-V0-002: qualified synthetic receipts, exact endpoint binding, contradiction and drift.
func TestANLV0002EvidenceAndDrift(t *testing.T) {
	root, rev, m, f, row := ledgerFixture(t)
	r := pwpReceipt(t, rev, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)})
	d := edgeBindings(row.ID, r.Tests[0].ID)
	l, _, ok := readLedger(t, m, v, d, Options{Root: root, Full: true}, row.ID)
	if !ok || edgeRow(t, l, row.ID).Status != EdgeProven {
		t.Fatalf("pass: %+v", l)
	}
	// Destination-only binding cannot prove the transition.
	v.binds[f.Steps[1].ID] = []string{r.Tests[0].ID}
	l, _, _ = readLedger(t, m, v, nil, Options{Root: root, Full: true})
	if edgeRow(t, l, row.ID).Status != EdgeUnproven {
		t.Fatal("one endpoint proved edge")
	}
	bad := pwpReceipt(t, rev, js.StateFailed)
	bv := verification(t, m, []string{writeReceipt(t, bad)})
	l, _, ok = readLedger(t, m, bv, edgeBindings(row.ID, bad.Tests[0].ID), Options{Root: root, Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status != Contradicted {
		t.Fatal(l)
	}
	// A selector change in the committed flow anchor demotes old placed evidence.
	p := "flows/book-tee-time.json"
	writeFile(t, root, p, strings.Replace(readText(t, root, p), `"slot"`, `"changed-slot"`, 1))
	commitAll(t, root, "selector drift")
	l, _, ok = readLedger(t, m, v, d, Options{Root: root, Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status != UnverifiedAtHead {
		t.Fatal(l)
	}
	l, _, ok = readLedger(t, m, v, d, Options{Root: root, Revision: "missing-revision", Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status == EdgeProven || l.Revision != FreshUnknown {
		t.Fatal(l)
	}
}

// ANL-V0-003: duplicate receipts and flow occurrences do not inflate redundancy; shortest wins.
func TestANLV0003Redundancy(t *testing.T) {
	root, rev, m, f, row := ledgerFixture(t)
	r := pwpReceipt(t, rev, js.StatePassed, js.StatePassed)
	name := writeReceipt(t, r)
	v := verification(t, m, []string{name, name})
	d := edgeBindings(row.ID, r.Tests[0].ID, r.Tests[1].ID)
	short := f
	short.ID = "flow:short"
	short.Steps = append([]Step{}, f.Steps[:2]...)
	m.Flows = append(m.Flows, short)
	l, raw, _ := readLedger(t, m, v, d, Options{Root: root, Full: true})
	got := edgeRow(t, l, row.ID)
	if got.Status != EdgeProven || got.Evidence.Steps != 2 || got.RedundantCount != 1 || len(got.RedundantCrossers) != 1 {
		t.Fatal(got)
	}
	m.Flows[0], m.Flows[len(m.Flows)-1] = m.Flows[len(m.Flows)-1], m.Flows[0]
	_, again, _ := readLedger(t, m, v, d, Options{Root: root, Full: true})
	if !bytes.Equal(raw, again) {
		t.Fatal("flow ordering changed bytes")
	}
}

// ANL-V0-005: budget trimming preserves digest, complete verdict and omitted counts.
func TestANLV0005Budgets(t *testing.T) {
	root, _, m, _, row := ledgerFixture(t)
	full, raw, ok := readLedger(t, m, nil, nil, Options{Root: root, Full: true}, row.ID)
	if ok || full.Satisfied {
		t.Fatal(full)
	}
	small, bounded, ok := readLedger(t, m, nil, nil, Options{Root: root, Budget: 700}, row.ID)
	if ok || small.Digest != full.Digest || len(bounded) > 700 || small.Omitted["edges"] == 0 {
		t.Fatalf("%s", bounded)
	}
	if len(raw) <= len(bounded) {
		t.Fatal("not trimmed")
	}
	_, _, err := ProjectEdges(context.Background(), m, nil, nil, nil, Options{Root: root, Budget: 256})
	if err == nil {
		t.Fatal("tiny header should refuse")
	}
}

// ANL-V0-002: evaluated uncertainty from another key blocks proof; an absent receipt does not.
func TestANLV0002MixedEvidenceFailsClosed(t *testing.T) {
	root, rev, m, _, row := ledgerFixture(t)
	r := pwpReceipt(t, rev, js.StatePassed, js.StateSkipped)
	// Qualify the fixture before exercising aggregation, so an invalid receipt cannot mask it.
	v := verification(t, m, []string{writeReceipt(t, r)})
	l, _, ok := readLedger(t, m, v, edgeBindings(row.ID, r.Tests[0].ID, r.Tests[1].ID), Options{Root: root, Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status != EdgeUnproven {
		t.Fatal("inconclusive evidence accepted", l)
	}
	d := edgeBindings(row.ID, r.Tests[0].ID, "absent-test")
	l, _, ok = readLedger(t, m, v, d, Options{Root: root, Full: true}, row.ID)
	if !ok || edgeRow(t, l, row.ID).Status != EdgeProven {
		t.Fatal("missing receipt blocked valid proof", l)
	}
	bad := pwpReceipt(t, "unresolved-app-revision", js.StatePassed)
	bad.Tests[0].FullName = "other-test"
	bad.Schedule.Starts[0].FullName = "other-test"
	bad.Tests[0].ID = receiptTestID(bad, bad.Tests[0])
	v = verification(t, m, []string{writeReceipt(t, r), writeReceipt(t, bad)})
	l, _, ok = readLedger(t, m, v, edgeBindings(row.ID, r.Tests[0].ID, bad.Tests[0].ID), Options{Root: root, Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status != EdgeUnproven {
		t.Fatal("unresolved evidence accepted", l)
	}
}

// ANL-V0-003: a skipped outcome for the proving test key blocks the proof even beside a fresh pass.
func TestANLV0003SameKeyUncertaintyBlocks(t *testing.T) {
	root, rev, m, _, row := ledgerFixture(t)
	pass := pwpReceipt(t, rev, js.StatePassed, js.StatePassed)
	skip := pwpReceipt(t, rev, js.StateSkipped, js.StateSkipped)
	if pass.Tests[0].ID != skip.Tests[0].ID {
		t.Skip("fixture receipts do not share a test key")
	}
	v := verification(t, m, []string{writeReceipt(t, pass), writeReceipt(t, skip)})
	l, _, ok := readLedger(t, m, v, edgeBindings(row.ID, pass.Tests[0].ID, pass.Tests[1].ID), Options{Root: root, Full: true}, row.ID)
	if ok || edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatal("same-key uncertainty laundered by a pass", l)
	}
}

func sealedObservation(t *testing.T, m *Map, rev string, key string, edges ...ObservedEdge) (*Observation, string) {
	t.Helper()
	obs := Observation{Schema: ObservationSchema, App: m.App, Revision: rev, MapDigest: m.Digest, Authority: AuthorityLearned,
		Traces: []ObservedTrace{{TestKey: key, TraceDigest: "d", Visits: []ObservedVisit{}, Edges: edges}}, Omitted: map[string]int{"visits": 0, "edges": 0}}
	complete, _ := json.Marshal(obs)
	obs.Digest = digest(complete)
	raw, _ := json.Marshal(obs)
	name := filepath.Join(t.TempDir(), "obs.json")
	if err := os.WriteFile(name, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return &obs, name
}

func observedLedger(t *testing.T, m *Map, v *Verification, obs []*Observation, o Options, require ...string) (EdgeLedger, bool) {
	t.Helper()
	raw, ok, err := ProjectObservedEdges(context.Background(), m, v, nil, obs, require, o)
	if err != nil {
		t.Fatal(err)
	}
	var l EdgeLedger
	if err = json.Unmarshal(raw, &l); err != nil {
		t.Fatal(err)
	}
	return l, ok
}

// ANL-V0-023: an observed crossing binds its test key to the matching edge without a declaration;
// proof still needs a qualified receipt at the observed revision; foreign or trimmed input refuses.
func TestANLV0023ObservedEdgesFeedLedger(t *testing.T) {
	root, rev, m, f, row := ledgerFixture(t)
	r := pwpReceipt(t, rev, js.StatePassed)
	key := r.Tests[0].ID
	v := verification(t, m, []string{writeReceipt(t, r)})
	crossing := ObservedEdge{FromSeq: 0, ToSeq: 1, From: row.From, To: row.To, Trigger: "click", Status: StatusResolved}
	stray := ObservedEdge{FromSeq: 1, ToSeq: 2, From: row.To, To: "screen:nowhere", Trigger: "click", Status: StatusResolved}
	obs, name := sealedObservation(t, m, rev, key, crossing, stray)
	loaded, err := LoadObservation(m, name)
	if err != nil || loaded.Digest != obs.Digest {
		t.Fatal(err)
	}
	if l, _ := observedLedger(t, m, v, nil, Options{Root: root, Full: true}); edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatal("proved without observation or declaration")
	}
	l, ok := observedLedger(t, m, v, []*Observation{loaded}, Options{Root: root, Full: true}, row.ID)
	got := edgeRow(t, l, row.ID)
	if !ok || got.Status != EdgeProven || len(got.ObservedBy) != 1 || got.ObservedBy[0] != key || l.Unmatched != 1 || l.Observations[0] != obs.Digest {
		t.Fatalf("%+v", l)
	}
	// An observation at another revision, or one without a receipt, proves nothing.
	other, _ := sealedObservation(t, m, strings.Repeat("0", 40), key, crossing)
	if l, ok = observedLedger(t, m, v, []*Observation{other}, Options{Root: root, Full: true}, row.ID); ok || edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatal("revision mismatch proved the edge")
	}
	unreceipted, _ := sealedObservation(t, m, rev, "unreceipted-test", crossing)
	if l, ok = observedLedger(t, m, v, []*Observation{unreceipted}, Options{Root: root, Full: true}, row.ID); ok || edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatal("observation alone proved the edge")
	}
	// An UNKNOWN observed trigger never binds a typed ledger edge.
	copied := *m
	typed := &copied
	typed.Flows = append([]Flow{}, m.Flows...)
	tf := f
	tf.Steps = append([]Step{}, f.Steps...)
	tf.Steps[1].Action = "click"
	typed.Flows[0] = tf
	unknown := crossing
	unknown.Trigger = StatusUnknown
	uo, _ := sealedObservation(t, typed, rev, key, unknown)
	typedRow := ledgerIdentity(tf.Steps[0], tf.Steps[1])
	if l, _ = observedLedger(t, typed, v, []*Observation{uo}, Options{Root: root, Full: true}); len(edgeRow(t, l, typedRow.ID).ObservedBy) != 0 {
		t.Fatal("UNKNOWN observation bound a typed edge")
	}
	// Tampered, trimmed and foreign observations refuse.
	raw, _ := os.ReadFile(name)
	for label, body := range map[string]string{
		"tampered": strings.Replace(string(raw), `"click"`, `"goto"`, 1),
		"trimmed":  strings.Replace(string(raw), `"edges":0`, `"edges":1`, 1),
		"foreign":  strings.Replace(string(raw), m.Digest, "sha256:other", 1),
	} {
		p := filepath.Join(t.TempDir(), "o.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadObservation(m, p); err == nil {
			t.Fatal(label, "accepted")
		}
	}
}
