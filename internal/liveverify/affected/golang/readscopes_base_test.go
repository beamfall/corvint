package golang_test

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const readScopeBaseDeclaration = `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["docs/"],"changes":["a/"],"gone":["g/"],"kept":["k/"],"reader":[".corvint/"]}}`

// readScopeBaseRepository commits a module whose declaration names a
// root-locating package, three plain packages and a package that reads the
// declaration's own directory, plus any extra files, and returns the root and
// that commit.
func readScopeBaseRepository(t *testing.T, declaration string, extra ...map[string]string) (string, string) {
	t.Helper()
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := readScopeRepository(t, declaration)
	files := map[string]string{}
	for _, name := range []string{"changes", "gone", "kept", "reader"} {
		files[name+"/"+name+".go"] = "package " + name + "\n"
		files[name+"/"+name+"_test.go"] = "package " + name + "\n"
	}
	writeFiles(t, root, files)
	for _, more := range extra {
		writeFiles(t, root, more)
	}
	runGit(t, gitExecutable, root, "init", "-q")
	runGit(t, gitExecutable, root, "add", "-A")
	runGit(t, gitExecutable, root, "commit", "-q", "-m", "base")
	head, err := exec.Command(gitExecutable, "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(head))
}

// declarationSelections returns, sorted, each unit the plan selects with a
// DECLARED_READ_SCOPE witness, and every selected unit.
func declarationSelections(plan affected.Plan) (declared, selected []string) {
	for _, selection := range plan.Selected {
		selected = append(selected, selection.UnitID)
		if selection.Witness.Kind == affected.WitnessDeclaredReadScope {
			declared = append(declared, selection.UnitID)
		}
	}
	slices.Sort(declared)
	slices.Sort(selected)
	return declared, selected
}

// AFP-V0-050: measured against the committed declaration, an edit that adds
// keys and a new package selects the new package, each unit whose own
// declaration was added, changed or removed, and each unit whose declared
// scope holds the declaration path; an unchanged declaration no longer selects
// its unit. With the base unknown the AFP-V0-023 rule still selects every
// declared unit.
func TestReadScopeEditSelectsOnlyChangedDeclarations_AFPV0050(t *testing.T) {
	root, base := readScopeBaseRepository(t, readScopeBaseDeclaration)
	writeFiles(t, root, map[string]string{
		"fresh/fresh.go":        "package fresh\n",
		"fresh/fresh_test.go":   "package fresh\n",
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"caller":["docs/"],"changes":["b/"],"fresh":["f/"],"kept":["k/"],"plain":["p/"],"reader":[".corvint/"]}}`,
	})
	ctx := context.Background()
	graph, err := affected.Build(ctx, root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	prior, err := golang.CommittedReadScopes(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"caller", "changes", "gone", "kept", "reader"}; !slices.Equal(slices.Sorted(maps.Keys(prior)), want) {
		t.Fatalf("prior declaration = %v, want keys %v", prior, want)
	}
	dirty := []string{affected.ReadScopesPath, "fresh/fresh.go", "fresh/fresh_test.go"}
	declared, selected := declarationSelections(affected.Select(graph.WithReadScopeBase(prior), dirty))
	if want := []string{"go:example.test/m/changes", "go:example.test/m/gone", "go:example.test/m/plain", "go:example.test/m/reader"}; !slices.Equal(declared, want) {
		t.Errorf("declaration selections = %v, want %v", declared, want)
	}
	if want := []string{"go:example.test/m/changes", "go:example.test/m/climb", "go:example.test/m/fresh", "go:example.test/m/gone", "go:example.test/m/plain", "go:example.test/m/reader"}; !slices.Equal(selected, want) {
		t.Errorf("selected = %v, want %v", selected, want)
	}
	unknownBase, _ := declarationSelections(affected.Select(graph.WithReadScopeBase(nil), dirty))
	if want := []string{"go:example.test/m/caller", "go:example.test/m/changes", "go:example.test/m/kept", "go:example.test/m/plain", "go:example.test/m/reader"}; !slices.Equal(unknownBase, want) {
		t.Errorf("unknown base declaration selections = %v, want %v", unknownBase, want)
	}
	if plain, _ := declarationSelections(affected.Select(graph, dirty)); !slices.Equal(plain, unknownBase) {
		t.Errorf("a graph without a base selected %v, want the AFP-V0-023 set %v", plain, unknownBase)
	}
	// A declaration path that is not dirty is not compared at all.
	if declared, _ := declarationSelections(affected.Select(graph.WithReadScopeBase(prior), []string{"fresh/fresh.go"})); len(declared) != 0 {
		t.Errorf("a source edit selected %v by declaration", declared)
	}
}

// AFP-V0-050: a base that holds no declaration is empty, so every declared
// unit is an added key; a base whose declaration is invalid, including one
// naming a directory that holds no package at the base, is unknown.
func TestCommittedReadScopesFailClosed_AFPV0050(t *testing.T) {
	ctx := context.Background()
	root, base := readScopeBaseRepository(t, "")
	prior, err := golang.CommittedReadScopes(ctx, root, base)
	if err != nil || prior == nil || len(prior) != 0 {
		t.Fatalf("absent declaration = %v, %v; want an empty map", prior, err)
	}
	writeFiles(t, root, map[string]string{affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":["k/"]}}`})
	graph, err := affected.Build(ctx, root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	declared, _ := declarationSelections(affected.Select(graph.WithReadScopeBase(prior), []string{affected.ReadScopesPath}))
	if want := []string{"go:example.test/m/kept"}; !slices.Equal(declared, want) {
		t.Errorf("added declaration selections = %v, want %v", declared, want)
	}
	for name, body := range map[string]string{
		"malformed":     "{",
		"wrong profile": `{"profile":"corvint-test-read-scopes/1","packages":{}}`,
		"no package":    `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[],"missing":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root, base := readScopeBaseRepository(t, body)
			if prior, err := golang.CommittedReadScopes(ctx, root, base); err == nil || prior != nil {
				t.Fatalf("invalid base declaration = %v, %v; want unknown", prior, err)
			}
		})
	}
	if prior, err := golang.CommittedReadScopes(ctx, root, strings.Repeat("0", 40)); err == nil || prior != nil {
		t.Fatalf("missing commit = %v, %v; want unknown", prior, err)
	}
}

// AFP-V0-050 (V1-1400): a package of a nested module is not a unit of the
// observed module, so a base declaring a directory a nested go.mod owns is
// unknown, as the observed tree's declaration would be invalid. A go.work that
// lists the nested module makes its packages observed, and a go.mod below a
// listed module's testdata is fixture data that owns nothing.
func TestCommittedReadScopesFollowModuleOwnership_AFPV0050(t *testing.T) {
	ctx := context.Background()
	nested := map[string]string{
		"nested/go.mod":        "module example.test/nested\n",
		"nested/x/x.go":        "package x\n",
		"nested/x/x_test.go":   "package x\n",
		"inner/go.mod":         "module example.test/inner\n",
		"inner/inner.go":       "package inner\n",
		"kept/testdata/go.mod": "module example.test/fixture\n",
		"kept/testdata/f/f.go": "package f\n",
	}
	for name, test := range map[string]struct {
		declaration string
		workspace   string
		extra       map[string]string
		known       bool
	}{
		"below a nested module":           {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[],"nested/x":[]}}`},
		"a nested module's root":          {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"inner":[],"kept":[]}}`},
		"in a fixture module":             {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept/testdata/f":[]}}`},
		"a workspace-listed module":       {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[],"nested/x":[]}}`, workspace: "go 1.27\n\nuse (\n\t.\n\t./nested\n)\n", known: true},
		"the root module beside them":     {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[]}}`, known: true},
		"an unlisted root in a workspace": {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[]}}`, workspace: "go 1.27\n\nuse ./nested\n"},
		"a directory named go.mod":        {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"kept":[]}}`, extra: map[string]string{"kept/go.mod/data": "x\n"}, known: true},
		"in a vendor directory":           {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"vendor/v":[]}}`, extra: map[string]string{"vendor/v/v.go": "package v\n"}},
		"in a hidden directory":           {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{".hidden/h":[]}}`, extra: map[string]string{".hidden/h/h.go": "package h\n"}},
		"in an admitted build directory":  {declaration: `{"profile":"corvint-test-read-scopes/0","packages":{"build/b":[]}}`, extra: map[string]string{"build/b/b.go": "package b\n"}, known: true},
	} {
		t.Run(name, func(t *testing.T) {
			files := maps.Clone(nested)
			if test.workspace != "" {
				files["go.work"] = test.workspace
			}
			maps.Copy(files, test.extra)
			root, base := readScopeBaseRepository(t, test.declaration, files)
			prior, err := golang.CommittedReadScopes(ctx, root, base)
			if test.known && (err != nil || prior == nil) {
				t.Fatalf("base declaration = %v, %v; want it read", prior, err)
			}
			if !test.known && (err == nil || prior != nil) {
				t.Fatalf("base declaration = %v, %v; want unknown", prior, err)
			}
			// The same tree observed on disk agrees on whether it declares anything.
			observed, err := golang.New().Units(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if invalid := slices.Contains(observed.Frontier, golang.FrontierReadScopesInvalid); invalid == test.known {
				t.Errorf("observed frontier = %v; base read = %v", observed.Frontier, test.known)
			}
		})
	}
}

