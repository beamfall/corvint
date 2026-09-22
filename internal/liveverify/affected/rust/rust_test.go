package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestLanguageOwnsOnlyRustSource(t *testing.T) {
	language := New()
	if language.Name() != "rust" {
		t.Fatalf("name=%q", language.Name())
	}
	for _, relative := range []string{"src/lib.rs", "tests/browser.rs"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{"Cargo.toml", "features/login.feature", ".config/nextest.toml"} {
		if language.Owns(relative) {
			t.Fatalf("%s must not be owned", relative)
		}
	}
}

func TestWorkspacePackagesFormAddressableDependencyUnits(t *testing.T) {
	root := workspaceFixture(t)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{
		"rust:core/Cargo.toml",
		"rust:leaf/Cargo.toml",
		"rust:mid/Cargo.toml",
		"rust:solo/Cargo.toml",
	}
	if !equal(graph.UnitIDs(), wantIDs) {
		t.Fatalf("ids=%v want=%v", graph.UnitIDs(), wantIDs)
	}
	plan := affected.Select(graph, []string{"core/src/lib.rs"})
	wantTests := []string{"core/src/lib.rs", "leaf/tests/leaf.rs", "mid/src/lib.rs"}
	if got := plan.SelectedTests(); !equal(got, wantTests) {
		t.Fatalf("selected=%v want=%v", got, wantTests)
	}
	if plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	solo, ok := graph.Unit("rust:solo/Cargo.toml")
	if !ok || !equal(solo.Tests, []string{"solo/src/lib.rs"}) {
		t.Fatalf("solo=%+v ok=%v", solo, ok)
	}
}

func TestTestFormsAndDoctestsAreTestAnchors(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"forms\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "/// ```\n/// assert_eq!(2 + 2, 4);\n/// ```\npub fn value() {}\n")
	write(t, root, "src/async_case.rs", "#[tokio::test]\n#[ignore]\nasync fn waits() {}\n")
	write(t, root, "src/block_doc.rs", "/**\n * ```rust,no_run\n * forms::value();\n * ```\n */\npub fn documented() {}\n")
	write(t, root, "src/char_then_test.rs", "const QUOTE: char = '\"';\n#[test]\nfn after_quote() {}\n")
	write(t, root, "src/comment_then_test.rs", "// prints an unmatched \" and r#\"\n#[test]\nfn after_comment() {}\n")
	write(t, root, "src/commented_out.rs", "// #[test]\n// fn phantom() {}\npub fn real() {}\n")
	write(t, root, "src/panic_case.rs", "#[test]\n#[should_panic]\nfn panics() { panic!() }\n")
	write(t, root, "src/plain.rs", "pub fn plain() {}\n")
	write(t, root, "src/template.rs", "const TEMPLATE: &str = r#\"\n//! ```rust\n//! phantom();\n//! ```\n\"#;\n")
	write(t, root, "tests/browser.rs", "mod common;\n")
	write(t, root, "tests/common/mod.rs", "pub fn setup() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 {
		t.Fatalf("units=%d", len(result.Units))
	}
	unit := result.Units[0]
	wantTests := []string{"src/async_case.rs", "src/block_doc.rs", "src/char_then_test.rs", "src/comment_then_test.rs", "src/lib.rs", "src/panic_case.rs", "tests/browser.rs", "tests/common/mod.rs"}
	if !equal(unit.Tests, wantTests) {
		t.Fatalf("tests=%v want=%v", unit.Tests, wantTests)
	}
	if !equal(unit.Sources, []string{"src/commented_out.rs", "src/plain.rs", "src/template.rs"}) {
		t.Fatalf("sources=%v", unit.Sources)
	}
}

func TestTildeFencedDoctestIsATestAnchor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"tilde\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "/// ~~~\n/// assert_eq!(2 + 2, 4);\n/// ~~~\npub fn value() {}\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/lib.rs"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || !equal(got, []string{"src/lib.rs"}) {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, got, plan.Unknown)
	}
}

