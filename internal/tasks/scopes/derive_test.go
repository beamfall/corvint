package scopes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestCALV0022_PackScopeAndAbstention(t *testing.T) {
	t.Run("CAL-V0-022 pack scope and conservative defaults", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		git := func(args ...string) {
			t.Helper()
			c := exec.Command("git", args...)
			c.Dir = root
			c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
		}
		write := func(path, text string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
		}
		git("init", "-q", "-b", "main")
		git("config", "user.name", "test")
		git("config", "user.email", "test@example.test")
		write("go.mod", "module example.test/widget\n\ngo 1.27\n")
		write("widget.go", "package widget\nfunc HydrateWidget(value string) string { return value }\n")
		git("add", ".")
		git("commit", "-qm", "fixture")
		t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
		index, err := contextindex.BuildForSnapshot(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		title, body := "Repair HydrateWidget validation", "widget.go"
		if paths, _, ok := Derive(ctx, root, index.Revision, title, body); ok || paths != nil {
			t.Fatal("missing snapshot derived")
		}
		t.Setenv("CORVINT_SNAPSHOT_FORMAT", "")
		if _, err = contextindex.WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
		if _, _, ok := Derive(ctx, root, index.Revision, title, body); ok {
			t.Fatal("gob fallback derived without a pack")
		}
		receipt, err := contextindex.WriteSnapshot(index)
		if err != nil {
			t.Fatal(err)
		}
		pack, err := os.ReadFile(receipt.PackPath)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(receipt.PackPath, []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := Derive(ctx, root, index.Revision, title, body); ok {
			t.Fatal("refused pack fell back to gob")
		}
		if err = os.WriteFile(receipt.PackPath, pack, 0600); err != nil {
			t.Fatal(err)
		}
		paths, digest, ok := Derive(ctx, root, index.Revision, title, body)
		if !ok || len(paths) == 0 || len(digest) != 64 {
			t.Fatalf("derive: %v %q %v", paths, digest, ok)
		}
		found := false
		for _, p := range paths {
			if p == "widget.go" {
				found = true
			}
		}
		if !found {
			t.Fatalf("no source: %v", paths)
		}
		packet, err := contextindex.TaskContext(ctx, index, title+"\n"+body, "", 50)
		if err != nil {
			t.Fatal(err)
		}
		coverage := packet["coverage"].(map[string]any)
		for _, gap := range []struct {
			key   string
			value any
		}{
			{"omitted_results", 1}, {"critical_missing", []string{"missing"}},
			{"budget_shortage", "insufficient"}, {"governance_refused", []string{"unknown"}},
			{"answerability", map[string]any{"verdict": "unsupported-conjunction"}},
			{"unexamined", []any{map[string]any{"state": "examined", "withheld": 1}}},
		} {
			saved := coverage[gap.key]
			coverage[gap.key] = gap.value
			if p := packetPaths(packet, index); p != nil {
				t.Fatalf("%s narrowed scope: %v", gap.key, p)
			}
			coverage[gap.key] = saved
		}
		_, again, ok := Derive(ctx, root, index.Revision, title, body)
		if !ok || again != digest {
			t.Fatal("same inputs changed digest")
		}
		if _, _, ok := Derive(ctx, root, strings.Repeat("a", 40), title, body); ok {
			t.Fatal("wrong base derived")
		}
		if _, _, ok := Derive(ctx, root, index.Revision, "unrelated QuuxMissingSymbol", ""); ok {
			t.Fatal("unsupported context derived")
		}
		t.Setenv("CORVINT_SNAPSHOT_FORMAT", "")
		if _, _, ok := Derive(ctx, root, index.Revision, title, body); ok {
			t.Fatal("default enabled pack")
		}
		t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
		write("widget.go", "package widget\n")
		if _, _, ok := Derive(ctx, root, index.Revision, title, body); ok {
			t.Fatal("dirty source derived")
		}
		git("add", "widget.go")
		git("commit", "-qm", "change")
		newIndex, err := contextindex.BuildForSnapshot(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, ok := Derive(ctx, root, newIndex.Revision, title, body); ok {
			t.Fatal("stale snapshot derived")
		}
	})
}
