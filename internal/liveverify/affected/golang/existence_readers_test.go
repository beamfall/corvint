package golang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const existenceDeclaration = `{"profile":"corvint-existence-readers/0","entries":[` +
	`{"package":"index","manifests":["specs/INDEX.json","docs/CAT.md"],"reason":"stats every implementation path and catalog link"}]}`

// existenceFixture writes a module whose index package stats the paths two
// manifests name: a JSON index naming repository paths and a Markdown catalog
// whose links resolve against docs/.
func existenceFixture(t *testing.T, declaration string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.test/m\n",
		"index/index.go":      "package index\n",
		"index/index_test.go": "package index\n\nimport \"testing\"\n\nfunc TestPaths(t *testing.T) {}\n",
		"other/other.go":      "package other\n",
		"other/other_test.go": "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n",
		"specs/INDEX.json":    `[{"path":"specs/a.md","implementation":["skills/x/SKILL.md","other/other.go"]}]`,
		"docs/CAT.md":         "| row | [guide](guides/g.md) and [up](../tools/t.md) |\n",
		"skills/y/SKILL.md":   "unnamed\n",
	}
	if declaration != "" {
		files[affected.ExistenceReadersPath] = declaration
	}
	writeFiles(t, root, files)
	return root
}

func selectedWitness(plan affected.Plan, unit string) (affected.Witness, bool) {
	for _, selection := range plan.Selected {
		if selection.UnitID == unit {
			return selection.Witness, true
		}
	}
	return affected.Witness{}, false
}

// AFP-V0-980 (V1-3772): a dirty path a declared manifest names and that is
// absent from the checkout (deleted or renamed away) selects the declared
// package with a DECLARED_EXISTENCE_READER witness; a JSON name and a Markdown
// link relative to its file both count. A named path still on disk, or an
// absent path no manifest names, does not select it. Before the change no rule
// selected the package for the deletion.
func TestDeletedManifestNameSelectsExistenceReader_AFPV0980(t *testing.T) {
	root := existenceFixture(t, existenceDeclaration)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const reader = "go:example.test/m/index"
	for _, deleted := range []string{"skills/x/SKILL.md", "docs/guides/g.md", "tools/t.md"} {
		plan := affected.Select(graph, []string{deleted})
		witness, ok := selectedWitness(plan, reader)
		if !ok || witness.Kind != affected.WitnessDeclaredExistenceReader || witness.DirtyPath != deleted {
			t.Fatalf("deleted %s: selected=%+v, want %s by %s", deleted, plan.Selected, reader, affected.WitnessDeclaredExistenceReader)
		}
		if _, other := selectedWitness(plan, "go:example.test/m/other"); other {
			t.Fatalf("deleted %s also selected other: %+v", deleted, plan.Selected)
		}
	}
	for _, path := range []string{"skills/y/SKILL.md", "skills/z/SKILL.md"} {
		if witness, ok := selectedWitness(affected.Select(graph, []string{path}), reader); ok {
			t.Fatalf("%s selected the reader by %+v", path, witness)
		}
	}
	// A path the manifest names that is only edited keeps existing.
	writeFiles(t, root, map[string]string{"skills/x/SKILL.md": "present\n"})
	if witness, ok := selectedWitness(affected.Select(graph, []string{"skills/x/SKILL.md"}), reader); ok {
		t.Fatalf("an edited named path selected the reader by %+v", witness)
	}
	// An edit of the declaration selects every declared reader.
	plan := affected.Select(graph, []string{affected.ExistenceReadersPath})
	if witness, ok := selectedWitness(plan, reader); !ok || witness.Kind != affected.WitnessDeclaredExistenceReader {
		t.Fatalf("dirty declaration selected=%+v", plan.Selected)
	}

	undeclared, err := affected.Build(context.Background(), existenceFixture(t, ""), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if witness, ok := selectedWitness(affected.Select(undeclared, []string{"skills/x/SKILL.md"}), reader); ok {
		t.Fatalf("without the declaration the deletion selected the reader by %+v", witness)
	}
	if undeclared.Digest() == graph.Digest() {
		t.Fatal("the declaration does not reach the graph digest")
	}
}

// AFP-V0-980: an invalid declaration, an unreadable manifest or a package the
// Go plugin does not observe fails the build, so the plan cannot narrow past
// a declaration it could not read.
func TestExistenceReadersDeclarationIsValidated_AFPV0980(t *testing.T) {
	for name, declaration := range map[string]string{
		"profile":           `{"profile":"x","entries":[]}`,
		"unknown member":    `{"profile":"corvint-existence-readers/0","entries":[],"extra":1}`,
		"no manifests":      `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":[],"reason":"r"}]}`,
		"missing manifest":  `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":["specs/none.json"],"reason":"r"}]}`,
		"climbing manifest": `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":["../x.json"],"reason":"r"}]}`,
		"empty reason":      `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":["specs/INDEX.json"],"reason":""}]}`,
		"declared names":    `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":["specs/INDEX.json"],"reason":"r","names":["a"]}]}`,
		"unordered": `{"profile":"corvint-existence-readers/0","entries":[{"package":"other","manifests":["specs/INDEX.json"],"reason":"r"},` +
			`{"package":"index","manifests":["specs/INDEX.json"],"reason":"r"}]}`,
		"unobserved package": `{"profile":"corvint-existence-readers/0","entries":[{"package":"gone","manifests":["specs/INDEX.json"],"reason":"r"}]}`,
	} {
		_, err := affected.Build(context.Background(), existenceFixture(t, declaration), golang.New())
		if err == nil || !strings.Contains(err.Error(), affected.ExistenceReadersPath) {
			t.Errorf("%s: err = %v, want a %s error", name, err, affected.ExistenceReadersPath)
		}
	}
}