func TestFencedDoctestLanguageAndClosingFence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"fences\"\nversion = \"0.1.0\"\n")
	for name, info := range map[string]string{
		"bare":         "",
		"compile_fail": "compile_fail",
		"edition":      "edition2024",
		"ignore":       "ignore",
		"no_run":       "no_run",
		"rust":         "rust",
		"should_panic": "should_panic",
	} {
		write(t, root, "src/"+name+".rs", "/// ```"+info+"\n/// assert!(true);\npub fn value() {}\n")
	}
	for name, info := range map[string]string{
		"json":      "json",
		"rust_text": "rust,text",
		"shell":     "sh",
		"text":      "text",
	} {
		write(t, root, "src/"+name+".rs", "/// ```"+info+"\n/// not Rust\n/// ```\npub fn value() {}\n")
	}
	write(t, root, "src/quad_text.rs", "/// ````text\n/// ~~~~\n/// ```\n/// ```rust\n/// phantom();\n/// ```\n/// ````\npub fn value() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	unit := result.Units[0]
	wantTests := []string{"src/bare.rs", "src/compile_fail.rs", "src/edition.rs", "src/ignore.rs", "src/no_run.rs", "src/rust.rs", "src/should_panic.rs"}
	wantSources := []string{"src/json.rs", "src/quad_text.rs", "src/rust_text.rs", "src/shell.rs", "src/text.rs"}
	if !equal(unit.Tests, wantTests) || !equal(unit.Sources, wantSources) {
		t.Fatalf("tests=%v sources=%v", unit.Tests, unit.Sources)
	}
}

func TestIndentedDoctestIsATestAnchor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"indented\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "/// Adds numbers.\n///\n///     assert_eq!(2 + 2, 4);\npub fn value() {}\n")
	write(t, root, "src/block.rs", "/**\n *\tassert_eq!(2 + 2, 4);\n */\npub fn block() {}\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/lib.rs"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || !equal(got, []string{"src/block.rs", "src/lib.rs"}) {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, got, plan.Unknown)
	}
}

func TestIndentedTextOutsideACodeBlockIsNotADoctest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"prose\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/paragraph.rs", "/// Adds\n///     numbers.\npub fn paragraph() {}\n")
	write(t, root, "src/fenced.rs", "/// ```text\n///\n///     not rust\npub fn fenced() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	unit := result.Units[0]
	if len(unit.Tests) != 0 || !equal(unit.Sources, []string{"src/fenced.rs", "src/paragraph.rs"}) {
		t.Fatalf("unit=%+v", unit)
	}
}

func TestDoctestFalseLeavesDocumentationAsSource(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"no-doc\"\nversion = \"0.1.0\"\n[lib]\ndoctest = false\n")
	write(t, root, "src/lib.rs", "/// ```rust\n/// no_doc::call();\n/// ```\npub fn call() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	unit := result.Units[0]
	if len(unit.Tests) != 0 || !equal(unit.Sources, []string{"src/lib.rs"}) {
		t.Fatalf("unit=%+v", unit)
	}
}

func TestFrameworkBoundariesWidenInsteadOfClaimingCoverage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"e2e\"\nversion = \"0.1.0\"\n[dev-dependencies]\nthirtyfour = \"0.36\"\ncucumber = \"0.22\"\n")
	write(t, root, "src/lib.rs", "/// ```\n/// e2e::check();\n/// ```\npub fn check() {}\n")
	write(t, root, "tests/web.rs", "#[tokio::test]\nasync fn browser() {}\n")
	write(t, root, "features/login.feature", "Feature: login\n  Scenario: success\n")
	write(t, root, ".config/nextest.toml", "[profile.default]\nretries = 1\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{FrontierCucumberOwnership, FrontierNextestDoctest, FrontierWebDriverRuntime}
	if !equal(result.Frontier, want) {
		t.Fatalf("frontier=%v want=%v", result.Frontier, want)
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"features/login.feature"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s", plan.Scope)
	}
	assertUnknown(t, plan, affected.UnknownUnownedDirtyPath, "features/login.feature")
	assertUnknown(t, plan, affected.UnknownLanguageFrontier, FrontierCucumberOwnership)
}

