package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const entryRoutes = `
r.state('home', {url:'/home'});
r.state('base', {url:'/teams/:teamId', abstract:true});
r.state('base.child', {url:'/items/:itemId?search'});
r.state('base.absolute', {url:'^/absolute/:id'});
r.state('guard', {url:'/guard', data:{permissions:['view'], flags:['beta']}});
r.state('guard.child', {url:'/child'});
r.state('resolve', {url:'/resolve', resolve:{data: loadData}});
r.state('resolve.child', {url:'/child'});
r.state('hook', {url:'/hook', onEnter: enter});
r.state('redirect', {url:'/redirect', redirectTo:'home'});
r.state('params', {url:'/params', params:{record:null}});
r.state('empty', {url:'/empty', resolve:{}, params:{}});
r.state('dynamic', {url:'/dynamic', data:{permissions: permissions}});
r.state('dynamic.child', {url:'/child'});
r.state('urlLess', {parent:'base'});
r.state('collisionA', {url:'/duplicate'});
r.state('collisionB', {url:'/duplicate'});
r.state('orphan.child', {url:'/orphan'});
`

func entriesFixture(t *testing.T) (string, *Map) {
	t.Helper()
	root, _ := fixtureRepo(t)
	writeFile(t, root, "app/routes.js", entryRoutes)
	return root, build(t, root, commitAll(t, root, "entry routes"))
}

func entriesRows(t *testing.T, m *Map, o Options) ([]entryView, []byte) {
	t.Helper()
	raw, err := ProjectEntries(context.Background(), m, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Entries []entryView `json:"entries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Entries, raw
}

// AMAP-V0-050 AMAP-V0-051 AMAP-V0-052 AMAP-V0-053: source declarations, inherited
// blockers and guards, parent-only parameters, ambiguity and query separation survive projection.
func TestEntriesClassification(t *testing.T) {
	root, m := entriesFixture(t)
	rows, raw := entriesRows(t, m, Options{Root: root, Full: true})
	if len(rows) != len(m.Screens) {
		t.Fatalf("lost rows: %d/%d", len(rows), len(m.Screens))
	}
	want := map[string]string{"home": "deep-linkable", "base": "abstract", "base.child": "requires-parent-params",
		"base.absolute": "requires-parent-params", "guard": "guarded(permission/flag)", "guard.child": "guarded(permission/flag)",
		"resolve": "UNKNOWN", "resolve.child": "UNKNOWN", "hook": "UNKNOWN", "redirect": "UNKNOWN", "params": "UNKNOWN",
		"empty": "deep-linkable", "dynamic": "UNKNOWN", "dynamic.child": "UNKNOWN", "urlLess": "UNKNOWN",
		"collisionA": "UNKNOWN", "collisionB": "UNKNOWN", "orphan.child": "UNKNOWN"}
	for i, row := range rows {
		if row.Classification != want[row.State] {
			t.Errorf("%s: %+v", row.State, row)
		}
		if row.Verification != "STATIC_ONLY" || row.LastVerifiedRevision != "" {
			t.Errorf("invented run: %+v", row)
		}
		if i > 0 && rows[i-1].ID >= row.ID {
			t.Fatal("unordered rows")
		}
		if row.State == "base.child" {
			if strings.Join(row.RequiredParams, ",") != "itemId,teamId" || strings.Join(row.QueryParams, ",") != "search" || row.URL != "/#!/teams/{teamId}/items/{itemId}" {
				t.Errorf("parameters: %+v", row)
			}
		}
		if row.State == "base.absolute" && strings.Join(row.RequiredParams, ",") != "id,teamId" {
			t.Errorf("parent-only params: %+v", row)
		}
		if row.State == "guard.child" && (strings.Join(row.Guard.Permissions, ",") != "view" || strings.Join(row.Guard.Flags, ",") != "beta") {
			t.Errorf("guard: %+v", row)
		}
		if row.Classification == StatusUnknown && len(row.Reasons) == 0 {
			t.Errorf("missing reason: %+v", row)
		}
	}
	_, again := entriesRows(t, m, Options{Root: root, Full: true})
	if string(raw) != string(again) {
		t.Fatal("nondeterministic")
	}
	encoded, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ParseMap(encoded)
	if err != nil {
		t.Fatal(err)
	}
	_, roundtrip := entriesRows(t, loaded, Options{Root: root, Full: true})
	if string(raw) != string(roundtrip) {
		t.Fatal("metadata lost in roundtrip")
	}
}

// AMAP-V0-054 AMAP-V0-055: source changes invalidate entry evidence, including ancestor
// guards and edits outside a screen span; dirty files do not masquerade as committed evidence.
func TestEntriesRevisionBinding(t *testing.T) {
	for _, change := range []string{"template", "guard", "ancestor", "outside-span", "manifest", "unrelated"} {
		t.Run(change, func(t *testing.T) {
			root, m := entriesFixture(t)
			switch change {
			case "template":
				writeFile(t, root, "app/routes.js", strings.Replace(entryRoutes, "/home", "/landing", 1))
			case "guard":
				writeFile(t, root, "app/routes.js", strings.Replace(entryRoutes, "permissions:['view']", "permissions:['admin']", 1))
			case "ancestor":
				writeFile(t, root, "app/routes.js", strings.Replace(entryRoutes, "abstract:true", "abstract:true, data:{flags:['new']}", 1))
			case "outside-span":
				writeFile(t, root, "app/routes.js", entryRoutes+"\n// new source\n")
			case "manifest":
				writeFile(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"changed"}`)
			case "unrelated":
				writeFile(t, root, "readme.txt", "unrelated")
			}
			dirty, _ := entriesRows(t, m, Options{Root: root, Full: true})
			for _, row := range dirty {
				if row.Verification != "STATIC_ONLY" {
					t.Fatal("read dirty source")
				}
			}
			commitAll(t, root, change)
			rows, raw := entriesRows(t, m, Options{Root: root, Full: true})
			for _, row := range rows {
				if change == "unrelated" {
					if row.Verification != "STATIC_ONLY" {
						t.Errorf("unrelated: %+v", row)
					}
					continue
				}
				if row.Verification != "UNVERIFIED_AT_HEAD" || row.Classification != StatusUnknown || row.LastVerifiedRevision != "" {
					t.Errorf("stale: %+v", row)
				}
			}
			if !strings.Contains(string(raw), m.Revision) {
				t.Fatal("lost revision")
			}
			historical, _ := entriesRows(t, m, Options{Root: root, Revision: m.Revision, Full: true})
			for _, row := range historical {
				if row.Verification != "STATIC_ONLY" {
					t.Fatal("historical check drifted")
				}
			}
		})
	}
	root, m := entriesFixture(t)
	for _, o := range []Options{{Root: root, Revision: "missing", Full: true}, {Full: true}} {
		rows, _ := entriesRows(t, m, o)
		for _, row := range rows {
			if row.Verification != StatusUnknown || row.Classification != StatusUnknown {
				t.Errorf("unavailable: %+v", row)
			}
		}
	}
	m.Manifest.Repo = "other"
	rows, _ := entriesRows(t, m, Options{Root: root, Full: true})
	for _, row := range rows {
		if row.Verification != StatusUnknown {
			t.Fatal("aliased root used local source")
		}
	}
}

