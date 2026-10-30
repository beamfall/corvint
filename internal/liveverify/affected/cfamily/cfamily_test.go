package cfamily

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsExactlyTheCFamilySuffixes_NSAV0001(t *testing.T) {
	positive := []string{
		"a.c", "a.h", "a.cc", "a.cpp", "a.cxx", "a.c++", "a.hh", "a.hpp", "a.hxx", "a.h++",
		"a.inl", "a.ipp", "a.tpp", "a.m", "a.mm", "deep/dir/jni_bridge.c",
	}
	negative := []string{
		"a.metal", "a.glsl", "a.hlsl", "a.wgsl", "a.swift", "a.sh", "a.java", "a.cs",
		"a.hs", "a.ccc", "a.mmm", "a.c.orig", "Makefile", "CMakeLists.txt", "a.html",
	}
	language := New()
	if language.Name() != "cfamily" {
		t.Fatalf("name=%s", language.Name())
	}
	for _, path := range positive {
		if !language.Owns(path) {
			t.Fatalf("%s should be owned", path)
		}
	}
	for _, path := range negative {
		if language.Owns(path) {
			t.Fatalf("%s should not be owned", path)
		}
	}
}

func TestTestPathFollowsConventions_NSAV0002(t *testing.T) {
	positive := []string{
		"test/a.c", "tests/a.cc", "Tests/A.m", "UnitTests/A.mm", "spec/a.cpp",
		"a_test.c", "a_tests.cc", "a_unittest.cpp", "ATest.m", "ATests.mm", "test_a.c",
	}
	negative := []string{"src/a.c", "latest/a.c", "contest.c", "attestation.cc", "Test.m", "testing/a.c"}
	for _, path := range positive {
		if !testPath(path) {
			t.Fatalf("%s should be a conventional test path", path)
		}
	}
	for _, path := range negative {
		if testPath(path) {
			t.Fatalf("%s should not be a conventional test path", path)
		}
	}
}

// Every test-signal family named by NSA-V0-002 makes a conventional test path a
// test; the same path without a signal, a header, or a signal outside a test
// path stays a source unit.
func TestEveryTestSignalFamilyMarksATestUnit_NSAV0002(t *testing.T) {
	families := map[string]string{
		"googletest":   "TEST(Suite, Name) { EXPECT_EQ(1, 1); }\n",
		"googletest_f": "TEST_F(Fixture, Name) {}\n",
		"googletest_p": "TEST_P(Param, Name) {}\n",
		"typed":        "TYPED_TEST(Typed, Name) {}\n",
		"catch2":       "TEST_CASE(\"adds\", \"[math]\") { REQUIRE(1 == 1); }\n",
		"scenario":     "SCENARIO(\"vectors grow\") {}\n",
		"boost":        "BOOST_AUTO_TEST_CASE(adds) { BOOST_CHECK(true); }\n",
		"cpputest":     "TEST_GROUP(Group) {};\n",
		"unity":        "int main(void) { UNITY_BEGIN(); RUN_TEST(test_adds); return UNITY_END(); }\n",
		"unity_only":   "void run(void) { RUN_TEST(test_adds); }\n",
		"check":        "START_TEST(test_adds) { ck_assert(1); } END_TEST\n",
		"cunit":        "void reg(CU_pSuite s) { CU_add_test(s, \"adds\", adds); }\n",
		"cmocka":       "const struct CMUnitTest tests[] = { cmocka_unit_test(adds) };\n",
		"ctest_main":   "int main(int argc, char **argv) { return 0; }\n",
	}
	root := t.TempDir()
	for name, body := range families {
		write(t, root, "tests/"+name+".cc", body)
		write(t, root, "src/"+name+".cc", body)
		write(t, root, "tests/"+name+"_helper.h", body)
	}
	write(t, root, "tests/plain.c", "int helper(void) { return 1; }\n")
	write(t, root, "tests/XCTFoo.m", "@interface FooTests : XCTestCase\n@end\n")
	write(t, root, "tests/xct_in_c.c", "/* not objc */ int x;\n@interface FooTests : XCTestCase\n")
	result := units(t, root)
	for name := range families {
		assertTest(t, result, "tests/"+name+".cc", true)
		assertTest(t, result, "src/"+name+".cc", false)
		assertTest(t, result, "tests/"+name+"_helper.h", false)
	}
	assertTest(t, result, "tests/plain.c", false)
	assertTest(t, result, "tests/XCTFoo.m", true)
	assertTest(t, result, "tests/xct_in_c.c", false)
}

