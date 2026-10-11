package appmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type prefixFixture struct {
	m          *Map
	root, rev  string
	a, b, c, d string
}

func prefixSetup(t *testing.T) prefixFixture {
	t.Helper()
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	var ids []string
	for _, s := range m.Screens {
		if s.Status == StatusResolved && !s.Abstract && s.Template != "" {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) < 4 {
		t.Fatal("fixture too small", ids)
	}
	f := prefixFixture{m: m, root: root, rev: rev, a: ids[0], b: ids[1], c: ids[2], d: ids[3]}
	add := func(id, path string, chain ...string) {
		m.Files = append(m.Files, TestFile{ID: id, Path: path, Role: roleSpec, Join: StatusResolved})
		for i := 1; i < len(chain); i++ {
			m.Edges = append(m.Edges, Edge{From: chain[i-1], To: chain[i], Basis: "test-sequence", Source: id, Anchor: Anchor{Start: i, End: i}})
		}
	}
	add("file:long", "e2e/specs/long.spec.ts", f.a, f.b, f.c)
	add("file:short", "e2e/specs/short.spec.ts", f.a, f.b)
	add("file:unknown", "e2e/specs/unknown.spec.ts", f.a, f.b, f.d)
	m.Files[len(m.Files)-1].Join = StatusUnknown
	return f
}

func provenLedger(f prefixFixture, pairs ...[2]string) *EdgeLedger {
	l := &EdgeLedger{Schema: EdgeLedgerSchema, App: f.m.App, Revision: f.rev, MapDigest: f.m.Digest, Digest: "sha256:l"}
	for _, p := range pairs {
		sv := StepVerification{Status: Verified, Receipt: &ReceiptRef{TestKey: "login-test"}}
		l.Edges = append(l.Edges, LedgerEdge{From: p[0], To: p[1], Status: EdgeProven, Evidence: &EdgeCrossing{Verification: sv}})
	}
	return l
}

func runPrefixes(t *testing.T, f prefixFixture, l *EdgeLedger, facts *PrefixFacts, o Options) PrefixReport {
	t.Helper()
	o.Root = f.root
	raw, err := Prefixes(context.Background(), f.m, l, facts, o)
	if err != nil {
		t.Fatal(err)
	}
	var r PrefixReport
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func reasonFor(r PrefixReport, test string) string {
	for _, a := range r.Abstained {
		if a.Test == test {
			return a.Reason
		}
	}
	return ""
}

// ANL-V0-009: a prefix over proven, read-only edges ending on a fresh deep-linkable screen flags
// with its replacement and saved steps; unknown joins and one-edge prefixes abstain.
func TestANLV0009FlagsProvenPrefix(t *testing.T) {
	f := prefixSetup(t)
	facts := &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{{Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{prefixEdgeID(f.a, f.b), prefixEdgeID(f.b, f.c)}}}}
	l := provenLedger(f, [2]string{f.a, f.b}, [2]string{f.b, f.c})
	r := runPrefixes(t, f, l, facts, Options{})
	if len(r.Flagged) != 1 || r.Flagged[0].Test != "e2e/specs/long.spec.ts" || r.Flagged[0].SavedSteps != 1 || r.Flagged[0].EntryScreen != f.c || r.Flagged[0].ProvenBy[0] != "login-test" || r.Flagged[0].EntryTemplate == "" {
		t.Fatalf("%+v", r)
	}
	if reasonFor(r, "e2e/specs/unknown.spec.ts") != "join-unknown" || reasonFor(r, "e2e/specs/short.spec.ts") != "effects-unknown" {
		t.Fatalf("%+v", r.Abstained)
	}
	if again := runPrefixes(t, f, l, facts, Options{}); again.Digest != r.Digest {
		t.Fatal("digest not deterministic")
	}
	if none := runPrefixes(t, f, nil, facts, Options{}); len(none.Flagged) != 0 || reasonFor(none, "e2e/specs/long.spec.ts") != "edge-not-proven" {
		t.Fatalf("flagged without a ledger: %+v", none)
	}
}

// ANL-V0-010: an assertion-target or mutating edge, or an undeclared effect, is never flagged.
func TestANLV0010NeverFlagsTargetsOrChanges(t *testing.T) {
	f := prefixSetup(t)
	ab, bc := prefixEdgeID(f.a, f.b), prefixEdgeID(f.b, f.c)
	l := provenLedger(f, [2]string{f.a, f.b}, [2]string{f.b, f.c})
	for want, fact := range map[string]PrefixFact{
		"assertion-target": {Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{ab}, AssertionEdges: []string{bc}},
		"state-change":     {Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{ab, bc}, MutatingEdges: []string{ab}},
		"effects-unknown":  {Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{ab}},
	} {
		r := runPrefixes(t, f, l, &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{fact}}, Options{})
		if len(r.Flagged) != 0 || reasonFor(r, "e2e/specs/long.spec.ts") != want {
			t.Fatalf("%s: %+v", want, r)
		}
	}
}

// ANL-V0-011: a stale entry template abstains and the byte cap reports omitted counts.
func TestANLV0011StaleEntryAndBudget(t *testing.T) {
	f := prefixSetup(t)
	facts := &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{{Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{prefixEdgeID(f.a, f.b), prefixEdgeID(f.b, f.c)}}}}
	l := provenLedger(f, [2]string{f.a, f.b}, [2]string{f.b, f.c})
	for _, s := range f.m.Screens {
		file := filepath.Join(f.root, s.Anchor.Path)
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, append([]byte("// shifted\n"), body...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, f.root, "shift")
	if _, err := Prefixes(context.Background(), f.m, l, facts, Options{Root: f.root}); err == nil {
		t.Fatal("ledger from an older revision accepted")
	}
	fresh := *l
	fresh.Revision = gitTest(t, f.root, "rev-parse", "HEAD")
	l = &fresh
	stale := runPrefixes(t, f, l, facts, Options{})
	if len(stale.Flagged) != 0 || reasonFor(stale, "e2e/specs/long.spec.ts") != "entry-template-not-fresh" {
		t.Fatalf("%+v", stale)
	}
	f = prefixSetup(t)
	facts = &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{{Path: "e2e/specs/long.spec.ts", ReadOnlyEdges: []string{prefixEdgeID(f.a, f.b), prefixEdgeID(f.b, f.c)}}}}
	l = provenLedger(f, [2]string{f.a, f.b}, [2]string{f.b, f.c})
	full := runPrefixes(t, f, l, facts, Options{})
	small := runPrefixes(t, f, l, facts, Options{Budget: 512})
	if small.Digest != full.Digest || small.Omitted["abstained"]+small.Omitted["flagged"] == 0 {
		t.Fatalf("%+v", small.Omitted)
	}
}

// ANL-V0-010: duplicate facts for one test path are refused and a ledger from another map is
// refused rather than trusted.
func TestANLV0010RefusesDuplicateFactsAndForeignLedger(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "facts.json")
	body := `{"schema":"` + PrefixFactsSchema + `","tests":[{"path":"a.spec.ts","mutating_edges":["x>y"]},{"path":"a.spec.ts","read_only_edges":["x>y"]}]}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrefixFacts(p); err == nil {
		t.Fatal("duplicate facts accepted")
	}
	f := prefixSetup(t)
	l := provenLedger(f, [2]string{f.a, f.b})
	l.MapDigest = "sha256:other"
	if _, err := Prefixes(context.Background(), f.m, l, nil, Options{Root: f.root}); err == nil {
		t.Fatal("foreign ledger accepted")
	}
}
