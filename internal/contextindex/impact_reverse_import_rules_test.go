package contextindex

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// This file holds the regressions for the two reverse-import rules
// `GPK-V0-027` gained on 2026-08-29 (decision 0007, D2): `.py` and the web
// set. Before they were implemented, `reverseImporters` built one target only
// -- a slash-qualified Go import path -- so every assertion below failed by
// construction rather than by observation, which is what DR-0006 recorded.

// impactRepositoryWithFiles commits files into a fresh repository. The web and
// Python fixtures differ only in their trees, so they share one builder.
func impactRepositoryWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if command := testGitOptional(t, root, "init", "-q"); command != "" {
		t.Skipf("git init unsupported: %s", command)
	}
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	for relative, content := range files {
		writeTestFile(t, root, relative, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "impact rule fixture")
	return root
}

// reverseImportIDs maps each reverse-import result's id to the import reasons
// its evidence carries. One importer can reach the changed file by more than
// one specifier, and the receipt merges those into one result, so the value is
// a set rather than a single string.
func reverseImportIDs(receipt map[string]any) map[string]map[string]bool {
	found := make(map[string]map[string]bool)
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] != "reverse-import" {
			continue
		}
		id := stringValue(item["id"])
		if found[id] == nil {
			found[id] = make(map[string]bool)
		}
		for _, evidenceItem := range mapsFromAny(item["evidence"]) {
			found[id][stringValue(evidenceItem["reason"])] = true
		}
	}
	return found
}

func impactReverseImportIDs(t *testing.T, root, changedPath string) map[string]map[string]bool {
	t.Helper()
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{changedPath}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return reverseImportIDs(receipt)
}

// TestImpactResolvesPythonReverseImporters is rule (b). `src/cli.py` imports
// the bare name `harness`, which is `src/harness.py` seen with the source root
// on `sys.path` -- one of the dotted spellings `PythonImportCandidates`
// enumerates. The Go-shaped target `example.test/fixture/src` this search used
// to build is unproducible by any Python import statement, so before the rule
// landed this file resolved nothing.
func TestImpactResolvesPythonReverseImporters(t *testing.T) {
	root := pythonImpactRepository(t)
	found := impactReverseImportIDs(t, root, "src/harness.py")
	if !found["src/cli.py"]["imports harness"] {
		t.Fatalf("reverse-import results = %v, want src/cli.py via `imports harness`", found)
	}
}

// TestImpactResolvesPythonPackageReverseImporters covers the dotted spellings
// the bare-name case does not: a package import that keeps the `src` prefix,
// and the `__init__` form that names the package directory rather than the
// file. `from src.pkg import VERSION` is the second of those: the oracle
// records an `ImportFrom` by its module only, never by the names it binds, so
// that statement is an edge to `src/pkg/__init__.py` and not to anything
// under it.
func TestImpactResolvesPythonPackageReverseImporters(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                "module example.test/fixture\n\ngo 1.27.0\n",
		"src/pkg/__init__.py":   "VERSION = \"1\"\n",
		"src/pkg/engine.py":     "def run() -> str:\n    return \"engine\"\n",
		"tools/dotted.py":       "from src.pkg.engine import run\n\n\ndef main() -> str:\n    return run()\n",
		"tools/rooted.py":       "import pkg.engine\n\n\ndef main() -> str:\n    return pkg.engine.run()\n",
		"tools/packageonly.py":  "from src.pkg import VERSION\n\n\ndef main() -> str:\n    return VERSION\n",
		"internal/api/keep.go":  "package api\n\nfunc Keep() string { return \"keep\" }\n",
		"testing/features.yaml": "features: []\n",
	})
	engine := impactReverseImportIDs(t, root, "src/pkg/engine.py")
	if !engine["tools/dotted.py"]["imports src.pkg.engine"] {
		t.Fatalf("src/pkg/engine.py reverse-imports = %v, want tools/dotted.py via `imports src.pkg.engine`", engine)
	}
	if !engine["tools/rooted.py"]["imports pkg.engine"] {
		t.Fatalf("src/pkg/engine.py reverse-imports = %v, want tools/rooted.py via `imports pkg.engine`", engine)
	}
	initializer := impactReverseImportIDs(t, root, "src/pkg/__init__.py")
	if !initializer["tools/packageonly.py"]["imports src.pkg"] {
		t.Fatalf("src/pkg/__init__.py reverse-imports = %v, want tools/packageonly.py via `imports src.pkg`", initializer)
	}
}

