package golang_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

func TestImmutableReadScopeParity_AFPV0023(t *testing.T) {
	t.Run("DLT-V0-002 AFP-V0-023 immutable declared read scopes", func(t *testing.T) {
		const declaration = `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["docs/"]}}`
		for _, tc := range []struct {
			name, body               string
			symlink, refuse, invalid bool
		}{
			{name: "declared", body: declaration},
			{name: "missing"},
			{name: "exact one MiB", body: declaration + strings.Repeat(" ", (1<<20)-len(declaration))},
			{name: "oversized", body: declaration + strings.Repeat(" ", (1<<20)+1-len(declaration)), refuse: true},
			{name: "malformed", body: "{", invalid: true},
			{name: "unmatched directory", body: `{"profile":"corvint-test-read-scopes/0","packages":{"missing":[]}}`, invalid: true},
			{name: "nonregular hidden manifest", symlink: true, refuse: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := readScopeRepository(t, tc.body)
				manifest := filepath.Join(root, filepath.FromSlash(affected.ReadScopesPath))
				if tc.symlink {
					writeFiles(t, root, map[string]string{"real.json": declaration, ".corvint/other": ""})
					if err := os.Symlink("../real.json", manifest); err != nil {
						t.Fatal(err)
					}
				}
				live, err := affected.Build(root, golang.New())
				if err != nil {
					t.Fatal(err)
				}
				liveUnits, err := golang.New().Units(root)
				if err != nil {
					t.Fatal(err)
				}
				runGit(t, "git", root, "init", "--quiet")
				runGit(t, "git", root, "add", ".")
				runGit(t, "git", root, "commit", "--quiet", "-m", "declared source")
				auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
				if err != nil {
					t.Fatal(err)
				}
				defer auth.BeginObjectSession()()
				ctx := context.Background()
				pinned, err := auth.Resolve(ctx, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				source, err := auth.RevisionFS(ctx, pinned, affected.MaxSourceBytes)
				if err != nil {
					t.Fatal(err)
				}
				// Move both HEAD and ambient contents away from the explicitly read revision.
				if tc.symlink {
					if err := os.Remove(manifest); err != nil {
						t.Fatal(err)
					}
				}
				writeFiles(t, root, map[string]string{affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"caller":[]}}`})
				runGit(t, "git", root, "add", ".")
				runGit(t, "git", root, "commit", "--quiet", "-m", "conflicting ambient declaration")
				current, err := auth.Resolve(ctx, "HEAD")
				if err != nil || current == pinned {
					t.Fatalf("fixture did not move HEAD: %s %v", current, err)
				}
				immutable, err := affected.BuildFS(source, golang.New())
				if tc.refuse {
					if err == nil || immutable != nil {
						t.Fatalf("invalid immutable input admitted: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if live.Digest() != immutable.Digest() {
					t.Fatalf("pinned and live graph differ: %s / %s", live.Digest(), immutable.Digest())
				}
				immutableUnits, err := golang.New().UnitsSource(affected.FSSource(source))
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(liveUnits, immutableUnits) {
					t.Fatalf("pinned units differ from original live units")
				}
				if tc.invalid && !reflect.DeepEqual(immutableUnits.Frontier, []string{golang.FrontierReadScopesInvalid}) {
					t.Fatalf("invalid declaration frontier: %v", immutableUnits.Frontier)
				}
				for _, dirty := range []string{"docs/guide.md", "other/notes.md", "caller/caller.go", affected.ReadScopesPath} {
					a, b := affected.Select(live, []string{dirty}), affected.Select(immutable, []string{dirty})
					if !reflect.DeepEqual(a, b) {
						t.Fatalf("selection differs for %s", dirty)
					}
					if tc.invalid && b.Scope != affected.ScopeUnknown {
						t.Fatalf("invalid declaration narrowed scope: %+v", b)
					}
				}
				ambient, err := affected.Build(root, golang.New())
				if err != nil || ambient.Digest() == immutable.Digest() {
					t.Fatalf("ambient declaration did not differ: %v", err)
				}
			})
		}
	})
}

func TestImmutableReadScopeRefusals_AFPV0023(t *testing.T) {
	t.Run("DLT-V0-002 AFP-V0-023 immutable read failure refuses graph", func(t *testing.T) {
		root := readScopeRepository(t, "")
		for _, source := range []fs.FS{
			readScopeFailureFS{FS: os.DirFS(root)},
			os.DirFS(root), // Missing capability must not be misreported as missing manifest.
		} {
			graph, err := affected.BuildFS(source, golang.New())
			if err == nil || graph != nil {
				t.Fatalf("failed immutable reader admitted: %v", err)
			}
			if !errors.Is(err, fs.ErrPermission) && !errors.Is(err, affected.ErrWalkUnrepresentable) {
				t.Fatalf("unexpected refusal: %v", err)
			}
		}
	})
}

type readScopeFailureFS struct{ fs.FS }

func (readScopeFailureFS) OpenBounded(string, int) (fs.File, error) { return nil, fs.ErrPermission }

func readScopeRepository(t *testing.T, declaration string) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"caller/caller.go":      "package caller\n\nimport \"runtime\"\n\nfunc F() { runtime.Caller(0) }\n",
		"caller/caller_test.go": "package caller\n",
		"climb/climb.go":        "package climb\n\nvar data = \"../shared/data.json\"\n",
		"climb/climb_test.go":   "package climb\n",
		"plain/plain.go":        "package plain\n",
		"plain/plain_test.go":   "package plain\n",
	})
	if declaration != "" {
		writeFiles(t, root, map[string]string{affected.ReadScopesPath: declaration})
	}
	return root
}

// AFP-V0-023: a declared root-locating package leaves rule (d) and is selected
// only by its own subtree, a declared entry, an entry's ancestor, or the
// declaration itself; reaching it through the graph keeps the graph witness.
func TestDeclaredReadScopeNarrowsAnUnboundedReader_AFPV0023(t *testing.T) {
	root := readScopeRepository(t, `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["docs/","shared/data.json"]}}`)
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	graph, err := affected.Build(root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want, witness string }{
		{"other/notes.md", "[climb/climb_test.go]", ""},
		{"docs/guide/a.md", "[caller/caller_test.go climb/climb_test.go]", affected.WitnessDeclaredReadScope},
		{"docs", "[caller/caller_test.go climb/climb_test.go]", affected.WitnessDeclaredReadScope},
		{"shared", "[caller/caller_test.go climb/climb_test.go]", affected.WitnessDeclaredReadScope},
		{"shared/data.json", "[caller/caller_test.go climb/climb_test.go]", affected.WitnessDeclaredReadScope},
		{"shared/data.json.bak", "[climb/climb_test.go]", ""},
		{"docsx/a.md", "[climb/climb_test.go]", ""},
		{affected.ReadScopesPath, "[caller/caller_test.go climb/climb_test.go]", affected.WitnessDeclaredReadScope},
		{"caller/testdata/fixture.json", "[caller/caller_test.go climb/climb_test.go]", ""},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: selected tests = %s, want %s", tc.dirty, got, tc.want)
		}
		for _, selection := range plan.Selected {
			if tc.witness != "" && fmt.Sprint(selection.Tests) == "[caller/caller_test.go]" && selection.Witness.Kind != tc.witness {
				t.Errorf("%s: caller witness = %+v, want %s", tc.dirty, selection.Witness, tc.witness)
			}
		}
	}
	plan := affected.Select(graph, []string{"caller/caller.go"})
	for _, selection := range plan.Selected {
		if selection.Witness.Kind == affected.WitnessDeclaredReadScope {
			t.Errorf("a source edit selected %s by its declaration, not its graph edge", selection.UnitID)
		}
	}
	if empty := affected.Select(graph, nil); len(empty.Selected) != 0 {
		t.Errorf("empty dirty set selected %+v", empty.Selected)
	}
}

// AFP-V0-023: any defect in a present declaration declares nothing and raises
// the module-level frontier, so selection widens rather than narrows.
func TestInvalidReadScopeDeclarationDeclaresNothing_AFPV0023(t *testing.T) {
	for name, declaration := range map[string]string{
		"unknown package":  `{"profile":"corvint-test-read-scopes/0","packages":{"nowhere":[]}}`,
		"unknown member":   `{"profile":"corvint-test-read-scopes/0","packages":{"caller":[]},"x":1}`,
		"wrong profile":    `{"profile":"corvint-test-read-scopes/1","packages":{"caller":[]}}`,
		"null entries":     `{"profile":"corvint-test-read-scopes/0","packages":{"caller":null}}`,
		"unsorted entries": `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["z","a"]}}`,
		"git entry":        `{"profile":"corvint-test-read-scopes/0","packages":{"caller":[".git/"]}}`,
		"escaping entry":   `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["../x"]}}`,
		"malformed":        `{`,
	} {
		t.Run(name, func(t *testing.T) {
			root := readScopeRepository(t, declaration)
			result, err := golang.New().Units(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierReadScopesInvalid}) {
				t.Fatalf("frontier = %v, want %s", result.Frontier, golang.FrontierReadScopesInvalid)
			}
			for _, unit := range result.Units {
				if unit.ReadScoped || unit.ReadScope != nil {
					t.Fatalf("%s is declared from an invalid declaration", unit.ID)
				}
			}
		})
	}
	t.Run("symbolic link", func(t *testing.T) {
		root := readScopeRepository(t, "")
		target := filepath.Join(root, "real.json")
		writeFiles(t, root, map[string]string{"real.json": `{"profile":"corvint-test-read-scopes/0","packages":{"caller":[]}}`, ".corvint/other": ""})
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(affected.ReadScopesPath))); err != nil {
			t.Fatal(err)
		}
		result, err := golang.New().Units(root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierReadScopesInvalid}) {
			t.Fatalf("frontier = %v", result.Frontier)
		}
	})
}