// An XCTest suite in Objective-C reaches the implementation behind the header
// it imports.
func TestXCTestObjectiveCFixtureSelectsThroughItsHeader(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Sources/Widget.h", "#import <Foundation/Foundation.h>\n@interface Widget : NSObject\n- (int)value;\n@end\n")
	write(t, root, "Sources/Widget.m", "#import \"Widget.h\"\n@implementation Widget\n- (int)value { return 1; }\n@end\n")
	write(t, root, "Tests/WidgetTests.m", "#import <XCTest/XCTest.h>\n#import \"../Sources/Widget.h\"\n"+
		"@interface WidgetTests : XCTestCase\n@end\n@implementation WidgetTests\n"+
		"- (void)testValue { XCTAssertEqual([[Widget new] value], 1); }\n@end\n")
	write(t, root, "Tests/OtherTests.m", "#import <XCTest/XCTest.h>\n@interface OtherTests : XCTestCase\n@end\n")
	plan := selectFor(t, root, "Sources/Widget.m")
	if got := plan.SelectedTests(); !equal(got, []string{"Tests/WidgetTests.m"}) {
		t.Fatalf("selected=%v", got)
	}
	assertUnknown(t, plan, FrontierLinkGraph)
}

// A dirty implementation reaches every test that includes its header, through
// the header's counterpart edge, in both the flat and the include/src layout.
func TestHeaderCounterpartReachesTestsIncludingTheHeader_NSAV0003(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/foo.h", "int foo(void);\n")
	write(t, root, "lib/foo.c", "#include \"foo.h\"\nint foo(void) { return 1; }\n")
	write(t, root, "lib/bar.h", "int bar(void);\n")
	write(t, root, "lib/bar.cpp", "#include \"bar.h\"\nint bar(void) { return 2; }\n")
	write(t, root, "tests/foo_test.cc", "#include \"lib/foo.h\"\nTEST(Foo, One) {}\n")
	write(t, root, "tests/bar_test.cc", "#include \"lib/bar.h\"\nTEST(Bar, Two) {}\n")
	write(t, root, "pkg/include/pkg/api.hpp", "int api();\n")
	write(t, root, "pkg/src/pkg/api.cpp", "#include \"pkg/api.hpp\"\nint api() { return 3; }\n")
	write(t, root, "pkg/include/pkg/flat.hpp", "int flat();\n")
	write(t, root, "pkg/src/flat.cpp", "#include <pkg/flat.hpp>\nint flat() { return 4; }\n")
	write(t, root, "pkg/tests/api_test.cpp", "#include <pkg/api.hpp>\n#include <pkg/flat.hpp>\nTEST_CASE(\"api\") {}\n")

	if got := selectFor(t, root, "lib/foo.c").SelectedTests(); !equal(got, []string{"tests/foo_test.cc"}) {
		t.Fatalf("flat counterpart selected=%v", got)
	}
	if got := selectFor(t, root, "pkg/src/pkg/api.cpp").SelectedTests(); !equal(got, []string{"pkg/tests/api_test.cpp"}) {
		t.Fatalf("mirrored include/src counterpart selected=%v", got)
	}
	if got := selectFor(t, root, "pkg/src/flat.cpp").SelectedTests(); !equal(got, []string{"pkg/tests/api_test.cpp"}) {
		t.Fatalf("flat include/src counterpart selected=%v", got)
	}
	header := unitByID(t, units(t, root), "cfamily:lib/foo.h")
	if !equal(header.Imports, []string{"cfamily:lib/foo.c"}) {
		t.Fatalf("header imports=%v", header.Imports)
	}
}

