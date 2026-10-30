package doccorpus

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// readFixtureScale sizes the large read fixture (DCR-V1-418): native documentation pages in the
// declared scope, source files outside it that a full live re-index would read, and the pinned
// evidence files of each additional repository. manyScopes declares one single-file scope per
// documentation page and per additional-repository file instead of one directory scope each
// (DCR-V1-450), the shape an export of thousands of individually scoped files produces.
type readFixtureScale struct {
	docs, sections, sources, additional, additionalFiles int
	manyScopes                                           bool
}

// largeReadFixture writes, under parent, a primary repository with a native docs scope and a
// records provider whose subjects cite every additional repository, builds the corpus and writes
// manifest.json and corpus.json at the primary root, outside every declared scope. It returns the
// primary root and the additional checkouts.
func largeReadFixture(t testing.TB, parent string, scale readFixtureScale) (string, []string) {
	t.Helper()
	run := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(parent, "app")
	for n := range scale.docs {
		var b strings.Builder
		fmt.Fprintf(&b, "# Feature %d\n\nFeature %d lets an operator manage quokka ledger %d.\n", n, n, n)
		for s := range scale.sections {
			fmt.Fprintf(&b, "\n## Step %d of feature %d\n\nThe operator opens ledger %d and confirms entry %d before saving.\nA saved entry %d appears in the audit trail.\n", s, n, n, s, s)
		}
		write(filepath.Join(root, "docs", fmt.Sprintf("area%02d", n%50), fmt.Sprintf("feature%05d.md", n)), []byte(b.String()))
	}
	for n := range scale.sources {
		write(filepath.Join(root, "src", fmt.Sprintf("pkg%03d", n%100), fmt.Sprintf("file%05d.go", n)),
			fmt.Appendf(nil, "package pkg%03d\n\n// Ledger%d records quokka entry %d.\nfunc Ledger%d(entry int) int {\n\treturn entry + %d\n}\n", n%100, n, n, n, n))
	}
	run(parent, "init", "-q", root)
	run(root, "add", ".")
	run(root, "commit", "-qm", "app")
	revision := run(root, "rev-parse", "HEAD")
	ctx := context.Background()
	scopes := []string{"docs"}
	if scale.manyScopes {
		scopes = scopes[:0]
		for n := range scale.docs {
			scopes = append(scopes, fmt.Sprintf("docs/area%02d/feature%05d.md", n%50, n))
		}
	}
	m, err := InventoryScopes(ctx, root, revision, scopes, ManifestSchemaV2, "2026-10-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{},
		Capabilities: []CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "explicit provider subjects"}, {Name: "claims", State: "present", Reason: "explicit provider claims"}}}
	dirs := []string{}
	extra := []AdditionalRepository{}
	for i := range scale.additional {
		dir := filepath.Join(parent, fmt.Sprintf("lib%d", i))
		for n := range scale.additionalFiles {
			write(filepath.Join(dir, "guide", fmt.Sprintf("p%05d.md", n)), fmt.Appendf(nil, "# lib%d page %d\n\nThe lib%d quokka step %d.\n", i, n, i, n))
		}
		run(parent, "init", "-q", dir)
		run(dir, "add", ".")
		run(dir, "commit", "-qm", "lib")
		rev := run(dir, "rev-parse", "HEAD")
		dirs = append(dirs, dir)
		extra = append(extra, AdditionalRepository{ID: rev, Revision: rev, Root: fmt.Sprintf("../lib%d", i)})
		if !scale.manyScopes {
			m.Scopes = append(m.Scopes, Scope{Path: "guide", Revision: rev})
		}
		for n := range scale.additionalFiles {
			path := fmt.Sprintf("guide/p%05d.md", n)
			if scale.manyScopes {
				m.Scopes = append(m.Scopes, Scope{Path: path, Revision: rev})
			}
			raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			blob := fmt.Sprintf("%x", sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(raw)), raw...)))
			m.Inputs = append(m.Inputs, Input{Path: path, Revision: rev, Blob: blob, SHA256: Digest(raw), Provider: "adapter", Purpose: "evidence"})
			if n < 50 {
				lines := strings.SplitAfter(string(raw), "\n")
				ev := Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Limitations: []string{"provider declaration; structural anchor does not prove behavior"},
					Anchors: []Anchor{{Repository: rev, Revision: rev, Path: path, Blob: blob, SHA256: Digest(raw), Start: 3, End: 3, SpanSHA256: Digest([]byte(lines[2])), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}}}
				subject := fmt.Sprintf("adapter:lib%d-%d", i, n)
				record.Subjects = append(record.Subjects, Subject{ID: subject, Kind: "capability", Name: fmt.Sprintf("lib%d quokka %d", i, n), Provider: "adapter", Evidence: ev})
				record.Claims = append(record.Claims, Claim{ID: subject + "-claim", Subject: subject, Text: fmt.Sprintf("The lib%d quokka step %d", i, n), Provider: "adapter", Evidence: ev})
			}
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })
	m.AdditionalRepositories = extra
	if len(extra) > 0 {
		data, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(root, "evidence", "provider.json"), data)
		run(root, "add", "evidence/provider.json")
		run(root, "commit", "-qm", "provider")
		providerRevision := run(root, "rev-parse", "HEAD")
		m.Providers = append(m.Providers, Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRevision, Record: "evidence/provider.json"})
		m.Scopes = append(m.Scopes, Scope{Path: "evidence/provider.json", Revision: providerRevision})
		m.Inputs = append(m.Inputs, Input{Path: "evidence/provider.json", Revision: providerRevision, Blob: run(root, "rev-parse", providerRevision+":evidence/provider.json"), SHA256: Digest(data), Provider: "adapter", Purpose: "provider"})
	}
	manifest, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "manifest.json"), manifest)
	write(filepath.Join(root, "corpus.json"), encoded)
	// Pack every repository, as a long-lived checkout is, so the object store is not tens of
	// thousands of loose files.
	for _, dir := range append([]string{root}, dirs...) {
		run(dir, "repack", "-adq")
		run(dir, "prune-packed")
	}
	return root, dirs
}

// TestWriteLargeReadFixture persists the large read fixture under CORVINT_READ_FIXTURE_DIR for
// CLI and MCP measurements; it is skipped otherwise.
func TestWriteLargeReadFixture(t *testing.T) {
	dir := os.Getenv("CORVINT_READ_FIXTURE_DIR")
	if dir == "" {
		t.Skip("CORVINT_READ_FIXTURE_DIR not set")
	}
	scale := readFixtureScale{docs: 4000, sections: 6, sources: 6000, additional: 4, additionalFiles: 1500}
	if os.Getenv("CORVINT_READ_FIXTURE_SCOPES") == "many" {
		scale.manyScopes = true
	}
	root, _ := largeReadFixture(t, dir, scale)
	info, err := os.Stat(filepath.Join(root, "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fixture %s: artifact %d bytes", root, info.Size())
}
