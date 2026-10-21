package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

const rubyOrder = `class Order
  def total
    # sum the line items
    items.sum { |item| item.price * qty(item) }
  end

  def label
    "Order #{id}"
  end
end
`

const webCart = "export function total(items: Item[]): number {\n" +
	"  // sum prices\n" +
	"  return items.reduce((sum, item) => sum + item.price, 0);\n" +
	"}\n" +
	"\n" +
	"export function label(id: string): string {\n" +
	"  return `Order ${id}`;\n" +
	"}\n"

func commitFiles(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for p, data := range files {
		full := filepath.Join(root, p)
		if data == "" {
			if err := os.Remove(full); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "change")
	return git(t, root, "rev-parse", "HEAD")
}

func symbolRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Symbol test")
	git(t, root, "config", "user.email", "symbol@example.invalid")
	return root, commitFiles(t, root, files)
}

func pinnedSymbolAnchor(t *testing.T, root, rev, path string, start, end int, symbol string) Anchor {
	t.Helper()
	id, err := contextindex.CorpusRepositoryID(context.Background(), root, rev)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(git(t, root, "show", rev+":"+path) + "\n")
	return Anchor{Repository: id, Revision: rev, Path: path, Blob: git(t, root, "rev-parse", rev+":"+path), SHA256: Digest(raw), Start: start, End: end, SpanSHA256: spanDigest(raw, start, end), Symbol: symbol}
}

func resolveAt(t *testing.T, root, rev string, a Anchor, dirty ...string) SymbolAnchorCheck {
	t.Helper()
	ctx := context.Background()
	index, err := contextindex.BuildRevisionContext(ctx, root, rev)
	if err != nil {
		t.Fatal(err)
	}
	id, err := contextindex.CorpusRepositoryID(ctx, root, rev)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	release := auth.BeginObjectSession()
	defer release()
	check, err := NewSymbolResolver(auth, index, Repository{ID: id, Revision: rev}, dirty).Resolve(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	return check
}

func insertLines(text string, count int, line string) string {
	return strings.Repeat(line, count) + text
}

// TestSymbolAnchorReresolution covers the issue 1223 acceptance for Ruby and
// TypeScript: insertion above, a one-token edit, deletion and a reformat.
func TestSymbolAnchorReresolution(t *testing.T) {
	t.Parallel()
	languages := []struct {
		name, path, source, padding string
		start, end                  int
		edit, reformat              func(string) string
		deleted                     string
	}{
		{
			name: "ruby", path: "app/models/order.rb", source: rubyOrder, padding: "# generated padding line\n", start: 2, end: 5,
			edit: func(s string) string { return strings.Replace(s, "item.price", "item.cost", 1) },
			reformat: func(s string) string {
				s = strings.Replace(s, "  def total\n    # sum the line items\n    items.sum { |item| item.price * qty(item) }\n  end\n",
					"  def total # totals\n\n      # sums every line item\n      # including tax\n      items.sum {   |item|  item.price * qty(item) }\n  end\n", 1)
				return s
			},
			deleted: "class Order\n  def label\n    \"Order #{id}\"\n  end\nend\n",
		},
		{
			name: "typescript", path: "app/cart.ts", source: webCart, padding: "const padding = 1;\n", start: 1, end: 4,
			edit: func(s string) string { return strings.Replace(s, "item.price", "item.cost", 1) },
			reformat: func(s string) string {
				return strings.Replace(s, "  // sum prices\n  return items.reduce((sum, item) => sum + item.price, 0);\n",
					"    /* sums every\n       price */\n    return items.reduce((sum,   item) => sum + item.price, 0); // total\n", 1)
			},
			deleted: "export function label(id: string): string {\n  return `Order ${id}`;\n}\n",
		},
	}
	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			setup := func(t *testing.T) (string, Anchor) {
				root, rev := symbolRepo(t, map[string]string{lang.path: lang.source, "README.md": "fixture\n"})
				return root, pinnedSymbolAnchor(t, root, rev, lang.path, lang.start, lang.end, "total")
			}
			t.Run("DCP-V1-140 unchanged is live", func(t *testing.T) {
				root, a := setup(t)
				rev := commitFiles(t, root, map[string]string{"README.md": "fixture two\n"})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorLive || check.Live.Start != lang.start || check.Live.End != lang.end || check.LiveNormalizedSHA256 != check.NormalizedSHA256 {
					t.Fatalf("unchanged: %+v", check)
				}
			})
			t.Run("DCP-V1-140 insertion above is moved", func(t *testing.T) {
				root, a := setup(t)
				rev := commitFiles(t, root, map[string]string{lang.path: insertLines(lang.source, 20, lang.padding)})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorMoved || check.Live == nil || check.Live.Path != lang.path || check.Live.Start != lang.start+20 || check.Live.End != lang.end+20 {
					t.Fatalf("insertion: %+v %+v", check, check.Live)
				}
				raw := []byte(git(t, root, "show", rev+":"+lang.path) + "\n")
				if check.Live.SpanSHA256 != a.SpanSHA256 || check.Live.SpanSHA256 != spanDigest(raw, check.Live.Start, check.Live.End) {
					t.Fatalf("moved span digest does not bind the live lines: %+v", check.Live)
				}
			})
			t.Run("DCP-V1-140 one-token edit is changed", func(t *testing.T) {
				root, a := setup(t)
				rev := commitFiles(t, root, map[string]string{lang.path: lang.edit(lang.source)})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorChanged || check.Live == nil || check.Live.Start != lang.start || check.LiveNormalizedSHA256 == check.NormalizedSHA256 {
					t.Fatalf("edit: %+v", check)
				}
			})
			t.Run("DCP-V1-140 deletion is gone", func(t *testing.T) {
				root, a := setup(t)
				rev := commitFiles(t, root, map[string]string{lang.path: lang.deleted})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorGone || check.Live != nil {
					t.Fatalf("deletion: %+v", check)
				}
			})
			t.Run("DCP-V1-140 reformat is never changed", func(t *testing.T) {
				root, a := setup(t)
				reformatted := lang.reformat(lang.source)
				if reformatted == lang.source {
					t.Fatal("fixture reformat did not apply")
				}
				rev := commitFiles(t, root, map[string]string{lang.path: reformatted})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorLive && check.State != SymbolAnchorMoved {
					t.Fatalf("reformat: %+v", check)
				}
				if check.LiveNormalizedSHA256 != check.NormalizedSHA256 {
					t.Fatalf("reformat changed the normalized digest: %+v", check)
				}
			})
			t.Run("DCP-V1-140 relocation to another path is moved", func(t *testing.T) {
				root, a := setup(t)
				other := strings.Replace(lang.path, ".", "_moved.", 1)
				rev := commitFiles(t, root, map[string]string{lang.path: lang.deleted, other: insertLines(lang.source, 3, lang.padding)})
				check := resolveAt(t, root, rev, a)
				if check.State != SymbolAnchorMoved || check.Live.Path != other || check.Live.Start != lang.start+3 {
					t.Fatalf("relocation: %+v %+v", check, check.Live)
				}
			})
			t.Run("DCP-V1-142 working-tree change is unresolved", func(t *testing.T) {
				root, a := setup(t)
				rev := git(t, root, "rev-parse", "HEAD")
				check := resolveAt(t, root, rev, a, lang.path)
				if check.State != SymbolAnchorUnresolved || !strings.Contains(check.Reason, "working-tree") {
					t.Fatalf("dirty: %+v", check)
				}
			})
		})
	}
}