// TestImpactCitesTheRelativeImportStatementLine: `from .. import helpers`
// resolves to `pkg` without spelling it, so the evidence line must come from
// the import statement itself, not a fallback to line 1 (invariant 2); and a
// source that imports nothing it can show yields no line.
func TestImpactCitesTheRelativeImportStatementLine(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"pkg/__init__.py":     "VALUE = 1\n",
		"pkg/sub/__init__.py": "",
		"pkg/sub/mod.py":      "\"\"\"Mentions pkg in prose.\"\"\"\n\n\nfrom .. import helpers\n",
		"pkg/user.py":         "\"\"\"User.\"\"\"\n\nfrom . import helpers\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"pkg/__init__.py"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string]any{}
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] == "reverse-import" {
			lines[stringValue(item["id"])] = mapsFromAny(item["evidence"])[0]["line"]
		}
	}
	if lines["pkg/sub/mod.py"] != 4 || lines["pkg/user.py"] != 3 {
		t.Fatalf("reverse-import lines = %v, want mod.py:4 and user.py:3", lines)
	}
	if line, ok := importEvidenceLine(index.Sources["pkg/user.py"], "other"); ok || line != 0 {
		t.Fatalf("unimported specifier = %d %v, want no line", line, ok)
	}
}

// webImpactFiles is the web tree the next three tests share: one changed
// component, a sibling importing it relatively, a nested importer reaching it
// through `..`, an importer naming its directory (which resolves through the
// `/index` child), and an importer using the profile's alias.
func webImpactFiles() map[string]string {
	return map[string]string{
		"go.mod":                                    "module example.test/fixture\n\ngo 1.27.0\n",
		"internal/web/app/src/widget/button.ts":     "export const Button = () => \"button\";\n",
		"internal/web/app/src/widget/index.ts":      "export * from \"./button\";\n",
		"internal/web/app/src/pages/home.ts":        "import { Button } from \"../widget/button\";\nexport const Home = Button;\n",
		"internal/web/app/src/pages/deep/detail.ts": "import { Button } from \"../../widget/button\";\nexport const Detail = Button;\n",
		"internal/web/app/src/pages/directory.ts":   "import { Button } from \"../widget\";\nexport const Directory = Button;\n",
		"internal/web/app/src/pages/aliased.ts":     "import { Button } from \"@/widget/button\";\nexport const Aliased = Button;\n",
		"internal/web/app/src/pages/bare.ts":        "import { Button } from \"widget/button\";\nexport const Bare = Button;\n",
	}
}

// withBeamfallSignals adds the two ledgers `projectprofile.Detect` recognises
// the `beamfall` profile by, which is the only thing that makes the `@/` alias
// resolvable.
func withBeamfallSignals(files map[string]string) map[string]string {
	files["testing/features.yaml"] = "features: []\n"
	files["testing/scenarios.yaml"] = "scenarios: []\n"
	return files
}

// TestImpactResolvesWebReverseImporters is rule (c) for relative specifiers:
// a sibling, a two-level `..` climb, and a directory specifier that resolves
// through the `/index` child.
func TestImpactResolvesWebReverseImporters(t *testing.T) {
	root := impactRepositoryWithFiles(t, withBeamfallSignals(webImpactFiles()))
	found := impactReverseImportIDs(t, root, "internal/web/app/src/widget/button.ts")
	for id, reason := range map[string]string{
		"internal/web/app/src/pages/home.ts":        "imports ../widget/button",
		"internal/web/app/src/pages/deep/detail.ts": "imports ../../widget/button",
		"internal/web/app/src/widget/index.ts":      "imports ./button",
	} {
		if !found[id][reason] {
			t.Fatalf("reverse-import results = %v, want %s via `%s`", found, id, reason)
		}
	}
	if _, present := found["internal/web/app/src/pages/bare.ts"]; present {
		t.Fatalf("reverse-import results = %v: a bare specifier names a dependency, not a repository file", found)
	}
	index := impactReverseImportIDs(t, root, "internal/web/app/src/widget/index.ts")
	if !index["internal/web/app/src/pages/directory.ts"]["imports ../widget"] {
		t.Fatalf("index reverse-imports = %v, want pages/directory.ts via `imports ../widget`", index)
	}
}

// TestImpactWebAliasComesFromTheProjectProfile is the half of rule (c) that
// `GPK-V0-027` singles out. The oracle hardcodes `internal/web/app/src/` as the
// `@/` root and would therefore resolve an aliased specifier into that tree
// from any repository at all. The clause requires the prefix to be read from
// the project profile, so the SAME tree resolves `@/widget/button` when the
// profile that configures the alias is detected and resolves nothing when it is
// not -- while the relative specifiers, which need no configuration, resolve in
// both.
func TestImpactWebAliasComesFromTheProjectProfile(t *testing.T) {
	changed := "internal/web/app/src/widget/button.ts"
	aliased := "internal/web/app/src/pages/aliased.ts"
	relative := "internal/web/app/src/pages/home.ts"

	configured := impactReverseImportIDs(t, impactRepositoryWithFiles(t, withBeamfallSignals(webImpactFiles())), changed)
	if !configured[aliased]["imports @/widget/button"] {
		t.Fatalf("configured-profile reverse-imports = %v, want %s via `imports @/widget/button`", configured, aliased)
	}

	unconfigured := impactReverseImportIDs(t, impactRepositoryWithFiles(t, webImpactFiles()), changed)
	if _, present := unconfigured[aliased]; present {
		t.Fatalf("fallback-profile reverse-imports = %v: `@/` resolved with no profile configuring it", unconfigured)
	}
	if !unconfigured[relative]["imports ../widget/button"] {
		t.Fatalf("fallback-profile reverse-imports = %v, want %s still resolved: only the alias is profile-configured", unconfigured, relative)
	}
}

