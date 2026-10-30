package doccorpus

import (
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// pairwiseInScope is the comparison every scope loop made before DCR-V1-450.
func pairwiseInScope(p, s string) bool { return p == s || strings.HasPrefix(p, s+"/") }

// pairwiseOverlap is inventoryScopeSet's nested-scope refusal before DCR-V1-450.
func pairwiseOverlap(scopes []string) string {
	for _, outer := range scopes {
		for _, inner := range scopes {
			if outer != inner && pairwiseInScope(inner, outer) {
				return "overlapping scopes: " + outer + " contains " + inner
			}
		}
	}
	return ""
}

// scopeTestPaths draws paths over a small alphabet that includes the bytes
// sorting either side of '/', so prefixes, siblings ("a", "a-b", "a.b",
// "a/b") and nesting all occur.
func scopeTestPaths(r *rand.Rand, n int) []string {
	parts := []string{"a", "a-b", "a.b", "ab", "b", "a0", "z"}
	out := make([]string, n)
	for i := range out {
		depth := 1 + r.IntN(4)
		segments := make([]string, depth)
		for j := range segments {
			segments[j] = parts[r.IntN(len(parts))]
		}
		out[i] = strings.Join(segments, "/")
	}
	return out
}

// TestDCRV1450ScopeMatchingMatchesPairwise: inScope allocates nothing and
// agrees with the old comparison; a scope set visits exactly the scopes the
// pairwise comparison admits, once each, for every revision; and the inventory
// refusal names the same pair the pairwise loop named.
func TestDCRV1450ScopeMatchingMatchesPairwise(t *testing.T) {
	r := rand.New(rand.NewPCG(1450, 1))
	for range 200 {
		scopes := scopeTestPaths(r, 1+r.IntN(12))
		paths := append(scopeTestPaths(r, 40), scopes...)
		paths = append(paths, "", "a/", "a//b", "/a")
		manifest := []Scope{}
		seen := map[string]bool{}
		for i, s := range scopes {
			revision := []string{"r1", "r2"}[i%2]
			if !seen[revision+s] {
				seen[revision+s] = true
				manifest = append(manifest, Scope{Path: s, Revision: revision})
			}
		}
		set := newScopeSet(manifest, nil)
		only := newScopeSet(manifest, func(revision string) bool { return revision == "r2" })
		for _, p := range paths {
			for _, s := range scopes {
				if inScope(p, s) != pairwiseInScope(p, s) {
					t.Fatalf("inScope(%q, %q) = %v", p, s, inScope(p, s))
				}
			}
			for _, revision := range []string{"r1", "r2", "r3"} {
				want := []string{}
				for _, s := range manifest {
					if s.Revision == revision && pairwiseInScope(p, s.Path) {
						want = append(want, s.Path)
					}
				}
				got := []string{}
				set.each(revision, p, func(s string) { got = append(got, s) })
				sort.Strings(want)
				sort.Strings(got)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("scopes of %s:%q = %v, want %v", revision, p, got, want)
				}
				if set.holds(revision, p) != (len(want) > 0) {
					t.Fatalf("holds(%s, %q) = %v", revision, p, set.holds(revision, p))
				}
				if revision != "r2" && only.holds(revision, p) {
					t.Fatalf("member-filtered set holds %s:%q", revision, p)
				}
			}
		}
		requested := append([]string(nil), scopes...)
		sort.Strings(requested)
		requested = slicesCompact(requested)
		_, err := inventoryScopeSet(requested, ManifestSchemaV2)
		want := pairwiseOverlap(requested)
		got := ""
		if err != nil {
			got = err.(*Error).Message
		}
		if got != want {
			t.Fatalf("inventoryScopeSet(%v) = %q, want %q", requested, got, want)
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = inScope("docs/area01/feature00001.md", "docs/area01") }); allocations != 0 {
		t.Fatalf("inScope allocates %v per call", allocations)
	}
	set := newScopeSet([]Scope{{Path: "docs/area01", Revision: "r"}}, nil)
	if allocations := testing.AllocsPerRun(100, func() { _ = set.holds("r", "docs/area01/feature00001.md") }); allocations != 0 {
		t.Fatalf("scope set lookup allocates %v per call", allocations)
	}
}

