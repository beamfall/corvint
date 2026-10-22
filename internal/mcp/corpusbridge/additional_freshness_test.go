package corpusbridge

import (
	"context"
	"crypto/sha1"
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// multiRepositoryFixture builds a primary repository with a native docs scope and a records
// provider whose subjects cite each of two additional repositories; every additional file is a
// pinned evidence input. It returns the primary root and each additional checkout's directory.
func multiRepositoryFixture(t testing.TB, files [3]int) (string, []string) {
	t.Helper()
	parent := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	names := []string{"app", "lib1", "lib2"}
	scopes := []string{"docs", "guide1", "guide2"}
	dirs := make([]string, 3)
	revisions := make([]string, 3)
	for i, name := range names {
		dirs[i] = filepath.Join(parent, name)
		if err := os.MkdirAll(filepath.Join(dirs[i], scopes[i]), 0o700); err != nil {
			t.Fatal(err)
		}
		for n := range files[i] {
			path := filepath.Join(dirs[i], scopes[i], fmt.Sprintf("p%05d.md", n))
			if err := os.WriteFile(path, fmt.Appendf(nil, "# %s page %d\n\nThe %s quokka step %d.\n", name, n, name, n), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		git(dirs[i], "init", "-q")
		git(dirs[i], "add", ".")
		git(dirs[i], "commit", "-qm", name)
		revisions[i] = git(dirs[i], "rev-parse", "HEAD")
	}
	ctx := context.Background()
	m, err := doccorpus.InventoryScopes(ctx, dirs[0], revisions[0], []string{scopes[0]}, doccorpus.ManifestSchemaV2, "2026-10-21T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	record := doccorpus.ProviderRecord{Schema: doccorpus.ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{}, Claims: []doccorpus.Claim{}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{},
		Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "explicit provider subjects"}, {Name: "claims", State: "present", Reason: "explicit provider claims"}}}
	extra := []doccorpus.AdditionalRepository{}
	for i := 1; i < 3; i++ {
		id, rev := revisions[i], revisions[i]
		extra = append(extra, doccorpus.AdditionalRepository{ID: id, Revision: rev, Root: "../" + names[i]})
		m.Scopes = append(m.Scopes, doccorpus.Scope{Path: scopes[i], Revision: rev})
		for n := range files[i] {
			path := fmt.Sprintf("%s/p%05d.md", scopes[i], n)
			raw, err := os.ReadFile(filepath.Join(dirs[i], filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			blob := blobID(raw)
			m.Inputs = append(m.Inputs, doccorpus.Input{Path: path, Revision: rev, Blob: blob, SHA256: doccorpus.Digest(raw), Provider: "adapter", Purpose: "evidence"})
			if n < 2 {
				// Two cited spans per subject, the heading and the step line (DCP-V1-282).
				lines := strings.SplitAfter(string(raw), "\n")
				anchor := func(line int) doccorpus.Anchor {
					return doccorpus.Anchor{Repository: id, Revision: rev, Path: path, Blob: blob, SHA256: doccorpus.Digest(raw), Start: line, End: line, SpanSHA256: doccorpus.Digest([]byte(lines[line-1])), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}
				}
				ev := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Limitations: []string{"provider declaration; structural anchor does not prove behavior"},
					Anchors: []doccorpus.Anchor{anchor(1), anchor(3)}}
				subject := fmt.Sprintf("adapter:%s-%d", names[i], n)
				record.Subjects = append(record.Subjects, doccorpus.Subject{ID: subject, Kind: "capability", Name: fmt.Sprintf("%s quokka %d", names[i], n), Provider: "adapter", Evidence: ev})
				record.Claims = append(record.Claims, doccorpus.Claim{ID: subject + "-claim", Subject: subject, Text: fmt.Sprintf("The %s quokka step %d", names[i], n), Provider: "adapter", Evidence: ev})
				if i == 2 && n == 1 {
					// Two gap kinds for the docs_gaps kind filter (DCP-V1-284): a claim without
					// collected evidence and a test declaration without an observation.
					unknown := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "fresh", Anchors: []doccorpus.Anchor{}, Unknown: "provider has not collected evidence", Limitations: []string{}}
					record.Claims = append(record.Claims, doccorpus.Claim{ID: subject + "-unknown", Subject: subject, Text: "The quokka step is uncollected", Provider: "adapter", Evidence: unknown})
					record.Subjects = append(record.Subjects, doccorpus.Subject{ID: "adapter:test-0", Kind: "test", Name: "quokka test", Provider: "adapter", Evidence: ev})
				}
			}
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })
	m.AdditionalRepositories = extra
	data, err := doccorpus.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dirs[0], "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirs[0], "evidence", "provider.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	git(dirs[0], "add", "evidence/provider.json")
	git(dirs[0], "commit", "-qm", "provider")
	providerRevision := git(dirs[0], "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRevision, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "evidence/provider.json", Revision: providerRevision})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "evidence/provider.json", Revision: providerRevision, Blob: git(dirs[0], "rev-parse", providerRevision+":evidence/provider.json"), SHA256: doccorpus.Digest(data), Provider: "adapter", Purpose: "provider"})
	a, err := doccorpus.Build(ctx, dirs[0], m)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirs[0], "corpus.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return dirs[0], dirs[1:]
}

// blobID is Git's SHA-1 blob object id for raw.
func blobID(raw []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(raw)), raw...)))
}

