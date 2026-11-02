package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// prechangeRepo is a one-commit repository with its evidence directory.
func prechangeRepo(t *testing.T) (root, base, baseTree, evidence string) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	baseTree = testGit(t, root, "rev-parse", "HEAD^{tree}")
	evidence = filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint")
	return root, base, baseTree, evidence
}

// prechangeReads answers context, query and impact with a receipt naming its
// tool and tree, records each command, and refuses every other command.
func prechangeReads(tree string, commands *[][]string) Runner {
	return Runner{Path: "/bin/sh", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		*commands = append(*commands, args)
		switch args[0] {
		case "context", "query", "impact":
			_, _ = io.WriteString(stdout, `{"tool":"`+args[0]+`","context":{"revision":"`+tree+`"}}`+"\n")
			return 0
		case "index":
			return 0
		}
		return 1
	}}
}

// DCW-V0-880 (V1-3183): dogfood prechange records the three receipts and a
// binding at BASE, refuses once the change began or over an agent's own
// receipt, and reruns over the receipts it wrote.
func TestDCWV0880PrechangeRecordsBoundReceipts(t *testing.T) {
	root, base, baseTree, evidence := prechangeRepo(t)
	var commands [][]string
	reads := prechangeReads(baseTree, &commands)
	run := func(paths ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code, err := Prechange(context.Background(), PrechangeOptions{Root: root, Base: base, Task: "Fix a", Paths: paths, Steps: reads}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, stdout.String(), stderr.String()
	}
	code, _, stderr := run("a.txt")
	if code != 0 {
		t.Fatalf("prechange = %d:\n%s", code, stderr)
	}
	for _, name := range []string{"prechange-context", "prechange-query", "prechange-impact"} {
		digest, err := fileSHA256(filepath.Join(evidence, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		want := "dogfood-prechange: RECORD " + name + " sha256=" + digest + " command=corvint " + strings.TrimPrefix(name, "prechange-")
		if !slices.Contains(strings.Split(stderr, "\n"), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	wantCommands := [][]string{{"index", "--if-stale"}, {"context", "--task", "Fix a", "--limit", "20"}, {"query", "--task", "Fix a", "--limit", "1"}, {"impact", "a.txt", "--limit", "10"}}
	if !slices.EqualFunc(commands, wantCommands, slices.Equal[[]string]) {
		t.Errorf("commands = %q; want %q", commands, wantCommands)
	}
	binding, err := readBinding(evidence)
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Dir(evidence)
	if binding.Base != base || binding.BaseTree != baseTree || binding.GitDir != gitDir || binding.Task != "Fix a" || len(binding.Slots) != 3 {
		t.Errorf("binding = %+v", binding)
	}
	if slot, _ := binding.slot("prechange-context"); slot.Role != "orientation" {
		t.Errorf("context slot role = %q; want orientation", slot.Role)
	}

	// A rerun over its own receipts is allowed; without PATH the impact slot
	// is NOT_PRODUCED and the run exits 1.
	if code, _, stderr = run(); code != 1 || !strings.Contains(stderr, "dogfood-prechange: NOT_PRODUCED prechange-impact prechange-impact-paths-absent\n") {
		t.Errorf("rerun without PATH = %d:\n%s", code, stderr)
	}
	if exists(filepath.Join(evidence, "prechange-impact.json")) {
		t.Error("an unproduced slot kept its earlier receipt")
	}

	// An agent's own receipt is never overwritten.
	agent := []byte(`{"tool":"impact","context":{"revision":"` + baseTree + `"}}` + "\n")
	if err := os.WriteFile(filepath.Join(evidence, "prechange-impact.json"), agent, 0o666); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr = run("a.txt"); code != 2 || !strings.Contains(stderr, "dogfood-prechange: REFUSE prechange-receipt-exists: ") {
		t.Errorf("over an agent receipt = %d:\n%s", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(evidence, "prechange-impact.json")); !bytes.Equal(got, agent) {
		t.Errorf("agent receipt rewritten: %q", got)
	}
	_ = os.Remove(filepath.Join(evidence, "prechange-impact.json"))

	// A dirty tracked file, then a commit past BASE, refuse and write nothing.
	before, _ := os.ReadFile(filepath.Join(evidence, prechangeBindingFile))
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("edited\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr = run("a.txt"); code != 2 || !strings.Contains(stderr, "REFUSE prechange-after-change: a tracked file differs from HEAD") {
		t.Errorf("dirty tree = %d:\n%s", code, stderr)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	if code, _, stderr = run("a.txt"); code != 2 || !strings.Contains(stderr, "REFUSE prechange-after-change: HEAD's tree is not BASE's tree") {
		t.Errorf("after a commit = %d:\n%s", code, stderr)
	}
	if after, _ := os.ReadFile(filepath.Join(evidence, prechangeBindingFile)); !bytes.Equal(before, after) {
		t.Error("a refused run rewrote the binding")
	}
}

// DCW-V0-881 (V1-3183): dogfood change notes receipts the recorder bound as
// OBSERVED, names each wrong-tool, edited, other-base, other-worktree or
// malformed binding case, and never changes its exit status or report.
func TestDCWV0881ChangeVerifiesPrechangeBinding(t *testing.T) {
	root, base, baseTree, evidence := prechangeRepo(t)
	var commands [][]string
	if code, err := Prechange(context.Background(), PrechangeOptions{Root: root, Base: base, Paths: []string{"a.txt"}, Steps: prechangeReads(baseTree, &commands)}, io.Discard, io.Discard); err != nil || code != 0 {
		t.Fatalf("prechange = %d, %v", code, err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	saved := map[string][]byte{}
	for _, name := range []string{"prechange-context.json", "prechange-query.json", "prechange-impact.json", prechangeBindingFile} {
		saved[name], _ = os.ReadFile(filepath.Join(evidence, name))
	}
	restore := func() {
		for name, data := range saved {
			if err := os.WriteFile(filepath.Join(evidence, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
	change := func() (int, string, []byte) {
		var stderr bytes.Buffer
		code, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		report, _ := os.ReadFile(filepath.Join(root, ".corvint", "dogfood-report.json"))
		return code, stderr.String(), report
	}
	editBinding := func(edit func(*prechangeBinding)) {
		binding, err := readBinding(evidence)
		if err != nil {
			t.Fatal(err)
		}
		edit(&binding)
		data, _ := json.Marshal(binding)
		if err := os.WriteFile(filepath.Join(evidence, prechangeBindingFile), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(name string) string {
		value, _ := fileSHA256(filepath.Join(evidence, name+".json"))
		return value
	}
	wantCode, _, wantReport := change()
	cases := []struct {
		name  string
		setup func()
		want  []string
	}{
		{"bound", func() {}, []string{
			"dogfood-change: NOTE prechange-query OBSERVED bound sha256=" + digest("prechange-query") + " command=corvint query",
			"dogfood-change: NOTE prechange-impact OBSERVED bound sha256=" + digest("prechange-impact") + " command=corvint impact",
		}},
		{"context as query", func() {
			if err := os.WriteFile(filepath.Join(evidence, "prechange-query.json"), saved["prechange-context.json"], 0o600); err != nil {
				t.Fatal(err)
			}
		}, []string{`dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-unsupported tool="context"`}},
		{"query absent beside context", func() { _ = os.Remove(filepath.Join(evidence, "prechange-query.json")) }, []string{
			"dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-absent",
			"dogfood-change: NOTE prechange-context orientation-only satisfies-no-slot",
		}},
		{"edited receipt", func() {
			if err := os.WriteFile(filepath.Join(evidence, "prechange-impact.json"), append(saved["prechange-impact.json"], ' '), 0o600); err != nil {
				t.Fatal(err)
			}
		}, []string{"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-binding-mismatch"}},
		{"other base", func() { editBinding(func(b *prechangeBinding) { b.Base = strings.Repeat("0", 40) }) },
			[]string{"dogfood-change: NOTE prechange-query STALE agent-receipt-binding-other-base base=" + strings.Repeat("0", 40),
				"  fix: BASE " + base + " does not descend from the receipts' base " + strings.Repeat("0", 40) + "; rerun with the BASE this lane started from, or after a mid-lane merge see docs/DOGFOOD.md, Mid-lane base change"}},
		{"other worktree", func() { editBinding(func(b *prechangeBinding) { b.GitDir = "/elsewhere/.git" }) },
			[]string{"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-wrong-worktree"}},
		{"malformed binding", func() {
			if err := os.WriteFile(filepath.Join(evidence, prechangeBindingFile), []byte("{\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, []string{"dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-binding-malformed"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore()
			tc.setup()
			code, stderr, report := change()
			lines := strings.Split(stderr, "\n")
			for _, want := range tc.want {
				if !slices.Contains(lines, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
			if slot := strings.Fields(tc.want[0])[2]; tc.name != "bound" && strings.Contains(stderr, "NOTE "+slot+" OBSERVED bound ") {
				t.Errorf("faulted %s still OBSERVED:\n%s", slot, stderr)
			}
			if code != wantCode || !bytes.Equal(report, wantReport) {
				t.Errorf("exit %d, report changed=%t; want exit %d and the same report", code, !bytes.Equal(report, wantReport), wantCode)
			}
		})
	}
}

// DCW-V0-925 (V1-3779): after a mid-lane merge moves BASE to a descendant of
// the base the pre-change receipts were bound to, dogfood change keeps each
// receipt's STALE not-base-tree note and names the recovery once: the receipts describe the pre-edit
// context at the old base and no rebind exists.
func TestDCWV0925MidLaneMergeNamesTheReceiptRecovery(t *testing.T) {
	root, base, baseTree, _ := prechangeRepo(t)
	var commands [][]string
	if code, err := Prechange(context.Background(), PrechangeOptions{Root: root, Base: base, Paths: []string{"a.txt"}, Steps: prechangeReads(baseTree, &commands)}, io.Discard, io.Discard); err != nil || code != 0 {
		t.Fatalf("prechange = %d, %v", code, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("mainline\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "mainline moved")
	moved := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: moved, Steps: steps}, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stderr.String()
	fix := "  fix: BASE " + moved + " descends from the receipts' base " + base + ", so the lane merged its mainline after its pre-change reads; keep BASE=" + moved + ": the receipts still describe the pre-edit context at " + base + ", no rebind exists, and this note changes no row or exit status (docs/DOGFOOD.md, Mid-lane base change)\n"
	if !strings.Contains(out, "NOTE prechange-query STALE agent-receipt-not-base-tree tree="+baseTree+" ") || strings.Count(out, fix) != 1 {
		t.Fatalf("mid-lane merge notes:\n%s", out)
	}
}