func TestGeneratedAndConditionalTestsRaiseFrontiers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"dynamic\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "#[cfg(feature = \"slow\")]\nmod slow;\ninclude!(concat!(env!(\"OUT_DIR\"), \"/tests.rs\"));\nparameterized_test!(case);\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{FrontierConditionalCompilation, FrontierGeneratedSource, FrontierGeneratedTests}
	if !equal(result.Frontier, want) {
		t.Fatalf("frontier=%v want=%v", result.Frontier, want)
	}
	if !equal(result.Units[0].Tests, []string{"src/lib.rs"}) {
		t.Fatalf("unit=%+v", result.Units[0])
	}
}

func TestAliasedAttributesAndArbitraryMacrosCannotLookExcluded(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"generated\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/aliased.rs", "use tokio::test as case;\n#[case]\nasync fn aliased() {}\n")
	write(t, root, "src/cases.rs", "use generator::*;\nmake_cases! { one, two }\nvec![1, 2];\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(result.Frontier, []string{FrontierGeneratedTests}) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	unit := result.Units[0]
	if !equal(unit.Tests, []string{"src/aliased.rs"}) || !equal(unit.Sources, []string{"src/cases.rs"}) {
		t.Fatalf("unit=%+v", unit)
	}
}

func TestLifetimeBeforeApostropheDoesNotHideInlineTests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"message\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "pub const MSG: &'static str = \"can't parse\";\npub const QUOTE: char = '\\'';\n#[cfg(test)]\nmod tests {\n    #[test]\n    fn works() {}\n}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 || !equal(result.Units[0].Tests, []string{"src/lib.rs"}) {
		t.Fatalf("frontier=%v units=%+v", result.Frontier, result.Units)
	}
}

func TestLiteralStringsParseAndMalformedManifestWidens(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = 'literal-name'\nversion = '0.1.0'\nthis is not an assignment\n")
	write(t, root, "src/lib.rs", "#[test]\nfn works() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].ID != "rust:Cargo.toml" {
		t.Fatalf("units=%+v", result.Units)
	}
	if !equal(result.Frontier, []string{FrontierManifest}) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestNonDefaultCargoTestTargetsRaiseAFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"targets\"\nversion = \"0.1.0\"\n[lib]\ntest = false\n")
	write(t, root, "src/lib.rs", "#[test]\nfn disabled_by_manifest() {}\n")
	write(t, root, "examples/demo.rs", "#[test]\nfn example_test() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(result.Frontier, []string{FrontierTargetConfiguration}) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	unit := result.Units[0]
	if !equal(unit.Tests, []string{"src/lib.rs"}) || !equal(unit.Sources, []string{"examples/demo.rs"}) {
		t.Fatalf("unit=%+v", unit)
	}
}

