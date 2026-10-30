package breakagemap

import (
	"context"
	"strings"
	"testing"
)

// TestBuildManifestMatchesHandPins proves the helper pins exactly what the
// hand-written fixture pins and that the result compiles (BKM-V0-010).
func TestBuildManifestMatchesHandPins(t *testing.T) {
	m, bindings, _ := fixture(t)
	sources := []string{}
	for _, s := range m.Sources {
		sources = append(sources, s.Repository+":"+s.Path)
	}
	built, err := BuildManifest(context.Background(), bindings, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Repositories) != 2 || built.Repositories[0] != m.Repositories[0] || built.Repositories[1] != m.Repositories[1] {
		t.Fatalf("repositories = %+v want %+v", built.Repositories, m.Repositories)
	}
	want := map[string]Source{}
	for _, s := range m.Sources {
		want[s.Repository+":"+s.Path] = s
	}
	for _, s := range built.Sources {
		if want[s.Repository+":"+s.Path] != s {
			t.Fatalf("source %+v want %+v", s, want[s.Repository+":"+s.Path])
		}
	}
	built.Providers = m.Providers
	if _, err := Compile(context.Background(), encode(t, built), bindings, "api:pkg/api.go:Changed", ""); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		bindings map[string]string
		sources  []string
	}{
		{map[string]string{}, sources},
		{bindings, nil},
		{bindings, []string{"missing:go.mod"}},
		{bindings, []string{"api:../go.mod"}},
		{bindings, []string{"api:absent.go"}},
		{map[string]string{"BAD ID": bindings["api"]}, []string{"BAD ID:go.mod"}},
		{map[string]string{"api": bindings["api"] + "/pkg"}, []string{"api:go.mod"}},
	} {
		if _, err := BuildManifest(context.Background(), bad.bindings, bad.sources); err == nil {
			t.Fatalf("BuildManifest(%v, %v) admitted", bad.bindings, bad.sources)
		}
	}
}