// AMAP-V0-056 AMAP-V0-058: legacy metadata and inferred/test-sequence edges cannot
// manufacture loadability, a PROVEN path or a runtime verification revision.
func TestEntriesLegacyAndUnprovenEdges(t *testing.T) {
	root, m := entriesFixture(t)
	baseline, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMap(baseline); err != nil {
		t.Fatal(err)
	}
	for i := range m.Screens {
		m.Screens[i].Entry = nil
	}
	m.Edges = []Edge{{From: screenID(m.App, "home"), To: screenID(m.App, "guard"), Basis: "test-sequence"}}
	// Rebind the synthetic legacy artifact after removing the optional metadata.
	m.Digest, err = mapDigest(m)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	m, err = ParseMap(encoded)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := entriesRows(t, m, Options{Root: root, Full: true})
	for _, row := range rows {
		if row.State == "home" && (row.Classification != StatusUnknown || !strings.Contains(strings.Join(row.Reasons, ","), "entry-metadata-unavailable")) {
			t.Errorf("legacy: %+v", row)
		}
		if row.PathStatus != StatusUnknown || row.PathReason != "no-proven-edge-evidence" || len(row.ProvenPath) != 0 || row.LastVerifiedRevision != "" {
			t.Errorf("invented evidence: %+v", row)
		}
	}
}

