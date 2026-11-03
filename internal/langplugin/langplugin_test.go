package langplugin_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// root is the repository root, read for directory layout only.
const root = "../.."

func isDir(t *testing.T, rel string) bool {
	t.Helper()
	if strings.TrimSpace(rel) == "" || strings.TrimSpace(rel) != rel {
		return false // a blank path would resolve to the repository root
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.IsDir()
}

// TestIsDirRefusesBlankPaths_LGPV0007 keeps the conformance check from
// accepting a blank path, which would resolve to the repository root.
func TestIsDirRefusesBlankPaths_LGPV0007(t *testing.T) {
	for _, rel := range []string{"", " ", "\t", " internal"} {
		if isDir(t, rel) {
			t.Errorf("isDir(%q) = true, want false", rel)
		}
	}
	if !isDir(t, "internal") {
		t.Error(`isDir("internal") = false, want true`)
	}
}

// TestManifestsValidate_LGPV0007 checks every registered manifest's own
// declaration: each piece present or absent with a reason.
func TestManifestsValidate_LGPV0007(t *testing.T) {
	for _, m := range langplugin.All() {
		if err := langplugin.Validate(m); err != nil {
			t.Error(err)
		}
	}
}

type fakeLanguage struct{ name string }

func (f fakeLanguage) Name() string   { return f.name }
func (fakeLanguage) Owns(string) bool { return false }
func (fakeLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{}, nil
}

// TestValidateRefusesAnUndeclaredPiece_LGPV0007 falsifies Validate: a
// manifest that lacks a piece without declaring it absent, declares a piece
// both ways, gives no reason, or whose provider namespace differs, is refused.
func TestValidateRefusesAnUndeclaredPiece_LGPV0007(t *testing.T) {
	complete := func() langplugin.Manifest {
		return langplugin.Manifest{
			Name:      "demo",
			Paths:     []string{"internal/demo"},
			Provider:  func() affected.Language { return fakeLanguage{"demo"} },
			Analyzers: []langplugin.SubPlugin{{Name: "corvint-analyzer-demo", Paths: []string{"internal/demo"}}},
			Runners:   []string{"demo-test"},
			Absent: map[langplugin.Piece]string{
				langplugin.TestProvider: "none", langplugin.Coverage: "none", langplugin.Mutation: "none",
			},
		}
	}
	if err := langplugin.Validate(complete()); err != nil {
		t.Fatalf("complete manifest refused: %v", err)
	}
	for want, edit := range map[string]func(*langplugin.Manifest){
		"coverage is missing and not declared absent": func(m *langplugin.Manifest) { delete(m.Absent, langplugin.Coverage) },
		"runners is missing and not declared absent":  func(m *langplugin.Manifest) { m.Runners = nil },
		"analyzer is declared and also absent":        func(m *langplugin.Manifest) { m.Absent[langplugin.Analyzer] = "x" },
		"mutation is absent without a reason":         func(m *langplugin.Manifest) { m.Absent[langplugin.Mutation] = " " },
		"provider namespace differs":                  func(m *langplugin.Manifest) { m.Name = "other" },
		"unknown piece":                               func(m *langplugin.Manifest) { m.Absent["linter"] = "x" },
		"owns no path":                                func(m *langplugin.Manifest) { m.Paths = nil },
		"names no path":                               func(m *langplugin.Manifest) { m.Analyzers[0].Paths = nil },
		`owned path "" is empty or padded`:            func(m *langplugin.Manifest) { m.Paths = []string{""} },
		`owned path " " is empty or padded`:           func(m *langplugin.Manifest) { m.Paths = append(m.Paths, " ") },
		`path "" is empty or padded`:                  func(m *langplugin.Manifest) { m.Analyzers[0].Paths = []string{""} },
		`path " internal/demo" is empty or padded`:    func(m *langplugin.Manifest) { m.Analyzers[0].Paths = []string{" internal/demo"} },
		"is empty or padded":                          func(m *langplugin.Manifest) { m.Name = " demo" },
		"empty runner name":                           func(m *langplugin.Manifest) { m.Runners = append(m.Runners, "") },
	} {
		m := complete()
		edit(&m)
		if err := langplugin.Validate(m); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}

// TestManifestConformance_LGPV0008 binds the manifest to the repository: names
// are unique, owned paths exist and are disjoint, every sub-plugin path and
// analyzer binary exists, every analyzer binary and affected provider package
// is declared, and every declared runner resolves in the runner registry with
// Build and Parse while every registered runner belongs to exactly one plugin.
func TestManifestConformance_LGPV0008(t *testing.T) {
	names := map[string]bool{}
	owner := map[string]string{}
	analyzers := map[string]string{}
	runners := map[string]string{}
	for _, m := range langplugin.All() {
		if names[m.Name] {
			t.Errorf("plugin %s registered twice", m.Name)
		}
		names[m.Name] = true
		for _, p := range m.Paths {
			if prev, ok := owner[p]; ok {
				t.Errorf("%s owned by %s and %s", p, prev, m.Name)
			}
			owner[p] = m.Name
			if !isDir(t, p) {
				t.Errorf("%s: owned path %s is not a directory", m.Name, p)
			}
		}
		for _, list := range [][]langplugin.SubPlugin{m.Analyzers, m.TestProviders, m.Coverage, m.Mutation} {
			for _, s := range list {
				for _, p := range s.Paths {
					if !isDir(t, p) {
						t.Errorf("%s: %s path %s is not a directory", m.Name, s.Name, p)
					}
				}
			}
		}
		for _, a := range m.Analyzers {
			if prev, ok := analyzers[a.Name]; ok {
				t.Errorf("analyzer %s declared by %s and %s", a.Name, prev, m.Name)
			}
			analyzers[a.Name] = m.Name
			if !isDir(t, "cmd/"+a.Name) {
				t.Errorf("%s: analyzer binary cmd/%s missing", m.Name, a.Name)
			}
		}
		for _, r := range m.Runners {
			if prev, ok := runners[r]; ok {
				t.Errorf("runner %s declared by %s and %s", r, prev, m.Name)
			}
			runners[r] = m.Name
			f, ok := registry.Lookup(r)
			if !ok {
				t.Errorf("%s: runner %s is not registered", m.Name, r)
			} else if f.Build == nil || f.Parse == nil {
				t.Errorf("%s: runner %s family %s lacks Build or Parse", m.Name, r, f.Name)
			}
		}
	}
	for _, r := range registry.Runners() {
		if _, ok := runners[r]; !ok {
			t.Errorf("registered runner %s belongs to no language plugin", r)
		}
	}
	cmds, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range cmds {
		if e.IsDir() && strings.HasPrefix(e.Name(), "corvint-analyzer-") {
			if _, ok := analyzers[e.Name()]; !ok {
				t.Errorf("analyzer binary cmd/%s belongs to no language plugin", e.Name())
			}
		}
	}
	providers, err := os.ReadDir(filepath.Join(root, "internal", "liveverify", "affected"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range providers {
		p := "internal/liveverify/affected/" + e.Name()
		if e.IsDir() && e.Name() != "languages" && e.Name() != "testdata" && owner[p] == "" {
			t.Errorf("provider package %s belongs to no language plugin", p)
		}
	}
	got := []string{}
	for _, p := range langplugin.Providers() {
		got = append(got, p.Name())
		if lang, ok := langplugin.Lookup(p.Name()); !ok || lang.Provider == nil {
			t.Errorf("provider %s has no manifest", p.Name())
		}
	}
	// The order in which providers claim a path is the previous hand-written
	// languages.All list (LGP-V0-005, LGP-V0-012).
	want := []string{"cfamily", "dotnet", "go", "java", "kotlin", "python", "ruby", "rust", "shell", "swift", "typescript"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("providers %v, want %v", got, want)
	}
	for runner, lang := range map[string]string{"rspec": "ruby", "go-test": "go", "pgtap": "sql", "jest": "typescript"} {
		if got, ok := langplugin.RunnerLanguage(runner); !ok || got != lang {
			t.Errorf("RunnerLanguage(%s) = %q, %v; want %s", runner, got, ok, lang)
		}
	}
	if _, ok := langplugin.RunnerLanguage("no-such-runner"); ok {
		t.Error("RunnerLanguage answered an unknown runner")
	}
}

// coreNameLiterals is the ratchet baseline of string literals in core
// packages' non-test Go files that equal a plugin or runner name
// (LGP-V0-009). A count may only fall, and the baseline must fall with it;
// a new name in core code reads langplugin or the registry instead.
var coreNameLiterals = map[string]map[string]int{
	"cmd/corvint": {"go": 6, "pytest": 1, "python": 2, "rspec": 1, "ruby": 2, "typescript": 1},
	"internal/contextindex": {
		"go": 2, "java": 1, "kotlin": 5, "minitest": 1, "rspec": 1, "ruby": 22, "swift": 4, "typescript": 8,
	},
	"internal/testsource": {
		"ava": 2, "cypress": 4, "jasmine": 2, "jest": 9, "mocha": 4, "playwright": 5, "rspec": 5, "ruby": 1,
		"testcafe": 1, "vitest": 4,
	},
	"internal/liveverify/affected/languages": {},
	"internal/testrunner/registry":           {},
}

// TestCoreNameLiterals_LGPV0009 keeps language and runner names out of core
// packages: a literal beyond the baseline fails, and so does a stale baseline.
func TestCoreNameLiterals_LGPV0009(t *testing.T) {
	names := map[string]bool{}
	for _, n := range langplugin.Names() {
		names[n] = true
	}
	dirs := make([]string, 0, len(coreNameLiterals))
	for dir := range coreNameLiterals {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no Go files (%v)", dir, err)
		}
		got := map[string]int{}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil && names[s] {
						got[s]++
					}
				}
				return true
			})
		}
		want := coreNameLiterals[dir]
		for name, n := range got {
			if n > want[name] {
				t.Errorf("%s: %d literal %q, baseline %d: read the name from langplugin or the testrunner registry", dir, n, name, want[name])
			}
		}
		for name, n := range want {
			if got[name] < n {
				t.Errorf("%s: %d literal %q, baseline %d: lower the coreNameLiterals baseline", dir, got[name], name, n)
			}
		}
	}
}
