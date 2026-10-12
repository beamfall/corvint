package java

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func write(t *testing.T, root, relative, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func build(t *testing.T, root string) *affected.Graph {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func unit(t *testing.T, graph *affected.Graph, relative string) affected.Unit {
	t.Helper()
	found, ok := graph.Unit("java:" + relative)
	if !ok {
		t.Fatalf("no unit for %s in %v", relative, graph.UnitIDs())
	}
	return found
}

const junit = "import org.junit.jupiter.api.Test;\n"

func TestOwnershipAndTestDiscovery_JATV0001_JATV0002(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pom.xml", "<project/>\n")
	write(t, root, "src/main/java/p/Core.java", "package p;\npublic class Core { void test() {} }\n")
	write(t, root, "src/test/java/p/CoreTest.java", "package p;\n"+junit+"class CoreTest { @Test void works() { new Core(); } }\n")
	write(t, root, "src/test/java/p/Helper.java", "package p;\nclass Helper { Core core; }\n")
	write(t, root, "src/test/java/p/LegacyTest.java", "package p;\nimport junit.framework.TestCase;\npublic class LegacyTest extends TestCase {}\n")
	write(t, root, "src/test/java/p/NgTest.java", "package p;\nimport org.testng.annotations.Test;\npublic class NgTest { @Test public void works() {} }\n")
	write(t, root, "src/test/java/p/ParamsTest.java", "package p;\nclass ParamsTest { @org.junit.jupiter.params.ParameterizedTest void works() {} }\n")
	write(t, root, "src/integrationTest/java/p/Flow.java", "package p;\n"+junit+"class Flow { @Test void works() {} }\n")
	write(t, root, "src/main/java/p/Demo.java", "package p;\n"+junit+"class Demo { @Test void notRunByAnyRunner() {} }\n")
	graph := build(t, root)
	if len(graph.UnitIDs()) != 8 {
		t.Fatalf("units=%v", graph.UnitIDs())
	}
	if _, owned := graph.OwnerOf("pom.xml"); owned {
		t.Fatal("a build file is owned")
	}
	for relative, wantTest := range map[string]bool{
		"src/main/java/p/Core.java":            false,
		"src/test/java/p/CoreTest.java":        true,
		"src/test/java/p/Helper.java":          false,
		"src/test/java/p/LegacyTest.java":      true,
		"src/test/java/p/NgTest.java":          true,
		"src/test/java/p/ParamsTest.java":      true,
		"src/integrationTest/java/p/Flow.java": true,
		"src/main/java/p/Demo.java":            false,
	} {
		got := unit(t, graph, relative)
		if (len(got.Tests) == 1) != wantTest {
			t.Errorf("%s test=%v want %v", relative, got.Tests, wantTest)
		}
	}
	plan := affected.Select(graph, []string{"pom.xml"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("a dirty build file left scope %s", plan.Scope)
	}
}

func TestImportFormsResolveToDeclaringFiles_JATV0003(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/src/main/java/com/ex/model/Point.java", "package com.ex.model;\npublic class Point { public static class Axis {} public static int ORIGIN = 0; }\n")
	write(t, root, "a/src/main/java/com/ex/model/Unused.java", "package com.ex.model;\npublic class Unused {}\n")
	write(t, root, "a/src/main/java/com/ex/util/Strings.java", "package com.ex.util;\npublic final class Strings { public static String trim(String s) { return s; } }\n")
	write(t, root, "a/src/main/java/com/ex/util/Peer.java", "package com.ex.util;\nclass Peer { Strings strings; }\n")
	write(t, root, "a/src/main/java/com/ex/svc/Fqn.java", "package com.ex.svc;\nclass Fqn { Object o = new com.ex.util.Strings(); }\n")
	write(t, root, "a/src/main/java/com/ex/svc/Named.java", "package com.ex.svc;\nclass Named { Object o = Class.forName(\"com.ex.model.Unused\"); }\n")
	write(t, root, "b/src/test/java/com/ex/NestedTest.java", "package com.ex;\nimport com.ex.model.Point.Axis;\n"+junit+"class NestedTest { @Test void t() { new Axis(); } }\n")
	write(t, root, "b/src/test/java/com/ex/StaticTest.java", "package com.ex;\nimport static com.ex.model.Point.ORIGIN;\n"+junit+"class StaticTest { @Test void t() { int o = ORIGIN; } }\n")
	write(t, root, "b/src/test/java/com/ex/WildTest.java", "package com.ex;\nimport com.ex.model.*;\n"+junit+"class WildTest { @Test void t() { new Point(); } }\n")
	graph := build(t, root)
	for relative, want := range map[string][]string{
		"a/src/main/java/com/ex/util/Peer.java":    {"java:a/src/main/java/com/ex/util/Strings.java"},
		"a/src/main/java/com/ex/svc/Fqn.java":      {"java:a/src/main/java/com/ex/util/Strings.java"},
		"a/src/main/java/com/ex/svc/Named.java":    {"java:a/src/main/java/com/ex/model/Unused.java"},
		"b/src/test/java/com/ex/NestedTest.java":   {"java:a/src/main/java/com/ex/model/Point.java"},
		"b/src/test/java/com/ex/StaticTest.java":   {"java:a/src/main/java/com/ex/model/Point.java"},
		"b/src/test/java/com/ex/WildTest.java":     {"java:a/src/main/java/com/ex/model/Point.java"},
		"a/src/main/java/com/ex/model/Unused.java": nil,
	} {
		if got := unit(t, graph, relative).Imports; !slices.Equal(got, want) {
			t.Errorf("%s imports=%v want %v", relative, got, want)
		}
	}
	plan := affected.Select(graph, []string{"a/src/main/java/com/ex/model/Point.java"})
	want := []string{"b/src/test/java/com/ex/NestedTest.java", "b/src/test/java/com/ex/StaticTest.java", "b/src/test/java/com/ex/WildTest.java"}
	if got := plan.SelectedTests(); !slices.Equal(got, want) || plan.Scope != affected.ScopeBounded {
		t.Fatalf("selected=%v scope=%s unknown=%v", got, plan.Scope, plan.Unknown)
	}
	for _, selection := range plan.Selected {
		if selection.Witness.Kind != affected.WitnessDependency {
			t.Fatalf("%s witness=%+v", selection.UnitID, selection.Witness)
		}
	}
}

func TestCommentsAndStringsDoNotCreateEdges_JATV0004(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/Core.java", "package p;\npublic class Core {}\n")
	write(t, root, "src/main/java/p/Quiet.java", "package p;\n// Core\n/* Core */\nclass Quiet { String s = \"Core\"; String block = \"\"\"\n  Core\n  \"\"\"; char c = '\"'; }\n")
	graph := build(t, root)
	if got := unit(t, graph, "src/main/java/p/Quiet.java").Imports; len(got) != 0 {
		t.Fatalf("imports=%v", got)
	}
	write(t, root, "src/main/java/p/Broken.java", "package p;\n/* never closed\nclass Broken {}\n")
	graph = build(t, root)
	if !slices.Contains(graph.Frontier(), FrontierUnparsedSource) {
		t.Fatalf("frontier=%v", graph.Frontier())
	}
}

func TestDuplicateTypeKeepsEveryEdgeAndRaisesAmbiguity_JATV0005(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/src/main/java/p/Dup.java", "package p;\npublic class Dup {}\n")
	write(t, root, "b/src/main/java/p/Dup.java", "package p;\npublic class Dup {}\n")
	write(t, root, "c/src/test/java/q/DupTest.java", "package q;\nimport p.Dup;\n"+junit+"class DupTest { @Test void t() { new Dup(); } }\n")
	graph := build(t, root)
	if got := unit(t, graph, "c/src/test/java/q/DupTest.java").Imports; len(got) != 2 {
		t.Fatalf("imports=%v", got)
	}
	if !slices.Contains(graph.Frontier(), FrontierAmbiguousType) {
		t.Fatalf("frontier=%v", graph.Frontier())
	}
}

func TestUnresolvedFirstPartyImportAndReflectionAreUnbounded_JATV0006_JATV0007(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/com/ex/app/App.java", "package com.ex.app;\nimport com.ex.gen.Generated;\nimport java.util.List;\nimport org.slf4j.Logger;\npublic class App { Generated g; }\n")
	write(t, root, "src/main/java/com/ex/app/Plugins.java", "package com.ex.app;\nimport java.util.ServiceLoader;\nclass Plugins { Object load(String n) throws Exception { return Class.forName(n); } }\n")
	write(t, root, "src/main/java/com/ex/app/Plain.java", "package com.ex.app;\nimport java.util.List;\nclass Plain { Object o = Class.forName(\"com.ex.app.App\"); }\n")
	write(t, root, "src/main/java/com/ex/other/Lone.java", "package com.ex.other;\npublic class Lone {}\n")
	write(t, root, "src/test/java/com/ex/app/AppTest.java", "package com.ex.app;\n"+junit+"class AppTest { @Test void t() { new App(); } }\n")
	write(t, root, "src/test/java/com/ex/app/GenTest.java", "package com.ex.app;\nimport com.ex.gen.Fixture;\n"+junit+"class GenTest { @Test void t() {} }\n")
	write(t, root, "src/test/java/com/ex/other/LoneTest.java", "package com.ex.other;\n"+junit+"class LoneTest { @Test void t() { new Lone(); } }\n")
	graph := build(t, root)
	for relative, want := range map[string]struct {
		reads  string
		locate bool
	}{
		"src/main/java/com/ex/app/App.java":        {UnresolvedImport, true},
		"src/main/java/com/ex/app/Plugins.java":    {Reflection, true},
		"src/main/java/com/ex/app/Plain.java":      {"", false},
		"src/test/java/com/ex/app/GenTest.java":    {UnresolvedImport, false},
		"src/test/java/com/ex/other/LoneTest.java": {"", false},
	} {
		got := unit(t, graph, relative)
		if got.UnboundedReads != want.reads || got.LocatesRoot != want.locate {
			t.Errorf("%s reads=%q locates=%v want %q %v", relative, got.UnboundedReads, got.LocatesRoot, want.reads, want.locate)
		}
	}
	// A dirty path the graph cannot connect still selects the tests that reach
	// an unbounded reader, and the unrelated test is not selected through it.
	plan := affected.Select(graph, []string{"src/main/java/com/ex/other/Lone.java"})
	for _, want := range []string{"src/test/java/com/ex/app/AppTest.java", "src/test/java/com/ex/app/GenTest.java", "src/test/java/com/ex/other/LoneTest.java"} {
		if !slices.Contains(plan.SelectedTests(), want) {
			t.Errorf("selected=%v misses %s", plan.SelectedTests(), want)
		}
	}
}

func TestKotlinReferencedJavaCarriesUnitFrontier_JATV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/com/ex/bridge/Bridge.java", "package com.ex.bridge;\npublic class Bridge {}\n")
	write(t, root, "src/main/java/com/ex/bridge/Alone.java", "package com.ex.bridge;\npublic class Alone {}\n")
	write(t, root, "src/main/java/com/ex/bridge/Aliased.java", "package com.ex.bridge;\npublic class Aliased {}\n")
	write(t, root, "src/test/java/com/ex/bridge/AloneTest.java", "package com.ex.bridge;\n"+junit+"class AloneTest { @Test void t() { new Alone(); } }\n")
	write(t, root, "src/test/kotlin/com/ex/app/AppTest.kt", "package com.ex.app\nimport com.ex.bridge.Bridge\nimport com.ex.bridge.Aliased as Other\nclass AppTest { val b = Bridge() }\n")
	graph := build(t, root)
	for relative, want := range map[string]bool{
		"src/main/java/com/ex/bridge/Bridge.java":  true,
		"src/main/java/com/ex/bridge/Aliased.java": true,
		"src/main/java/com/ex/bridge/Alone.java":   false,
	} {
		if got := slices.Contains(unit(t, graph, relative).Frontier, FrontierKotlinDependent); got != want {
			t.Errorf("%s kotlin-dependent=%v want %v", relative, got, want)
		}
	}
	if plan := affected.Select(graph, []string{"src/main/java/com/ex/bridge/Bridge.java"}); plan.Scope != affected.ScopeUnknown {
		t.Fatalf("Kotlin-referenced change scope=%s", plan.Scope)
	}
	if plan := affected.Select(graph, []string{"src/main/java/com/ex/bridge/Alone.java"}); plan.Scope != affected.ScopeBounded {
		t.Fatalf("unreferenced change scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestGraphIsDeterministic_JATV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/A.java", "package p;\npublic class A { B b; }\n")
	write(t, root, "src/main/java/p/B.java", "package p;\npublic class B { A a; }\n")
	write(t, root, "src/test/java/p/ATest.java", "package p;\n"+junit+"class ATest { @Test void t() { new A(); } }\n")
	first, second := build(t, root), build(t, root)
	if first.Digest() != second.Digest() {
		t.Fatalf("digest drift %s %s", first.Digest(), second.Digest())
	}
	plan := affected.Select(first, []string{"src/main/java/p/B.java"})
	if got := plan.SelectedTests(); !slices.Equal(got, []string{"src/test/java/p/ATest.java"}) {
		t.Fatalf("selected=%v", got)
	}
}

// The review cases below each pin one form that an earlier lexer missed.

func TestSpacedQualifiedNamesAndForNameBinaryNames_JATV0003(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/Core.java", "package p;\npublic class Core {}\n")
	write(t, root, "src/main/java/com/ex/core/Deep.java", "package com.ex.core;\npublic class Deep {}\n")
	write(t, root, "src/main/java/com/ex/core/Outer.java", "package com.ex.core;\npublic class Outer { public static class Inner {} }\n")
	write(t, root, "src/main/java/q/Short.java", "package q;\nclass Short { Object o = new p.Core(); }\n")
	write(t, root, "src/main/java/q/Spaced.java", "package q;\nclass Spaced { com . ex . core . Deep d; }\n")
	write(t, root, "src/main/java/q/Commented.java", "package q;\nclass Commented { com.ex.core./*c*/Deep d; }\n")
	write(t, root, "src/main/java/q/Binary.java", "package q;\nclass Binary { Object o = Class.forName(\"com.ex.core.Outer$Inner\"); }\n")
	graph := build(t, root)
	for relative, want := range map[string]string{
		"src/main/java/q/Short.java":     "java:src/main/java/p/Core.java",
		"src/main/java/q/Spaced.java":    "java:src/main/java/com/ex/core/Deep.java",
		"src/main/java/q/Commented.java": "java:src/main/java/com/ex/core/Deep.java",
		"src/main/java/q/Binary.java":    "java:src/main/java/com/ex/core/Outer.java",
	} {
		got := unit(t, graph, relative)
		if !slices.Equal(got.Imports, []string{want}) {
			t.Errorf("%s imports=%v want %s", relative, got.Imports, want)
		}
		if got.UnboundedReads != "" {
			t.Errorf("%s reads=%q", relative, got.UnboundedReads)
		}
	}
}

func TestUnicodeEscapesAndCarriageReturnsEndComments_JATV0004(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/Core.java", "package p;\npublic class Core {}\n")
	write(t, root, "src/main/java/p/Escaped.java", "package p;\nclass Escaped {\n// hidden \\u000a Core core;\n}\n")
	write(t, root, "src/main/java/p/Classic.java", "package p;\rclass Classic {\r// note\rCore core;\r}\r")
	write(t, root, "src/main/java/p/Doubled.java", "package p;\nclass Doubled {\n// \\\\u000a Core core;\n}\n")
	graph := build(t, root)
	for relative, want := range map[string]int{
		"src/main/java/p/Escaped.java": 1,
		"src/main/java/p/Classic.java": 1,
		"src/main/java/p/Doubled.java": 0,
	} {
		if got := unit(t, graph, relative).Imports; len(got) != want {
			t.Errorf("%s imports=%v want %d", relative, got, want)
		}
	}
	write(t, root, "src/main/java/p/Bad.java", "package p;\nclass Bad { char c = '\\uZZZZ'; }\n")
	if graph := build(t, root); !slices.Contains(graph.Frontier(), FrontierUnparsedSource) {
		t.Fatalf("a malformed escape lexed: frontier=%v", graph.Frontier())
	}
}

func TestSpacedAnnotationsAndInheritedTests_JATV0002(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/Core.java", "package p;\npublic class Core {}\n")
	write(t, root, "src/test/java/p/SpacedTest.java", "package p;\nclass SpacedTest { @ Test void a() {} }\n")
	write(t, root, "src/test/java/p/CommentedTest.java", "package p;\nclass CommentedTest { @/*c*/Test void a() {} }\n")
	write(t, root, "src/test/java/p/BaseTest.java", "package p;\n"+junit+"abstract class BaseTest { @Test void covers() { new Core(); } }\n")
	write(t, root, "src/test/java/p/MissedTest.java", "package p;\nclass MissedTest extends BaseTest {}\n")
	write(t, root, "src/test/java/p/DeeperTest.java", "package p;\nclass DeeperTest extends MissedTest {}\n")
	write(t, root, "src/test/java/p/Fixture.java", "package p;\nclass Fixture extends Core {}\n")
	graph := build(t, root)
	for relative, wantTest := range map[string]bool{
		"src/test/java/p/SpacedTest.java":    true,
		"src/test/java/p/CommentedTest.java": true,
		"src/test/java/p/MissedTest.java":    true,
		"src/test/java/p/DeeperTest.java":    true,
		"src/test/java/p/Fixture.java":       false,
	} {
		if got := unit(t, graph, relative); (len(got.Tests) == 1) != wantTest || len(got.Tests) == len(got.Sources) {
			t.Errorf("%s tests=%v sources=%v want test=%v", relative, got.Tests, got.Sources, wantTest)
		}
	}
	plan := affected.Select(graph, []string{"src/main/java/p/Core.java"})
	for _, want := range []string{"src/test/java/p/BaseTest.java", "src/test/java/p/MissedTest.java", "src/test/java/p/DeeperTest.java"} {
		if !slices.Contains(plan.SelectedTests(), want) {
			t.Errorf("selected=%v misses %s", plan.SelectedTests(), want)
		}
	}
}

func TestSpacedReflectionCallsAreUnbounded_JATV0007(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/p/Loader.java", "package p;\nclass Loader { Object o(ClassLoader l, String n) throws Exception { return l.loadClass (n); } }\n")
	write(t, root, "src/main/java/p/Maker.java", "package p;\nclass Maker { Object o(Class<?> c) throws Exception { return c . getDeclaredConstructor ( ).newInstance(); } }\n")
	write(t, root, "src/main/java/p/Named.java", "package p;\nclass Named { Object o(String n) throws Exception { return Class.forName (n); } }\n")
	graph := build(t, root)
	for _, relative := range []string{"src/main/java/p/Loader.java", "src/main/java/p/Maker.java", "src/main/java/p/Named.java"} {
		if got := unit(t, graph, relative); got.UnboundedReads != Reflection || !got.LocatesRoot {
			t.Errorf("%s reads=%q locates=%v", relative, got.UnboundedReads, got.LocatesRoot)
		}
	}
}

func TestUnresolvedFirstPartyNamesAreUnbounded_JATV0006(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/com/ex/core/Core.java", "package com.ex.core;\npublic class Core {}\n")
	write(t, root, "src/main/java/com/ex/app/Wild.java", "package com.ex.app;\nimport com.ex.core.*;\nclass Wild { Core c; Generated g; }\n")
	write(t, root, "src/main/java/com/ex/core/Peer.java", "package com.ex.core;\nclass Peer { CoreBuilder b; }\n")
	write(t, root, "src/main/java/com/ex/app/External.java", "package com.ex.app;\nimport java.util.*;\nclass External { List<String> names; }\n")
	write(t, root, "src/main/java/com/ex/app/Plain.java", "package com.ex.app;\nimport java.util.List;\nclass Plain { List<String> names; String s; Override o; Inner i; static final int MAX_SIZE = 1; class Inner {} <T> T id(T t) { return t; } }\n")
	write(t, root, "src/main/kotlin/com/ex/app/Shared.kt", "package com.ex.app\nclass Shared\n")
	write(t, root, "src/main/java/com/ex/app/UsesKotlin.java", "package com.ex.app;\nclass UsesKotlin { Shared s; Object f = SharedKt.class; }\n")
	graph := build(t, root)
	for relative, want := range map[string]string{
		"src/main/java/com/ex/app/Wild.java":       UnresolvedName,
		"src/main/java/com/ex/core/Peer.java":      UnresolvedName,
		"src/main/java/com/ex/app/External.java":   "",
		"src/main/java/com/ex/app/Plain.java":      "",
		"src/main/java/com/ex/app/UsesKotlin.java": "",
	} {
		got := unit(t, graph, relative)
		if got.UnboundedReads != want || got.LocatesRoot != (want != "") {
			t.Errorf("%s reads=%q locates=%v want %q", relative, got.UnboundedReads, got.LocatesRoot, want)
		}
	}
}

func TestKotlinHeaderAfterCommentStillReachesJava_JATV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/main/java/com/ex/bridge/Bridge.java", "package com.ex.bridge;\npublic class Bridge {}\n")
	write(t, root, "src/main/kotlin/com/ex/bridge/Use.kt", "/* Licensed. */ package com.ex.bridge\n\nclass Use { val b = Bridge() }\n")
	graph := build(t, root)
	if got := unit(t, graph, "src/main/java/com/ex/bridge/Bridge.java").Frontier; !slices.Contains(got, FrontierKotlinDependent) {
		t.Fatalf("frontier=%v", got)
	}
}