// AMAP-V0-057: byte caps apply to whole rows with honest omitted counts; invalid caps refuse.
func TestEntriesBudget(t *testing.T) {
	root, m := entriesFixture(t)
	rows, raw := entriesRows(t, m, Options{Root: root, Budget: 2048})
	var doc struct {
		Omitted map[string]int `json:"omitted"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(raw) > 2048 || len(rows) == 0 || doc.Omitted["entries"] != len(m.Screens)-len(rows) || doc.Omitted["entries"] == 0 {
		t.Fatalf("budget: %s", raw)
	}
	for _, o := range []Options{{Budget: 255}, {Budget: 65537}, {Budget: 4096, Full: true}, {Budget: 256}} {
		if _, err := ProjectEntries(context.Background(), m, nil, o); err == nil {
			t.Fatalf("admitted %+v", o)
		}
	}
}

func entriesLedger(m *Map, edges ...LedgerEdge) *EdgeLedger {
	return &EdgeLedger{Schema: EdgeLedgerSchema, App: m.App, Revision: m.Revision, MapDigest: m.Digest, Digest: "sha256:ledger", Edges: edges}
}

func provenEdge(m *Map, from, to, test string) LedgerEdge {
	sv := StepVerification{Status: Verified, Receipt: &ReceiptRef{TestKey: test}}
	return LedgerEdge{From: screenID(m.App, from), To: screenID(m.App, to), Status: EdgeProven, Evidence: &EdgeCrossing{Verification: sv}}
}

// AMAP-V0-080 AMAP-V0-081 AMAP-V0-082: a bound ledger gives each screen its shortest path of
// PROVEN, receipt-backed edges from a deep-linkable entry; other screens read NOT_OBSERVED, and
// a stale screen or an unbound ledger never yields a path.
func TestAMAPV0080EntriesLedgerPaths(t *testing.T) {
	root, m := entriesFixture(t)
	unproven := provenEdge(m, "home", "hook", "t-hook")
	unproven.Status = EdgeUnproven
	noReceipt := provenEdge(m, "home", "params", "t-params")
	noReceipt.Evidence.Verification.Receipt = nil
	contradicted := provenEdge(m, "home", "redirect", "t-redirect")
	contradicted.Evidence.Verification.Status = Contradicted
	emptyKey := provenEdge(m, "home", "base", "")
	l := entriesLedger(m,
		provenEdge(m, "home", "guard", "t-z"),
		provenEdge(m, "home", "guard", "t-g"),
		provenEdge(m, "guard", "guard.child", "t-gc"),
		provenEdge(m, "home", "base.child", "t-b"),
		provenEdge(m, "base.child", "base.absolute", "t-ba"),
		provenEdge(m, "base.absolute", "guard.child", "t-long"),
		unproven, noReceipt, contradicted, emptyKey)
	run := func(l *EdgeLedger, o Options) ([]entryView, string, error) {
		o.Root, o.Full = root, true
		raw, err := ProjectEntries(context.Background(), m, l, o)
		if err != nil {
			return nil, "", err
		}
		var doc struct {
			Basis        string      `json:"basis"`
			LedgerDigest string      `json:"ledger_digest"`
			Entries      []entryView `json:"entries"`
		}
		if err = json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Basis != "static-entries+edge-ledger" || doc.LedgerDigest != l.Digest {
			t.Fatalf("head: %s", raw)
		}
		return doc.Entries, string(raw), nil
	}
	rows, raw, err := run(l, Options{})
	if err != nil {
		t.Fatal(err)
	}
	id := func(s string) string { return screenID(m.App, s) }
	want := map[string]struct {
		status string
		path   []string
		by     []string
	}{
		"home":          {PathEntry, []string{id("home")}, nil},
		"empty":         {PathEntry, []string{id("empty")}, nil},
		"guard":         {EdgeProven, []string{id("home"), id("guard")}, []string{"t-g"}},
		"guard.child":   {EdgeProven, []string{id("home"), id("guard"), id("guard.child")}, []string{"t-g", "t-gc"}},
		"base.absolute": {EdgeProven, []string{id("home"), id("base.child"), id("base.absolute")}, []string{"t-b", "t-ba"}},
		"hook":          {PathNotObserved, nil, nil},
		"params":        {PathNotObserved, nil, nil},
		"base":          {PathNotObserved, nil, nil},
		"redirect":      {PathNotObserved, nil, nil},
	}
	for _, row := range rows {
		w, ok := want[row.State]
		if !ok {
			if row.State == "base.child" && row.PathStatus != EdgeProven {
				t.Errorf("base.child: %+v", row)
			}
			continue
		}
		if row.PathStatus != w.status || strings.Join(row.ProvenPath, " ") != strings.Join(w.path, " ") || strings.Join(row.ProvenBy, " ") != strings.Join(w.by, " ") {
			t.Errorf("%s: %+v", row.State, row)
		}
		if row.PathStatus == PathNotObserved && (row.PathReason != "no-proven-path-from-entry" || len(row.ProvenPath) != 0) {
			t.Errorf("%s: %+v", row.State, row)
		}
		if row.Verification != "STATIC_ONLY" || row.LastVerifiedRevision != "" {
			t.Errorf("ledger upgraded loading: %+v", row)
		}
		delete(want, row.State)
	}
	if len(want) != 0 {
		t.Fatalf("missing rows %v", want)
	}
	if _, again, _ := run(l, Options{}); again != raw {
		t.Fatal("nondeterministic")
	}
	if _, plain := entriesRows(t, m, Options{Root: root, Full: true}); strings.Contains(string(plain), "proven_by") || strings.Contains(string(plain), "ledger_digest") {
		t.Fatal("static projection changed without a ledger")
	}
	for name, bad := range map[string]func(*EdgeLedger){
		"digest":   func(b *EdgeLedger) { b.MapDigest = "sha256:other" },
		"app":      func(b *EdgeLedger) { b.App = "other" },
		"revision": func(b *EdgeLedger) { b.Revision = strings.Repeat("0", 40) },
		"trimmed":  func(b *EdgeLedger) { b.Omitted = map[string]int{"edges": 1} },
	} {
		b := *l
		bad(&b)
		if _, _, err := run(&b, Options{}); err == nil {
			t.Errorf("%s: admitted an unbound ledger", name)
		}
	}
	writeFile(t, root, "app/routes.js", strings.Replace(entryRoutes, "/home", "/landing", 1))
	moved := *l
	moved.Revision = commitAll(t, root, "template changed")
	stale, _, err := run(&moved, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range stale {
		if row.PathStatus != StatusUnknown || row.PathReason != "screen-source-not-fresh" || len(row.ProvenPath) != 0 {
			t.Errorf("stale path: %+v", row)
		}
	}
}
