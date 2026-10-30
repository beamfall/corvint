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
	// The visits are the edges' resolved endpoints, as Observe emits them.
	for i, e := range edges {
		if i == 0 {
			obs.Traces[0].Visits = append(obs.Traces[0].Visits, ObservedVisit{Seq: 0, URL: "https://x.test/", Screen: e.From, Status: StatusResolved})
		}
		obs.Traces[0].Visits = append(obs.Traces[0].Visits, ObservedVisit{Seq: i + 1, URL: "https://x.test/", Screen: e.To, Status: StatusResolved})
	}
	complete, _ := json.Marshal(obs)
	obs.Digest = digest(complete)
	raw, _ := json.Marshal(obs)
	name := filepath.Join(t.TempDir(), "obs.json")
	if err := os.WriteFile(name, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return &obs, name
}

// resealed edits a sealed observation and recomputes its digest, so only consistency can refuse it.
func resealed(t *testing.T, raw []byte, edit func(*Observation)) string {
	t.Helper()
	var o Observation
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatal(err)
	}
	edit(&o)
	o.Digest = ""
	complete, _ := json.Marshal(o)
	o.Digest = digest(complete)
	out, _ := json.Marshal(o)
	return string(out)
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
	// stray crosses from row.To to a map screen no ledger edge reaches from it.
	full, _, _ := readLedger(t, m, nil, nil, Options{Root: root, Full: true})
	nowhere := ""
	for _, sc := range m.Screens {
		hit := false
		for _, e := range full.Edges {
			hit = hit || (e.From == row.To && e.To == sc.ID)
		}
		if !hit {
			nowhere = sc.ID
			break
		}
	}
	if nowhere == "" {
		t.Fatal("fixture has no unreached screen")
	}
	stray := ObservedEdge{FromSeq: 1, ToSeq: 2, From: row.To, To: nowhere, Trigger: "click", Status: StatusResolved}
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
	// With outcomes for the key at two revisions, the one at the observed revision is read,
	// whichever sorts first; a declared key's proof is unaffected by another key's observation.
	writeFile(t, root, "notes.txt", "unrelated\n")
	rev2 := commitAll(t, root, "unrelated")
	r2 := pwpReceipt(t, rev2, js.StatePassed)
	if r2.Tests[0].ID != key {
		t.Fatal("fixture key differs across revisions")
	}
	v2 := verification(t, m, []string{writeReceipt(t, r), writeReceipt(t, r2)})
	at2, _ := sealedObservation(t, m, rev2, key, crossing)
	for _, o := range []*Observation{loaded, at2} {
		l, ok = observedLedger(t, m, v2, []*Observation{o}, Options{Root: root, Full: true}, row.ID)
		if got := edgeRow(t, l, row.ID); !ok || got.Status != EdgeProven || got.Evidence.Verification.Revision != o.Revision {
			t.Fatalf("revision %s: %+v", o.Revision, got)
		}
	}
	// A skipped outcome placed at another revision does not block the observed revision's pass;
	// the same skip at the observed revision does.
	skipped := pwpReceipt(t, rev2, js.StateSkipped)
	if skipped.Tests[0].ID != key {
		t.Fatal("fixture key differs for a skip")
	}
	v3 := verification(t, m, []string{writeReceipt(t, r), writeReceipt(t, skipped)})
	if l, ok = observedLedger(t, m, v3, []*Observation{loaded}, Options{Root: root, Full: true}, row.ID); !ok || edgeRow(t, l, row.ID).Status != EdgeProven {
		t.Fatalf("skip elsewhere blocked: %+v", l)
	}
	// A skip at a well-formed but unresolvable revision may be at the observed one, so it blocks.
	lost := pwpReceipt(t, strings.Repeat("0", 40), js.StateSkipped)
	if lost.Tests[0].ID != key {
		t.Fatal("fixture key differs for an unresolvable revision")
	}
	v5 := verification(t, m, []string{writeReceipt(t, r), writeReceipt(t, lost)})
	if l, ok = observedLedger(t, m, v5, []*Observation{loaded}, Options{Root: root, Full: true}, row.ID); ok || edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatalf("unresolvable skip discarded: %+v", l)
	}
	v4 := verification(t, m, []string{writeReceipt(t, r2), writeReceipt(t, skipped)})
	if l, ok = observedLedger(t, m, v4, []*Observation{at2}, Options{Root: root, Full: true}, row.ID); ok || edgeRow(t, l, row.ID).Status == EdgeProven {
		t.Fatalf("skip at the observed revision did not block: %+v", l)
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
		"tampered":            strings.Replace(string(raw), `"click"`, `"goto"`, 1),
		"trimmed":             strings.Replace(string(raw), `"edges":0`, `"edges":1`, 1),
		"foreign":             strings.Replace(string(raw), m.Digest, "sha256:other", 1),
		"no visits":           resealed(t, raw, func(o *Observation) { o.Traces[0].Visits = []ObservedVisit{} }),
		"edge off its visits": resealed(t, raw, func(o *Observation) { o.Traces[0].Edges[0].To = nowhere }),
		"unmapped screen":     resealed(t, raw, func(o *Observation) { o.Traces[0].Visits[2].Screen = "screen:nowhere" }),
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

// ANL-V0-024: an edge assumed by a test is satisfied only when some other test proves it; trimming
// redundant crossers out of the projection does not change that verdict.
func TestANLV0024SelfProofExcluded(t *testing.T) {
	root, rev, m, _, row := ledgerFixture(t)
	r := pwpReceipt(t, rev, js.StatePassed, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)})
	k0, k1 := r.Tests[0].ID, r.Tests[1].ID
	decl := func(tests ...EdgeDeclaration) *EdgeDeclarations {
		return &EdgeDeclarations{Schema: EdgeDeclarationsSchema, Tests: tests}
	}
	self := decl(EdgeDeclaration{TestKey: k0, Proves: []string{row.ID}, Assumes: []string{row.ID}})
	l, _, ok := readLedger(t, m, v, self, Options{Root: root, Full: true})
	if ok || l.Satisfied || edgeRow(t, l, row.ID).Status != EdgeProven || len(l.Unsatisfied) != 1 || l.Unsatisfied[0] != row.ID {
		t.Fatalf("self-proof satisfied: %+v", l)
	}
	cases := map[string]*EdgeDeclarations{
		"another prover":  decl(EdgeDeclaration{TestKey: k0, Proves: []string{row.ID}, Assumes: []string{row.ID}}, EdgeDeclaration{TestKey: k1, Proves: []string{row.ID}}),
		"another assumer": decl(EdgeDeclaration{TestKey: k0, Proves: []string{row.ID}}, EdgeDeclaration{TestKey: k1, Assumes: []string{row.ID}}),
	}
	for name, d := range cases {
		full, _, ok := readLedger(t, m, v, d, Options{Root: root, Full: true})
		if !ok || !full.Satisfied {
			t.Fatalf("%s: %+v", name, full)
		}
		small, bounded, ok := readLedger(t, m, v, d, Options{Root: root, Budget: 700})
		if !ok || !small.Satisfied || small.Digest != full.Digest || len(bounded) > 700 {
			t.Fatalf("%s trimmed: %s", name, bounded)
		}
	}
	// --require has no test identity and keeps the plain proven verdict.
	if l, _, ok := readLedger(t, m, v, edgeBindings(row.ID, k0), Options{Root: root, Full: true}, row.ID); !ok || !l.Satisfied {
		t.Fatal(l)
	}
}
