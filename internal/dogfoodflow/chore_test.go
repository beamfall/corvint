package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// DCW-V0-048, DCW-V0-049: a change is a chore only when every path it touches
// is the bind-loop map or a sealed map one segment below .corvint/changes/.
func TestChoreRoleDCWV0048ClassifiesOnlyCEMPaths(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		{nil, ""},
		{[]string{".corvint/change.cem.json"}, wire.StableRoleBindChore},
		{[]string{".corvint/changes/0123.cem.json"}, wire.StableRoleSealChore},
		{[]string{".corvint/change.cem.json", ".corvint/changes/0123.cem.json"}, wire.StableRoleSealChore},
		{[]string{".corvint/change.cem.json", "a.go"}, ""},
		{[]string{".corvint/test-read-scopes.json"}, ""},
		{[]string{".corvint/changes/.cem.json"}, ""},
		{[]string{".corvint/changes/x/y.cem.json"}, ""},
		{[]string{".corvint/changes/x.cem.json.bak"}, ""},
		{[]string{`".corvint/changes/\303\251.cem.json"`}, ""},
	} {
		if got := ChoreRole(tc.paths); got != tc.want {
			t.Errorf("ChoreRole(%q) = %q; want %q", tc.paths, got, tc.want)
		}
	}
}

// DCW-V0-048: a plan that cites one span (path and START:END) for every hunk
// of a non-chore change with more than one hunk is refused before any cite
// runs, and its fix line names the hunk count and the span. Distinct spans, a
// kept-unknown hunk, a single-hunk map and a chore change are admitted.
func TestChangeDCWV0048RefusesUniformPlan(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("base\nline\n"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", "a.txt", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\nline\nmore\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	change := testGit(t, root, "rev-parse", "HEAD")
	cemPrepared := ""
	cites := 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(cemPrepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			cites++
			return 0
		}
		return 1
	}}
	cemMap := func(ids ...string) string {
		hunks := []string{}
		for _, id := range ids {
			hunks = append(hunks, `{"disposition": "unknown", "id": "`+id+`", "path": "a.txt", "reason": "no-evidence"}`)
		}
		return `{"hunks": [` + strings.Join(hunks, ", ") + "]}\n"
	}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	run := func(base, rows string, ids ...string) (string, string) {
		t.Helper()
		cemPrepared, cites = cemMap(ids...), 0
		if err := os.WriteFile(plan, []byte(rows), 0o666); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}, &stderr); err != nil {
			t.Fatal(err)
		}
		report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(report), "\n") {
			if strings.Contains(line, `"name": "cem-cite"`) {
				return strings.TrimSpace(line), stderr.String()
			}
		}
		t.Fatalf("report has no cem-cite row:\n%s", report)
		return "", ""
	}
	uniform := "1\ta.txt\t1:2\timplementation\n2\ta.txt\t1:2\tspecification\n3\ta.txt\t1:2\ttest-claim\n"
	got, stderr := run(base, uniform, "hunk:a", "hunk:b", "hunk:c")
	if !strings.Contains(got, `"reason": "citation-plan-uniform"`) || cites != 0 {
		t.Fatalf("uniform plan: cem-cite = %s after %d cites; want citation-plan-uniform before any cite", got, cites)
	}
	if !strings.Contains(stderr, "fix: the plan cites one span for every hunk: hunks=3 span=a.txt:1:2;") {
		t.Errorf("uniform plan: fix line does not name the hunk count and span:\n%s", stderr)
	}
	// DCW-V0-150, V1-2214: the fix shows a per-hunk example and names the draft helper.
	if !strings.Contains(stderr, "for example 1<TAB>docs/specs/x-v0.md<TAB>40:44<TAB>specification for a spec hunk and 2<TAB>internal/x/x.go<TAB>120:126<TAB>dependency") ||
		!strings.Contains(stderr, "corvint dogfood propose-citations "+base+" prints a per-hunk draft") {
		t.Errorf("uniform plan: fix line does not show a per-hunk example and the draft helper:\n%s", stderr)
	}
	spelled := "1\ta.txt\t1:2\timplementation\n2\ta.txt\t01:02\timplementation\n3\ta.txt\t+1:2\timplementation\n"
	if got, _ := run(base, spelled, "hunk:a", "hunk:b", "hunk:c"); !strings.Contains(got, `"reason": "citation-plan-uniform"`) {
		t.Errorf("uniform plan with respelled spans: cem-cite = %s; want citation-plan-uniform", got)
	}
	byID := "hunk:a\ta.txt\t1:2\timplementation\nhunk:b\ta.txt\t1:2\timplementation\n"
	if got, _ := run(base, byID, "hunk:a", "hunk:b"); !strings.Contains(got, `"reason": "citation-plan-uniform"`) {
		t.Errorf("uniform plan by full hunk ID: cem-cite = %s; want citation-plan-uniform", got)
	}
	for _, tc := range []struct {
		name, rows string
		ids        []string
	}{
		{"distinct spans", "1\ta.txt\t1:2\timplementation\n2\tb.txt\t1:2\timplementation\n", []string{"hunk:a", "hunk:b"}},
		{"same path, another span", "1\ta.txt\t1:1\timplementation\n2\ta.txt\t1:2\timplementation\n", []string{"hunk:a", "hunk:b"}},
		{"a hunk kept unnamed", "1\ta.txt\t1:2\timplementation\n2\ta.txt\t1:2\timplementation\n", []string{"hunk:a", "hunk:b", "hunk:c"}},
		{"single hunk", "1\ta.txt\t1:2\timplementation\n", []string{"hunk:a"}},
	} {
		if got, _ := run(base, tc.rows, tc.ids...); strings.Contains(got, "citation-plan-uniform") {
			t.Errorf("%s: cem-cite = %s; want no citation-plan-uniform", tc.name, got)
		}
	}
	// A bind-chore change, BASE..HEAD touching only .corvint/change.cem.json,
	// is exempt: its map's hunks are evidence records, not code.
	if err := os.Remove(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".corvint/change.cem.json")
	testGit(t, root, "commit", "-q", "-m", "chore: bind change evidence")
	if got, _ := run(change, "1\ta.txt\t1:2\timplementation\n2\ta.txt\t1:2\timplementation\n", "hunk:a", "hunk:b"); strings.Contains(got, "citation-plan-uniform") {
		t.Errorf("bind-chore change: cem-cite = %s; want no citation-plan-uniform", got)
	}
}

