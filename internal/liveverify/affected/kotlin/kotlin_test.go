package kotlin

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsOnlyKotlinSource(t *testing.T) {
	language := New()
	for _, relative := range []string{"src/A.kt", "build.gradle.kts"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{"flow.yaml", "journey.feature", "Test.java", "A.kt.txt"} {
		if language.Owns(relative) {
			t.Fatalf("%s must not be owned", relative)
		}
	}
}

func TestScanSourceDetectsJUnitAndEveryRequiredKotestStyle(t *testing.T) {
	cases := map[string]string{
		"junit":       "import org.junit.jupiter.api.Test\nclass ExampleTest { @Test fun works() {} }",
		"junit-alias": "import org.junit.jupiter.api.Test as Check\nclass ExampleTest { @Check fun works() {} }",
		"junit-fq":    "class ExampleTest { @org.junit.jupiter.api.Test fun works() {} }",
		"kotlin-test": "import kotlin.test.Test\nclass ExampleTest { @Test fun works() {} }",
		"string-spec": "import io.kotest.core.spec.style.StringSpec\nclass ExampleSpec : StringSpec({ })",
		"fun-spec":    "import io.kotest.core.spec.style.FunSpec\nclass ExampleSpec : FunSpec({ })",
		"behavior":    "import io.kotest.core.spec.style.BehaviorSpec\nclass ExampleSpec : BehaviorSpec({ })",
		"describe":    "import io.kotest.core.spec.style.DescribeSpec\nclass ExampleSpec : DescribeSpec({ })",
		"should":      "import io.kotest.core.spec.style.ShouldSpec\nclass Payments : ShouldSpec({ })",
		"feature":     "import io.kotest.core.spec.style.FeatureSpec\nclass Payments : FeatureSpec({ })",
		"free":        "import io.kotest.core.spec.style.FreeSpec\nclass Payments : FreeSpec({ })",
		"word":        "import io.kotest.core.spec.style.WordSpec\nclass Payments : WordSpec({ })",
		"expect":      "import io.kotest.core.spec.style.ExpectSpec\nclass Payments : ExpectSpec({ })",
		"annotation":  "import io.kotest.core.spec.style.AnnotationSpec\nclass Payments : AnnotationSpec()",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			scan := scanSource("src/test/kotlin/example/ExampleTest.kt", body)
			if !scan.test || len(scan.classes) != 1 {
				t.Fatalf("scan=%+v", scan)
			}
		})
	}
}

func TestAbstractJUnitBaseIsSourceNotARunnableTest(t *testing.T) {
	body := "import org.junit.jupiter.api.Test\nabstract class BaseTest { @Test fun inherited() = Unit }\n"
	scan := scanSource("src/test/kotlin/example/BaseTest.kt", body)
	if scan.test {
		t.Fatalf("abstract base became runnable: %+v", scan)
	}
}

func TestAbstractJUnitBaseRaisesInheritedDiscoveryFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/BaseTest.kt", "import org.junit.jupiter.api.Test\nabstract class BaseTest { @Test fun inherited() = Unit }\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestAbstractKotestBaseRaisesInheritedDiscoveryFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/BaseSpec.kt", "import io.kotest.core.spec.style.FunSpec\nabstract class BaseSpec : FunSpec({ })\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestAbstractKotestBaseInMixedFileRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/BaseSpec.kt", "import io.kotest.core.spec.style.FunSpec\nabstract class BaseSpec : FunSpec({ })\nclass Helper\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestUnknownInheritedTestRaisesFrontierRatherThanDisappearing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/Payments.kt", "package example\nclass Payments : ExternalSpec()\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestMultilineKotestInheritanceIsDetected(t *testing.T) {
	body := "import io.kotest.core.spec.style.ShouldSpec\nclass Payments\n    : ShouldSpec({ })\n"
	scan := scanSource("src/test/kotlin/example/Payments.kt", body)
	if !scan.test || len(scan.classes) != 1 || scan.classes[0] != "Payments" {
		t.Fatalf("scan=%+v", scan)
	}
}