func slicesCompact(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// TestDCRV1450NestedScopesKeepRefusalsAndReasons: a hand-written manifest may
// nest scopes. It still builds, refuses an undeclared input before a missing
// scope in manifest order, and reports a new live file under the scopes once
// per holding scope.
func TestDCRV1450NestedScopesKeepRefusalsAndReasons(t *testing.T) {
	t.Parallel()
	root, m := fixture(t)
	revision := m.Repository.Revision
	nested := m
	nested.Scopes = []Scope{{Path: "src", Revision: revision}, {Path: "src/value.go", Revision: revision}}
	a, err := Build(context.Background(), root, nested)
	if err != nil {
		t.Fatal(err)
	}
	refusal := func(edit func(*Manifest)) string {
		changed := nested
		changed.Scopes = append([]Scope{}, nested.Scopes...)
		changed.Inputs = append([]Input{}, nested.Inputs...)
		edit(&changed)
		_, err := Build(context.Background(), root, changed)
		if err == nil {
			return ""
		}
		return err.Error()
	}
	dropReadme := func(m *Manifest) {
		for i, in := range m.Inputs {
			if in.Path == "src/readme.md" {
				m.Inputs = append(m.Inputs[:i], m.Inputs[i+1:]...)
				return
			}
		}
		t.Fatal("fixture lost src/readme.md")
	}
	missing := func(m *Manifest) { m.Scopes = append(m.Scopes, Scope{Path: "src/zz-missing.go", Revision: revision}) }
	if got := refusal(dropReadme); !strings.Contains(got, "undeclared input in scope") {
		t.Fatalf("undeclared input: %q", got)
	}
	if got := refusal(missing); !strings.Contains(got, "declared scope missing") {
		t.Fatalf("missing scope: %q", got)
	}
	if got := refusal(func(m *Manifest) { missing(m); dropReadme(m) }); !strings.Contains(got, "undeclared input in scope") {
		t.Fatalf("an earlier scope's undeclared input must refuse first: %q", got)
	}
	earlierMissing := func(m *Manifest) {
		m.Scopes = append([]Scope{{Path: "go.mod/missing", Revision: revision}}, m.Scopes...)
		dropReadme(m)
	}
	if got := refusal(earlierMissing); !strings.Contains(got, "declared scope missing") {
		t.Fatalf("an earlier empty scope must refuse first: %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "new.md"), []byte("# new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := LiveFreshness(context.Background(), root, a)
	if err != nil {
		t.Fatal(err)
	}
	notes := []string{}
	for _, reason := range overlay.Limitations {
		if strings.Contains(reason, "src/new.md") {
			notes = append(notes, reason)
		}
	}
	if overlay.State != "stale" || !reflect.DeepEqual(notes, []string{"new working-tree input in declared scope: src/new.md"}) {
		t.Fatalf("new live file: state %s, notes %v", overlay.State, notes)
	}
}

// TestDCRV1450NewLiveFileNotedPerHoldingScope: a new live file under two nested
// scopes is reported once for each, as the per-scope pass reported it.
func TestDCRV1450NewLiveFileNotedPerHoldingScope(t *testing.T) {
	t.Parallel()
	root, _ := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "src", "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "sub", "a.md"), []byte("# a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "sub")
	revision := git(t, root, "rev-parse", "HEAD")
	m, err := Inventory(context.Background(), root, revision, "src", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	m.Scopes = append(m.Scopes, Scope{Path: "src/sub", Revision: revision})
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "sub", "new.md"), []byte("# new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := LiveFreshness(context.Background(), root, a)
	if err != nil {
		t.Fatal(err)
	}
	want := "new working-tree input in declared scope: src/sub/new.md"
	notes := 0
	for _, reason := range overlay.Limitations {
		if reason == want {
			notes++
		}
	}
	if notes != 2 {
		t.Fatalf("new file under two nested scopes noted %d times, want 2: %v", notes, overlay.Limitations)
	}
}
