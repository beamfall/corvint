package affected

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// documentationGraph builds chain() after edit, with a frontier on the x
// plugin, over a root holding declaration as .corvint/documentation-paths.json
// (none when empty).
func documentationGraph(t *testing.T, declaration string, edit func(*fake)) (*Graph, error) {
	t.Helper()
	root := t.TempDir()
	if declaration != "" {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(DocumentationPathsPath)), []byte(declaration), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	language := chain()
	language.frontier = []string{"x:dynamic-dispatch"}
	if edit != nil {
		edit(&language)
	}
	return Build(context.Background(), root, language)
}

const buildLogDeclaration = `{"profile":"corvint-documentation-paths/0","paths":["NOTES.md","docs/build-log/"]}`

func documentationPlan(t *testing.T, edit func(*fake), dirty ...string) Plan {
	t.Helper()
	graph, err := documentationGraph(t, buildLogDeclaration, edit)
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

func groupPaths(plan Plan, class string) []string {
	paths := []string{}
	for _, group := range plan.NonSource {
		if group.Class == class {
			paths = append(paths, group.Paths...)
		}
	}
	return paths
}

// AFP-V0-160 (2), (4): a dirty path the repository declares as documentation
// raises no UNOWNED_DIRTY_PATH entry and involves no plugin's frontier, so a
// change of documentation alone is BOUNDED and lists it in nonSource; an
// undeclared unowned path keeps the AFP-V0-004 widening.
func TestDeclaredDocumentationIsNonSourceNotUnknown_AFPV0160(t *testing.T) {
	documentation := []string{"NOTES.md", "docs/build-log/2026-10-21-entry.md"}
	plan := documentationPlan(t, nil, documentation...)
	if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 || len(plan.Selected) != 0 {
		t.Fatalf("scope=%s unknown=%v selected=%+v", plan.Scope, unknownDetails(plan), plan.Selected)
	}
	if len(plan.NonSource) != 1 || plan.NonSource[0].Reason != declaredDocumentationReason || !slices.Equal(groupPaths(plan, ClassDeclaredDocumentation), documentation) {
		t.Fatalf("nonSource=%+v", plan.NonSource)
	}
	for _, undeclared := range []string{"docs/other.md", "docs/build-log", "docs/build-logx/a.md", "sub/NOTES.md", "NOTES.md.bak"} {
		plan := documentationPlan(t, nil, undeclared)
		if plan.Scope != ScopeUnknown || len(plan.NonSource) != 0 || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+undeclared) {
			t.Fatalf("%s: scope=%s unknown=%v nonSource=%v", undeclared, plan.Scope, unknownDetails(plan), plan.NonSource)
		}
	}
	graph, err := documentationGraph(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan := Select(graph, documentation); plan.Scope != ScopeUnknown || len(plan.NonSource) != 0 || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH NOTES.md") {
		t.Fatalf("no declaration: scope=%s unknown=%v nonSource=%v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
}

// AFP-V0-160 (3): every unit that names a declared documentation path is
// still selected, as its documentation check, by path literal or declared read
// scope, and an unbounded reader still is; the path itself is classified, and
// a path a plugin claims as source is never reclassified.
func TestDeclaredDocumentationKeepsEveryReader_AFPV0160(t *testing.T) {
	entry := "docs/build-log/2026-10-21-entry.md"
	cases := []struct {
		name    string
		edit    func(*fake)
		witness string
	}{
		{"directory literal", func(language *fake) { language.units[3].PathTokens = []string{"docs/build-log/"} }, WitnessPathLiteralReader},
		{"file literal", func(language *fake) { language.units[3].PathTokens = []string{entry} }, WitnessPathLiteralReader},
		{"declared read scope", func(language *fake) {
			language.units[3].Sources = []string{"solo/solo.x"}
			language.units[3].Tests = []string{"solo/solo_test.x"}
			language.units[3].ReadScoped = true
			language.units[3].ReadScope = []string{"docs/"}
		}, WitnessDeclaredReadScope},
		{"unbounded reader", func(language *fake) { language.units[3].UnboundedReads = "root-locator" }, WitnessUnboundedReader},
	}
	for _, tc := range cases {
		plan := documentationPlan(t, tc.edit, entry)
		if got := witnesses(plan)["x:solo"]; got.Kind != tc.witness || got.DirtyPath != entry {
			t.Fatalf("%s: selected=%+v", tc.name, plan.Selected)
		}
		if slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+entry) || !slices.Equal(groupPaths(plan, ClassDeclaredDocumentation), []string{entry}) {
			t.Fatalf("%s: unknown=%v nonSource=%v", tc.name, unknownDetails(plan), plan.NonSource)
		}
	}
	claimed := "docs/build-log/tool.x"
	plan := documentationPlan(t, nil, claimed)
	if plan.Scope != ScopeUnknown || len(plan.NonSource) != 0 || !slices.Contains(unknownDetails(plan), "UNINDEXED_SOURCE_PATH "+claimed) {
		t.Fatalf("claimed source: scope=%s unknown=%v nonSource=%v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
}

// AFP-V0-160 (5): a Tasks store projection path stays under AFP-V0-088 even
// inside a declared directory, and beside a source change documentation adds
// nothing to the selection or the unknown entries.
func TestDeclaredDocumentationLeavesTaskStateAndSourceAlone_AFPV0160(t *testing.T) {
	declaration := `{"profile":"corvint-documentation-paths/0","paths":[".taskman/"]}`
	graph, err := documentationGraph(t, declaration, func(language *fake) { language.units[3].PathTokens = []string{"policy.json"} })
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{".taskman/policy.json", ".taskman/queue.json"})
	if !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH .taskman/policy.json") || len(groupPaths(plan, ClassDeclaredDocumentation)) != 0 || !slices.Equal(groupPaths(plan, ClassTaskStoreProjection), []string{".taskman/queue.json"}) {
		t.Fatalf("task state: unknown=%v nonSource=%+v", unknownDetails(plan), plan.NonSource)
	}
	alone := documentationPlan(t, nil, "core.x")
	mixed := documentationPlan(t, nil, "core.x", "docs/build-log/a.md")
	if !slices.Equal(unknownDetails(alone), unknownDetails(mixed)) || !slices.EqualFunc(alone.Selected, mixed.Selected, func(a, b Selection) bool { return a.UnitID == b.UnitID }) {
		t.Fatalf("alone unknown=%v selected=%+v; mixed unknown=%v selected=%+v", unknownDetails(alone), alone.Selected, unknownDetails(mixed), mixed.Selected)
	}
}

// AFP-V0-160 (1): the declaration is closed and validated; an invalid one
// refuses the graph rather than classifying anything.
func TestDocumentationDeclarationIsValidated_AFPV0160(t *testing.T) {
	for name, declaration := range map[string]string{
		"profile":        `{"profile":"corvint-documentation-paths/1","paths":["docs/"]}`,
		"missing paths":  `{"profile":"corvint-documentation-paths/0"}`,
		"unknown member": `{"profile":"corvint-documentation-paths/0","paths":["docs/"],"extra":true}`,
		"unsorted":       `{"profile":"corvint-documentation-paths/0","paths":["z/","a/"]}`,
		"duplicate":      `{"profile":"corvint-documentation-paths/0","paths":["a/","a/"]}`,
		"absolute":       `{"profile":"corvint-documentation-paths/0","paths":["/docs/"]}`,
		"parent":         `{"profile":"corvint-documentation-paths/0","paths":["../docs/"]}`,
		"git":            `{"profile":"corvint-documentation-paths/0","paths":[".git/"]}`,
		"corvint":        `{"profile":"corvint-documentation-paths/0","paths":[".corvint/"]}`,
		"declaration":    `{"profile":"corvint-documentation-paths/0","paths":[".corvint/documentation-paths.json"]}`,
		"not json":       `docs/`,
	} {
		if _, err := documentationGraph(t, declaration, nil); err == nil || !strings.Contains(err.Error(), DocumentationPathsPath) {
			t.Errorf("%s: err=%v", name, err)
		}
	}
	if _, err := documentationGraph(t, `{"profile":"corvint-documentation-paths/0","paths":[]}`, nil); err != nil {
		t.Fatalf("an empty declaration: %v", err)
	}
}

// AFP-V0-160 (1): the declaration is part of the graph digest when present
// and absent from its canonical form otherwise, so graphs of repositories
// without it keep their digest.
func TestDocumentationDeclarationIsInTheDigest_AFPV0160(t *testing.T) {
	without, err := documentationGraph(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	with, err := documentationGraph(t, buildLogDeclaration, nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := without.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), "documentation") || without.Digest() == with.Digest() {
		t.Fatalf("canonical=%s digests %s %s", canonical, without.Digest(), with.Digest())
	}
	wire, err := json.Marshal(Select(with, []string{"NOTES.md"}))
	if err != nil {
		t.Fatal(err)
	}
	want := `"nonSource":[{"class":"DECLARED_DOCUMENTATION","reason":"` + declaredDocumentationReason + `","paths":["NOTES.md"]}]`
	if !strings.Contains(string(wire), want) {
		t.Fatalf("wire=%s want %s", wire, want)
	}
}

// AFP-V0-160 (6) as AFP-V0-465 and AFP-V0-508 amend it: this repository's
// declaration names `docs/build-log/`, `docs/specs/` and
// `docs/decisions/`, and every top-level `docs/*.md` file; every declared
// file exists, so no stale entry hides a path that has become something else.
func TestRepositoryDeclaresBuildLogDocumentation_AFPV0160(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	_, declared, err := ReadDocumentationDeclaration(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"docs/build-log/", "docs/specs/", "docs/decisions/"} {
		if !slices.Contains(declared, directory) {
			t.Errorf("%s does not declare %s", DocumentationPathsPath, directory)
		}
	}
	for _, entry := range declared {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); err != nil {
			t.Errorf("declared %s: %v", entry, err)
		}
	}
	docs, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		if name := "docs/" + filepath.Base(doc); !slices.Contains(declared, name) {
			t.Errorf("%s is not declared in %s", name, DocumentationPathsPath)
		}
	}
}

