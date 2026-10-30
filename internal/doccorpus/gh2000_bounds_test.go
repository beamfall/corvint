package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

// TestDCIV1024OverBoundInputsAreDeclaredGaps: a scope holding inputs over the per-input bound,
// including one over gitauth.MaxBlobBytes, still inventories and builds. Each over-bound file is
// pinned by blob and size in over_bound_inputs, never read, and reported as an input-over-bound
// gap naming its size and the bound, while the rest of the scope is collected (DCI-V1-011,
// DCI-V1-024; GitHub #2000, #2008). A changed, missing or forged entry is refused.
func TestDCIV1024OverBoundInputsAreDeclaredGaps(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	large := bytes.Repeat([]byte("a"), MaxBytes+1)
	huge := bytes.Repeat([]byte("b"), gitauth.MaxBlobBytes+1)
	write := func(root, p string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(app, "src/data/large.bin", large)
	write(app, "src/data/huge.bin", huge)
	write(tests, "pages/large.bin", large)
	appRevision := commitFiles(t, app, map[string]string{})
	testsRevision := commitFiles(t, tests, map[string]string{})
	m, err := ManifestInventory(context.Background(), app, appRevision, []string{"src"}, ManifestSchema, "2026-10-29T00:00:00Z")
	if err != nil {
		t.Fatalf("a scope with over-bound inputs should inventory: %v", err)
	}
	want := []OverBoundInput{
		{Path: "src/data/huge.bin", Revision: appRevision, Blob: git(t, app, "rev-parse", appRevision+":src/data/huge.bin"), Bytes: gitauth.MaxBlobBytes + 1, Provider: "native"},
		{Path: "src/data/large.bin", Revision: appRevision, Blob: git(t, app, "rev-parse", appRevision+":src/data/large.bin"), Bytes: MaxBytes + 1, Provider: "native"},
	}
	if !slices.Equal(m.OverBoundInputs, want) || len(m.Inputs) != 1 || m.Inputs[0].Path != "src/app.go" {
		t.Fatalf("over-bound inputs not pinned apart from the read inputs: %+v %+v", m.OverBoundInputs, m.Inputs)
	}
	data, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	if m, err = ParseManifest(data); err != nil || !slices.Equal(m.OverBoundInputs, want) {
		t.Fatalf("over_bound_inputs did not round-trip: %v", err)
	}
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatalf("a scope with over-bound inputs should build: %v", err)
	}
	gap := func(a *Artifact, subject string, bytes int) {
		t.Helper()
		i := slices.IndexFunc(a.Gaps, func(g Gap) bool { return g.Subject == subject })
		if i < 0 || a.Gaps[i].Kind != "input-over-bound" || !strings.HasPrefix(a.Gaps[i].Reason, "input is "+strconv.Itoa(bytes)+" bytes, over the "+strconv.Itoa(MaxBytes)+"-byte per-input bound; pinned at blob ") {
			t.Fatalf("over-bound input %s is not a named gap: %+v", subject, a.Gaps)
		}
	}
	gap(a, "native:file:src/data/huge.bin", gitauth.MaxBlobBytes+1)
	gap(a, "native:file:src/data/large.bin", MaxBytes+1)
	if len(subjectsWithPrefix(a, "native:file:src/app.go")) == 0 || len(subjectsWithPrefix(a, "native:file:src/data/")) != 0 {
		t.Fatal("the rest of the scope should be collected, the over-bound files not read")
	}
	if state, limits, err := Freshness(context.Background(), app, a); err != nil || state != "fresh" {
		t.Fatalf("an over-bound input is declared, so the live scope is fresh: %s %v %v", state, limits, err)
	}
	refused := func(edit func(*Manifest), message string) {
		t.Helper()
		bad := m
		bad.OverBoundInputs = slices.Clone(m.OverBoundInputs)
		edit(&bad)
		if _, err := Build(context.Background(), app, bad); err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("want %q, got %v", message, err)
		}
	}
	refused(func(b *Manifest) { b.OverBoundInputs[1].Bytes++ }, "over-bound input changed after pinning: src/data/large.bin")
	refused(func(b *Manifest) { b.OverBoundInputs[1].Blob = b.OverBoundInputs[0].Blob }, "over-bound input changed after pinning: src/data/large.bin")
	refused(func(b *Manifest) { b.OverBoundInputs = b.OverBoundInputs[:1] }, "undeclared input in scope")
	refused(func(b *Manifest) { b.OverBoundInputs[1].Bytes = MaxBytes }, "invalid over-bound input: src/data/large.bin")
	refused(func(b *Manifest) { b.OverBoundInputs = append(b.OverBoundInputs, b.OverBoundInputs[0]) }, "invalid over-bound input: src/data/huge.bin")
	refused(func(b *Manifest) { b.OverBoundInputs[0].Path = "src/app.go" }, "invalid over-bound input: src/app.go")
	refused(func(b *Manifest) { b.OverBoundInputs[0].Path = "elsewhere/huge.bin" }, "invalid over-bound input: elsewhere/huge.bin")
	// The review witnesses: an entry for a file the revision does not hold, an entry for a file
	// within the bound, and more entries than the record limit are refused.
	refused(func(b *Manifest) {
		b.OverBoundInputs = append(b.OverBoundInputs, OverBoundInput{Path: "src/data/missing.bin", Revision: appRevision, Blob: b.OverBoundInputs[1].Blob, Bytes: MaxBytes + 1, Provider: "native"})
	}, "over-bound input changed after pinning: src/data/missing.bin")
	refused(func(b *Manifest) {
		in := b.Inputs[0]
		b.Inputs = nil
		b.OverBoundInputs = append([]OverBoundInput{{Path: in.Path, Revision: in.Revision, Blob: in.Blob, Bytes: MaxBytes + 1, Provider: "native"}}, b.OverBoundInputs...)
	}, "over-bound input changed after pinning: src/app.go")
	refused(func(b *Manifest) { b.OverBoundInputs = slices.Repeat(b.OverBoundInputs[:1], MaxRecords+1) }, "manifest over-bound input count 4097 exceeds the limit of 4096")
	write(app, "src/data/large.bin", append(large, 'c'))
	commitFiles(t, app, map[string]string{})
	if _, limits, err := Freshness(context.Background(), app, a); err != nil || !slices.Contains(limits, "over-bound input changed or absent: src/data/large.bin") {
		t.Fatalf("a changed over-bound input should be noted: %v %v", limits, err)
	}
	w, err := InventoryWorkspace(context.Background(), app, appRevision, []string{"docs"}, ManifestSchema, "2026-10-29T00:00:00Z", []WorkspaceRepository{{Alias: "e2e", Root: tests, Scopes: []string{"pages"}}})
	if err != nil || len(w.OverBoundInputs) != 1 || w.OverBoundInputs[0].Path != "pages/large.bin" || w.OverBoundInputs[0].Revision != testsRevision {
		t.Fatalf("a further repository's over-bound input should be pinned at its revision: %+v %v", w.OverBoundInputs, err)
	}
	if a, err = Build(context.Background(), app, w); err != nil {
		t.Fatal(err)
	}
	gap(a, "native:file:e2e:pages/large.bin", MaxBytes+1)
	clean, err := ManifestInventory(context.Background(), app, appRevision, []string{"docs"}, ManifestSchema, "2026-10-29T00:00:00Z")
	if data, _ := Encode(clean); err != nil || bytes.Contains(data, []byte("over_bound_inputs")) {
		t.Fatalf("a manifest without over-bound inputs keeps its bytes: %v", err)
	}
}