// freshnessCall answers one tool call and reports its timing, receipt freshness and limitations.
func freshnessCall(t testing.TB, r *Registry, tool, arguments string) (Timing, string, []string) {
	t.Helper()
	_, text, failure, protocol, timing := r.CallTimed(context.Background(), tool, []byte(arguments))
	var receipt struct {
		Trust struct {
			Freshness   string   `json:"freshness"`
			Limitations []string `json:"limitations"`
		} `json:"trust_envelope"`
	}
	if failure != nil || protocol != nil || json.Unmarshal([]byte(text), &receipt) != nil {
		t.Fatalf("%s %s: %v %v %s", tool, arguments, failure, protocol, text)
	}
	return timing, receipt.Trust.Freshness, receipt.Trust.Limitations
}

// expectStaleNaming asserts one call recomputed freshness and reported stale with a reason
// naming the changed additional repository.
func expectStaleNaming(t testing.TB, r *Registry, id string) {
	t.Helper()
	timing, freshness, limits := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`)
	if timing.Freshness != "computed" || freshness != "stale" || !strings.Contains(strings.Join(limits, "\n"), "additional repository "+id+": ") {
		t.Fatalf("changed additional repository: timing=%+v freshness=%q limits=%v", timing, freshness, limits)
	}
}

func repositoryID(t testing.TB, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-list", "--max-parents=0", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// TestDCPV1280AdditionalRepositoryFreshnessReused: with additional repositories declared, the
// overlay is reused while every checkout's observation holds; a change in one recomputes, names
// that repository, and is then reused itself.
func TestDCPV1280AdditionalRepositoryFreshnessReused(t *testing.T) {
	root, extra := multiRepositoryFixture(t, [3]int{3, 3, 3})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"computed", "reused", "reused"} {
		if timing, freshness, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`); timing.Freshness != want || freshness != "fresh" {
			t.Fatalf("call %d: freshness=%q (%s), want %q", i, timing.Freshness, freshness, want)
		}
	}
	lib := extra[1]
	page := filepath.Join(lib, "guide2", "p00001.md")
	original, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("# changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectStaleNaming(t, r, repositoryID(t, lib))
	if timing, freshness, _ := freshnessCall(t, r, "corvint.docs_search", `{"query":"quokka"}`); timing.Freshness != "reused" || freshness != "stale" {
		t.Fatalf("unchanged stale state not reused: %+v %q", timing, freshness)
	}
	if err := os.WriteFile(page, original, 0o600); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"computed", "reused"} {
		if timing, freshness, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`); timing.Freshness != want || freshness != "fresh" {
			t.Fatalf("restored call %d: freshness=%q (%s), want %q", i, timing.Freshness, freshness, want)
		}
	}
	// A commit in the other additional repository moves its HEAD, so the key changes even with a
	// clean worktree.
	other := extra[0]
	if err := os.WriteFile(filepath.Join(other, "note.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "note.txt"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "note"}} {
		if out, err := exec.Command("git", append([]string{"-C", other}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if timing, _, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`); timing.Freshness != "computed" {
		t.Fatalf("moved additional repository HEAD reused the overlay: %+v", timing)
	}
}

// TestDCPV1281AdditionalRepositoryWarmCallsAtScale measures the primary + 2 additional
// repository fixture with 17,001 pinned inputs, one additional checkout holding 12,000 tracked
// files (DCP-V1-317): warm calls reuse freshness, the median reused freshness costs under 100 ms
// and the warm p95 of docs_get and docs_search is under 1 s.
func TestDCPV1281AdditionalRepositoryWarmCallsAtScale(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing bound measured without -short and without the race detector")
	}
	root, extra := multiRepositoryFixture(t, [3]int{4000, 12000, 1000})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	first, _, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`)
	freshness, total, verify := []int64{}, []int64{}, []int64{}
	for i := range 20 {
		tool, arguments := "corvint.docs_get", `{"id":"adapter:lib1-0"}`
		if i%2 == 1 {
			tool, arguments = "corvint.docs_search", `{"query":"quokka"}`
		}
		timing, state, _ := freshnessCall(t, r, tool, arguments)
		if timing.Freshness != "reused" || state != "fresh" {
			t.Fatalf("warm call %d: %+v %q", i, timing, state)
		}
		freshness, total, verify = append(freshness, timing.FreshnessMS), append(total, timing.TotalMS), append(verify, timing.VerifyMS)
	}
	slices.Sort(freshness)
	slices.Sort(total)
	median, p95 := freshness[len(freshness)/2], total[len(total)*95/100]
	t.Logf("DCP-V1-281 first call %+v; warm freshness_ms sorted %v; warm total_ms sorted %v; p95 %d ms; warm verify_ms in call order %v", first, freshness, total, p95, verify)
	if median >= 100 || p95 >= int64(time.Second/time.Millisecond) {
		t.Fatalf("warm freshness median %d ms (bound 100), total p95 %d ms (bound 1000)", median, p95)
	}
	page := filepath.Join(extra[0], "guide1", "p00007.md")
	if err := os.WriteFile(page, []byte("# changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectStaleNaming(t, r, repositoryID(t, extra[0]))
	if timing, state, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`); timing.Freshness != "reused" || state != "stale" {
		t.Fatalf("stale state not reused: %+v %q", timing, state)
	}
}
