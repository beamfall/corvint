package corpusbridge

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func lazyFixture(t *testing.T) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	for i := range 3 {
		path := filepath.Join(root, "docs", fmt.Sprintf("p%d.md", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, fmt.Appendf(nil, "# Page %d\n\nCheckout step %d.\n", i, i), 0o600); err != nil {
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
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, data
}

func TestDCPV1143LazyLoadAndMemo(t *testing.T) {
	root, data := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	if r.artifact != nil || len(r.Tools()) == 0 {
		t.Fatal("startup rederived the artifact or advertised no tools")
	}
	want, err := doccorpus.ReadQuery(context.Background(), root, data, doccorpus.Request{Operation: "locate", Path: "docs/p1.md", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	wantText, err := doccorpus.Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	for i, load := range []string{"first-call", "reused"} {
		_, text, failure, protocol, timing := r.CallTimed(context.Background(), "corvint.docs_locate", []byte(`{"path":"docs/p1.md"}`))
		if failure != nil || protocol != nil || timing.Load != load || text != string(wantText) {
			t.Fatalf("call %d: load=%q failure=%v protocol=%v parity=%v", i, timing.Load, failure, protocol, text == string(wantText))
		}
	}
	if len(want.Results) == 0 {
		t.Fatal("fixture locate found nothing; parity is vacuous")
	}
	// A pinned revision that no longer resolves drops the memo and rederives.
	r.pins = append(r.pins, strings.Repeat("0", 40))
	if _, _, failure, protocol, timing := r.CallTimed(context.Background(), "corvint.docs_locate", []byte(`{"path":"docs/p1.md"}`)); failure != nil || protocol != nil || timing.Load != "first-call" {
		t.Fatalf("unverified memo reused: load=%q failure=%v protocol=%v", timing.Load, failure, protocol)
	}
	// A changed artifact is refused, never answered from the memo.
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, failure, _, _ := r.CallTimed(context.Background(), "corvint.docs_locate", []byte(`{"path":"docs/p1.md"}`)); failure == nil || failure.Code != "corpus-refused" {
		t.Fatalf("changed artifact answered: %v", failure)
	}
}

func TestDCPV1143StartupRefusesUnreadableArtifact(t *testing.T) {
	root, _ := lazyFixture(t)
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, e := New(root, "corpus.json"); e == nil || e.Code != "corpus-unavailable" {
		t.Fatalf("unparseable artifact accepted at startup: %v", e)
	}
}

func TestDCPV1145PathsBatchEqualsUnion(t *testing.T) {
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	paths := []string{"docs/p2.md", "docs/p0.md", "docs/absent.md"}
	args, _ := json.Marshal(map[string]any{"paths": paths})
	_, text, failure, protocol, _ := r.CallTimed(context.Background(), "corvint.docs_locate", args)
	if failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	var batch struct {
		Schema   string `json:"schema"`
		Receipts []struct {
			Path    string         `json:"path"`
			Receipt jsontext.Value `json:"receipt"`
		} `json:"receipts"`
		Unresolved []map[string]string `json:"unresolved"`
	}
	if err := json.Unmarshal([]byte(text), &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Schema != batchSchema || len(batch.Receipts) != len(paths) || len(batch.Unresolved) != 0 {
		t.Fatalf("batch shape: %s", text)
	}
	for i, path := range paths {
		single, _ := json.Marshal(map[string]any{"path": path})
		_, want, failure, protocol := r.Call(context.Background(), "corvint.docs_locate", single)
		if failure != nil || protocol != nil || batch.Receipts[i].Path != path || string(batch.Receipts[i].Receipt)+"\n" != want {
			t.Fatalf("batch receipt %d differs from the single call for %s: %v %v\n%s\n%s", i, path, failure, protocol, batch.Receipts[i].Receipt, want)
		}
	}
	tooMany := make([]string, MaxBatchPaths+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("docs/p%d.md", i)
	}
	for _, bad := range []map[string]any{
		{"paths": []string{"docs/p0.md", "docs/p0.md"}},
		{"paths": []string{}},
		{"paths": tooMany},
		{"path": "docs/p0.md", "paths": []string{"docs/p1.md"}},
		{},
	} {
		args, _ := json.Marshal(bad)
		if _, _, _, protocol := r.Call(context.Background(), "corvint.docs_locate", args); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("invalid batch accepted: %s", args)
		}
	}
}

func TestDCPV1145BatchDeadlineNamesUnresolved(t *testing.T) {
	root, data := lazyFixture(t)
	a, err := doccorpus.Open(context.Background(), root, data)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := doccorpus.LiveFreshness(context.Background(), root, a)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := queryBatch(ctx, a, doccorpus.Request{Operation: "locate", Limit: 20}, overlay, []string{"docs/p0.md", "docs/p1.md"})
	if err != nil {
		t.Fatal(err)
	}
	var batch struct {
		Receipts   []any               `json:"receipts"`
		Unresolved []map[string]string `json:"unresolved"`
	}
	if err := json.Unmarshal(out, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Receipts) != 0 || len(batch.Unresolved) != 2 || batch.Unresolved[1]["path"] != "docs/p1.md" || batch.Unresolved[1]["code"] != "deadline-exceeded" {
		t.Fatalf("deadline batch: %s", out)
	}
}

func TestDCPV1144LiveFreshnessReusedUntilTheRepositoryChanges(t *testing.T) {
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	call := func() Timing {
		t.Helper()
		_, _, failure, protocol, timing := r.CallTimed(context.Background(), "corvint.docs_locate", []byte(`{"path":"docs/p1.md"}`))
		if failure != nil || protocol != nil {
			t.Fatal(failure, protocol)
		}
		return timing
	}
	for i, want := range []string{"computed", "reused"} {
		if got := call().Freshness; got != want {
			t.Fatalf("call %d: freshness=%q, want %q", i, got, want)
		}
	}
	// A worktree edit changes the observed status and forces a recompute.
	if err := os.WriteFile(filepath.Join(root, "docs", "p1.md"), []byte("# Page 1\n\nEdited.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := call().Freshness; got != "computed" {
		t.Fatalf("edited worktree reused the overlay: freshness=%q", got)
	}
}