func TestRenamedAndWorkspaceDependenciesResolve(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[workspace]\nmembers = [\"core\", \"app\"]\ndependencies.alias.package = \"core-lib\"\ndependencies.alias.path = \"core\"\n")
	write(t, root, "core/Cargo.toml", "[package]\nname = \"core-lib\"\nversion = \"0.1.0\"\n")
	write(t, root, "core/src/lib.rs", "pub fn core() {}\n")
	write(t, root, "app/Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n[dependencies]\nalias.workspace = true\n")
	write(t, root, "app/src/lib.rs", "#[test]\nfn app() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	app := findUnit(t, result.Units, "rust:app/Cargo.toml")
	if !equal(app.Imports, []string{"rust:core/Cargo.toml"}) {
		t.Fatalf("imports=%v", app.Imports)
	}
}

func TestWorkspaceWordInsideDependencyPathIsNotInherited(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[workspace]\nmembers = [\"workspace/core\", \"other\", \"app\"]\n[workspace.dependencies]\ncore = { package = \"other\", path = \"other\" }\n")
	write(t, root, "workspace/core/Cargo.toml", "[package]\nname = \"core\"\nversion = \"0.1.0\"\n")
	write(t, root, "workspace/core/src/lib.rs", "pub fn core() {}\n")
	write(t, root, "other/Cargo.toml", "[package]\nname = \"other\"\nversion = \"0.1.0\"\n")
	write(t, root, "other/src/lib.rs", "pub fn other() {}\n")
	write(t, root, "app/Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n[dependencies]\ncore = { path = \"../workspace/core\" }\n")
	write(t, root, "app/tests/app.rs", "#[test]\nfn works() {}\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"workspace/core/src/lib.rs"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || !equal(got, []string{"app/tests/app.rs"}) {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, got, plan.Unknown)
	}
}

func TestLegacyProjectTableIsAPackage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"root\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "pub fn root() {}\n")
	write(t, root, "crates/foo/Cargo.toml", "[project]\nname = \"foo\"\nversion = \"0.1.0\"\n")
	write(t, root, "crates/foo/src/lib.rs", "pub fn foo() {}\n")
	write(t, root, "crates/bar/Cargo.toml", "[package]\nname = \"bar\"\nversion = \"0.1.0\"\n[dependencies]\nfoo = { path = \"../foo\" }\n")
	write(t, root, "crates/bar/tests/bar.rs", "#[test]\nfn works() {}\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"crates/foo/src/lib.rs"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || !equal(got, []string{"crates/bar/tests/bar.rs"}) {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, got, plan.Unknown)
	}
}

func TestPackageWordInsideDependencyValueIsNotARename(t *testing.T) {
	root := t.TempDir()
	write(t, root, "packages/core/Cargo.toml", "[package]\nname = \"core\"\nversion = \"0.1.0\"\n")
	write(t, root, "packages/core/src/lib.rs", "pub fn core() {}\n")
	write(t, root, "packages/util/Cargo.toml", "[package]\nname = \"util-lib\"\nversion = \"0.1.0\"\n")
	write(t, root, "packages/util/src/lib.rs", "pub fn util() {}\n")
	write(t, root, "app/Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n[dependencies]\ncore = { path = \"../packages/core\", version = \"0.1\" }\nutil = { path = \"../packages/util\", package = \"util-lib\" }\n")
	write(t, root, "app/src/lib.rs", "#[test]\nfn app() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	app := findUnit(t, result.Units, "rust:app/Cargo.toml")
	if !equal(app.Imports, []string{"rust:packages/core/Cargo.toml", "rust:packages/util/Cargo.toml"}) {
		t.Fatalf("imports=%v", app.Imports)
	}
}

func TestUnderscoreDependencyTablesFormEdges(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/Cargo.toml", "[package]\nname = \"core\"\nversion = \"0.1.0\"\n")
	write(t, root, "core/src/lib.rs", "pub fn core() {}\n")
	write(t, root, "codegen/Cargo.toml", "[package]\nname = \"codegen\"\nversion = \"0.1.0\"\n")
	write(t, root, "codegen/src/lib.rs", "pub fn generate() {}\n")
	write(t, root, "app/Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n[dev_dependencies]\ncore = { path = \"../core\" }\n[target.x86_64-unknown-linux-gnu.build_dependencies]\ncodegen = { path = \"../codegen\" }\n")
	write(t, root, "app/src/lib.rs", "#[test]\nfn app() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	app := findUnit(t, result.Units, "rust:app/Cargo.toml")
	if !equal(app.Imports, []string{"rust:codegen/Cargo.toml", "rust:core/Cargo.toml"}) {
		t.Fatalf("imports=%v", app.Imports)
	}
}

func TestRepeatedExplicitTestTargetsAreNotManifestUncertainty(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"targets\"\nversion = \"0.1.0\"\n[[test]]\nname = \"first\"\npath = \"checks/first.rs\"\n[[test]]\nname = \"second\"\npath = \"checks/second.rs\"\n")
	write(t, root, "src/lib.rs", "pub fn value() {}\n")
	write(t, root, "checks/first.rs", "fn helper() {}\n")
	write(t, root, "checks/second.rs", "fn helper() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	if !equal(result.Units[0].Tests, []string{"checks/first.rs", "checks/second.rs"}) {
		t.Fatalf("tests=%v", result.Units[0].Tests)
	}
}