// TestReadSpanDigestsPhysicalLines binds the span digest to the same joined
// physical lines breakage anchors digest and refuses mutable or wide spans.
func TestReadSpanDigestsPhysicalLines(t *testing.T) {
	m, bindings, _ := fixture(t)
	commit := m.Repositories[0].Commit
	pin, err := ReadSpan(context.Background(), bindings["api"], commit, "pkg/api.go", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if pin.SHA256 != digest([]byte("func Changed() int { return 3 }")) || pin.Blob != m.Sources[1].Blob || pin.Commit != commit {
		t.Fatalf("span = %+v", pin)
	}
	whole, err := ReadSpan(context.Background(), bindings["api"], commit, "pkg/api.go", 1, 0)
	if err != nil || whole.End != m.Sources[1].End {
		t.Fatalf("whole = %+v %v", whole, err)
	}
	for _, bad := range []struct {
		commit, path string
		start, end   int
	}{
		{"HEAD", "pkg/api.go", 1, 1},
		{commit, "/pkg/api.go", 1, 1},
		{commit, "pkg/api.go", 0, 1},
		{commit, "pkg/api.go", 3, 2},
		{commit, "pkg/api.go", 1, 99},
		{commit, "pkg/absent.go", 1, 1},
	} {
		if _, err := ReadSpan(context.Background(), bindings["api"], bad.commit, bad.path, bad.start, bad.end); err == nil {
			t.Fatalf("ReadSpan(%+v) admitted", bad)
		}
	}
}

// TestBreakageNonGoChangeIsFileBounded keeps non-Go targets unresolved while
// reporting whole-blob identity against the base (BKM-V0-009).
func TestBreakageNonGoChangeIsFileBounded(t *testing.T) {
	m, bindings, _ := fixture(t)
	base := m.Repositories[1].Commit
	r, err := Compile(context.Background(), encode(t, m), bindings, "client:flow.json:checkout", base)
	if err != nil || r.Change != "declaration-unresolved-file-unchanged" {
		t.Fatalf("unchanged: %+v %v", r, err)
	}
	put(t, bindings["client"], "flow.json", "{\"flow_id\":\"checkout\",\"declared\":false}\n")
	commit(t, bindings["client"])
	m.Repositories[1] = pinRepo(t, "client", bindings["client"])
	for i, s := range m.Sources {
		if s.Repository == "client" {
			m.Sources[i] = pinSource(t, m.Repositories[1], bindings["client"], s.Path)
		}
	}
	m.Providers = nil
	r, err = Compile(context.Background(), encode(t, m), bindings, "client:flow.json:checkout", base)
	if err != nil || r.Change != "declaration-unresolved-file-changed" {
		t.Fatalf("changed: %+v %v", r, err)
	}
	found := false
	for _, u := range r.Unknowns {
		found = found || strings.Contains(u.Reason, "not inferred")
	}
	if !found {
		t.Fatal("file-level change claimed a resolved declaration")
	}
}

// TestBreakageDeclaredSpanChangeIsSeparateFromDeclaration compares the declared span of an
// unsupported-language source at the base and the target, while the whole-file bound still
// reports a change (BKM-V0-092).
func TestBreakageDeclaredSpanChangeIsSeparateFromDeclaration(t *testing.T) {
	m, bindings, _ := fixture(t)
	base := m.Repositories[1].Commit
	put(t, bindings["client"], "flow.json", "{\"flow_id\":\"checkout\",\"declared\":true}\nappended\nmore\n")
	commit(t, bindings["client"])
	m.Repositories[1] = pinRepo(t, "client", bindings["client"])
	for i, s := range m.Sources {
		if s.Repository == "client" {
			m.Sources[i] = pinSource(t, m.Repositories[1], bindings["client"], s.Path)
			if s.Path == "flow.json" {
				m.Sources[i].End = 1
			}
		}
	}
	m.Providers = nil
	r, err := Compile(context.Background(), encode(t, m), bindings, "client:flow.json:checkout", base)
	if err != nil || r.Change != "declaration-unresolved-file-changed" || r.DeclaredSpanChange != "declared-span-unchanged" {
		t.Fatalf("unchanged span: change=%s span=%s %v", r.Change, r.DeclaredSpanChange, err)
	}
	for i := range m.Sources {
		if m.Sources[i].Path == "flow.json" {
			m.Sources[i].End = 2
		}
	}
	r, err = Compile(context.Background(), encode(t, m), bindings, "client:flow.json:checkout", base)
	if err != nil || r.DeclaredSpanChange != "declared-span-changed" {
		t.Fatalf("changed span: span=%s %v", r.DeclaredSpanChange, err)
	}
}

// TestBreakageEEPV2SpanNamesField proves a /2 record carrying an endpoint span refuses with the
// field and the schema it needs (BKM-V0-093).
func TestBreakageEEPV2SpanNamesField(t *testing.T) {
	raw := []byte(`{"schema":"external-evidence-provider/2","relations":[{"from":{"repository":"a","path":"x","span":{"start":1,"end":2}},"to":{}}]}`)
	if field := v3OnlyField(raw); field != "span" {
		t.Fatalf("field = %q", field)
	}
	if v3OnlyField([]byte(`{"schema":"external-evidence-provider/3","relations":[{"from":{"span":{}},"to":{}}]}`)) != "" {
		t.Fatal("/3 span named as v3-only")
	}
}

// TestBuildManifestPinsOlderCommitSpanAndSkeleton covers DIR@COMMIT, PATH:START-END and the
// provider skeleton (BKM-V0-094).
func TestBuildManifestPinsOlderCommitSpanAndSkeleton(t *testing.T) {
	m, bindings, _ := fixture(t)
	old := m.Repositories[0].Commit
	put(t, bindings["api"], "pkg/later.go", "package pkg\n")
	commit(t, bindings["api"])
	if _, err := BuildManifest(context.Background(), map[string]string{"api": bindings["api"] + "@" + old}, []string{"api:pkg/later.go"}); err == nil {
		t.Fatal("source absent at the older commit admitted")
	}
	built, err := BuildManifestWith(context.Background(), map[string]string{"api": bindings["api"] + "@" + old}, []string{"api:pkg/api.go:2-2"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if built.Repositories[0].Commit != old || built.Sources[0].Start != 2 || built.Sources[0].End != 2 || len(built.Providers) != 1 {
		t.Fatalf("built = %+v", built)
	}
	if !strings.Contains(string(built.Providers[0]), `"revision":"`+old+`"`) || !strings.Contains(string(built.Providers[0]), "external-evidence-provider/3") {
		t.Fatalf("skeleton = %s", built.Providers[0])
	}
	if _, err := BuildManifestWith(context.Background(), map[string]string{"api": bindings["api"] + "@HEAD"}, []string{"api:pkg/api.go"}, false); err == nil {
		t.Fatal("non-oid @COMMIT admitted")
	}
}