func TestSymbolAnchorUncertainty(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-142 unsupported language is unresolved", func(t *testing.T) {
		root, rev := symbolRepo(t, map[string]string{"app/report.py": "def report():\n    return 1\n"})
		a := pinnedSymbolAnchor(t, root, rev, "app/report.py", 1, 2, "report")
		check := resolveAt(t, root, rev, a)
		if check.State != SymbolAnchorUnresolved || !strings.Contains(check.Reason, "Ruby and TypeScript/JavaScript only") {
			t.Fatalf("python: %+v", check)
		}
	})
	t.Run("DCP-V1-142 literal whitespace is a change", func(t *testing.T) {
		root, rev := symbolRepo(t, map[string]string{"app/models/order.rb": rubyOrder})
		a := pinnedSymbolAnchor(t, root, rev, "app/models/order.rb", 7, 9, "label")
		rev = commitFiles(t, root, map[string]string{"app/models/order.rb": strings.Replace(rubyOrder, `"Order #{id}"`, `"Order  #{id}"`, 1)})
		if check := resolveAt(t, root, rev, a); check.State != SymbolAnchorChanged {
			t.Fatalf("string literal edit: %+v", check)
		}
	})
	t.Run("DCP-V1-142 same-name symbol elsewhere is not identity", func(t *testing.T) {
		root, rev := symbolRepo(t, map[string]string{"app/models/order.rb": rubyOrder, "app/models/invoice.rb": "class Invoice\n  def total\n    0\n  end\nend\n"})
		a := pinnedSymbolAnchor(t, root, rev, "app/models/order.rb", 2, 5, "total")
		rev = commitFiles(t, root, map[string]string{"app/models/order.rb": "class Order\nend\n"})
		if check := resolveAt(t, root, rev, a); check.State != SymbolAnchorUnresolved || check.Live != nil {
			t.Fatalf("cross-path identity: %+v", check)
		}
	})
	t.Run("DCP-V1-142 unparsed same-language source withholds gone", func(t *testing.T) {
		root, rev := symbolRepo(t, map[string]string{"app/cart.ts": webCart})
		a := pinnedSymbolAnchor(t, root, rev, "app/cart.ts", 1, 4, "total")
		rev = commitFiles(t, root, map[string]string{"app/cart.ts": "export const other = 1;\n", "app/broken.ts": "export const x = `unterminated\n"})
		check := resolveAt(t, root, rev, a)
		if check.State != SymbolAnchorUnresolved || check.Live != nil {
			t.Fatalf("an inventory gap must withhold gone: %+v", check)
		}
	})
}