// DCW-V0-049, CEM-V1-095: the producer writes a chore role into a cem/1.0 map
// through the stable encoder, so the map parses with that effective role; a
// second write is byte-identical and a Core 0.x map is never given a role.
func TestMarkChoreRoleDCWV0049WritesStableRoleOnly(t *testing.T) {
	fixture, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	if document, err := wire.ParseStable(fixture); err != nil || document.EffectiveRole() != wire.StableRoleChange {
		t.Fatalf("fixture role: %v", err)
	}
	dir := t.TempDir()
	for _, role := range []string{wire.StableRoleBindChore, wire.StableRoleSealChore} {
		mapPath := filepath.Join(dir, role+".cem.json")
		if err := os.WriteFile(mapPath, fixture, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := markChoreRole(mapPath, role); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		marked, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		document, err := wire.ParseStable(marked)
		if err != nil || document.EffectiveRole() != role {
			t.Fatalf("%s: marked map role=%v err=%v", role, document, err)
		}
		if info, err := os.Stat(mapPath); err != nil || info.Mode().Perm() != 0o644 {
			t.Errorf("%s: marked map mode changed: %v", role, err)
		}
		if err := markChoreRole(mapPath, role); err != nil {
			t.Fatal(err)
		}
		if again, _ := os.ReadFile(mapPath); !bytes.Equal(again, marked) {
			t.Errorf("%s: a second write changed the map", role)
		}
	}
	core := filepath.Join(dir, "core.cem.json")
	coreMap := []byte(`{"spec": "cem/0.2", "hunks": []}` + "\n")
	if err := os.WriteFile(core, coreMap, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := markChoreRole(core, wire.StableRoleBindChore); err == nil {
		t.Fatal("a Core 0.x map was given a role")
	}
	if kept, _ := os.ReadFile(core); !bytes.Equal(kept, coreMap) {
		t.Errorf("a refused role write changed the Core map: %s", kept)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Errorf("role writes left temporary files: %d entries", len(entries))
	}
}

// DCW-V0-049: once cem prepare writes cem/1.0 (V1-1276), the bind loop marks
// the prepared map of a bind-chore range and leaves a non-chore map's role
// absent; the stable map then continues through the loop and is never
// refused for citation (CEM-V1-147).
func TestChangeDCWV0049MarksPreparedStableChoreMap(t *testing.T) {
	fixture, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	change := testGit(t, root, "rev-parse", "HEAD")
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		if len(args) > 1 && args[0] == "cem" && args[1] == "prepare" {
			if os.MkdirAll(filepath.Join(root, ".corvint"), 0o777) != nil || os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), fixture, 0o666) != nil {
				return 1
			}
			return 0
		}
		return 1
	}}
	prepared := func(from string) *wire.StableMap {
		t.Helper()
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: from, Steps: steps}, &stderr); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json"))
		if err != nil {
			t.Fatal(err)
		}
		document, err := wire.ParseStable(data)
		if err != nil {
			t.Fatalf("prepared map does not parse: %v", err)
		}
		report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		if len(report) == 0 || bytes.Contains(report, []byte("stable-unsupported")) {
			t.Errorf("stable map was refused by the bind loop:\n%s", report)
		}
		return document
	}
	if role := prepared(base).Role; role != "" {
		t.Errorf("non-chore range: role = %q; want absent", role)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".corvint/change.cem.json")
	testGit(t, root, "commit", "-q", "-m", "chore: bind change evidence")
	if role := prepared(change).EffectiveRole(); role != wire.StableRoleBindChore {
		t.Errorf("bind-chore range: role = %q; want %q", role, wire.StableRoleBindChore)
	}
}