// AFP-V0-443: a dirty .corvint/documentation-paths.json that no unit names or
// reaches is the CORVINT_DECLARATION group, not UNOWNED_DIRTY_PATH, so a
// change that adds the declaration is BOUNDED; a unit whose path literal names
// the file or its directory, or an unbounded reader, keeps the unknown entry.
func TestDocumentationDeclarationIsCorvintDeclaration_AFPV0443(t *testing.T) {
	plan := documentationPlan(t, nil, DocumentationPathsPath, "NOTES.md")
	if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 || !slices.Equal(groupPaths(plan, ClassCorvintDeclaration), []string{DocumentationPathsPath}) || !slices.Equal(groupPaths(plan, ClassDeclaredDocumentation), []string{"NOTES.md"}) {
		t.Fatalf("scope=%s unknown=%v nonSource=%+v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
	if last := plan.NonSource[len(plan.NonSource)-1]; last.Class != ClassCorvintDeclaration || last.Reason != corvintDeclarationReason {
		t.Fatalf("nonSource order=%+v", plan.NonSource)
	}
	for name, edit := range map[string]func(*fake){
		"file literal":      func(language *fake) { language.units[3].PathTokens = []string{DocumentationPathsPath} },
		"directory literal": func(language *fake) { language.units[3].PathTokens = []string{"../../.corvint"} },
		"unbounded reader":  func(language *fake) { language.units[3].UnboundedReads = "root-locator" },
	} {
		plan := documentationPlan(t, edit, DocumentationPathsPath)
		if plan.Scope != ScopeUnknown || len(groupPaths(plan, ClassCorvintDeclaration)) != 0 || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+DocumentationPathsPath) {
			t.Fatalf("%s: scope=%s unknown=%v nonSource=%+v", name, plan.Scope, unknownDetails(plan), plan.NonSource)
		}
	}
}
