package appmap

import (
	"bytes"
	"context"
	"encoding/json"
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
