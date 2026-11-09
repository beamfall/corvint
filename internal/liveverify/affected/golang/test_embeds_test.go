// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-3350, AFP-V0-3351: an embedded package's source is test data even
// without an import edge. Its comments are observable bytes too.
func TestTestOnlyEmbedSelectsNonImportedSource_AFPV03350(t *testing.T) {
	for _, pkg := range []string{"cat", "cat_test"} {
		for _, edit := range []string{"package sub\nconst X = 2\n", "package sub\n// changed comment\nconst X = 1\n"} {
			t.Run(fmt.Sprintf("%s/%q", pkg, edit), func(t *testing.T) {
				root := t.TempDir()
				writeFiles(t, root, map[string]string{
					"go.mod":            "module example.test/m\n",
					"cat/cat.go":        "package cat\n",
					"cat/cat_test.go":   "package " + pkg + "\nimport _ \"embed\"\n//go:embed sub/x.go\nvar payload string\n",
					"cat/sub/x.go":      "package sub\nconst X = 1\n",
					"cat/sub/x_test.go": "package sub\n",
					"user/user.go":      "package user\nimport _ \"example.test/m/cat\"\n",
					"user/user_test.go": "package user\n",
				})
				writeFiles(t, root, map[string]string{"cat/sub/x.go": edit})
				graph, err := affected.Build(context.Background(), root, golang.New())
				if err != nil {
					t.Fatal(err)
				}
				plan := affected.Select(graph, []string{"cat/sub/x.go"})
				if got := plan.SelectedTests(); !slices.Contains(got, "cat/cat_test.go") {
					t.Fatalf("test embed owner missing: selected %v", got)
				} else if slices.Contains(got, "user/user_test.go") {
					t.Fatalf("test-only embed propagated to production importer: %v", got)
				}
				unit, _ := graph.Unit("go:example.test/m/cat")
				if unit.Embeds || len(unit.EmbedFiles) != 0 {
					t.Fatalf("test embed contaminated production facts: %+v", unit)
				}
				if !unit.TestEmbeds || !slices.Equal(unit.TestEmbedFiles, []string{"cat/sub/x.go"}) {
					t.Fatalf("test embed facts missing: %+v", unit)
				}
				readers := graph.Readers([]string{"cat/sub/x.go"})
				if _, ok := readers[unit.ID]; !ok {
					t.Fatalf("embedded bytes missing from reader guard: %v", readers)
				}
				if _, ok := readers["go:example.test/m/user"]; ok {
					t.Fatalf("test embed propagated to production reader: %v", readers)
				}
			})
		}
	}
}

// AFP-V0-3350: malformed directives retain the same uncertainty in either
// scope, and cannot silently discard a test's read of a non-imported package.
func TestTestOnlyEmbedMalformedFailsClosed_AFPV03350(t *testing.T) {
	for _, pattern := range []string{"\"open", "", "[bad", "../escape"} {
		for _, filename := range []string{"embed.go", "embed_test.go"} {
			t.Run(filename+"/"+pattern, func(t *testing.T) {
				root := t.TempDir()
				writeFiles(t, root, map[string]string{
					"go.mod":            "module example.test/m\n",
					"cat/cat.go":        "package cat\n",
					"cat/cat_test.go":   "package cat\n",
					"cat/" + filename:   "package cat\nimport _ \"embed\"\n//go:embed " + pattern + "\nvar payload string\n",
					"cat/sub/x.go":      "package sub\n",
					"cat/sub/x_test.go": "package sub\n",
				})
				result, err := golang.New().Units(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(result.Frontier, golang.FrontierUnparsedSource) {
					t.Fatalf("malformed embed lost frontier: %+v", result)
				}
				graph, err := affected.Build(context.Background(), root, golang.New())
				if err != nil {
					t.Fatal(err)
				}
				plan := affected.Select(graph, []string{"cat/sub/x.go"})
				if plan.Scope != affected.ScopeUnknown {
					t.Fatalf("malformed embed did not fail closed: %+v", plan)
				}
				if strings.HasSuffix(filename, "_test.go") && !slices.Contains(plan.SelectedTests(), "cat/cat_test.go") {
					t.Fatalf("malformed test embed lost enclosing reader: %+v", plan)
				}
			})
		}
	}
}
