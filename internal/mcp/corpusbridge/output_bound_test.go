package corpusbridge

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// boundFixture builds a corpus of pages that one search matches, in the paged
// v2 schema or the unpaged v1 schema.
func boundFixture(t *testing.T, pages int, paged bool) string {
	t.Helper()
	root := t.TempDir()
	for i := range pages {
		path := filepath.Join(root, "docs", fmt.Sprintf("p%03d.md", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, fmt.Appendf(nil, "# Checkout page %d\n\nCheckout step %d.\n", i, i), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", "docs")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "docs")
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "docs", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if paged {
		m.Schema = doccorpus.ManifestSchemaV2
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if paged != (a.Schema == doccorpus.SchemaV2) {
		t.Fatalf("fixture schema %q, paged=%v", a.Schema, paged)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

type boundPage struct {
	Results    []jsontext.Value `json:"results"`
	NextOffset *int             `json:"next_offset"`
}

func withFrameBound(t *testing.T, bound int) {
	t.Helper()
	saved := singleFrameBytes
	singleFrameBytes = bound
	t.Cleanup(func() { singleFrameBytes = saved })
}

func TestDCPV1200SingleCallOverBoundPagesWithNextOffset(t *testing.T) {
	root := boundFixture(t, 40, true)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	call := func(args string) (string, *ToolFailure) {
		t.Helper()
		_, text, failure, protocol := r.Call(context.Background(), "corvint.docs_search", []byte(args))
		if protocol != nil {
			t.Fatalf("%s: protocol error %v", args, protocol)
		}
		return text, failure
	}
	whole, failure := call(`{"query":"Checkout","limit":256}`)
	if failure != nil {
		t.Fatal(failure)
	}
	var all boundPage
	if err := json.Unmarshal([]byte(whole), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Results) < 20 || all.NextOffset != nil {
		t.Fatalf("unbounded call: %d results, next_offset %v", len(all.Results), all.NextOffset)
	}
	bound := singleFrameCost([]byte(whole)) / 4
	withFrameBound(t, bound)

	t.Run("DCP-V1-200 DCP-V1-201 an over-bound call returns a page within the bound with next_offset", func(t *testing.T) {
		text, failure := call(`{"query":"Checkout","limit":256}`)
		if failure != nil {
			t.Fatalf("over-bound call refused: %v", failure)
		}
		var page boundPage
		if err := json.Unmarshal([]byte(text), &page); err != nil {
			t.Fatal(err)
		}
		if cost := singleFrameCost([]byte(text)); cost > bound || len(page.Results) == 0 || page.NextOffset == nil || *page.NextOffset != len(page.Results) {
			t.Fatalf("page cost %d bound %d results %d next_offset %v", cost, bound, len(page.Results), page.NextOffset)
		}
		// The page is the CLI's own receipt for the limit it carries.
		singleFrameBytes = MaxBatchBytes
		want, failure := call(`{"query":"Checkout","limit":` + strconv.Itoa(len(page.Results)) + `}`)
		singleFrameBytes = bound
		if failure != nil || want != text {
			t.Fatalf("bounded page differs from the same-limit receipt: %v", failure)
		}
	})

	t.Run("DCP-V1-201 paging with next_offset returns the union of one unbounded call", func(t *testing.T) {
		var union []jsontext.Value
		offset, pages := 0, 0
		for {
			text, failure := call(`{"query":"Checkout","limit":256,"offset":` + strconv.Itoa(offset) + `}`)
			if failure != nil {
				t.Fatalf("offset %d refused: %v", offset, failure)
			}
			var page boundPage
			if err := json.Unmarshal([]byte(text), &page); err != nil {
				t.Fatal(err)
			}
			union = append(union, page.Results...)
			pages++
			if page.NextOffset == nil {
				break
			}
			if *page.NextOffset <= offset || pages > len(all.Results) {
				t.Fatalf("next_offset %d does not advance past %d", *page.NextOffset, offset)
			}
			offset = *page.NextOffset
		}
		if pages < 2 || len(union) != len(all.Results) {
			t.Fatalf("pages %d union %d want %d", pages, len(union), len(all.Results))
		}
		for i := range union {
			if string(union[i]) != string(all.Results[i]) {
				t.Fatalf("union record %d differs:\n%s\n%s", i, union[i], all.Results[i])
			}
		}
	})

	t.Run("DCP-V1-202 a receipt that cannot shrink is refused naming the bound", func(t *testing.T) {
		singleFrameBytes = 64
		_, failure := call(`{"query":"Checkout","limit":256}`)
		if failure == nil || failure.Code != "output-bound-exceeded" || !strings.Contains(failure.Message, "64-byte") {
			t.Fatalf("unshrinkable receipt: %v", failure)
		}
		singleFrameBytes = bound
	})
}

func TestDCPV1202UnpagedSchemaRefusesOverBound(t *testing.T) {
	root := boundFixture(t, 40, false)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	_, whole, failure, protocol := r.Call(context.Background(), "corvint.docs_search", []byte(`{"query":"Checkout","limit":256}`))
	if failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	bound := singleFrameCost([]byte(whole)) / 4
	withFrameBound(t, bound)
	_, _, failure, protocol = r.Call(context.Background(), "corvint.docs_search", []byte(`{"query":"Checkout","limit":256}`))
	if protocol != nil || failure == nil || failure.Code != "output-bound-exceeded" || !strings.Contains(failure.Message, strconv.Itoa(bound)+"-byte") {
		t.Fatalf("over-bound v1 receipt: failure %v protocol %v", failure, protocol)
	}
}
