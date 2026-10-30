package affected_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestCommittedRangePathsClassifyExactlyAsWorktreePaths_V1_2073 is the V1-2073
// decision (AFP-V0-010): a path committed in BASE..HEAD is classified exactly
// as the same change left uncommitted. A committed owned path is selected by
// its unit, never reported unowned; a committed build-driver input (`Makefile`,
// a `.github/workflows/` file) and a committed Corvint tool record no unit
// reads stay UNOWNED_DIRTY_PATH, because a change to them is no less unbounded
// for being committed (AFP-V0-601 rule (3)). The reason keeps its name; the
// receipt's range.paths member is what tells a committed contribution from a
// worktree one. A path that is really uncommitted is still captured by
// DirtyPaths and still widens the plan.
func TestCommittedRangePathsClassifyExactlyAsWorktreePaths_V1_2073(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(relative, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(argv ...string) string {
		t.Helper()
		command := exec.Command(gitExecutable, append([]string{"-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, argv...)...)
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Skipf("git %v is unavailable here: %v: %s", argv, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write("go.mod", "module example.test/ranged\n\ngo 1.27.0\n")
	write("core/core.go", "package core\n\nfunc Value() int { return 1 }\n")
	write("core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n")
	write("Makefile", "all:\n\ttrue\n")
	write(".github/workflows/ci.yml", "name: ci\n")
	write(".corvint/test-read-scope-literals.json", "{}\n")
	git("init", "--quiet")
	git("add", ".")
	git("commit", "--quiet", "-m", "base")
	base := git("rev-parse", "HEAD")

	// The V1-1894 closeout shape: the change is committed and the worktree is clean.
	write("core/core.go", "package core\n\nfunc Value() int { return 2 }\n")
	write("Makefile", "all:\n\tfalse\n")
	write(".github/workflows/ci.yml", "name: ci2\n")
	write(".corvint/test-read-scope-literals.json", "{\"profile\":\"x\"}\n")
	git("add", ".")
	git("commit", "--quiet", "-m", "change")

	ctx := context.Background()
	graph, err := affected.Build(ctx, root, golang.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) != 0 {
		t.Fatalf("clean worktree reported dirty paths %v", dirty)
	}
	committed, err := affected.RangePaths(ctx, gitExecutable, root, base)
	if err != nil {
		t.Fatal(err)
	}
	unowned := []string{".corvint/test-read-scope-literals.json", ".github/workflows/ci.yml", "Makefile"}
	if want := append(slices.Clone(unowned), "core/core.go"); !slices.Equal(affected.NormalizePaths(committed), affected.NormalizePaths(want)) {
		t.Fatalf("range paths = %v, want %v", committed, want)
	}
	fromRange := affected.Select(graph, affected.NormalizePaths(committed))
	assertClassification(t, "committed", fromRange, unowned)

	// The same change uncommitted: the worktree capture yields the same paths
	// and the plan is byte-identical, so the committed range carries no
	// classification of its own.
	git("reset", "--quiet", "--soft", base)
	staged, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(affected.NormalizePaths(staged), affected.NormalizePaths(committed)) {
		t.Fatalf("worktree paths %v differ from range paths %v for one change", staged, committed)
	}
	fromRangeJSON, err := fromRange.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	fromWorktreeJSON, err := affected.Select(graph, affected.NormalizePaths(staged)).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fromRangeJSON, fromWorktreeJSON) {
		t.Fatalf("committed and uncommitted plans differ:\nrange=   %s\nworktree=%s", fromRangeJSON, fromWorktreeJSON)
	}

	// Real dirty detection is unweakened: an uncommitted unowned path on top of
	// a committed range is captured, kept out of the range paths, and widens.
	git("commit", "--quiet", "-m", "change again")
	write("notes.txt", "uncommitted\n")
	dirty, err = affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dirty, []string{"notes.txt"}) {
		t.Fatalf("dirty = %v, want [notes.txt]", dirty)
	}
	committed, err = affected.RangePaths(ctx, gitExecutable, root, base)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(committed, "notes.txt") {
		t.Fatalf("range paths %v include the uncommitted notes.txt", committed)
	}
	union := affected.NormalizePaths(append(slices.Clone(dirty), committed...))
	assertClassification(t, "mixed", affected.Select(graph, union), append(slices.Clone(unowned), "notes.txt"))
}

// assertClassification checks the plan selects core by its direct source
// change and reports exactly the given paths, sorted, as UNOWNED_DIRTY_PATH.
func assertClassification(t *testing.T, label string, plan affected.Plan, unowned []string) {
	t.Helper()
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("%s: scope = %s, want UNKNOWN", label, plan.Scope)
	}
	var core *affected.Selection
	for i := range plan.Selected {
		if strings.HasSuffix(plan.Selected[i].UnitID, "/core") {
			core = &plan.Selected[i]
		}
	}
	if core == nil || core.Witness.Kind != affected.WitnessDirectSource || core.Witness.DirtyPath != "core/core.go" {
		t.Fatalf("%s: core not selected by its direct source change: %+v", label, plan.Selected)
	}
	var got []string
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnownedDirtyPath {
			got = append(got, unknown.Detail)
		}
	}
	want := affected.NormalizePaths(slices.Clone(unowned))
	if !slices.Equal(affected.NormalizePaths(got), want) {
		t.Fatalf("%s: UNOWNED_DIRTY_PATH = %v, want %v", label, got, want)
	}
	if slices.Contains(got, "core/core.go") {
		t.Fatalf("%s: owned committed path reported unowned", label)
	}
}