// A directive continued with a backslash is spliced before it is read.
func TestContinuedIncludeKeepsItsEdge_NSAV0003(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core.h", "int core(void);\n")
	write(t, root, "core.c", "#include \"core.h\"\nint core(void) { return 0; }\n")
	write(t, root, "tests/core_test.c", "#include \\\n\"../core.h\"\nint main(void) { return core(); }\n")
	write(t, root, "tests/crlf_test.c", "#include \\\r\n\"../core.h\"\r\nint main(void) { return core(); }\r\n")
	want := []string{"tests/core_test.c", "tests/crlf_test.c"}
	if got := selectFor(t, root, "core.c").SelectedTests(); !equal(got, want) {
		t.Fatalf("selected=%v want %v", got, want)
	}
}

func TestIncludeResolutionFollowsTheSpecifiedSearch_NSAV0003(t *testing.T) {
	root := t.TempDir()
	// Directory-relative wins over the root-relative candidate of the same name.
	write(t, root, "a/util.h", "")
	write(t, root, "util.h", "")
	write(t, root, "a/user.c", "#include \"util.h\"\n")
	// Root-relative and ancestor include/src directories.
	write(t, root, "core/config.h", "")
	write(t, root, "mod/include/mod/types.h", "")
	write(t, root, "mod/src/inner/impl.c", "#include \"core/config.h\"\n#include \"mod/types.h\"\n")
	// Angle includes resolve only to a unique path suffix.
	write(t, root, "vendored/once/only.h", "")
	write(t, root, "x/dup.h", "")
	write(t, root, "y/dup.h", "")
	write(t, root, "angle.c", "#include <once/only.h>\n#include <dup.h>\n#include <stdio.h>\n")
	// An include of a repository file no plugin owns resolves without an edge.
	write(t, root, "gen/table.def", "X(a)\n")
	write(t, root, "gen/table.c", "#include \"table.def\"\n")
	// Includes in comments and strings are not directives.
	write(t, root, "quiet.c", "// #include \"missing_a.h\"\n/* #include \"missing_b.h\" */\nconst char *s = \"#include \\\"missing_c.h\\\"\";\n")

	result := units(t, root)
	assertImports(t, result, "cfamily:a/user.c", "cfamily:a/util.h")
	assertImports(t, result, "cfamily:mod/src/inner/impl.c", "cfamily:core/config.h", "cfamily:mod/include/mod/types.h")
	assertImports(t, result, "cfamily:angle.c", "cfamily:vendored/once/only.h")
	assertImports(t, result, "cfamily:gen/table.c")
	assertImports(t, result, "cfamily:quiet.c")
	if !equal(result.Frontier, []string{FrontierLinkGraph}) {
		t.Fatalf("a fully resolved tree raised frontier %v", result.Frontier)
	}
}

func TestEveryFrontierReasonIsRaised_NSAV0004(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		reason string
		check  func(t *testing.T, result affected.Result)
	}{
		{
			name: "ambiguous",
			files: map[string]string{
				"one/src/common.h":     "",
				"one/include/common.h": "",
				"one/deep/user.c":      "#include \"common.h\"\n",
			},
			reason: FrontierAmbiguousInclude,
			check: func(t *testing.T, result affected.Result) {
				assertImports(t, result, "cfamily:one/deep/user.c", "cfamily:one/include/common.h", "cfamily:one/src/common.h")
			},
		},
		{
			name: "unresolved",
			files: map[string]string{
				"elsewhere/lib/thing.h": "",
				"app/main.c":            "#include \"lib/thing.h\"\n#include \"nowhere.h\"\n",
			},
			reason: FrontierUnresolvedInclude,
			check: func(t *testing.T, result affected.Result) {
				// The unique suffix match is still kept as an edge.
				assertImports(t, result, "cfamily:app/main.c", "cfamily:elsewhere/lib/thing.h")
			},
		},
		{
			name:   "macro",
			files:  map[string]string{"m.c": "#define HDR \"x.h\"\n#include HDR\n"},
			reason: FrontierMacroInclude,
		},
		{
			name:   "unreadable",
			files:  map[string]string{"bad.h": "", "latin1.c": "/* caf\xe9 */\n#include \"bad.h\"\n"},
			reason: FrontierUnreadableSource,
			check: func(t *testing.T, result affected.Result) {
				// Undecodable text is still scanned for directives.
				assertImports(t, result, "cfamily:latin1.c", "cfamily:bad.h")
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			for relative, body := range testCase.files {
				write(t, root, relative, body)
			}
			result := units(t, root)
			if !contains(result.Frontier, testCase.reason) || !contains(result.Frontier, FrontierLinkGraph) {
				t.Fatalf("frontier=%v want %s and %s", result.Frontier, testCase.reason, FrontierLinkGraph)
			}
			if testCase.check != nil {
				testCase.check(t, result)
			}
		})
	}
}

