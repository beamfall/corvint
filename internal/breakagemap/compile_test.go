package breakagemap

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/extevidence"
)

func command(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s: %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func put(t *testing.T, root, p, text string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, p), []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
}
func commit(t *testing.T, root string) {
	command(t, root, "add", ".")
	command(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
}
func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command(t, root, "init", "-q")
	return root
}
func pinRepo(t *testing.T, id, root string) Repository {
	return Repository{id, command(t, root, "rev-list", "--max-parents=0", "HEAD"), command(t, root, "rev-parse", "HEAD"), command(t, root, "rev-parse", "HEAD^{tree}")}
}
func pinSource(t *testing.T, r Repository, root, p string) Source {
	b, e := os.ReadFile(filepath.Join(root, p))
	if e != nil {
		t.Fatal(e)
	}
	return Source{r.ID, p, command(t, root, "rev-parse", r.Commit+":"+p), 1, len(strings.Split(string(b), "\n"))}
}
func encode(t *testing.T, m Manifest) []byte {
	t.Helper()
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func fixture(t *testing.T) (Manifest, map[string]string, string) {
	t.Helper()
	a, b := initRepo(t), initRepo(t)
	put(t, a, "go.mod", "module github.com/acme/library\n\ngo 1.27.1\n")
	put(t, a, "pkg/api.go", "package api\nfunc Changed() int { return 1 }\nfunc Other() int { return 2 }\n")
	commit(t, a)
	base := command(t, a, "rev-parse", "HEAD")
	put(t, a, "pkg/api.go", "package api\nfunc Changed() int { return 3 }\nfunc Other() int { return 2 }\n")
	commit(t, a)
	files := map[string]string{
		"go.mod":         "module github.com/acme/client\n\ngo 1.27.1\n",
		"caller.go":      "package client\nimport alias \"github.com/acme/library/pkg\"\nfunc Call() int { return alias.Changed() }\n",
		"other.go":       "package client\nimport \"github.com/acme/library/pkg\"\nfunc Other() int { return api.Other() }\n",
		"caller_test.go": "package client\nimport \"github.com/acme/library/pkg\"\nfunc TestCall() { api.Changed() }\n",
		"shadow.go":      "package client\nimport alias \"github.com/acme/library/pkg\"\nfunc Shadow() { alias := struct{ Changed func() }{}; alias.Changed() }\n",
		"dot.go":         "package client\nimport . \"github.com/acme/library/pkg\"\nfunc Dot() { Changed() }\n",
		"method.go":      "package client\nfunc Method(receiver Thing) { receiver.Changed() }\n",
		"same.go":        "package client\nimport \"different.example/lib/api\"\nfunc Same() { api.Changed() }\n",
		"tag.go":         "//go:build linux\n\npackage client\nimport \"github.com/acme/library/pkg\"\nfunc Tag() { api.Changed() }\n",
		"malformed.go":   "package client\nfunc (\n",
		"flow.json":      "{\"flow_id\":\"checkout\",\"declared\":true}\n",
		"docs.md":        "# API use\nThis page declares the checkout API relationship.\n",
	}
	for p, text := range files {
		put(t, b, p, text)
	}
	commit(t, b)
	ra, rb := pinRepo(t, "api", a), pinRepo(t, "client", b)
	m := Manifest{Schema: "corvint-breakage-manifest/0", Repositories: []Repository{ra, rb}, Providers: []json.RawMessage{}}
	for _, p := range []string{"go.mod", "pkg/api.go"} {
		m.Sources = append(m.Sources, pinSource(t, ra, a, p))
	}
	for _, p := range []string{"go.mod", "caller.go", "other.go", "caller_test.go", "shadow.go", "dot.go", "method.go", "same.go", "tag.go", "malformed.go", "flow.json", "docs.md"} {
		m.Sources = append(m.Sources, pinSource(t, rb, b, p))
	}
	ep := func(repo, p string) extevidence.Endpoint1 {
		for _, s := range m.Sources {
			if s.Repository == repo && s.Path == p {
				return extevidence.Endpoint1{Repository: repo, Path: p, Blob: s.Blob}
			}
		}
		panic("fixture endpoint")
	}
	repos := []extevidence.Repository1{{ID: ra.ID, Origin: ra.Origin, Revision: ra.Commit, Tree: ra.Tree}, {ID: rb.ID, Origin: rb.Origin, Revision: rb.Commit, Tree: rb.Tree}}
	rec := extevidence.Record1{Schema: extevidence.Schema2, Provider: extevidence.Identity{ID: "fixture", Revision: "v1"}, Repositories: repos, Entities: []extevidence.Entity{}, Relations: []extevidence.Relation1{
		{From: ep("api", "pkg/api.go"), To: ep("client", "flow.json"), Type: "implements", Evidence: "declared", Rule: "caller supplied flow association", Reference: "fixture"},
		{From: ep("client", "flow.json"), To: ep("client", "docs.md"), Type: "documents", Evidence: "declared", Rule: "caller supplied documentation association", Reference: "fixture"},
	}}
	raw, e := json.Marshal(rec)
	if e != nil {
		t.Fatal(e)
	}
	m.Providers = append(m.Providers, raw)
	return m, map[string]string{"api": a, "client": b}, base
}

func TestCrossRepositoryBreakageMap(t *testing.T) {
	m, bindings, base := fixture(t)
	raw := encode(t, m)
	r, e := Compile(context.Background(), raw, bindings, "api:pkg/api.go:Changed", base)
	if e != nil {
		t.Fatal(e)
	}
	if r.Change != "selected-declaration-changed" || r.Scope != "INCOMPLETE" || r.Mutates {
		t.Fatalf("bad report: %+v", r)
	}
	found := map[string]string{}
	providers := 0
	for _, edge := range r.Edges {
		if edge.Relation != nil {
			providers++
			if !strings.Contains(edge.Reason, "not symbol-specific coupling") {
				t.Fatal("file association presented as symbol dependency")
			}
			if edge.Relation.Evidence != "declared" || edge.ByteState != "verified" {
				t.Fatalf("declaration upgraded or unbound: %+v", edge)
			}
		} else {
			found[edge.From.Path] = edge.Kind
			if edge.From.Blob == "" || edge.To.Commit == "" || edge.From.SpanSHA256 == "" {
				t.Fatal("missing anchor")
			}
		}
	}
	if found["caller.go"] != "syntax-call" || found["caller_test.go"] != "test-syntax-call" || found["other.go"] != "import-only" || found["shadow.go"] != "import-only" || found["dot.go"] != "import-only" || providers != 2 {
		t.Fatalf("bad edge set: %+v providers=%d", found, providers)
	}
	for _, p := range []string{"method.go", "same.go", "tag.go", "malformed.go"} {
		if found[p] != "" {
			t.Fatalf("invented caller %s", p)
		}
	}
	for _, reason := range []string{"shadowed", "dot import", "receiver/package", "build constraints", "malformed"} {
		hit := false
		for _, u := range r.Unknowns {
			hit = hit || strings.Contains(u.Reason, reason)
		}
		if !hit {
			t.Errorf("missing gap %s", reason)
		}
	}
	again, e := Compile(context.Background(), raw, bindings, "api:pkg/api.go:Changed", base)
	if e != nil {
		t.Fatal(e)
	}
	one, _ := json.Marshal(r)
	two, _ := json.Marshal(again)
	if string(one) != string(two) {
		t.Fatal("nondeterministic map")
	}
	for _, root := range bindings {
		if command(t, root, "status", "--porcelain") != "" {
			t.Fatal("read mutated fixture")
		}
	}
}

func TestBreakagePinsFailClosed(t *testing.T) {
	for _, mode := range []string{"blob", "tree", "origin", "endpoint", "missing", "provider-revision"} {
		t.Run(mode, func(t *testing.T) {
			m, b, _ := fixture(t)
			switch mode {
			case "blob":
				m.Sources[1].Blob = strings.Repeat("1", 40)
			case "tree":
				m.Repositories[0].Tree = strings.Repeat("1", 40)
			case "origin":
				m.Repositories[0].Origin = strings.Repeat("1", 40)
			case "missing":
				delete(b, "client")
			case "endpoint", "provider-revision":
				rec, _ := extevidence.Decode1(m.Providers[0])
				if mode == "endpoint" {
					rec.Relations[0].To.Blob = strings.Repeat("1", 40)
				} else {
					rec.Repositories[1].Revision = strings.Repeat("1", 40)
				}
				m.Providers[0], _ = json.Marshal(rec)
			}
			r, e := Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", "")
			if e != nil {
				t.Fatal(e)
			}
			unresolved := false
			for _, u := range r.Unknowns {
				if strings.Contains(u.Reason, "mismatch") || strings.Contains(u.Reason, "unbound") || strings.Contains(u.Reason, "unavailable") {
					unresolved = true
				}
			}
			if !unresolved {
				t.Fatalf("missing failure: %+v", r)
			}
			if mode == "blob" || mode == "tree" || mode == "origin" {
				for _, edge := range r.Edges {
					if edge.Relation == nil {
						t.Fatal("unbound API acquired caller")
					}
				}
			}
		})
	}
}

func TestBreakageManifestRefusals(t *testing.T) {
	m, _, _ := fixture(t)
	raw := encode(t, m)
	for _, bad := range [][]byte{append(raw, []byte(" {}")...), []byte(strings.Replace(string(raw), `"schema":`, `"schema":"other","schema":`, 1)), []byte(strings.Replace(string(raw), `"sources":`, `"unknown":true,"sources":`, 1)), []byte(strings.Replace(string(raw), `"schema":`, `"Schema":`, 1))} {
		if _, e := Decode(bad); e == nil {
			t.Fatal("accepted ambiguous input")
		}
	}
	for _, p := range []string{"../go.mod", "/etc/passwd", "a/../go.mod", "a\\b", "a\nfile", "a*"} {
		m.Sources[0].Path = p
		if _, e := Decode(encode(t, m)); e == nil {
			t.Fatalf("accepted path %q", p)
		}
	}
}

func TestBreakageSymlinkAndHistoricalPins(t *testing.T) {
	m, b, base := fixture(t)
	root := b["client"]
	if e := os.Symlink("docs.md", filepath.Join(root, "link.md")); e != nil {
		t.Fatal(e)
	}
	commit(t, root)
	r, e := Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", base)
	if e != nil {
		t.Fatal(e)
	}
	if r.Repositories[1].State != "pinned-historical" {
		t.Fatal("historical scope hidden")
	}
	if _, e := readBlob(context.Background(), root, command(t, root, "rev-parse", "HEAD"), "link.md", ""); e == nil {
		t.Fatal("symlink admitted")
	}
}

func TestBreakageEdgeBound(t *testing.T) {
	r := Report{}
	for i := 0; i < MaxEdges+1; i++ {
		r.add(Edge{})
	}
	if !r.Truncated || len(r.Edges) != MaxEdges {
		t.Fatal("bound lost")
	}
}

func TestBreakageV1EntityComposition(t *testing.T) {
	m, b, _ := fixture(t)
	rec, _ := extevidence.Decode1(m.Providers[0])
	rec.Schema = extevidence.Schema1
	rec.Entities = []extevidence.Entity{{ID: "checkout", Kind: "flow", Summary: "Declared checkout"}}
	entity := extevidence.Endpoint1{Provider: rec.Provider.ID, Entity: "checkout"}
	rec.Relations[0].To = entity
	rec.Relations[1].From = entity
	m.Providers[0], _ = json.Marshal(rec)
	r, e := Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", "")
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, edge := range r.Edges {
		if edge.Relation != nil {
			count++
			if edge.ByteState != "entity-declaration" || edge.Relation.Evidence != "declared" {
				t.Fatal("entity acquired byte or assertion verification")
			}
		}
	}
	if count != 2 {
		t.Fatal("entity chain missing")
	}
	rec.Relations[0].To.Entity = "undeclared"
	m.Providers[0], _ = json.Marshal(rec)
	r, e = Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", "")
	if e != nil {
		t.Fatal(e)
	}
	for _, edge := range r.Edges {
		if edge.Relation != nil && edge.ByteState != "unresolved" {
			t.Fatal("unresolved chain traversed")
		}
	}
}

func TestBreakageBaseAndModuleGaps(t *testing.T) {
	m, b, _ := fixture(t)
	r, e := Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", m.Repositories[0].Commit)
	if e != nil || r.Change != "selected-declaration-unchanged" {
		t.Fatalf("unchanged: %+v %v", r, e)
	}
	if _, e = Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", m.Repositories[1].Commit); e == nil {
		t.Fatal("foreign base admitted")
	}
	old := m.Repositories[0].Commit
	put(t, b["api"], "pkg/api.go", "package api\nfunc Renamed() int { return 3 }\n")
	commit(t, b["api"])
	m.Repositories[0] = pinRepo(t, "api", b["api"])
	m.Sources[1] = pinSource(t, m.Repositories[0], b["api"], "pkg/api.go")
	m.Providers = nil
	r, e = Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", old)
	if e != nil || r.Change != "declaration-unresolved" {
		t.Fatalf("renamed: %+v %v", r, e)
	}
	put(t, b["api"], "pkg/go.mod", "module different.example/nested\n")
	commit(t, b["api"])
	m.Repositories[0] = pinRepo(t, "api", b["api"])
	r, e = Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Renamed", "")
	if e != nil {
		t.Fatal(e)
	}
	for _, edge := range r.Edges {
		if edge.Relation == nil {
			t.Fatal("undeclared nested module acquired caller")
		}
	}
	found := false
	for _, u := range r.Unknowns {
		found = found || strings.Contains(u.Reason, "module boundary")
	}
	if !found {
		t.Fatal("module gap missing")
	}
	if e = os.Remove(filepath.Join(b["api"], "pkg/api.go")); e != nil {
		t.Fatal(e)
	}
	commit(t, b["api"])
	m.Repositories[0] = pinRepo(t, "api", b["api"])
	r, e = Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", old)
	if e != nil || r.Change != "target-path-unavailable" {
		t.Fatalf("deleted: %+v %v", r, e)
	}
}

