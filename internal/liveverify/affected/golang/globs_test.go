package golang_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-401: a Go test whose literal filepath.Glob matches an added file is
// selected with a witness naming the glob (the V1-1803 shape: a workflow added
// under .github/workflows); a non-literal glob records nothing, so the path
// stays an UNOWNED_DIRTY_PATH unknown. Since AFP-V0-2241 round 8 the lone
// ".github" and "workflows" of the V1-1803 Join name those directories at any
// depth themselves (the root is supplied at run time), so that unit is
// selected by its literal first; the glob witness is shown by the specs unit,
// whose climbing pattern names the path only as a glob.
func TestLiteralGlobSelectsTheReaderOfAnAddedFile_AFPV0401(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n",
		"cmd/tool/main.go":          "package main\n\nfunc main() {}\n",
		"cmd/tool/workflow_test.go": "package main\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestWorkflows(t *testing.T) {\n\troot := locate()\n\t_, _ = filepath.Glob(filepath.Join(root, \".github\", \"workflows\", \"*.y*ml\"))\n}\n\nfunc locate() string { return \"\" }\n",
		"specs/specs_test.go":       "package specs\n\nimport (\n\tfp \"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestIndex(t *testing.T) { _, _ = fp.Glob(\"../docs/sp*cs/*.md\") }\n",
		"binary/binary_test.go":     "package binary\n\nimport \"path/filepath\"\n\nvar matches, _ = filepath.Glob(\"testdata/\\xff*\")\n",
		"retired/retired_test.go":   "package retired\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestRetired(t *testing.T) {\n\tfor _, pattern := range []string{\"src/*.py\"} {\n\t\t_, _ = filepath.Glob(filepath.Join(t.TempDir(), pattern))\n\t}\n}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, unit, kind, glob string }{
		{".github/workflows/spec-toolchain-conformance.yml", "go:example.test/m/cmd/tool", affected.WitnessPathLiteralReader, ""},
		{"docs/specs/README.md", "go:example.test/m/specs", affected.WitnessPathGlobReader, "/docs/sp*cs/*.md"},
	}
	for _, c := range cases {
		plan := affected.Select(graph, []string{c.dirty})
		if len(plan.Selected) != 1 {
			t.Fatalf("%s selected %+v", c.dirty, plan.Selected)
		}
		want := affected.Witness{Kind: c.kind, DirtyPath: c.dirty, Via: []string{c.unit}, Glob: c.glob}
		if got := plan.Selected[0]; got.UnitID != c.unit || fmt.Sprint(got.Witness) != fmt.Sprint(want) {
			t.Fatalf("%s selected %s witness %+v, want %+v", c.dirty, got.UnitID, got.Witness, want)
		}
	}
	// A pattern that matches no component run, and a non-literal glob,
	// witness nothing as globs; the lone literals ".github" and "src/" still
	// name their directories.
	for _, dirty := range []string{".github/workflows/README.md", "src/retired.py"} {
		for _, selection := range affected.Select(graph, []string{dirty}).Selected {
			if selection.Witness.Kind == affected.WitnessPathGlobReader {
				t.Errorf("%s: %s selected by glob %+v", dirty, selection.UnitID, selection.Witness)
			}
		}
	}
	// An anchored glob matches from the repository root only.
	for _, dirty := range []string{"x/docs/specs/README.md"} {
		plan := affected.Select(graph, []string{dirty})
		if len(plan.Selected) != 0 {
			t.Fatalf("%s selected %+v", dirty, plan.Selected)
		}
		if !slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool {
			return unknown.Reason == affected.UnknownUnownedDirtyPath && unknown.Detail == dirty
		}) {
			t.Fatalf("%s unknown=%+v", dirty, plan.Unknown)
		}
	}
}

// AFP-V0-401: the call is found however it is spaced or commented, not only as
// the text `Glob(`. Each class has two members, so no lone literal token
// names the directory (a Go unit's lone token names any component since
// AFP-V0-2241 round 8) and only the glob witnesses it.
func TestLiteralGlobIsFoundThroughSpacingAndComments_AFPV0401(t *testing.T) {
	calls := map[string]string{
		"space":   "filepath.Glob (\"../d[o]cs/[bw]/*.md\")",
		"newline": "filepath.\n\t\tGlob(\n\t\t\t\"../d[o]cs/[cx]/*.md\")",
		"block":   "filepath.Glob/* pattern */(\"../d[o]cs/[dy]/*.md\")",
		"line":    "filepath.Glob( // pattern\n\t\t\"../d[o]cs/[ez]/*.md\")",
	}
	files := map[string]string{"go.mod": "module example.test/m\n"}
	for name, call := range calls {
		files[name+"/"+name+"_test.go"] = "package " + name + "\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestGlob(t *testing.T) {\n\t_, _ = " + call + "\n}\n"
	}
	root := t.TempDir()
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for name, directory := range map[string]string{"space": "w", "newline": "x", "block": "y", "line": "z"} {
		dirty := "docs/" + directory + "/README.md"
		plan := affected.Select(graph, []string{dirty})
		unit := "go:example.test/m/" + name
		if len(plan.Selected) != 1 || plan.Selected[0].UnitID != unit || plan.Selected[0].Witness.Kind != affected.WitnessPathGlobReader {
			t.Fatalf("%s: %s selected %+v, want %s by its glob", name, dirty, plan.Selected, unit)
		}
	}
}
