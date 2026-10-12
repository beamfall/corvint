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