// The JNI boundary is a unit frontier: it widens a plan only when the JNI unit
// is reached.
func TestJNIBoundaryIsAUnitFrontier_NSAV0004(t *testing.T) {
	root := t.TempDir()
	write(t, root, "jni/bridge.c", "#include <jni.h>\nJNIEXPORT jint JNICALL Java_Foo_bar(JNIEnv *env, jobject self) { return 1; }\n")
	write(t, root, "solo/solo.c", "int solo(void) { return 0; }\n")
	write(t, root, "solo/solo_test.c", "int main(void) { return 0; }\n")
	write(t, root, "commented.c", "/* JNIEXPORT */ int x;\n")
	result := units(t, root)
	bridge := unitByID(t, result, "cfamily:jni/bridge.c")
	if !equal(bridge.Frontier, []string{FrontierJNIBoundary}) {
		t.Fatalf("bridge frontier=%v", bridge.Frontier)
	}
	if contains(result.Frontier, FrontierJNIBoundary) {
		t.Fatalf("jni boundary leaked into the global frontier %v", result.Frontier)
	}
	if commented := unitByID(t, result, "cfamily:commented.c"); len(commented.Frontier) != 0 {
		t.Fatalf("a commented JNIEXPORT raised %v", commented.Frontier)
	}
	assertUnknown(t, selectFor(t, root, "jni/bridge.c"), FrontierJNIBoundary)
	for _, unknown := range selectFor(t, root, "solo/solo.c").Unknown {
		if unknown.Detail == FrontierJNIBoundary {
			t.Fatal("an unreached JNI unit widened the plan")
		}
	}
}

// The link graph frontier keeps every plan cfamily takes part in UNKNOWN, even
// one that selects its tests, and a changed unit no test reaches is named.
func TestLinkGraphFrontierAndUnreachedChangedUnit_NSAV0005(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core.h", "int core(void);\n")
	write(t, root, "core.c", "#include \"core.h\"\nint core(void) { return 1; }\n")
	write(t, root, "tests/core_test.c", "#include \"../core.h\"\nint main(void) { return core() - 1; }\n")
	write(t, root, "lonely.c", "int lonely(void) { return 0; }\n")
	selected := selectFor(t, root, "core.c")
	if got := selected.SelectedTests(); !equal(got, []string{"tests/core_test.c"}) {
		t.Fatalf("selected=%v", got)
	}
	assertUnknown(t, selected, FrontierLinkGraph)
	lonely := selectFor(t, root, "lonely.c")
	found := false
	for _, unknown := range lonely.Unknown {
		found = found || (unknown.Reason == affected.UnknownNoSelectableTest && unknown.Detail == "cfamily:lonely.c")
	}
	if !found || lonely.Scope != affected.ScopeUnknown {
		t.Fatalf("unreached changed unit not named: %+v", lonely.Unknown)
	}
	empty := t.TempDir()
	write(t, empty, "README.md", "no native code\n")
	if result := units(t, empty); len(result.Units) != 0 || len(result.Frontier) != 0 {
		t.Fatalf("a tree with no C-family file reported %+v", result)
	}
}