func TestStyleSubstringAndJUnitLifecycleAnnotationAreNotTests(t *testing.T) {
	cases := []string{
		"class StringSpecHelper\n",
		"import org.junit.jupiter.api.TestInstance\n@TestInstance(TestInstance.Lifecycle.PER_CLASS)\nclass LifecycleHelper\n",
		"import org.junit.jupiter.api.TestInstance as Test\n@Test(TestInstance.Lifecycle.PER_CLASS)\nclass LifecycleHelper\n",
		"import io.kotest.core.spec.style.StringSpec\nclass Helper(val style: StringSpec)\n",
		"import io.kotest.core.spec.style.StringSpec\nclass Helper : Wrapper(StringSpec({ }))\n",
		"import io.kotest.core.spec.style.StringSpec\nclass Helper(val value: Any = StringSpec({ }))\n",
	}
	for _, body := range cases {
		scan := scanSource("src/test/kotlin/example/Helper.kt", body)
		if scan.test {
			t.Fatalf("helper became runnable: %+v", scan)
		}
	}
}

func TestComposedJUnitAnnotationIsUnknownSourceNotRunnable(t *testing.T) {
	body := "import org.junit.jupiter.api.Test\n@Test\nannotation class FastTest\n"
	scan := scanSource("src/test/kotlin/example/FastTest.kt", body)
	if scan.test || !scan.ambiguous {
		t.Fatalf("scan=%+v", scan)
	}
}

func TestMockKIsAuxiliaryRatherThanARunner(t *testing.T) {
	body := "package example\nimport io.mockk.mockk\nclass Collaborator { val dependency = mockk<Any>() }\n"
	scan := scanSource("src/test/kotlin/example/Collaborator.kt", body)
	if scan.test {
		t.Fatalf("MockK-only helper became a test: %+v", scan)
	}
	if !scan.frameworks[frameworkMockK] {
		t.Fatal("MockK marker was not observed")
	}
}

func TestCommentsAndStringsDoNotInventTestsOrImports(t *testing.T) {
	raw := "package example\n// import org.junit.jupiter.api.Test\nval sample = \"@Test Class.forName(\\\"x\\\")\"\nclass Helper\n"
	clean, ok := sourceText(raw)
	if !ok {
		t.Fatal("valid source was rejected")
	}
	scan := scanSource("src/test/kotlin/example/Helper.kt", clean)
	if scan.test || scan.dynamic || len(scan.imports) != 0 {
		t.Fatalf("scan=%+v", scan)
	}
}

func TestApostropheInBacktickedNameDoesNotHideQualifiedReference(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "src/main/kotlin/example/util/Helper.kt", "package example.util\nobject Helper { fun go() = Unit }\n")
	write(t, root, "src/test/kotlin/example/app/AppTest.kt", "package example.app\nimport org.junit.jupiter.api.Test\nclass AppTest { @Test fun `it's fine`() = example.util.Helper.go() // it's\n}\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/main/kotlin/example/util/Helper.kt"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || !equal(got, []string{"src/test/kotlin/example/app/AppTest.kt"}) {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, got, plan.Unknown)
	}
}

func TestSourceTextRejectsUnterminatedLexicalConstructs(t *testing.T) {
	for _, body := range []string{"/* no end", "val x = \"no end", "val x = \"\"\"no end"} {
		if _, ok := sourceText(body); ok {
			t.Fatalf("accepted %q", body)
		}
	}
}

