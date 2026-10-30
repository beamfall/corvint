package lspevidence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// V1-2536, TCP-V0-054: the task attachment must bound gopls path relations
// subject-first. Lexically, the ranked seed's a->b and b->c rows sort before
// every z/z.go row, so a lexical cut at two drops the subject's definition
// and caller, as the pre-TCP-V0-054 projection dropped the
// go-attach-expand gold. The cut rows stay counted as omitted.
func TestAttachLiveGoplsKeepsSubjectRelationsAtBound(t *testing.T) {
	executable, err := exec.LookPath("gopls")
	if err != nil || !filepath.IsAbs(executable) {
		t.Skip("gopls not installed")
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/m\n\ngo 1.22\n",
		"a/a.go": "package a\n\nimport \"example.test/m/b\"\n\nfunc A() int { return b.B() }\n",
		"b/b.go": "package b\n\nimport \"example.test/m/c\"\n\nfunc B() int { return c.C() }\n",
		"c/c.go": "package c\n\nfunc C() int { return 1 }\n",
		"z/z.go": "package z\n\nimport \"example.test/m/y\"\n\nfunc Z() int { return y.Y() }\n",
		"y/y.go": "package y\n\nfunc Y() int { return 2 }\n",
		"w/w.go": "package w\n\nimport \"example.test/m/z\"\n\nfunc W() int { return z.Z() }\n",
	}
	for path, text := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "first"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	ctx := context.Background()
	index, err := contextindex.Build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{"results": []any{map[string]any{"id": "a/a.go"}}}
	if err := Attach(ctx, index, "z/z.go", packet, 2, executable); err != nil {
		t.Fatal(err)
	}
	section := packet["external"].(map[string]any)
	relations, _ := section["path_relations"].([]any)
	edges := []string{}
	for _, raw := range relations {
		relation := raw.(map[string]any)["relation"].(map[string]any)
		edges = append(edges, relation["from"].(map[string]any)["path"].(string)+">"+relation["to"].(map[string]any)["path"].(string)+" "+relation["type"].(string))
	}
	got := strings.Join(edges, ", ")
	for _, want := range []string{"z/z.go>y/y.go gopls:uses-definition", "z/z.go>w/w.go gopls:referenced-by"} {
		if !strings.Contains(got, want) {
			t.Errorf("bounded relations %q lack subject relation %q", got, want)
		}
	}
	if omitted, _ := section["omitted"].(map[string]any)["path_relations"].(int); omitted < 2 {
		t.Errorf("omitted path relations = %v, want the cut seed rows counted (section %v)", section["omitted"], section)
	}
}