// TestWebImportTargetPortsTheOracle pins `webImportTarget` against
// `_web_import_target` (`src/context_corvint_index.py:1518-1524`) case by case,
// including the two the shared `path.Ext` helper would get wrong: a name whose
// only dot is its first character is a hidden file with no suffix, and a
// non-web extension is never stripped.
func TestWebImportTargetPortsTheOracle(t *testing.T) {
	beamfall := projectprofile.ByID("beamfall")
	fallback := projectprofile.Fallback()
	for _, test := range []struct {
		name, importer, imported, want string
		profile                        projectprofile.Profile
	}{
		{"sibling", "a/b/c.ts", "./d", "a/b/d", beamfall},
		{"parent", "a/b/c.ts", "../d/e", "a/d/e", beamfall},
		{"explicit web extension", "a/b/c.ts", "./d.tsx", "a/b/d", beamfall},
		{"uppercase web extension", "a/b/c.ts", "./d.TS", "a/b/d", beamfall},
		{"non-web extension kept", "a/b/c.ts", "./theme.css", "a/b/theme.css", beamfall},
		{"hidden file has no suffix", "a/b/c.ts", "./.ts", "a/b/.ts", beamfall},
		{"root importer", "c.ts", "./d", "d", beamfall},
		{"climbs above root", "a/c.ts", "../../d", "../d", beamfall},
		{"alias from profile", "a/b/c.ts", "@/widget/button", "internal/web/app/src/widget/button", beamfall},
		{"alias unconfigured is relative", "a/b/c.ts", "@/widget", "a/b/@/widget", fallback},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := webImportTarget(test.importer, test.imported, test.profile); got != test.want {
				t.Fatalf("webImportTarget(%q, %q) = %q, want %q", test.importer, test.imported, got, test.want)
			}
		})
	}
}

// TestImpactRuleNamedIsClosed pins the admitted suffix set. Decision 0007 (D2)
// widened `GPK-V0-027` to `.py` and the web set and explicitly declined to
// widen it into the languages admitted for indexing on the same day, because
// they have no oracle branch and any behaviour would be candidate-authored.
func TestImpactRuleNamedIsClosed(t *testing.T) {
	for _, named := range []string{"pkg/a.go", "src/a.py", "web/a.ts", "web/a.tsx", "web/a.js", "web/a.jsx", "web/a.mjs", "web/a.cjs", "web/a.TS"} {
		if !ImpactRuleNamed(named) {
			t.Fatalf("ImpactRuleNamed(%q) = false, want true", named)
		}
	}
	for _, unruled := range []string{"a/b.rs", "a/b.cs", "a/b.swift", "a/b.kt", "a/b.kts", "a/b.rb", "a/b.sql", "a/b.m", "a/b.md", "a/b.sh", "a/b", "a/.ts"} {
		if ImpactRuleNamed(unruled) {
			t.Fatalf("ImpactRuleNamed(%q) = true, want false", unruled)
		}
	}
}

// TestImpactAdmitsARootPackagePath is decision 0023: a `.go` file in the
// repository root is a package like any other, its import path is the module
// path, and an importer naming the module reaches it. Before the decision,
// `Impact` refused the path as `unsupported-impact-path` before any rule ran.
func TestImpactAdmitsARootPackagePath(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                "module example.test/fixture\n\ngo 1.27.0\n",
		"logger.go":             "package fixture\n\nfunc LoggerName() string { return \"logger\" }\n",
		"logger_test.go":        "package fixture\n\nfunc TestLoggerName() {}\n",
		"cmd/tool/main.go":      "package main\n\nimport \"example.test/fixture\"\n\nfunc main() { _ = fixture.LoggerName() }\n",
		"testing/features.yaml": "features: []\n",
	})
	found := impactReverseImportIDs(t, root, "logger.go")
	if !found["cmd/tool/main.go"]["imports example.test/fixture"] {
		t.Fatalf("reverse-import results = %v, want cmd/tool/main.go via the module path", found)
	}
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"logger.go"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	tests := make([]string, 0)
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] == "test" {
			tests = append(tests, stringValue(item["id"]))
		}
	}
	if len(tests) != 1 || tests[0] != "logger_test.go" {
		t.Fatalf("same-package tests = %v, want logger_test.go", tests)
	}
}