func TestScrubBlanksCommentsAndLiteralContents(t *testing.T) {
	body := "int a = 1'000; // TEST(x, y)\n" +
		"const char *s = \"TEST_CASE(\\\"q\\\") // not a comment\";\n" +
		"auto r = R\"tag(RUN_TEST(x) )\" still raw)tag\";\n" +
		"char c = '\"'; /* START_TEST(\n multi */ int b;\n" +
		"// continued \\\n CU_add_test(s)\n"
	code, masked := scrub([]byte(body))
	if len(code) != len(body) || len(masked) != len(body) {
		t.Fatal("scrub changed the text length")
	}
	if hasTestSignal("tests/x.c", masked) {
		t.Fatalf("a signal survived masking:\n%s", masked)
	}
	if !strings.Contains(string(code), "not a comment") {
		t.Fatalf("code view lost string contents:\n%s", code)
	}
	if strings.Contains(string(code), "TEST(x, y)") || strings.Contains(string(code), "multi") {
		t.Fatalf("code view kept a comment:\n%s", code)
	}
	if !strings.Contains(string(masked), "int b;") {
		t.Fatalf("masking ran past a character literal:\n%s", masked)
	}
}

// NSA-V0-014: two builds, from disk and from an immutable filesystem, produce
// identical graph bytes, and units carry sorted unique lists.
func TestGraphIsDeterministic_NSAV0014(t *testing.T) {
	root := t.TempDir()
	write(t, root, "b.h", "#include \"a.h\"\n#include \"a.h\"\n")
	write(t, root, "a.h", "#include \"b.h\"\n#include \"a.h\"\n")
	write(t, root, "a.c", "#include \"b.h\"\n#include \"a.h\"\n")
	write(t, root, "tests/a_test.cc", "#include \"../a.h\"\n#include \"../b.h\"\nTEST(A, B) {}\n")
	first, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	second, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	immutable, err := affected.BuildFS(boundedDirFS{os.DirFS(root)}, New())
	if err != nil {
		t.Fatal(err)
	}
	firstBody, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBody) != string(secondBody) || first.Digest() != immutable.Digest() {
		t.Fatalf("graph drift:\n%s\n%s", firstBody, secondBody)
	}
	assertImports(t, units(t, root), "cfamily:a.h", "cfamily:a.c", "cfamily:b.h")
}

func TestUnitsHonoursCancellation_NSAV0014(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.c", "int a;\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Units(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}

func units(t *testing.T, root string) affected.Result {
	t.Helper()
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func selectFor(t *testing.T, root, dirty string) affected.Plan {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return affected.Select(graph, []string{dirty})
}

func assertTest(t *testing.T, result affected.Result, relative string, want bool) {
	t.Helper()
	unit := unitByID(t, result, "cfamily:"+relative)
	if got := len(unit.Tests) == 1 && unit.Tests[0] == relative; got != want {
		t.Fatalf("%s test=%v want %v (%+v)", relative, got, want, unit)
	}
	if want == (len(unit.Sources) != 0) {
		t.Fatalf("%s sources=%v", relative, unit.Sources)
	}
}

func assertImports(t *testing.T, result affected.Result, id string, want ...string) {
	t.Helper()
	if got := unitByID(t, result, id).Imports; !equal(got, want) {
		t.Fatalf("%s imports=%v want %v", id, got, want)
	}
}

func assertUnknown(t *testing.T, plan affected.Plan, reason string) {
	t.Helper()
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s want UNKNOWN", plan.Scope)
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == reason {
			return
		}
	}
	t.Fatalf("plan does not name %s: %+v", reason, plan.Unknown)
}

func unitByID(t *testing.T, result affected.Result, id string) affected.Unit {
	t.Helper()
	for _, unit := range result.Units {
		if unit.ID == id {
			return unit
		}
	}
	t.Fatalf("no unit %s", id)
	return affected.Unit{}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
