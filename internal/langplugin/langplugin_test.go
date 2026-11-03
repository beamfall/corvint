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
		t.Run(m.Name, func(t *testing.T) {
			if err := langplugin.Validate(m); err != nil {
				t.Error(err)
			}
		})
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
		"unknown dialect kind": func(m *langplugin.Manifest) {
			m.Dialects = []langplugin.Dialect{{Kind: "lint", Name: "demo", Suffixes: []string{".d"}}}
		},
		"is unnamed or reads no suffix": func(m *langplugin.Manifest) {
			m.Dialects = []langplugin.Dialect{{Kind: langplugin.ImpactDialect, Name: "demo"}}
		},
		"suffix \"d\" is empty or padded": func(m *langplugin.Manifest) {
			m.Dialects = []langplugin.Dialect{{Kind: langplugin.ImpactDialect, Name: "demo", Suffixes: []string{"d"}}}
		},
		"empty dependency name": func(m *langplugin.Manifest) { m.Dependencies = map[string]string{"": "demo-test"} },
		"names runner \"other\" the plugin does not declare": func(m *langplugin.Manifest) {
			m.Dependencies = map[string]string{"demo-pkg": "other"}
		},
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
	ran := 0
	for _, m := range langplugin.All() {
		t.Run(m.Name, func(t *testing.T) { ran++; conformPlugin(t, m, names, owner, analyzers, runners) })
	}
	if ran != len(langplugin.All()) {
		// One plugin's subtest ran alone (make plugin-test); the whole-registry
		// checks below need every plugin's names.
		return
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

// conformPlugin binds one manifest to the repository and records its names,
// paths, analyzers and runners for the cross-plugin checks (LGP-V0-008).
func conformPlugin(t *testing.T, m langplugin.Manifest, names map[string]bool, owner, analyzers, runners map[string]string) {
	t.Helper()
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

// coreLiteral is one ratchet entry: how many literals of a name a core
// package tree still holds, and why each remaining one stays.
type coreLiteral struct {
	n      int
	reason string
}

// coreNameLiterals is the ratchet baseline of string literals in core
// package trees' non-test Go files (subpackages included, testdata excluded)
// that equal a plugin or runner name (LGP-V0-009). A count may only fall, and
// the baseline must fall with it; every remaining literal carries its reason.
// A new name in core code reads langplugin or the registry instead.
var coreNameLiterals = map[string]map[string]coreLiteral{
	"cmd/corvint": {
		"go":     {5, "Go session baseline, go-test argv word, claim-gate engine and Go's reverse-import rule, which is no dialect"},
		"pytest": {1, "pytest.ini configuration section name"},
		"python": {1, "claim-gate engine set of prove (go, python)"},
		"ruby":   {1, "plugin whose dependencies prove the rspec runner (LGP-V0-017)"},
		"rspec":  {1, "runner the dependency map is read for (LGP-V0-017)"},
	},
	"internal/contextindex": {
		"go":         {2, "Go module unknown code and a source-suffix table key"},
		"java":       {1, "source-suffix table key of context anchors"},
		"kotlin":     {4, "dialect algorithm body compares the declared dialect name it implements (LGP-V0-016); moving the bodies into plugins is a follow-up"},
		"swift":      {3, "dialect algorithm body compares the declared dialect name it implements (LGP-V0-016); moving the bodies into plugins is a follow-up"},
		"ruby":       {22, "dialect algorithm body compares the declared dialect name it implements (LGP-V0-016); moving the bodies into plugins is a follow-up"},
		"typescript": {8, "dialect algorithm body compares the declared dialect name it implements (LGP-V0-016); moving the bodies into plugins is a follow-up"},
		"rspec":      {1, "runner the Ruby dependency map is read for (LGP-V0-017)"},
		"minitest":   {1, "runner the Ruby dependency map is read for (LGP-V0-017)"},
	},
	"internal/testsource": {
		"jest":       {8, "modelled runner constant and JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"vitest":     {3, "modelled runner constant and JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"mocha":      {3, "modelled runner constant and JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"playwright": {5, "modelled runner constant, assertion framework and JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"jasmine":    {1, "assertion framework label"},
		"cypress":    {3, "assertion framework label and JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"ava":        {1, "JavaScript source reading: a module specifier, global identifier or config key, not a plugin lookup"},
		"rspec":      {5, "assertion/teardown framework label and discovery runner switch"},
		"ruby":       {1, "example-join report language field"},
	},
	"internal/liveverify/affected/languages": {},
	"internal/testrunner/registry":           {},
}

// TestCoreNameLiterals_LGPV0009 keeps language and runner names out of core
// packages: a literal beyond the baseline fails, and so does a stale baseline
// or an entry without a reason.
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
		got := map[string]int{}
		files := 0
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(f string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(f, ".go") || strings.HasSuffix(f, "_test.go") {
				return nil
			}
			files++
			file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil && names[s] {
						got[s]++
					}
				}
				return true
			})
			return nil
		})
		if err != nil || files == 0 {
			t.Fatalf("%s: no Go files (%v)", dir, err)
		}
		want := coreNameLiterals[dir]
		for name, n := range got {
			if n > want[name].n {
				t.Errorf("%s: %d literal %q, baseline %d: read the name from langplugin or the testrunner registry", dir, n, name, want[name].n)
			}
		}
		for name, w := range want {
			if got[name] < w.n {
				t.Errorf("%s: %d literal %q, baseline %d: lower the coreNameLiterals baseline", dir, got[name], name, w.n)
			}
			if strings.TrimSpace(w.reason) == "" {
				t.Errorf("%s: literal %q has no recorded reason", dir, name)
			}
		}
	}
}