// V1-1417 (AFP-V0-050): checking the committed declaration costs Git
// operations per declared directory, so a fixed operation count exhausts on a
// large declaration (the frozen 1,024 default did at 1,100 directories) and
// every base turns unknown. With the base count lowered to a small fixture's
// fixed cost, the scaled budget reads 48 flat and 16 depth-three directories
// back intact; with no per-directory allowance the exhaustion surfaces as
// git-budget-exceeded, never as a declaration naming no Go package.
func TestCommittedReadScopesBudgetScalesWithDeclaration_V11417(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	files := map[string]string{"go.mod": "module example.test/m\n"}
	packages := map[string][]string{}
	for index := range 48 {
		packages[fmt.Sprintf("d%02d", index)] = []string{"data/"}
	}
	for index := range 16 {
		packages[fmt.Sprintf("a%02d/b/c", index)] = []string{"data/"}
	}
	for directory := range packages {
		files[directory+"/x.go"] = "package x\n"
	}
	declaration, err := json.Marshal(map[string]any{"profile": golang.ReadScopesProfile, "packages": packages})
	if err != nil {
		t.Fatal(err)
	}
	files[affected.ReadScopesPath] = string(declaration)
	writeFiles(t, root, files)
	runGit(t, gitExecutable, root, "init", "-q")
	runGit(t, gitExecutable, root, "add", "-A")
	runGit(t, gitExecutable, root, "commit", "-q", "-m", "base")
	head, err := exec.Command(gitExecutable, "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(head))
	// 16 operations cover the commit, the root, .corvint and declaration
	// reads and go.mod, but not one listing per declared directory.
	golang.SetReadScopeOperations(t, 16, 2)
	prior, err := golang.CommittedReadScopes(context.Background(), root, commit)
	if err != nil || len(prior) != len(packages) {
		t.Fatalf("scaled budget: CommittedReadScopes = %d packages, %v; want %d", len(prior), err, len(packages))
	}
	golang.SetReadScopeOperations(t, 16, 0)
	if _, err := golang.CommittedReadScopes(context.Background(), root, commit); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
		t.Fatalf("unscaled budget: CommittedReadScopes error %v; want git-budget-exceeded", err)
	}
}