func TestStandardDerivesDoNotImplyGeneratedTests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"derive\"\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "#[derive(Clone, Debug, PartialEq)]\npub struct Value;\n#[test]\nfn value() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	write(t, root, "src/custom.rs", "#[derive(serde::Serialize)]\npub struct Custom;\n")
	write(t, root, "src/shadowed.rs", "use generator::*;\n#[derive(Clone)]\npub struct Shadowed;\n")
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(result.Frontier, []string{FrontierGeneratedTests}) {
		t.Fatalf("custom frontier=%v", result.Frontier)
	}
}

func TestDuplicateOrdinaryManifestTablesWiden(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"duplicate\"\n[package]\nversion = \"0.1.0\"\n")
	write(t, root, "src/lib.rs", "pub fn value() {}\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(result.Frontier, []string{FrontierManifest}) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestInheritedFrameworkDependenciesAndNestedNextestAreDetected(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[workspace]\nmembers = [\"browser\"]\n[workspace.dependencies]\nwebdriver = { package = \"fantoccini\", version = \"0.22\" }\nbdd = { package = \"cucumber\", version = \"0.22\" }\n")
	write(t, root, "browser/Cargo.toml", "[package]\nname = \"browser\"\nversion = \"0.1.0\"\n[dev-dependencies]\nwebdriver = { workspace = true }\nbdd = { workspace = true }\n")
	write(t, root, "browser/src/lib.rs", "/// ```\n/// browser::check();\n/// ```\npub fn check() {}\n")
	write(t, root, "browser/.config/nextest.toml", "[profile.default]\nretries = 1\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{FrontierCucumberOwnership, FrontierNextestDoctest, FrontierWebDriverRuntime}
	if !equal(result.Frontier, want) {
		t.Fatalf("frontier=%v want=%v", result.Frontier, want)
	}
}

func workspaceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[workspace]\nmembers = [\"core\", \"mid\", \"leaf\", \"solo\"]\n")
	write(t, root, "core/Cargo.toml", "[package]\nname = \"core\"\nversion = \"0.1.0\"\n")
	write(t, root, "core/src/lib.rs", "#[test]\nfn core_test() {}\n")
	write(t, root, "mid/Cargo.toml", "[package]\nname = \"mid\"\nversion = \"0.1.0\"\n[dependencies]\ncore = { path = \"../core\" }\n")
	write(t, root, "mid/src/lib.rs", "#[test]\nfn mid_test() {}\n")
	write(t, root, "leaf/Cargo.toml", "[package]\nname = \"leaf\"\nversion = \"0.1.0\"\n[dev-dependencies]\nmid = { path = \"../mid\" }\n")
	write(t, root, "leaf/src/lib.rs", "pub fn leaf() {}\n")
	write(t, root, "leaf/tests/leaf.rs", "#[test]\nfn leaf_test() {}\n")
	write(t, root, "solo/Cargo.toml", "[package]\nname = \"solo\"\nversion = \"0.1.0\"\n")
	write(t, root, "solo/src/lib.rs", "#[test]\nfn solo_test() {}\n")
	return root
}

func assertUnknown(t *testing.T, plan affected.Plan, reason, detail string) {
	t.Helper()
	for _, unknown := range plan.Unknown {
		if unknown.Reason == reason && unknown.Detail == detail {
			return
		}
	}
	t.Fatalf("missing unknown %s/%s in %+v", reason, detail, plan.Unknown)
}

func findUnit(t *testing.T, units []affected.Unit, id string) affected.Unit {
	t.Helper()
	for _, unit := range units {
		if unit.ID == id {
			return unit
		}
	}
	t.Fatalf("missing unit %s", id)
	return affected.Unit{}
}

func write(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func equal(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
