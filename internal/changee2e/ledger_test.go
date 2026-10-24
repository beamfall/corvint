package changee2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// writeLedger writes an application-edge-ledger/0 document and returns its path.
func writeLedger(t *testing.T, l appmap.EdgeLedger) string {
	t.Helper()
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// boundLedger is an empty, untrimmed ledger of the head catalog's map.
func boundLedger(t *testing.T, root, head string) appmap.EdgeLedger {
	t.Helper()
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	return appmap.EdgeLedger{Schema: appmap.EdgeLedgerSchema, App: cat.m.App, Revision: cat.Revision, MapDigest: cat.m.Digest,
		Edges: []appmap.LedgerEdge{}, Omitted: map[string]int{"edges": 0}, Digest: "sha256:ledger-under-test"}
}

// CCE-V0-103: a --ledger bound to the head map reaches the impact relation instead of nil; the
// report lists it as consulted with its digest, and it never changes reach. A ledger of another
// revision, map or application, a trimmed ledger, or an unreadable document refuses.
func TestCCEV0103LedgerBoundToImpact(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitFiles(t, root, map[string]*string{"server/handler.go": text("package server\n")}, "backend")
	plain, err := BuildObligations(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if plain.LedgerDigest != "" || !slices.Contains(plain.Relations.NotConsulted, LedgerBasis) || slices.Contains(plain.Relations.Consulted, LedgerBasis) {
		t.Fatalf("no ledger must read not consulted: %+v digest %q", plain.Relations, plain.LedgerDigest)
	}
	opts := diffOpts(root, base, head)
	opts.Ledger = writeLedger(t, boundLedger(t, root, head))
	got, err := BuildObligations(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got.LedgerDigest != "sha256:ledger-under-test" || !slices.Contains(got.Relations.Consulted, LedgerBasis) || slices.Contains(got.Relations.NotConsulted, LedgerBasis) {
		t.Fatalf("bound ledger not consulted: %+v digest %q", got.Relations, got.LedgerDigest)
	}
	if got.ImpactDigest == plain.ImpactDigest || got.Digest == plain.Digest {
		t.Fatal("the impact relation did not bind the ledger")
	}
	a, _ := json.Marshal([]any{plain.State, plain.Obligations, plain.Unknown, plain.Exempt})
	b, _ := json.Marshal([]any{got.State, got.Obligations, got.Unknown, got.Exempt})
	if string(a) != string(b) {
		t.Fatalf("a ledger changed reach:\n%s\n%s", a, b)
	}
	if r, err := Compile(bg, opts); err != nil || r.ObligationsDigest != got.Digest {
		t.Fatalf("compile did not read the ledger obligations: %v", err)
	}

	stale := boundLedger(t, root, head)
	stale.Revision = base
	otherMap := boundLedger(t, root, head)
	otherMap.MapDigest = "sha256:other"
	otherApp := boundLedger(t, root, head)
	otherApp.App = "other-app"
	trimmed := boundLedger(t, root, head)
	trimmed.Omitted = map[string]int{"edges": 1}
	schema := boundLedger(t, root, head)
	schema.Schema = "application-edge-ledger/1"
	for name, l := range map[string]appmap.EdgeLedger{"stale revision": stale, "other map": otherMap, "other app": otherApp,
		"trimmed": trimmed, "wrong schema": schema} {
		opts := diffOpts(root, base, head)
		opts.Ledger = writeLedger(t, l)
		if _, err := BuildObligations(bg, opts); refusalCode(err) != "e2e-ledger-invalid" {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	opts.Ledger = filepath.Join(t.TempDir(), "missing.json")
	if _, err := BuildObligations(bg, opts); refusalCode(err) != "e2e-ledger-invalid" {
		t.Fatalf("missing file: %v", err)
	}
}

// rebookFlow is a flow that starts on the tee sheet, a screen with no admitted deep-link entry.
func rebookFlow(id string, preconditions string) string {
	return `{"schema":"application-flow-intent/1","flow_id":"` + id + `","revision":1,"kind":"ui","actor":"club admin",
"preconditions":[` + preconditions + `],
"steps":[{"step_id":"select-slot","action":"select an open slot"},{"step_id":"book","action":"book the slot"}],
"outcomes":[{"outcome_id":"rebooked","behavior":"slot shows as booked","matcher":"toHaveText","locator":"slot-status","value":"Booked"}],
"variations":[],"links":[],
"navigation":{"precondition_flows":[],"steps":[
{"step_id":"select-slot","state":"/clubs/{clubId}/teesheets","locator":{"test_id":"slot"},"expect":[]},
{"step_id":"book","state":"/clubs/{clubId}/teesheets","locator":{"role":"button","name":"Book"},"expect":["rebooked"]}]}}
`
}

// homeToTeeSheets is a PROVEN ledger edge home -> tee sheet whose crossing is book-tee-time's
// first two steps, verified by a receipt of test.
func homeToTeeSheets(t *testing.T, cat *Catalog, test string) appmap.LedgerEdge {
	t.Helper()
	for _, f := range cat.m.Flows {
		if f.FlowID == "book-tee-time" {
			a, b := f.Steps[0], f.Steps[1]
			return appmap.LedgerEdge{ID: "edge:home-teesheets", From: a.Screen, To: b.Screen, Status: appmap.EdgeProven, RedundantCrossers: []string{},
				Evidence: &appmap.EdgeCrossing{Flow: f.ID, FromStep: a.ID, Step: b.ID, Steps: len(f.Steps),
					Verification: appmap.StepVerification{Status: appmap.Verified, Revision: cat.Revision, Receipt: &appmap.ReceiptRef{TestKey: test}}}}
		}
	}
	t.Fatal("no book-tee-time flow")
	return appmap.LedgerEdge{}
}

func entryFor(r *CompileReport, obligation string) *CompiledEntry {
	for i := range r.Scenarios {
		if r.Scenarios[i].Obligation == obligation {
			return &r.Scenarios[i]
		}
	}
	return nil
}

// CCE-V0-104: a flow whose first screen has no admitted deep-link entry compiles over the bound
// ledger's PROVEN path from an admitted entry, each hop emitting the admitted method of the step
// a receipt saw cross it, and the entry names the path and its test keys. Without a ledger, or
// when a hop's crossing flow needs a precondition the flow does not set, it stays INCOMPLETE, and
// the scenarios that already compiled are byte-identical.
func TestCCEV0104ProvenPathSynthesis(t *testing.T) {
	root, _ := fixtureRepo(t)
	base := commitFiles(t, root, map[string]*string{
		"flows/rebook-slot.json": text(rebookFlow("rebook-slot", `"club has open slots"`)),
		"flows/rebook-bare.json": text(rebookFlow("rebook-bare", "")),
		"flows/menu-teesheets.json": text(`{"schema":"application-flow-intent/1","flow_id":"menu-teesheets","revision":1,"kind":"ui","actor":"club admin",
"preconditions":["club has open slots"],
"steps":[{"step_id":"open-menu","action":"open the menu"},{"step_id":"open-teesheets","action":"open the tee sheet"},{"step_id":"select-slot","action":"select an open slot"}],
"outcomes":[],"variations":[],"links":[],
"navigation":{"precondition_flows":[],"steps":[
{"step_id":"open-menu","state":"/home","locator":{"test_id":"nav-menu"},"expect":[]},
{"step_id":"open-teesheets","state":"/home","locator":{"test_id":"nav-teesheets"},"expect":[]},
{"step_id":"select-slot","state":"/clubs/{clubId}/teesheets","locator":{"test_id":"slot"},"expect":[]}]}}
`),
	}, "rebook flows")
	head := commitBehaviourChange(t, root)
	const key = "e2e/specs/booking-workflow.spec.ts::books-a-slot"
	const rebook, bare, booked = "obligation:rebook-slot/rebooked", "obligation:rebook-bare/rebooked", "obligation:book-tee-time/booked"
	plain, err := Compile(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{rebook, bare} {
		e := entryFor(plain, id)
		if e == nil || e.State != Incomplete || len(e.Missing) != 1 || e.Missing[0].Kind != KindPath || e.ProvenPath != nil {
			t.Fatalf("%s without a ledger: %+v\n%s", id, e, plain.Encoded())
		}
	}
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	l := boundLedger(t, root, head)
	edge := homeToTeeSheets(t, cat, key)
	l.Edges = []appmap.LedgerEdge{edge}
	opts := diffOpts(root, base, head)
	opts.Ledger = writeLedger(t, l)
	r, err := Compile(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	e := entryFor(r, rebook)
	if e == nil || e.State != Compiled || !slices.Equal(e.ProvenPath, []string{edge.From, edge.To}) || !slices.Equal(e.ProvenBy, []string{key}) ||
		!slices.Contains(e.Blocks, "method:e2e/pages/home.page.ts#goToTeeSheets") || !slices.Contains(e.Blocks, "path:entry:"+edge.From) {
		t.Fatalf("rebook over the ledger: %+v\n%s", e, r.Encoded())
	}
	var src string
	for _, sc := range r.Compiled() {
		if sc.Obligation == rebook {
			src = string(sc.Text)
		}
	}
	want := []string{"page.goto('/#!/home')", ".goToTeeSheets()", ".selectSlot()", ".book()", "assert:rebook-slot/rebooked').toHaveText('Booked')"}
	at := 0
	for _, w := range want {
		i := strings.Index(src[at:], w)
		if i < 0 {
			t.Fatalf("missing or out of order %q in:\n%s", w, src)
		}
		at += i + len(w)
	}
	if b := entryFor(r, bare); b == nil || b.State != Incomplete || len(b.Missing) != 1 || b.Missing[0].Reason != "proven-hop-precondition-not-set" {
		t.Fatalf("bare flow: %+v", b)
	}
	textOf := func(r *CompileReport, id string) []byte {
		for _, sc := range r.Compiled() {
			if sc.Obligation == id {
				return sc.Text
			}
		}
		return nil
	}
	if a, b := textOf(plain, booked), textOf(r, booked); a == nil || !bytes.Equal(a, b) {
		t.Fatalf("the ledger changed an already compiled scenario:\n%s\n%s", a, b)
	}

	// Each hop abstains with a named reason rather than generating a statement.
	for name, tc := range map[string]struct {
		edit   func(*appmap.LedgerEdge)
		reason string
	}{
		"steps not adjacent": {func(e *appmap.LedgerEdge) { e.Evidence.Step = e.Evidence.FromStep }, "proven-hop-evidence-inconsistent"},
		"unknown flow":       {func(e *appmap.LedgerEdge) { e.Evidence.Flow = "flow:nope" }, "proven-hop-evidence-inconsistent"},
		// The crossing's from-step follows another step on its screen that would not be emitted.
		"prefix dropped": {func(e *appmap.LedgerEdge) {
			for _, f := range cat.m.Flows {
				if f.FlowID == "menu-teesheets" {
					if f.Steps[1].Screen != e.From || f.Steps[2].Screen != e.To || f.Steps[0].Screen != e.From {
						t.Fatalf("menu-teesheets steps not on home, home, tee sheet: %+v", f.Steps)
					}
					e.Evidence.Flow, e.Evidence.FromStep, e.Evidence.Step = f.ID, f.Steps[1].ID, f.Steps[2].ID
					return
				}
			}
			t.Fatal("no menu-teesheets flow")
		}, "proven-hop-prefix-not-emitted"},
	} {
		l := boundLedger(t, root, head)
		e := homeToTeeSheets(t, cat, key)
		tc.edit(&e)
		l.Edges = []appmap.LedgerEdge{e}
		opts := diffOpts(root, base, head)
		opts.Ledger = writeLedger(t, l)
		r, err := Compile(bg, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := entryFor(r, rebook); got == nil || got.State != Incomplete || len(got.Missing) != 1 || got.Missing[0].Reason != tc.reason {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	obs, err := BuildObligations(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	route := obs.routes[edge.To]
	obs.routes[edge.To] = provenRoute{path: []string{"screen:admin:app.clubs.settings", edge.To}, by: route.by}
	if got := entryFor(must(CompileObligations(obs)), rebook); got.Missing[0].Reason != "proven-path-entry-not-admitted" {
		t.Fatalf("unadmitted origin: %+v", got)
	}
	obs.routes[edge.To] = provenRoute{path: route.path, by: []string{"another-test"}}
	if got := entryFor(must(CompileObligations(obs)), rebook); got.Missing[0].Reason != "proven-hop-edge-missing" {
		t.Fatalf("hop without its edge: %+v", got)
	}
}

func must(r *CompileReport, err error) *CompileReport {
	if err != nil {
		panic(err)
	}
	return r
}