// TestDCSV1590NativeDerivationFilterRestrictsNativeOwnedInputs: derive {kinds: [module],
// native: true} keeps every native-owned file subject and drops its symbol and test
// declarations, naming them in one gap; without native the filter leaves native-owned inputs
// alone, and the field round-trips (GitHub #2000).
func TestDCSV1590NativeDerivationFilterRestrictsNativeOwnedInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	rev := commitFiles(t, root, filterSources)
	base, err := Inventory(context.Background(), root, rev, "src", "2026-10-29T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	build := func(f *DerivationFilter) *Artifact {
		t.Helper()
		m := base
		m.Providers = slices.Clone(base.Providers)
		m.Providers[0].Derive = f
		data, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseManifest(data)
		if err != nil {
			t.Fatal(err)
		}
		if f != nil && parsed.Providers[0].Derive.Native != f.Native {
			t.Fatalf("native flag did not round-trip")
		}
		a, err := Build(context.Background(), root, parsed)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	full := build(nil)
	if got := derivedPaths(full); len(got) != 3 {
		t.Fatalf("an unfiltered native corpus should declare every source: %v", got)
	}
	if got := derivedPaths(build(&DerivationFilter{Kinds: []string{"module"}})); len(got) != 3 {
		t.Fatalf("a filter without native should not touch native-owned inputs: %v", got)
	}
	a := build(&DerivationFilter{Kinds: []string{"module"}, Native: true})
	if got := derivedPaths(a); len(got) != 0 {
		t.Fatalf("a module-only native filter should derive no declaration: %v", got)
	}
	if len(subjectsWithPrefix(a, "native:file:")) != len(subjectsWithPrefix(full, "native:file:")) {
		t.Fatalf("file subjects should be kept")
	}
	found := false
	for _, g := range a.Gaps {
		if g.Subject == "native:native-derivation-filter" {
			found = true
			if !strings.Contains(g.Reason, "kinds [module]) excluded 3 declarations of 3 native-owned inputs: src/app/cart.py, src/app/test_cart.py, src/lib/util.py") {
				t.Fatalf("unexpected gap reason: %s", g.Reason)
			}
		}
	}
	if !found {
		t.Fatal("the native filter should name what it excluded")
	}
	if got := derivedPaths(build(&DerivationFilter{Paths: []string{"src/app/"}, Kinds: []string{"symbol"}, Native: true})); !slices.Equal(got, []string{"src/app/cart.py"}) {
		t.Fatalf("a native path and kind filter should keep the selected declarations: %v", got)
	}
	m := Manifest{Schema: ManifestSchema, Providers: []Provider{{ID: "native", Kind: "native", Derive: &DerivationFilter{Native: true}}}}
	if err := validDerivationFilter(m, m.Providers[0]); err == nil {
		t.Fatal("a native filter that selects everything should be refused as vacuous")
	}
}