func TestUnitsSelectsStaticDependencyClosure(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "core/src/main/kotlin/core/Core.kt", "package core\nclass Core\n")
	write(t, root, "core/src/test/kotlin/core/CoreTest.kt", junit("core", "CoreTest", "core.Core"))
	write(t, root, "mid/src/main/kotlin/mid/Mid.kt", "package mid\nimport core.Core\nclass Mid(val core: Core)\n")
	write(t, root, "mid/src/test/kotlin/mid/MidTest.kt", junit("mid", "MidTest", "mid.Mid"))
	write(t, root, "leaf/src/main/kotlin/leaf/Leaf.kt", "package leaf\nimport mid.Mid\nclass Leaf(val mid: Mid)\n")
	write(t, root, "leaf/src/test/kotlin/leaf/LeafTest.kt", junit("leaf", "LeafTest", "leaf.Leaf"))
	write(t, root, "solo/src/main/kotlin/solo/Solo.kt", "package solo\nclass Solo\n")
	write(t, root, "solo/src/test/kotlin/solo/SoloTest.kt", junit("solo", "SoloTest", "solo.Solo"))

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	if frontier := graph.Frontier(); len(frontier) != 0 {
		t.Fatalf("frontier=%v", frontier)
	}
	plan := affected.Select(graph, []string{"core/src/main/kotlin/core/Core.kt"})
	want := []string{
		"core/src/test/kotlin/core/CoreTest.kt",
		"leaf/src/test/kotlin/leaf/LeafTest.kt",
		"mid/src/test/kotlin/mid/MidTest.kt",
	}
	if got := plan.SelectedTests(); !equal(got, want) {
		t.Fatalf("selected=%v want=%v", got, want)
	}
	if contains(plan.SelectedTests(), "solo/src/test/kotlin/solo/SoloTest.kt") {
		t.Fatal("unrelated test was selected")
	}
}

func TestJavaSiblingSourceWidensCrossLanguageDependency(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "core/src/main/kotlin/com/ex/core/Calc.kt", "package com.ex.core\nclass Calc\n")
	write(t, root, "core/src/main/java/com/ex/bridge/Bridge.java", "package com.ex.bridge;\nimport com.ex.core.Calc;\nclass Bridge { Calc calc; }\n")
	write(t, root, "app/src/test/kotlin/com/ex/app/AppTest.kt", "package com.ex.app\nimport com.ex.bridge.Bridge\nimport org.junit.jupiter.api.Test\nclass AppTest { @Test fun works() = Unit }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"core/src/main/kotlin/com/ex/core/Calc.kt"})
	if !contains(graph.Frontier(), FrontierJVMSiblingSource) {
		t.Fatalf("frontier=%v", graph.Frontier())
	}
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestSamePackageAndWildcardImportsAreConservativeEdges(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/main/kotlin/example/Core.kt", "package example\nclass Core\n")
	write(t, root, "src/main/kotlin/example/Peer.kt", "package example\nclass Peer(val core: Core)\n")
	write(t, root, "src/test/kotlin/check/PeerTest.kt", "package check\nimport example.*\nimport org.junit.jupiter.api.Test\nclass PeerTest { @Test fun works() = Unit }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/main/kotlin/example/Core.kt"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "src/test/kotlin/check/PeerTest.kt" {
		t.Fatalf("selected=%v", got)
	}
	peer, ok := graph.OwnerOf("src/main/kotlin/example/Peer.kt")
	if !ok {
		t.Fatal("peer has no owner")
	}
	unit, _ := graph.Unit(peer)
	if len(unit.Imports) == 0 {
		t.Fatal("same-package visibility produced no edge")
	}
}

func TestByteOrderMarkAndTabHeadersKeepSamePackageEdges(t *testing.T) {
	for name, header := range map[string]string{"bom": "\xef\xbb\xbfpackage example", "tab": "package\texample"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
			write(t, root, "src/main/kotlin/example/Core.kt", "package example\nclass Core\n")
			write(t, root, "src/test/kotlin/example/PeerTest.kt", header+"\nimport org.junit.jupiter.api.Test\nclass PeerTest { @Test fun works() = Unit }\n")
			graph, err := affected.Build(root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"src/main/kotlin/example/Core.kt"})
			if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "src/test/kotlin/example/PeerTest.kt" {
				t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
			}
		})
	}
}

