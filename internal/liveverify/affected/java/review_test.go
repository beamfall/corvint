package java

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/kotlin"
)

// reviewCase is one variant of a V1-1316 review finding, checked at the plan
// level with the Java and Kotlin plugins composed. Each fixture holds a
// control test that reaches the dirty file directly, so the plan cannot turn
// UNKNOWN through NO_SELECTABLE_TEST alone, and a reachable test that the
// reviewed adapter omitted from a BOUNDED plan.
type reviewCase struct {
	name    string
	files   map[string]string
	dirty   string
	control string
	reached string
	// unknown is empty when the reached test must be selected in a BOUNDED
	// plan; otherwise the plan must be UNKNOWN with this frontier.
	unknown string
	// reads, when set, is the UnboundedReads the reached Java test carries.
	reads string
}

const (
	kotlinBuild  = "plugins { kotlin(\"jvm\") version \"2.2.0\" }\n"
	kotlinJunit  = "import org.junit.jupiter.api.Test\n"
	coreJava     = "src/main/java/com/ex/core/Core.java"
	coreJavaTest = "src/test/java/com/ex/core/CoreTest.java"
	missedJava   = "src/test/java/q/MissedTest.java"
	samePackage  = "src/test/java/com/ex/core/MissedTest.java"
)

// javaTree is a Java tree with a dirty Core.java, its direct control test and
// the extra files of one case.
func javaTree(extra map[string]string) map[string]string {
	files := map[string]string{
		coreJava:     "package com.ex.core;\npublic class Core {}\n",
		coreJavaTest: "package com.ex.core;\n" + junit + "class CoreTest { @Test void direct() { new Core(); } }\n",
	}
	for path, body := range extra {
		files[path] = body
	}
	return files
}

// missedIn is a JUnit test class in package q whose one method holds body.
func missedIn(body string) map[string]string {
	return javaTree(map[string]string{missedJava: "package q;\n" + junit + "class MissedTest { @Test void t() throws Exception { " + body + " } }\n"})
}

// facadeTree is a Kotlin file p/<file> with a direct Kotlin control test and
// a Java test in q that reaches it only through its JVM facade name.
func facadeTree(file, header, javaImport, javaCall string) map[string]string {
	return map[string]string{
		"build.gradle.kts":                 kotlinBuild,
		"src/main/kotlin/p/" + file:        header + "package p\nfun value() = 1\n",
		"src/test/kotlin/p/ControlTest.kt": "package p\n" + kotlinJunit + "class ControlTest { @Test fun direct() { value() } }\n",
		missedJava:                         "package q;\n" + javaImport + "\n" + junit + "class MissedTest { @Test void t() { " + javaCall + " } }\n",
	}
}