// TestDCIV1012EvidenceAnchorBound: an evidence object holds at most 64 anchors and 64
// limitations; one more of either is refused as the evidence bound, whatever else it holds.
func TestDCIV1012EvidenceAnchorBound(t *testing.T) {
	t.Parallel()
	c := &compiler{}
	bound := fail("evidence bound exceeded").Error()
	for name, e := range map[string]Evidence{
		"anchors":     {Anchors: make([]Anchor, 65)},
		"limitations": {Limitations: make([]string, 65)},
	} {
		e.Derivation, e.State, e.Freshness, e.Trust = "declared", "supported", "fresh", "generated"
		if err := c.checkEvidence(&e, true); err == nil || err.Error() != bound {
			t.Fatalf("65 %s should exceed the evidence bound: %v", name, err)
		}
		if name == "anchors" {
			e.Anchors = e.Anchors[:64]
		} else {
			e.Limitations = e.Limitations[:64]
		}
		if err := c.checkEvidence(&e, true); err != nil && err.Error() == bound {
			t.Fatalf("64 %s should be within the evidence bound", name)
		}
	}
}

// TestDCIV1024OnlyOverBoundScopeBuildsWithGaps: a scope whose every file is over the per-input
// bound inventories with no read input and builds with one gap per file, and a malformed object
// header of a pinned entry refuses the build and the inventory (DCI-V1-024).
func TestDCIV1024OnlyOverBoundScopeBuildsWithGaps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	revision := commitFiles(t, root, map[string]string{"data/large.bin": strings.Repeat("a", MaxBytes+1), "README.md": "# Readme\n"})
	m, err := ManifestInventory(context.Background(), root, revision, []string{"data"}, ManifestSchema, "2026-10-29T00:00:00Z")
	if err != nil || len(m.Inputs) != 0 || len(m.OverBoundInputs) != 1 {
		t.Fatalf("a scope of over-bound inputs should inventory with no read input: %+v %+v %v", m.Inputs, m.OverBoundInputs, err)
	}
	data, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	if m, err = ParseManifest(data); err != nil {
		t.Fatalf("the manifest should parse: %v", err)
	}
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatalf("a scope of over-bound inputs should build with gaps: %v", err)
	}
	if i := slices.IndexFunc(a.Gaps, func(g Gap) bool { return g.Subject == "native:file:data/large.bin" }); i < 0 || a.Gaps[i].Kind != "input-over-bound" {
		t.Fatalf("the over-bound file should be a gap: %+v", a.Gaps)
	}
	blob := m.OverBoundInputs[0].Blob
	object := filepath.Join(root, ".git", "objects", blob[:2], blob[2:])
	if err := os.Chmod(object, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("not a zlib object header"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), root, m); err == nil {
		t.Fatal("a malformed header of a pinned over-bound input should refuse the build")
	}
	if _, err := ManifestInventory(context.Background(), root, revision, []string{"data"}, ManifestSchema, "2026-10-29T00:00:00Z"); err == nil {
		t.Fatal("a malformed header should refuse the inventory")
	}
}
