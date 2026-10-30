package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type envelopeReceipt struct {
	Freshness  string         `json:"freshness"`
	KindTotals map[string]int `json:"kind_totals"`
	Results    []struct {
		Kind string `json:"kind"`
	} `json:"results"`
	Trust struct {
		Freshness   string   `json:"freshness"`
		VerifiedAt  *string  `json:"verified_at"`
		Limitations []string `json:"limitations"`
		Citations   []struct {
			Path        string `json:"path"`
			Start       int    `json:"start_line"`
			AnchorState string `json:"anchor_state"`
		} `json:"citations"`
	} `json:"trust_envelope"`
}

func envelopeCall(t *testing.T, r *Registry, tool, arguments string) envelopeReceipt {
	t.Helper()
	_, text, failure, protocol, _ := r.CallTimed(context.Background(), tool, []byte(arguments))
	var receipt envelopeReceipt
	if failure != nil || protocol != nil || json.Unmarshal([]byte(text), &receipt) != nil {
		t.Fatalf("%s %s: %v %v %s", tool, arguments, failure, protocol, text)
	}
	return receipt
}

// citationStates maps each cited start line to its anchor state.
func citationStates(t *testing.T, receipt envelopeReceipt) map[int]string {
	t.Helper()
	states := map[int]string{}
	for _, c := range receipt.Trust.Citations {
		states[c.Start] = c.AnchorState
	}
	return states
}

// TestDCPV1282PerCitationAnchorState: a subject cites two spans of one additional-repository
// file; changing one span marks only that citation changed, and a shift, a committed change and a
// deletion report moved, changed and gone. The receipt freshness is never fresher than its
// citations.
func TestDCPV1282PerCitationAnchorState(t *testing.T) {
	root, extra := multiRepositoryFixture(t, [3]int{2, 2, 2})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	get := `{"id":"adapter:lib1-0"}`
	page := filepath.Join(extra[0], "guide1", "p00000.md")
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(page, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	expect := func(step string, want map[int]string, freshness string) {
		t.Helper()
		receipt := envelopeCall(t, r, "corvint.docs_get", get)
		if got := citationStates(t, receipt); len(got) != len(want) || !mapsEqual(got, want) || receipt.Trust.Freshness != freshness || receipt.Freshness != freshness {
			t.Fatalf("%s: states %v freshness %q/%q, want %v %q", step, got, receipt.Trust.Freshness, receipt.Freshness, want, freshness)
		}
	}
	expect("unchanged", map[int]string{1: "live", 3: "live"}, "fresh")
	write("# lib1 page 0\n\nThe lib1 quokka step zero.\n")
	expect("one span edited", map[int]string{1: "live", 3: "changed"}, "stale")
	write("\n# lib1 page 0\n\nThe lib1 quokka step 0.\n")
	expect("both spans shifted", map[int]string{1: "moved", 3: "moved"}, "stale")
	write("# lib1 page 0\n\nThe lib1 quokka step zero.\n")
	for _, args := range [][]string{{"add", "guide1/p00000.md"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "edit"}} {
		if out, err := exec.Command("git", append([]string{"-C", extra[0]}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	expect("committed edit", map[int]string{1: "live", 3: "changed"}, "stale")
	if err := os.Remove(page); err != nil {
		t.Fatal(err)
	}
	expect("deleted", map[int]string{1: "gone", 3: "gone"}, "stale")
}

func mapsEqual(a, b map[int]string) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return len(a) == len(b)
}

// TestDCPV1283UnverifiedCorpusVerifiedAtNull: no verification is recorded, so verified_at is
// null and a limitation says so.
func TestDCPV1283UnverifiedCorpusVerifiedAtNull(t *testing.T) {
	root, _ := multiRepositoryFixture(t, [3]int{2, 2, 2})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	_, text, _, _, _ := r.CallTimed(context.Background(), "corvint.docs_get", []byte(`{"id":"adapter:lib1-0"}`))
	if !strings.Contains(text, `"verified_at":null`) {
		t.Fatalf("verified_at is not an explicit null: %.400s", text)
	}
	receipt := envelopeCall(t, r, "corvint.docs_get", `{"id":"adapter:lib1-0"}`)
	if receipt.Trust.VerifiedAt != nil || !slices.ContainsFunc(receipt.Trust.Limitations, func(l string) bool { return strings.HasPrefix(l, "verified_at is null: ") }) {
		t.Fatalf("unverified corpus: verified_at=%v limitations=%v", receipt.Trust.VerifiedAt, receipt.Trust.Limitations)
	}
}

// TestDCPV1284GapsKindFilter: docs_gaps {kind} returns only that kind with a per-kind total,
// accepts a list, totals an absent kind as zero, and refuses kind on any other tool.
func TestDCPV1284GapsKindFilter(t *testing.T) {
	root, _ := multiRepositoryFixture(t, [3]int{2, 2, 2})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	all := envelopeCall(t, r, "corvint.docs_gaps", `{"limit":200}`)
	counts := map[string]int{}
	for _, g := range all.Results {
		counts[g.Kind]++
	}
	if len(counts) < 2 || all.KindTotals != nil {
		t.Fatalf("fixture needs gaps and an unfiltered call has no kind totals: %v %v", counts, all.KindTotals)
	}
	for kind, count := range counts {
		filtered := envelopeCall(t, r, "corvint.docs_gaps", `{"limit":200,"kind":"`+kind+`"}`)
		if len(filtered.Results) != count || !mapsEqualInt(filtered.KindTotals, map[string]int{kind: count}) {
			t.Fatalf("kind %q: %d results totals %v, want %d", kind, len(filtered.Results), filtered.KindTotals, count)
		}
		for _, g := range filtered.Results {
			if g.Kind != kind {
				t.Fatalf("kind %q filter returned %q", kind, g.Kind)
			}
		}
		listed := envelopeCall(t, r, "corvint.docs_gaps", `{"limit":200,"kind":["`+kind+`","no-such-kind"]}`)
		if len(listed.Results) != count || !mapsEqualInt(listed.KindTotals, map[string]int{kind: count, "no-such-kind": 0}) {
			t.Fatalf("kind list: %d results totals %v", len(listed.Results), listed.KindTotals)
		}
	}
	for _, args := range []string{`{"kind":[]}`, `{"kind":""}`, `{"kind":["a","a"]}`, `{"kind":7}`} {
		if _, _, _, protocol, _ := r.CallTimed(context.Background(), "corvint.docs_gaps", []byte(args)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("docs_gaps %s accepted: %v", args, protocol)
		}
	}
	if _, _, _, protocol, _ := r.CallTimed(context.Background(), "corvint.docs_get", []byte(`{"id":"adapter:lib1-0","kind":"x"}`)); protocol == nil || protocol.Code != "invalid-arguments" {
		t.Fatalf("kind accepted outside docs_gaps: %v", protocol)
	}
}

func mapsEqualInt(a, b map[string]int) bool {
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return len(a) == len(b)
}