func TestStarImportOfAnEnumResolvesToItsDeclaration(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/main/kotlin/example/model/Status.kt", "package example.model\nenum class Status { ACTIVE, CLOSED }\n")
	write(t, root, "src/main/kotlin/example/model/Other.kt", "package example.model\nclass Other\n")
	write(t, root, "src/test/kotlin/check/StatusTest.kt", "package check\nimport example.model.Status.*\nimport org.junit.jupiter.api.Test\nclass StatusTest { @Test fun works() { println(ACTIVE) } }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/main/kotlin/example/model/Status.kt"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "src/test/kotlin/check/StatusTest.kt" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
}

func TestImportOfAnUnparsedTopLevelSymbolFallsBackToItsPackage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/main/kotlin/example/Extensions.kt", "package example\nfun <T> List<T>.special() = size\n")
	write(t, root, "src/test/kotlin/check/ExtensionTest.kt", "package check\nimport example.special\nimport org.junit.jupiter.api.Test\nclass ExtensionTest { @Test fun works() = Unit }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/main/kotlin/example/Extensions.kt"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "src/test/kotlin/check/ExtensionTest.kt" {
		t.Fatalf("selected=%v", got)
	}
}

func TestTopLevelDeclarationDoesNotShadowASameNamedSubpackage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/main/kotlin/example/Top.kt", "package example\nval model = 1\n")
	write(t, root, "src/main/kotlin/example/model/Extensions.kt", "package example.model\nfun <T> List<T>.special() = size\n")
	write(t, root, "src/test/kotlin/check/ExtensionTest.kt", "package check\nimport example.model.special\nimport org.junit.jupiter.api.Test\nclass ExtensionTest { @Test fun works() = Unit }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/main/kotlin/example/model/Extensions.kt"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "src/test/kotlin/check/ExtensionTest.kt" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
	if !contains(graph.Frontier(), FrontierAmbiguousDeclaration) {
		t.Fatalf("frontier=%v", graph.Frontier())
	}
}

func TestRunnerIdentityDistinguishesLocalInstrumentationAndRobolectric(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "app/src/test/kotlin/example/LocalTest.kt", junit("example", "LocalTest", "org.robolectric.Robolectric"))
	write(t, root, "app/src/androidTest/kotlin/example/DeviceTest.kt", junit("example", "DeviceTest", "androidx.test.espresso.Espresso"))
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, unit := range result.Units {
		if len(unit.Tests) != 0 {
			ids[unit.Tests[0]] = unit.ID
		}
	}
	if !strings.HasPrefix(ids["app/src/test/kotlin/example/LocalTest.kt"], "kotlin:jvm:app:test:") {
		t.Fatalf("local id=%s", ids["app/src/test/kotlin/example/LocalTest.kt"])
	}
	if !strings.HasPrefix(ids["app/src/androidTest/kotlin/example/DeviceTest.kt"], "kotlin:instrumentation:app:connectedAndroidTest:") {
		t.Fatalf("instrumentation id=%s", ids["app/src/androidTest/kotlin/example/DeviceTest.kt"])
	}
	if contains(result.Frontier, FrontierRunnerMismatch) {
		t.Fatalf("correct source sets raised mismatch: %v", result.Frontier)
	}
}

func TestE2EFrameworkMarkersDoNotReplaceTheirHostRunner(t *testing.T) {
	cases := map[string]string{
		frameworkEspresso:    "androidx.test.espresso.Espresso",
		frameworkUIAutomator: "androidx.test.uiautomator.UiDevice",
		frameworkRobolectric: "org.robolectric.Robolectric",
		frameworkAppium:      "io.appium.java_client.android.AndroidDriver",
	}
	for framework, imported := range cases {
		t.Run(framework, func(t *testing.T) {
			body := junit("example", "FrameworkTest", imported)
			clean, ok := sourceText(body)
			if !ok {
				t.Fatal("source rejected")
			}
			scan := scanSource("src/test/kotlin/example/FrameworkTest.kt", clean)
			if !scan.test || !scan.frameworks[framework] {
				t.Fatalf("scan=%+v", scan)
			}
		})
	}
}

