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
	raw, err := ProjectEntries(context.Background(), m, o)
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
		if _, err := ProjectEntries(context.Background(), m, o); err == nil {
			t.Fatalf("admitted %+v", o)
		}
	}
}
