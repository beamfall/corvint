package golang_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// AFP-V0-600: only a line comment that starts its line is a directive; its
// arguments split as the go command splits them, and a malformed one makes
// the whole file unresolved rather than partly claimed.
func TestEmbedPatternsReadOnlyDirectives_AFPV0600(t *testing.T) {
	cases := []struct {
		name, body string
		want       string
		ok         bool
	}{
		{"plain and quoted", "package p\n\n//go:embed a.txt \"b c.txt\" `d/*.json`\nvar f string\n", "[a.txt b c.txt d/*.json]", true},
		{"indented and repeated", "package p\n\nvar (\n\t//go:embed all:static\n\t//go:embed x.txt\n\tf string\n)\n", "[all:static x.txt]", true},
		{"string literal is no directive", "package p\n\nvar s = \"//go:embed secret.txt\"\n", "[]", true},
		{"block comment is no directive", "package p\n\n/*\n//go:embed secret.txt\n*/\n", "[]", true},
		{"trailing comment is no directive", "package p\n\nvar s string //go:embed secret.txt\n", "[]", true},
		{"another directive name", "package p\n\n//go:embedded x.txt\n", "[]", true},
		{"no arguments", "package p\n\n//go:embed\nvar f string\n", "[]", false},
		{"unterminated quote", "package p\n\n//go:embed \"a.txt\nvar f string\n", "[]", false},
		{"quote run into text", "package p\n\n//go:embed \"a.txt\"b\nvar f string\n", "[]", false},
		{"unscannable file", "package p\n\nvar s = \"open\n//go:embed a.txt\n", "[]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			patterns, ok := golang.EmbedPatterns([]byte(tc.body))
			if ok != tc.ok || (ok && fmt.Sprint(patterns) != tc.want) || (!ok && patterns != nil) {
				t.Fatalf("patterns %v ok %v, want %s ok %v", patterns, ok, tc.want, tc.ok)
			}
		})
	}
}

// AFP-V0-600: patterns resolve relative to the package directory, component by
// component; a directory embeds its files except hidden or underscore names
// below it (unless all:), version-control names and nested modules; a pattern
// the go command rejects resolves nothing.
func TestEmbedTargetsResolveAsTheGoCommand_AFPV0600(t *testing.T) {
	files := []string{
		"other/x.txt",
		"web/.hidden/a.css",
		"web/_draft.txt",
		"web/a.txt",
		"web/data/b.json",
		"web/data/c.txt",
		"web/static/.keep",
		"web/static/_partial.css",
		"web/static/css/site.css",
		"web/static/mod/go.mod",
		"web/static/mod/x.css",
		"web/web.go",
		"webx/y.txt",
	}
	modules := map[string]bool{"web/static/mod": true}
	cases := []struct {
		name     string
		patterns []string
		want     string
	}{
		{"a file", []string{"a.txt"}, "[web/a.txt]"},
		{"a glob at depth two", []string{"data/*.json"}, "[web/data/b.json]"},
		{"a directory skips hidden, underscore and nested modules", []string{"static"}, "[web/static/css/site.css]"},
		{"all: keeps hidden and underscore names", []string{"all:static"}, "[web/static/.keep web/static/_partial.css web/static/css/site.css]"},
		{"a directly named hidden file is embedded", []string{"_draft.txt"}, "[web/_draft.txt]"},
		{"a glob directory match", []string{"d*"}, "[web/data/b.json web/data/c.txt]"},
		// A name the pattern matches itself is embedded even when hidden, as
		// `image/*` embeds image/.tempfile; webx/ is no part of web/.
		{"a star keeps matched hidden names, not a sibling prefix", []string{"*"}, "[web/.hidden/a.css web/_draft.txt web/a.txt web/data/b.json web/data/c.txt web/static/css/site.css web/web.go]"},
		{"a parent escape resolves nothing", []string{"a.txt", "../other/x.txt"}, "[]"},
		{"a bad glob resolves nothing", []string{"a.txt", "[x"}, "[]"},
		{"dot resolves nothing", []string{"."}, "[]"},
		{"a trailing slash resolves nothing", []string{"static/"}, "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmt.Sprint(golang.EmbedTargets("web", tc.patterns, files, modules)); got != tc.want {
				t.Fatalf("targets %s, want %s", got, tc.want)
			}
		})
	}
	if got := fmt.Sprint(golang.EmbedTargets(".", []string{"other"}, files, nil)); got != "[other/x.txt]" {
		t.Fatalf("root package targets %s", got)
	}
}