func TestMultipleRunnableClassesRaiseAddressFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/Pair.kt", "package example\nimport org.junit.jupiter.api.Test\nclass FirstTest { @Test fun first() = Unit }\nclass SecondTest { @Test fun second() = Unit }\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierTestAddress) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestDynamicAndUnresolvedDiscoveryRaiseFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/GeneratedTest.kt", "package example\nimport org.junit.jupiter.api.TestFactory\nclass GeneratedTest { @TestFactory fun generated() = Class.forName(\"example.Case\") }\n")
	write(t, root, "src/customTest/kotlin/example/InheritedSpec.kt", "package example\nclass InheritedSpec : ExternalSpec()\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{FrontierDynamicDiscovery, FrontierTestDiscovery, FrontierCustomSourceSet} {
		if !contains(result.Frontier, want) {
			t.Fatalf("frontier=%v missing %s", result.Frontier, want)
		}
	}
}

func TestDynamicDiscoveryHandlesWhitespaceAndAnnotationAliases(t *testing.T) {
	frameworks := map[string]bool{frameworkJUnit5: true}
	text := "import org.junit.jupiter.api.TestTemplate as Generated\n@Generated fun generated() = Class.forName (name)\n"
	if !dynamicDiscovery(text, frameworks) {
		t.Fatal("dynamic discovery was omitted")
	}
	if dynamicDiscovery("fun include(value: Any) = value\n", map[string]bool{}) {
		t.Fatal("unrelated production include forced an unknown frontier")
	}
}

func TestCustomTestSourceSetHasNoInventedTask(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/testIntegration/kotlin/example/IntegrationTest.kt", junit("example", "IntegrationTest", "example.Subject"))
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierCustomSourceSet) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	for _, unit := range result.Units {
		if len(unit.Tests) != 0 && !strings.Contains(unit.ID, ":unknown:") {
			t.Fatalf("custom source set produced exact id %s", unit.ID)
		}
	}
}

func TestCustomGradleProjectDirectoryDoesNotInventATarget(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "include(\":app\")\nproject(\":app\").projectDir = file(\"modules/mobile\")\n")
	write(t, root, "modules/mobile/src/test/kotlin/example/MappedTest.kt", junit("example", "MappedTest", "example.Subject"))
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierBuildTarget) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	for _, unit := range result.Units {
		if len(unit.Tests) != 0 && !strings.Contains(unit.ID, ":unknown:") {
			t.Fatalf("custom mapping produced exact id %s", unit.ID)
		}
	}
}

func TestBuildConfigurationChangeReachesEveryTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n")
	write(t, root, "src/test/kotlin/example/ExampleTest.kt", junit("example", "ExampleTest", "example.Subject"))
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"build.gradle.kts"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "src/test/kotlin/example/ExampleTest.kt" {
		t.Fatalf("selected=%v", got)
	}
}

func TestBuildLogicChangeReachesEveryTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "buildSrc/src/main/kotlin/TestConvention.kt", "class TestConvention\n")
	write(t, root, "app/src/test/kotlin/example/ExampleTest.kt", junit("example", "ExampleTest", "example.Subject"))
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"buildSrc/src/main/kotlin/TestConvention.kt"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "app/src/test/kotlin/example/ExampleTest.kt" {
		t.Fatalf("selected=%v", got)
	}
}

func TestAppliedKotlinGradleScriptReachesEveryTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", "rootProject.name = \"fixture\"\n")
	write(t, root, "gradle/test-conventions.gradle.kts", "tasks.withType<Test>().configureEach { }\n")
	write(t, root, "app/src/test/kotlin/example/ExampleTest.kt", junit("example", "ExampleTest", "example.Subject"))
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"gradle/test-conventions.gradle.kts"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "app/src/test/kotlin/example/ExampleTest.kt" {
		t.Fatalf("selected=%v", got)
	}
}