// TestDialectsAndDependencies_LGPV0015 binds the dialect, dependency and
// mutation-suffix hooks across plugins: no suffix is claimed twice for one
// dialect kind or by two mutation engines, every dependency names a runner
// the registry builds, and the lookups answer from the declarations
// (LGP-V0-015, LGP-V0-017, LGP-V0-018).
func TestDialectsAndDependencies_LGPV0015(t *testing.T) {
	claimed := map[string]string{}
	mutated := map[string]string{}
	for _, m := range langplugin.All() {
		t.Run(m.Name, func(t *testing.T) {
			for _, d := range m.Dialects {
				for _, suffix := range d.Suffixes {
					key := string(d.Kind) + " " + suffix
					if prev, ok := claimed[key]; ok && prev != m.Name+"/"+d.Name {
						t.Errorf("%s dialect suffix %s declared by %s and %s/%s", d.Kind, suffix, prev, m.Name, d.Name)
					}
					claimed[key] = m.Name + "/" + d.Name
					if got := langplugin.DialectFor(d.Kind, "a"+suffix); got != d.Name {
						t.Errorf("DialectFor(%s, a%s) = %q, want %s", d.Kind, suffix, got, d.Name)
					}
				}
			}
			for pkg, runner := range m.Dependencies {
				if _, ok := registry.Lookup(runner); !ok {
					t.Errorf("dependency %s names runner %s the registry does not build", pkg, runner)
				}
				if got := langplugin.DependencyRunners(m.Name)[pkg]; got != runner {
					t.Errorf("DependencyRunners(%s)[%s] = %q, want %s", m.Name, pkg, got, runner)
				}
			}
			for _, s := range m.Mutation {
				for _, suffix := range s.Suffixes {
					if prev, ok := mutated[suffix]; ok {
						t.Errorf("mutation suffix %s declared by %s and %s", suffix, prev, s.Name)
					}
					mutated[suffix] = s.Name
					if got := langplugin.MutationLanguage("a" + suffix); got != m.Name {
						t.Errorf("MutationLanguage(a%s) = %q, want %s", suffix, got, m.Name)
					}
				}
			}
		})
	}
	if got := langplugin.DialectFor(langplugin.ImpactDialect, "Makefile"); got != "" {
		t.Errorf("DialectFor(impact, Makefile) = %q, want none", got)
	}
	if got := langplugin.MutationLanguage("a.rs"); got != "" {
		t.Errorf("MutationLanguage(a.rs) = %q, want none", got)
	}
}

// corePackages are the packages a plugin must not reach (LGP-V0-020).
var corePackages = []string{"cmd/corvint", "internal/contextindex", "internal/testsource", "internal/langplugin"}

// isolationExemptions are plugin directories allowed to reach a core package,
// each with its reason (LGP-V0-020).
var isolationExemptions = map[string]string{
	"internal/swiftimpact": "Swift adapter that registers its SwiftPM reader into contextindex.RegisterSwiftPMReader",
}

// TestPluginIsolation_LGPV0020 fails, per plugin, when a non-test Go file in
// the plugin's directories reaches a core package through module-internal
// imports, so the plugin builds and tests apart from the core. A stale
// exemption fails too.
func TestPluginIsolation_LGPV0020(t *testing.T) {
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	module := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(gomod), "\n", 2)[0], "module"))
	imports := map[string][]string{}
	var importsOf func(dir string) []string
	importsOf = func(dir string) []string {
		if got, ok := imports[dir]; ok {
			return got
		}
		imports[dir] = nil
		files, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		seen := map[string]bool{}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range file.Imports {
				p, _ := strconv.Unquote(spec.Path.Value)
				if rel, ok := strings.CutPrefix(p, module+"/"); ok && !seen[rel] {
					seen[rel] = true
					imports[dir] = append(imports[dir], rel)
				}
			}
		}
		sort.Strings(imports[dir])
		return imports[dir]
	}
	// reaches returns the import chain from dir to a core package, or nil.
	reaches := func(dir string) []string {
		parent := map[string]string{dir: ""}
		queue := []string{dir}
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			for _, imp := range importsOf(next) {
				if _, ok := parent[imp]; ok {
					continue
				}
				parent[imp] = next
				for _, core := range corePackages {
					if imp == core {
						chain := []string{imp}
						for at := next; at != ""; at = parent[at] {
							chain = append([]string{at}, chain...)
						}
						return chain
					}
				}
				queue = append(queue, imp)
			}
		}
		return nil
	}
	for _, m := range langplugin.All() {
		t.Run(m.Name, func(t *testing.T) {
			dirs, _ := langplugin.Dirs(m.Name)
			for _, base := range dirs {
				err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(base)), func(f string, d os.DirEntry, err error) error {
					if err != nil || !d.IsDir() {
						return err
					}
					if d.Name() == "testdata" {
						return filepath.SkipDir
					}
					rel, err := filepath.Rel(root, f)
					if err != nil {
						return err
					}
					dir := filepath.ToSlash(rel)
					chain := reaches(dir)
					reason, exempt := isolationExemptions[dir]
					switch {
					case chain != nil && !exempt:
						t.Errorf("%s reaches core package %s: %s", dir, chain[len(chain)-1], strings.Join(chain, " -> "))
					case chain == nil && exempt:
						t.Errorf("%s exemption is stale (%s): it reaches no core package", dir, reason)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	for dir := range isolationExemptions {
		if !isDir(t, dir) {
			t.Errorf("exemption %s names no directory", dir)
		}
	}
}