func reviewCases() []reviewCase {
	return []reviewCase{
		{name: "1 two-segment qualified name", files: map[string]string{
			"src/main/java/p/Core.java":     "package p;\npublic class Core {}\n",
			"src/test/java/p/CoreTest.java": "package p;\n" + junit + "class CoreTest { @Test void direct() { new Core(); } }\n",
			missedJava:                      "package q;\n" + junit + "class MissedTest { @Test void t() { new p.Core(); } }\n",
		}, dirty: "src/main/java/p/Core.java", control: "src/test/java/p/CoreTest.java", reached: missedJava},
		{name: "1 spaced qualified name", files: missedIn("new com . ex . core . Core();"),
			dirty: coreJava, control: coreJavaTest, reached: missedJava},
		{name: "1 comment-split qualified name", files: missedIn("new com.ex.core./*comment*/Core();"),
			dirty: coreJava, control: coreJavaTest, reached: missedJava},
		{name: "2 spaced reflection markers", files: missedIn("ClassLoader.getSystemClassLoader().loadClass (\"com.ex.core.Core\").getDeclaredConstructor ().newInstance ();"),
			dirty: coreJava, control: coreJavaTest, reached: missedJava, reads: Reflection},
		{name: "3 forName binary name of a nested type", files: map[string]string{
			"src/main/java/com/ex/core/Outer.java":     "package com.ex.core;\npublic class Outer { public static class Inner {} }\n",
			"src/test/java/com/ex/core/OuterTest.java": "package com.ex.core;\n" + junit + "class OuterTest { @Test void direct() { new Outer(); } }\n",
			missedJava: "package q;\n" + junit + "class MissedTest { @Test void t() throws Exception { Class.forName(\"com.ex.core.Outer$Inner\"); } }\n",
		}, dirty: "src/main/java/com/ex/core/Outer.java", control: "src/test/java/com/ex/core/OuterTest.java", reached: missedJava},
		{name: "4 unicode-escaped line terminator", files: javaTree(map[string]string{
			samePackage: "package com.ex.core;\n" + junit + "class MissedTest {\n// comment \\u000a @Test void t(){new Core();}\n}\n",
		}), dirty: coreJava, control: coreJavaTest, reached: samePackage},
		{name: "4 CR-only line ends", files: javaTree(map[string]string{
			samePackage: "package com.ex.core;\rimport org.junit.jupiter.api.Test;\r// comment\rclass MissedTest {@Test void t(){new Core();}}\r",
		}), dirty: coreJava, control: coreJavaTest, reached: samePackage},
		{name: "5 spaced annotation", files: javaTree(map[string]string{
			samePackage: "package com.ex.core;\n" + junit + "class MissedTest { @ Test void t(){new Core();} }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: samePackage},
		{name: "5 comment-split annotation", files: javaTree(map[string]string{
			samePackage: "package com.ex.core;\n" + junit + "class MissedTest { @/*comment*/Test void t(){new Core();} }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: samePackage},
		{name: "6 inherited test methods", files: javaTree(map[string]string{
			"src/test/java/com/ex/core/Base.java": "package com.ex.core;\n" + junit + "public abstract class Base { @Test public void t(){new Core();} }\n",
			samePackage:                           "package com.ex.core;\npublic class MissedTest extends Base {}\n",
		}), dirty: coreJava, control: coreJavaTest, reached: samePackage},
		{name: "7 generated type behind a first-party wildcard import", files: javaTree(map[string]string{
			missedJava: "package q;\nimport com.ex.core.*;\n" + junit + "class MissedTest { @Test void t(){new Generated().callCore();} }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: missedJava, reads: UnresolvedName},
		{name: "8 Kotlin package header after a comment", files: javaTree(map[string]string{
			"build.gradle.kts":                      kotlinBuild,
			"src/main/kotlin/com/ex/core/Bridge.kt": "/*license*/ package com.ex.core\nclass Bridge { fun run() = Core() }\n",
			"src/test/kotlin/q/BridgeTest.kt":       "package q\nimport com.ex.core.Bridge\n" + kotlinJunit + "class BridgeTest { @Test fun t() { Bridge().run() } }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: "src/test/kotlin/q/BridgeTest.kt", unknown: FrontierKotlinDependent},
		{name: "8 Java package header after a comment", files: map[string]string{
			"build.gradle.kts":                 kotlinBuild,
			"src/main/kotlin/p/Core.kt":        "package p\nclass Core\n",
			"src/test/kotlin/p/ControlTest.kt": "package p\n" + kotlinJunit + "class ControlTest { @Test fun direct() { Core() } }\n",
			"src/test/java/p/MissedTest.java":  "/*license*/ package p;\n" + junit + "class MissedTest { @Test void t(){new Core();} }\n",
		}, dirty: "src/main/kotlin/p/Core.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: "src/test/java/p/MissedTest.java", unknown: kotlin.FrontierJavaDependent},
		{name: "9 capitalised file facade", files: facadeTree("core.kt", "", "import p.CoreKt;", "CoreKt.value();"),
			dirty: "src/main/kotlin/p/core.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: missedJava, unknown: kotlin.FrontierJavaDependent},
		{name: "9 kotlin.jvm-qualified JvmName", files: facadeTree("api.kt", "@file:kotlin.jvm.JvmName(\"Api\")\n", "import p.Api;", "Api.value();"),
			dirty: "src/main/kotlin/p/api.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: missedJava, unknown: kotlin.FrontierJavaDependent},
		{name: "9 spaced JvmName", files: facadeTree("api.kt", "@file:JvmName (\"Api\")\n", "import p.Api;", "Api.value();"),
			dirty: "src/main/kotlin/p/api.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: missedJava, unknown: kotlin.FrontierJavaDependent},
		{name: "1 qualified annotation name", files: map[string]string{
			"src/main/java/p/Core.java":     "package p;\npublic @interface Core {}\n",
			"src/test/java/p/CoreTest.java": "package p;\n" + junit + "class CoreTest { @Core @Test void direct() {} }\n",
			missedJava:                      "package q;\n" + junit + "class MissedTest { @p.Core @Test void t() {} }\n",
		}, dirty: "src/main/java/p/Core.java", control: "src/test/java/p/CoreTest.java", reached: missedJava},
		{name: "7 generated name also declared in an unrelated package", files: javaTree(map[string]string{
			"src/main/java/com/other/Generated.java": "package com.other;\npublic class Generated {}\n",
			missedJava:                               "package q;\nimport com.ex.core.*;\n" + junit + "class MissedTest { @Test void t(){new Generated().callCore();} }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: missedJava, reads: UnresolvedName},
		{name: "8 Kotlin package header after a nested comment", files: javaTree(map[string]string{
			"build.gradle.kts":                      kotlinBuild,
			"src/main/kotlin/com/ex/core/Bridge.kt": "/* outer /* inner */ still outer */ package com.ex.core\nclass Bridge { fun run() = Core() }\n",
			"src/test/kotlin/q/BridgeTest.kt":       "package q\nimport com.ex.core.Bridge\n" + kotlinJunit + "class BridgeTest { @Test fun t() { Bridge().run() } }\n",
		}), dirty: coreJava, control: coreJavaTest, reached: "src/test/kotlin/q/BridgeTest.kt", unknown: FrontierKotlinDependent},
		{name: "8 Java package header after an escaped line end", files: map[string]string{
			"build.gradle.kts":                 kotlinBuild,
			"src/main/kotlin/p/Core.kt":        "package p\nclass Core\n",
			"src/test/kotlin/p/ControlTest.kt": "package p\n" + kotlinJunit + "class ControlTest { @Test fun direct() { Core() } }\n",
			"src/test/java/p/MissedTest.java":  "// license \\u000a package p;\n" + junit + "class MissedTest { @Test void t(){new Core();} }\n",
		}, dirty: "src/main/kotlin/p/Core.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: "src/test/java/p/MissedTest.java", unknown: kotlin.FrontierJavaDependent},
		{name: "9 comment-split JvmName", files: facadeTree("api.kt", "@file:JvmName/*comment*/(\"Api\")\n", "import p.Api;", "Api.value();"),
			dirty: "src/main/kotlin/p/api.kt", control: "src/test/kotlin/p/ControlTest.kt", reached: missedJava, unknown: kotlin.FrontierJavaDependent},
	}
}

// TestReviewFindingsKeepReachableTests_V1_1316 pins every finding and variant
// of the V1-1316 review: the reachable test is selected or the plan is
// UNKNOWN for the right reason, never omitted from a BOUNDED plan.
func TestReviewFindingsKeepReachableTests_V1_1316(t *testing.T) {
	for _, tc := range reviewCases() {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for path, body := range tc.files {
				write(t, root, path, body)
			}
			graph, err := affected.Build(context.Background(), root, New(), kotlin.New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{tc.dirty})
			selected := plan.SelectedTests()
			if !slices.Contains(selected, tc.control) {
				t.Fatalf("control %s not selected: selected=%v", tc.control, selected)
			}
			for _, unknown := range plan.Unknown {
				if unknown.Reason == affected.UnknownNoSelectableTest {
					t.Fatalf("NO_SELECTABLE_TEST masks the case: unknown=%+v", plan.Unknown)
				}
			}
			if tc.reads != "" {
				if got := unit(t, graph, tc.reached).UnboundedReads; got != tc.reads {
					t.Fatalf("%s reads=%q want %q", tc.reached, got, tc.reads)
				}
			}
			if tc.unknown == "" {
				if !slices.Contains(selected, tc.reached) || plan.Scope != affected.ScopeBounded {
					t.Fatalf("scope=%s unknown=%+v selected=%v want %s selected", plan.Scope, plan.Unknown, selected, tc.reached)
				}
				return
			}
			if plan.Scope != affected.ScopeUnknown || !hasDetail(plan.Unknown, tc.unknown) {
				t.Fatalf("scope=%s unknown=%+v selected=%v want UNKNOWN by %s", plan.Scope, plan.Unknown, selected, tc.unknown)
			}
		})
	}
}

func hasDetail(unknowns []affected.Unknown, want string) bool {
	for _, unknown := range unknowns {
		if unknown.Detail == want || unknown.Reason == want {
			return true
		}
	}
	return false
}