// TestSymbolAnchorReaderReceipt checks a live corpus reader reports the
// re-resolution of each cited symbol anchor beside the unchanged overall
// freshness (DCP-V1-141).
func TestSymbolAnchorReaderReceipt(t *testing.T) {
	t.Parallel()
	root, m := fixture(t)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	before, err := ReadQuery(context.Background(), root, data, Request{Operation: "search", Query: "showCount"})
	if err != nil {
		t.Fatal(err)
	}
	pinned := symbolCheck(before, "src/view.ts", "showCount")
	if pinned == nil || pinned.State != SymbolAnchorLive {
		t.Fatalf("unchanged reader receipt: %+v", before.SymbolAnchors)
	}
	original, err := os.ReadFile(filepath.Join(root, "src/view.ts"))
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, root, map[string]string{"src/view.ts": insertLines(string(original), 20, "const padding = 1;\n")})
	after, err := ReadQuery(context.Background(), root, data, Request{Operation: "search", Query: "showCount"})
	if err != nil {
		t.Fatal(err)
	}
	if after.Freshness != "stale" {
		t.Fatalf("changed input must keep the artifact stale: %s", after.Freshness)
	}
	moved := symbolCheck(after, "src/view.ts", "showCount")
	if moved == nil || moved.State != SymbolAnchorMoved || moved.Live.Start != pinned.Live.Start+20 {
		t.Fatalf("moved reader receipt: %+v", after.SymbolAnchors)
	}
	if plain, err := Query(a, Request{Operation: "search", Query: "showCount"}, "fresh", nil); err != nil || plain.SymbolAnchors != nil {
		t.Fatalf("a reader without a live tree must not report symbol anchors: %+v %v", plain.SymbolAnchors, err)
	}
}

func symbolCheck(r Receipt, path, symbol string) *SymbolAnchorCheck {
	for i, c := range r.SymbolAnchors {
		if strings.HasPrefix(c.Anchor, path+":") && strings.Contains(c.Anchor, "#"+symbol+"@") {
			return &r.SymbolAnchors[i]
		}
	}
	return nil
}

// TestDCPV1144SharedOverlayMatchesAFreshReader: a reader that answers many
// requests, concurrently, under one overlay returns the bytes a fresh reader
// returns, and a reused parsed file is charged to the request's budget as if
// it had been read.
func TestDCPV1144SharedOverlayMatchesAFreshReader(t *testing.T) {
	t.Parallel()
	root, m := fixture(t)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "src/view.ts"))
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, root, map[string]string{"src/view.ts": insertLines(string(original), 20, "const padding = 1;\n")})
	requests := []Request{{Operation: "search", Query: "showCount"}, {Operation: "search", Query: "count"}, {Operation: "locate", Path: "src/view.ts"}}
	answer := func(overlay FreshnessOverlay, request Request) string {
		t.Helper()
		r, err := QueryLive(context.Background(), a, request, overlay)
		if err != nil {
			t.Fatal(err)
		}
		data, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	shared, err := LiveFreshness(context.Background(), root, a)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	got := make([]string, len(requests)*4)
	for i := range got {
		wg.Go(func() {
			r, err := QueryLive(context.Background(), a, requests[i%len(requests)], shared)
			if err == nil {
				data, _ := Encode(r)
				got[i] = string(data)
			}
		})
	}
	wg.Wait()
	moved := false
	for i, request := range requests {
		fresh, err := LiveFreshness(context.Background(), root, a)
		if err != nil {
			t.Fatal(err)
		}
		want := answer(fresh, request)
		moved = moved || strings.Contains(want, `"state":"moved"`)
		for j := i; j < len(got); j += len(requests) {
			if got[j] != want {
				t.Fatalf("request %+v under a shared overlay differs from a fresh reader:\n%s\n%s", request, got[j], want)
			}
		}
	}
	if !moved || shared.symbols.files.bytes == 0 {
		t.Fatal("no request parsed a moved symbol; the shared files are not exercised")
	}
	// A kept file is served without Git and charged its recorded cost.
	for key, kept := range shared.symbols.files.files {
		oid, name, _ := strings.Cut(key, "\x00")
		r := NewSymbolResolver(nil, nil, Repository{}, nil)
		r.shared = shared.symbols.files
		if f, err := r.file(context.Background(), oid, name); err != nil || f.refusal != kept.file.refusal || r.read != kept.cost {
			t.Fatalf("kept file %s: read %d, want %d (%v)", name, r.read, kept.cost, err)
		}
	}
}