func TestRealRepositoryGroundTruth(t *testing.T) {
	root := os.Getenv("CORVINT_KOTLIN_GROUNDTRUTH_ROOT")
	if root == "" {
		t.Skip("CORVINT_KOTLIN_GROUNDTRUTH_ROOT is not set")
	}
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Fields(`
libraries/cast/src/test/java/androidx/media3/cast/MediaRouteButtonTest.kt
libraries/common_ktx/src/test/java/androidx/media3/common/PlayerExtensionsTest.kt
libraries/effect/src/androidTest/java/androidx/media3/effect/GlTextureFrameCompositorTest.kt
libraries/effect/src/androidTest/java/androidx/media3/effect/GlTextureFrameRendererTest.kt
libraries/effect/src/test/java/androidx/media3/effect/GlShaderProgramPacketProcessorTest.kt
libraries/effect/src/test/java/androidx/media3/effect/SingleContextGlObjectsProviderTest.kt
libraries/exoplayer/src/test/java/androidx/media3/exoplayer/video/VideoFrameReleaseHelperTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/ContentFrameTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/PlayerSurfaceTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/MuteButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/NextButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/PlayPauseButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/PlaybackSpeedStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/PlayerStateObserverTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/PresentationStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/PreviousButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/ProgressStateWithTickCountTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/ProgressStateWithTickIntervalTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/RepeatButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/SeekBackButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/SeekForwardButtonStateTest.kt
libraries/ui_compose/src/test/java/androidx/media3/ui/compose/state/ShuffleButtonStateTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/PlayerTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/MuteButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/NextButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/PlayPauseButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/PlaybackSpeedButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/PreviousButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/RepeatButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/SeekBackButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/SeekForwardButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/buttons/ShuffleButtonTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/indicator/ProgressSliderTest.kt
libraries/ui_compose_material3/src/test/java/androidx/media3/ui/compose/material3/indicator/TimeTextTest.kt
testapps/controller/src/test/java/androidx/media3/testapp/controller/MediaIntToStringTest.kt`)
	assertRevision(t, root, "7ce3aa2619b19009e4799319b4dd694a4a7577df")
	discovered := make([]string, 0, len(want))
	local := 0
	instrumentation := 0
	for _, unit := range result.Units {
		if len(unit.Tests) == 0 {
			continue
		}
		discovered = append(discovered, unit.Tests...)
		switch {
		case strings.HasPrefix(unit.ID, "kotlin:jvm:"):
			local += len(unit.Tests)
		case strings.HasPrefix(unit.ID, "kotlin:instrumentation:"):
			instrumentation += len(unit.Tests)
		}
	}
	sort.Strings(discovered)
	if !equal(discovered, want) || local != 33 || instrumentation != 2 {
		t.Fatalf("discovered=%v local=%d instrumentation=%d; want exact pinned 35/33/2", discovered, local, instrumentation)
	}
	t.Logf("real Kotlin corpus: discovered 35/35 runnable files (local 33/33, instrumentation 2/2)")
	negativeRoot := os.Getenv("CORVINT_KOTLIN_NEGATIVE_ROOT")
	if negativeRoot == "" {
		return
	}
	assertRevision(t, negativeRoot, "ee9082a4f5477b3c87885ee8abe33f0a63bbe0d2")
	negative, err := New().Units(negativeRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range negative.Units {
		if len(unit.Tests) != 0 {
			t.Fatalf("negative corpus invented tests: %+v", unit)
		}
	}
	t.Log("negative Kotlin corpus: discovered 0/0 runnable files")
}

func assertRevision(t *testing.T, root, want string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	command := exec.Command(git, "-C", root, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("resolve ground-truth revision: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("ground-truth revision=%s want=%s", got, want)
	}
}

func junit(packageName, className, imported string) string {
	return "package " + packageName + "\nimport " + imported + "\nimport org.junit.jupiter.api.Test\nclass " + className + " { @Test fun works() = Unit }\n"
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

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