func TestBreakageLineDirectivesPinPhysicalLines(t *testing.T) {
	for _, tc := range []struct {
		name, api, caller                        string
		apiStart, apiEnd, callerStart, callerEnd int
	}{
		{"api-inverted", "package api\n\nfunc Changed() {\n//line fake.go:1\n}\n", "package client\nimport alias \"github.com/acme/library/pkg\"\nfunc Call() { alias.Changed() }\n", 3, 5, 3, 3},
		{"api-before", "package api\n//line fake.go:1\nfunc Changed() {}\n", "package client\nimport alias \"github.com/acme/library/pkg\"\nfunc Call() { alias.Changed() }\n", 3, 3, 3, 3},
		{"caller-before", "package api\nfunc Changed() {}\n", "package client\nimport alias \"github.com/acme/library/pkg\"\n//line fake.go:1000000\nfunc Call() { alias.Changed() }\n", 2, 2, 4, 4},
		{"caller-inverted", "package api\nfunc Changed() {}\n", "package client\nimport alias \"github.com/acme/library/pkg\"\nfunc Call() { alias.\n//line fake.go:1\nChanged() }\n", 2, 2, 3, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, b, _ := fixture(t)
			put(t, b["api"], "pkg/api.go", tc.api)
			commit(t, b["api"])
			put(t, b["client"], "caller.go", tc.caller)
			commit(t, b["client"])
			m.Repositories = []Repository{pinRepo(t, "api", b["api"]), pinRepo(t, "client", b["client"])}
			m.Sources = nil
			m.Providers = nil
			for i, paths := range [][]string{{"go.mod", "pkg/api.go"}, {"go.mod", "caller.go"}} {
				for _, p := range paths {
					r := m.Repositories[i]
					m.Sources = append(m.Sources, pinSource(t, r, b[r.ID], p))
				}
			}
			r, e := Compile(context.Background(), encode(t, m), b, "api:pkg/api.go:Changed", "")
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Edges) != 1 {
				t.Fatalf("expected physical caller witness: %+v", r)
			}
			edge := r.Edges[0]
			if edge.From.Start != tc.callerStart || edge.From.End != tc.callerEnd || edge.To.Start != tc.apiStart || edge.To.End != tc.apiEnd {
				t.Fatalf("adjusted rather than physical anchors: %+v -> %+v", edge.From, edge.To)
			}
			for _, v := range []struct {
				a    *Anchor
				text string
			}{{edge.From, tc.caller}, {edge.To, tc.api}} {
				lines := strings.Split(v.text, "\n")
				want := digest([]byte(strings.Join(lines[v.a.Start-1:v.a.End], "\n")))
				if v.a.SpanSHA256 != want {
					t.Fatal("anchor hash does not match physical source lines")
				}
			}
		})
	}
}