// AFP-V0-600: the Go plugin records each package's resolved embed targets, so
// an edit to one selects the embedding package and its importers; test files'
// directives embed nothing into the package build. While another Go package
// with tests runs unconfined the embedded file stays UNOWNED (AFP-V0-601);
// once every such package declares a read scope it is attributed, and a file
// no directive names stays UNOWNED.
func TestEmbeddedFilesAreOwnedByTheEmbeddingPackage_AFPV0600(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"cat/cat.go":            "package cat\n\nimport _ \"embed\"\n\n//go:embed catalog.json\nvar catalog []byte\n",
		"cat/cat_test.go":       "package cat\n\nimport _ \"embed\"\n\n//go:embed extra.json\nvar extra []byte\n",
		"cat/catalog.json":      "{}\n",
		"cat/extra.json":        "{}\n",
		"cat/notes.txt":         "x\n",
		"user/user.go":          "package user\n\nimport _ \"example.test/m/cat\"\n",
		"user/user_test.go":     "package user\n",
		"other/other.go":        "package other\n",
		"other/other_test.go":   "package other\n",
		"broken/broken.go":      "package broken\n\nimport _ \"embed\"\n\n//go:embed \"open\nvar b []byte\n",
		"broken/broken_test.go": "package broken\n",
		"broken/data.txt":       "x\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	embeds := map[string]string{}
	for _, unit := range result.Units {
		if unit.Embeds || len(unit.EmbedFiles) != 0 {
			embeds[unit.ID] = fmt.Sprint(unit.Embeds, unit.EmbedFiles)
		}
	}
	if got, want := fmt.Sprint(embeds), "map[go:example.test/m/broken:true [] go:example.test/m/cat:true [cat/catalog.json]]"; got != want {
		t.Fatalf("embeds %s, want %s", got, want)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"cat/catalog.json"})
	if got := fmt.Sprint(plan.SelectedTests()); got != "[cat/cat_test.go user/user_test.go]" {
		t.Fatalf("selected tests %s", got)
	}
	if !hasUnowned(plan, "cat/catalog.json") || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("unconfined readers: scope %s unknown %v, want UNOWNED", plan.Scope, plan.Unknown)
	}
	// An unparsed directive claims nothing: the file stays unowned and rule
	// (b) still selects the embedding package.
	plan = affected.Select(graph, []string{"broken/data.txt"})
	if !hasUnowned(plan, "broken/data.txt") || fmt.Sprint(plan.SelectedTests()) != "[broken/broken_test.go]" {
		t.Fatalf("unparsed directive plan: tests %v unknown %v", plan.SelectedTests(), plan.Unknown)
	}
	writeFiles(t, root, map[string]string{affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"broken":[],"other":[],"user":[]}}`})
	if graph, err = affected.Build(context.Background(), root, golang.New()); err != nil {
		t.Fatal(err)
	}
	plan = affected.Select(graph, []string{"cat/catalog.json"})
	if got := fmt.Sprint(plan.SelectedTests()); got != "[cat/cat_test.go user/user_test.go]" || hasUnowned(plan, "cat/catalog.json") {
		t.Fatalf("confined readers: tests %s unknown %v", got, plan.Unknown)
	}
	if plan = affected.Select(graph, []string{"cat/notes.txt"}); !hasUnowned(plan, "cat/notes.txt") {
		t.Fatalf("unembedded file: unknown %v, want UNOWNED", plan.Unknown)
	}
}

func hasUnowned(plan affected.Plan, path string) bool {
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnownedDirtyPath && unknown.Detail == path {
			return true
		}
	}
	return false
}

// AFP-V0-811 (V1-2898): embedding source bytes selects Go readers but does
// not establish a JavaScript test dependency. Keep that distinct unknown.
func TestEmbeddedJavaScriptKeepsUntestedLanguageUnknown_AFPV0811(t *testing.T) {
	root := t.TempDir()
	const asset = "internal/jstestprovider/qualified-reporter.cjs"
	writeFiles(t, root, map[string]string{
		"go.mod":                                   "module example.test/m\n",
		"internal/jstestprovider/external.go":      "package jstestprovider\nimport _ \"embed\"\n//go:embed qualified-reporter.cjs\nvar reporter []byte\n",
		"internal/jstestprovider/external_test.go": "package jstestprovider\nimport \"testing\"\nfunc TestReporter(t *testing.T) { if len(reporter) == 0 { t.Fatal(\"empty\") } }\n",
		asset: "module.exports = class Reporter {};\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), typescript.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{asset})
	if got := fmt.Sprint(plan.SelectedTests()); got != "[internal/jstestprovider/external_test.go]" {
		t.Fatalf("Go reader not selected: %s", got)
	}
	found := false
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownNoSelectableTest && unknown.Detail == "typescript:"+asset {
			found = true
		}
	}
	if !found || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("embedding bytes must not prove JS coverage: %+v", plan)
	}
}
