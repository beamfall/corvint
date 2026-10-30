package postmergeconnector

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func republishTestPlan(t *testing.T, docs string) RepublishPlan {
	t.Helper()
	p, e := BuildRepublishPlan(RepublishInput{Repository: strings.Repeat("1", 40), Target: "corpus", DocumentationRevision: strings.Repeat(docs, 40), SourceRevision: strings.Repeat("3", 40), PolicySHA256: strings.Repeat(docs, 64), ResultSHA256: strings.Repeat(docs, 64), CorpusSHA256: strings.Repeat("4", 64), IndexSHA256: strings.Repeat("5", 64), Added: 1, EvidenceURL: "https://example.invalid/evidence", URLOrigins: []string{"https://example.invalid"}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestRepublishLocalReplacement(t *testing.T) {
	t.Run("RCP-V0-009 one stable writer key across documentation revisions", func(t *testing.T) {
		a, b := republishTestPlan(t, "2"), republishTestPlan(t, "6")
		s, e := ApplyRepublishLocal(context.Background(), a, NewState())
		if e != nil {
			t.Fatal(e)
		}
		s, e = ApplyRepublishLocal(context.Background(), b, s)
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Forge) != 1 || a.Request.Key != b.Request.Key || s.Forge[b.Request.Key].Body != b.Request.Body {
			t.Fatal("pending draft stacked")
		}
		b.Request.Body = "edited"
		if _, e = ApplyRepublishLocal(context.Background(), b, s); e == nil {
			t.Fatal("edited body accepted")
		}
	})
}
func TestRepublishPending(t *testing.T) {
	t.Run("RCP-V0-009 RCP-V0-010 disk replacement exact retry stale and cancellation", func(t *testing.T) {
		root := t.TempDir()
		if raw, e := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); e != nil {
			t.Fatalf("git init %v %s", e, raw)
		}
		dir, _ := filepath.EvalSymlinks(t.TempDir())
		root, _ = filepath.EvalSymlinks(root)
		name := filepath.Join(dir, "pending.json")
		a, b := republishTestPlan(t, "2"), republishTestPlan(t, "6")
		state, noop, e := SaveRepublishPending(context.Background(), a, name, root, nil, []string{root}, 0, "NONE")
		if e != nil || noop || state.Generation != 1 {
			t.Fatal("initial", e)
		}
		prior, _ := os.ReadFile(name)
		_, noop, e = SaveRepublishPending(context.Background(), a, name, root, nil, []string{root}, 0, "NONE")
		if e != nil || !noop {
			t.Fatal("retry", e)
		}
		again, _ := os.ReadFile(name)
		if !bytes.Equal(prior, again) {
			t.Fatal("retry changed bytes")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, e = SaveRepublishPending(ctx, b, name, root, nil, []string{root}, 1, digestBytes(prior)); e == nil {
			t.Fatal("cancel accepted")
		}
		if _, _, e = SaveRepublishPending(context.Background(), b, name, root, nil, []string{root}, 0, "NONE"); e == nil {
			t.Fatal("stale replaced prior")
		}
		again, _ = os.ReadFile(name)
		if !bytes.Equal(prior, again) {
			t.Fatal("failed update changed state")
		}
		state, noop, e = SaveRepublishPending(context.Background(), b, name, root, nil, []string{root}, 1, digestBytes(prior))
		if e != nil || noop || state.Generation != 2 {
			t.Fatal("replace", e)
		}
		now, _ := os.ReadFile(name)
		var decoded RepublishPending
		if Decode(now, &decoded) != nil || decoded.Plan.Request.Key != a.Request.Key || decoded.Plan.Request.Body != b.Request.Body {
			t.Fatal("disk replacement invalid")
		}
		if _, _, e = SaveRepublishPending(context.Background(), a, name, root, nil, []string{root}, 0, "NONE"); e == nil {
			t.Fatal("old grant replaced newer")
		}
		again, _ = os.ReadFile(name)
		if !bytes.Equal(now, again) {
			t.Fatal("stale altered next")
		}
		inside := filepath.Join(root, "pending.json")
		if _, _, e = SaveRepublishPending(context.Background(), a, inside, root, nil, []string{root}, 0, "NONE"); e == nil {
			t.Fatal("author-root destination accepted")
		}
		link := filepath.Join(dir, "link.json")
		if e = os.Symlink(name, link); e != nil {
			t.Fatal(e)
		}
		if _, _, e = SaveRepublishPending(context.Background(), a, link, root, nil, []string{root}, 0, "NONE"); e == nil {
			t.Fatal("symlink destination accepted")
		}
		temp, _ := filepath.Glob(filepath.Join(dir, ".republish-*"))
		if len(temp) != 0 {
			t.Fatal("owned temps leaked")
		}
	})
}

func TestRepublishPendingRefusesEmptyExistingFile(t *testing.T) {
	t.Run("RCP-V0-009 RCP-V0-010 existing corrupt empty state cannot bootstrap an original first grant", func(t *testing.T) {
		root, _, _, _ := fixture(t)
		dir := tempDir(t)
		name := filepath.Join(dir, "pending.json")
		if err := os.WriteFile(name, nil, 0600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		state, noop, err := SaveRepublishPending(context.Background(), republishTestPlan(t, "2"), name, root, nil, []string{root}, 0, "NONE")
		if err == nil || noop || state.Generation != 0 {
			t.Errorf("corrupt existing empty state admitted: generation=%d noop=%t err=%v", state.Generation, noop, err)
		}
		raw, readErr := os.ReadFile(name)
		if readErr != nil || len(raw) != 0 {
			t.Errorf("corrupt prior bytes not preserved: len=%d err=%v", len(raw), readErr)
		}
		after, statErr := os.Stat(name)
		if statErr != nil || !os.SameFile(before, after) {
			t.Errorf("corrupt existing file replaced: %v", statErr)
		}
		temps, globErr := filepath.Glob(filepath.Join(dir, ".republish-*"))
		if globErr != nil || len(temps) != 0 {
			t.Errorf("temporary files leaked: %v %v", temps, globErr)
		}
	})
}