func TestBreakageAnchorRejectsInvalidPhysicalSpans(t *testing.T) {
	c := captured{source: Source{Start: 2, End: 3}, text: []byte("one\ntwo\nthree\nfour")}
	for _, span := range [][2]int{{0, 1}, {3, 2}, {2, 1000000}, {1, 2}, {3, 4}} {
		if a, ok := c.anchor(span[0], span[1]); ok || a != (Anchor{}) {
			t.Fatalf("invalid span %v admitted: %+v", span, a)
		}
	}
	if a, ok := c.anchor(2, 3); !ok || a.SpanSHA256 != digest([]byte("two\nthree")) {
		t.Fatalf("valid physical span refused: %+v %v", a, ok)
	}
}

func TestBreakageBuildConstraintLookalikes(t *testing.T) {
	// BKM-V0-004: only actual build constraints may withhold pinned callers.
	for _, tc := range []struct {
		name, apiPath, callerPath, marker string
	}{
		{"go-build-literal", "pkg/api.go", "caller.go", "const marker = \"//go:build linux\"\n"},
		{"plus-build-literal", "pkg/api.go", "caller.go", "const marker = `// +build linux`\n"},
		{"directive-after-package", "pkg/api.go", "caller.go", "//go:build linux\n"},
		{"os-in-middle", "pkg/api_linux_helpers.go", "caller_linux_helpers.go", ""},
		{"arch-in-middle-test", "pkg/api_amd64_helpers.go", "caller_amd64_helpers_test.go", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bindings := map[string]string{"api": initRepo(t), "client": initRepo(t)}
			put(t, bindings["api"], "go.mod", "module github.com/acme/library\n\ngo 1.27.1\n")
			put(t, bindings["client"], "go.mod", "module github.com/acme/client\n\ngo 1.27.1\n")
			put(t, bindings["api"], tc.apiPath, "package api\n"+tc.marker+"func Changed() {}\n")
			commit(t, bindings["api"])
			put(t, bindings["client"], tc.callerPath, "package client\nimport alias \"github.com/acme/library/pkg\"\n"+tc.marker+"func Call() { alias.Changed() }\n")
			commit(t, bindings["client"])
			m := Manifest{Schema: "corvint-breakage-manifest/0", Repositories: []Repository{pinRepo(t, "api", bindings["api"]), pinRepo(t, "client", bindings["client"])}}
			for i, paths := range [][]string{{"go.mod", tc.apiPath}, {"go.mod", tc.callerPath}} {
				for _, p := range paths {
					repo := m.Repositories[i]
					m.Sources = append(m.Sources, pinSource(t, repo, bindings[repo.ID], p))
				}
			}
			r, err := Compile(context.Background(), encode(t, m), bindings, "api:"+tc.apiPath+":Changed", "")
			if err != nil {
				t.Fatal(err)
			}
			kind := "syntax-call"
			if strings.HasSuffix(tc.callerPath, "_test.go") {
				kind = "test-syntax-call"
			}
			if len(r.Edges) != 1 || r.Edges[0].Kind != kind || r.Edges[0].From.Path != tc.callerPath {
				t.Fatalf("build-constraint lookalike withheld caller: %+v", r)
			}
		})
	}
}

func TestBreakageBuildConditional(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"plain.go", "package api\n", false},
		{"plain.go", "//go:build linux\n\npackage api\n", true},
		{"plain.go", "// +build linux\n\npackage api\n", true},
		{"plain.go", "//+build linux\n\npackage api\n", true},
		{"plain.go", "//go:buildings are ordinary prose\npackage api\n", false},
		{"plain.go", "package api\n//go:build linux\nfunc Changed() {}\n", false},
		{"plain.go", "package api\nfunc Changed() {\n// +build linux\n}\n", false},
		{"plain.go", "// +build linux\npackage api\n", false},
		{"plain.go", "// +build linux\n  package api\n", false},
		{"plain.go", "/* header */\n// +build linux\n\npackage api\n", false},
		{"plain.go", "/* header */\n//go:build linux\n\npackage api\n", true},
		{"plain.go", "/* header */ //go:build linux\n\npackage api\n", false},
		{"plain.go", "//go:build (\n\npackage api\n", true},
		{"plain.go", "/* //go:build linux */\npackage api\n", false},
		{"plain.go", "// Example: //go:build linux\npackage api\n", false},
		{"plain_linux.go", "package api\n", true},
		{"plain_amd64.go", "package api\n", true},
		{"plain_linux_amd64_test.go", "package api\n", true},
		{"plain_hurd.go", "package api\n", true},
		{"plain_mips64p32.go", "package api\n", true},
		{"plain_linux_helpers.go", "package api\n", false},
		{"plain_amd64_helpers_test.go", "package api\n", false},
		{"linux.go", "package api\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, tc.name, tc.source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if got := buildConditional(tc.name, []byte(tc.source), f, fs); got != tc.want {
				t.Fatalf("buildConditional(%q, %q) = %v; want %v", tc.name, tc.source, got, tc.want)
			}
		})
	}
}

func TestBreakageInvalidProviderAnchorCannotAdvance(t *testing.T) {
	m, b, _ := fixture(t)
	sources := map[string]captured{}
	for _, s := range m.Sources {
		var repo Repository
		for _, candidate := range m.Repositories {
			if candidate.ID == s.Repository {
				repo = candidate
			}
		}
		text, e := os.ReadFile(filepath.Join(b[s.Repository], s.Path))
		if e != nil {
			t.Fatal(e)
		}
		sources[sourceKey(s.Repository, s.Path)] = captured{s, repo, text}
	}
	bad := sources["client:flow.json"]
	bad.source.Start, bad.source.End = 2, 1
	sources["client:flow.json"] = bad
	r := Report{}
	composeProviders(&r, m, sources, "api:pkg/api.go")
	if len(r.Edges) != 1 || r.Edges[0].ByteState != "unresolved" || r.Edges[0].To != nil {
		t.Fatalf("invalid anchor advanced traversal: %+v", r.Edges)
	}
	found := false
	for _, u := range r.Unknowns {
		found = found || strings.Contains(u.Reason, "outside supplied physical span")
	}
	if !found {
		t.Fatal("invalid physical span gap missing")
	}
}
