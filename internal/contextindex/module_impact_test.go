package contextindex

import (
	"context"
	"testing"
)

// moduleImpactRows maps each result id of one kind to its evidence confidence.
func moduleImpactRows(receipt map[string]any, kind string) map[string]string {
	rows := map[string]string{}
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] != kind {
			continue
		}
		confidence := ""
		for _, evidenceItem := range mapsFromAny(item["evidence"]) {
			confidence = stringValue(evidenceItem["confidence"])
		}
		rows[stringValue(item["id"])] = confidence
	}
	return rows
}

func moduleImpact(t *testing.T, files map[string]string, changed string) (map[string]any, []any) {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return result, anySlice(result["coverage"].(map[string]any)["uncertainty"])
}

const moduleProfileGap = "reverse-import results for 1 changed paths with no named resolution rule are not computed by the impact profile"

var swiftPackageFixture = map[string]string{
	"Package.swift": `// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Demo",
    targets: [
        .target(name: "Core"),
        .target(name: "App", dependencies: ["Core"]),
        .target(name: "Tools"),
        .testTarget(name: "CoreTests", dependencies: ["Core"]),
    ]
)
`,
	"Sources/Core/Ledger.swift":         "public struct Ledger {\n    public init() {}\n}\n",
	"Sources/Core/Audit.swift":          "func audit(_ entry: Ledger) {}\n",
	"Sources/App/main.swift":            "import Core\n\nlet ledger = Ledger()\n",
	"Sources/Tools/Tool.swift":          "// import Core\nlet text = \"import Core\"\nstruct Tool { let name = \"Ledger\" }\n",
	"Tests/CoreTests/LedgerTests.swift": "import XCTest\n@testable import Core\n\nfinal class LedgerTests: XCTestCase {}\n",
}

// TestNGIV0980SwiftReverseImpactFollowsSPMTargetMembership: a changed Swift
// file's reverse dependents are the files outside its SwiftPM target that
// import the target's module, a test importer included, and an import spelled
// only in a comment or string is not one (NGI-V0-980, NGI-V0-985).
func TestNGIV0980SwiftReverseImpactFollowsSPMTargetMembership(t *testing.T) {
	t.Parallel()
	result, uncertainty := moduleImpact(t, swiftPackageFixture, "Sources/Core/Ledger.swift")
	imports := moduleImpactRows(result, "reverse-import")
	if imports["Sources/App/main.swift"] != "high" || imports["Tests/CoreTests/LedgerTests.swift"] != "high" {
		t.Fatalf("reverse imports = %v, want Sources/App/main.swift and Tests/CoreTests/LedgerTests.swift at high confidence", imports)
	}
	if _, ok := imports["Sources/Tools/Tool.swift"]; ok {
		t.Fatalf("a commented or quoted import was read as an import: %v", imports)
	}
	if _, ok := imports["Sources/Core/Audit.swift"]; ok {
		t.Fatalf("a same-target member was reported as an importer: %v", imports)
	}
	if anyContains(uncertainty, moduleProfileGap) || !anyContains(uncertainty, swiftModuleUncertainty) {
		t.Fatalf("uncertainty = %v, want the Swift precision limits and no profile gap", uncertainty)
	}
}

var kotlinFixture = map[string]string{
	"settings.gradle.kts":                        "rootProject.name = \"demo\"\n",
	"src/main/kotlin/com/acme/ledger/Ledger.kt":  "package com.acme.ledger\n\nclass Ledger {\n    fun total(): Int = 0\n}\n",
	"src/main/kotlin/com/acme/ledger/Audit.kt":   "package com.acme.ledger\n\nfun audit(entry: Ledger) = entry.total()\n",
	"src/main/kotlin/com/acme/app/Main.kt":       "package com.acme.app\n\nimport com.acme.ledger.Ledger\n\nfun main() { println(\"x\") }\n",
	"src/main/kotlin/com/acme/report/Report.kt":  "package com.acme.report\n\nimport com.acme.ledger.*\n\nfun report() = Ledger().total()\n",
	"src/main/kotlin/com/acme/idle/Idle.kt":      "package com.acme.idle\n\nimport com.acme.ledger.*\n\nfun idle() = 0\n",
	"src/main/kotlin/com/acme/other/Other.kt":    "package com.acme.other\n\n// Ledger in a comment\nval s = \"Ledger\"\n",
	"src/main/kotlin/com/acme/ledgerx/Shadow.kt": "package com.acme.ledgerx\n\nimport com.acme.ledgerx.Ledger\n",
	"src/test/kotlin/com/acme/app/LedgerTest.kt": "package com.acme.app\n\nimport com.acme.ledger.Ledger\n\nclass LedgerTest\n",
}

// TestNGIV0981KotlinReverseImpactFollowsPackageImports: an explicit import of
// a declaration the changed file declares is a high-confidence importer; a
// wildcard import of its package is a medium-confidence importer only when the
// importer names a declaration; a comment, a string or a lookalike package is
// not one (NGI-V0-981, NGI-V0-985).
func TestNGIV0981KotlinReverseImpactFollowsPackageImports(t *testing.T) {
	t.Parallel()
	result, uncertainty := moduleImpact(t, kotlinFixture, "src/main/kotlin/com/acme/ledger/Ledger.kt")
	imports := moduleImpactRows(result, "reverse-import")
	want := map[string]string{
		"src/main/kotlin/com/acme/app/Main.kt":       "high",
		"src/main/kotlin/com/acme/report/Report.kt":  "medium",
		"src/test/kotlin/com/acme/app/LedgerTest.kt": "high",
	}
	for id, confidence := range want {
		if imports[id] != confidence {
			t.Fatalf("reverse imports = %v, want %s at %s", imports, id, confidence)
		}
	}
	if len(imports) != len(want) {
		t.Fatalf("reverse imports = %v, want exactly %v", imports, want)
	}
	if anyContains(uncertainty, moduleProfileGap) || !anyContains(uncertainty, kotlinModuleUncertainty) {
		t.Fatalf("uncertainty = %v, want the Kotlin precision limits and no profile gap", uncertainty)
	}
}

var csharpFixture = map[string]string{
	"src/Ledger/LedgerBook.cs":   "namespace Acme.Ledger;\n\npublic class LedgerBook\n{\n    public static int Total() => 0;\n}\n",
	"src/Ledger/Checks/Audit.cs": "namespace Acme.Ledger.Checks\n{\n    class Audit { int Run() => LedgerBook.Total(); }\n}\n",
	"src/App/Program.cs":         "using Acme.Ledger;\n\nnamespace Acme.App;\n\nclass Program { static void Main() { new LedgerBook(); } }\n",
	"src/App/Static.cs":          "using static global::Acme.Ledger.LedgerBook;\n\nnamespace Acme.Tools;\n\nclass Tool { int X() => 1; }\n",
	"src/App/Alias.cs":           "using Book = Acme.Ledger.LedgerBook;\n\nnamespace Acme.Alias;\n\nclass Holder { }\n",
	"src/App/Idle.cs":            "using Acme.Ledger;\n\nnamespace Acme.Idle;\n\nclass Idle { }\n",
	"src/App/Global.cs":          "global using Acme.Ledger;\n",
	"src/Other/Unrelated.cs":     "namespace Acme.Other;\n// LedgerBook\nclass Unrelated { string s = \"LedgerBook\"; }\n",
}

// TestNGIV0982CSharpReverseImpactFollowsUsingDirectives: a static or alias
// using of a type the changed file declares is a high-confidence importer; a
// namespace using is a medium-confidence importer only when the importer names
// a declaration; a global using's project-wide reach is disclosed, not
// inferred (NGI-V0-982, NGI-V0-985).
func TestNGIV0982CSharpReverseImpactFollowsUsingDirectives(t *testing.T) {
	t.Parallel()
	result, uncertainty := moduleImpact(t, csharpFixture, "src/Ledger/LedgerBook.cs")
	imports := moduleImpactRows(result, "reverse-import")
	want := map[string]string{
		"src/App/Program.cs": "medium",
		"src/App/Static.cs":  "high",
		"src/App/Alias.cs":   "high",
	}
	for id, confidence := range want {
		if imports[id] != confidence {
			t.Fatalf("reverse imports = %v, want %s at %s", imports, id, confidence)
		}
	}
	if len(imports) != len(want) {
		t.Fatalf("reverse imports = %v, want exactly %v", imports, want)
	}
	if anyContains(uncertainty, moduleProfileGap) || !anyContains(uncertainty, csharpModuleUncertainty) {
		t.Fatalf("uncertainty = %v, want the C# precision limits and no profile gap", uncertainty)
	}
	if !anyContains(uncertainty, "a global using of namespace Acme.Ledger reaches every file of its project") {
		t.Fatalf("uncertainty = %v, want the global using reach disclosed", uncertainty)
	}
}

// TestNGIV0983SameModuleMembersAreReferences: a member of the changed path's
// SwiftPM target, Kotlin package or enclosing C# namespace that names one of
// its declarations is a same-module reference (NGI-V0-983).
func TestNGIV0983SameModuleMembersAreReferences(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, changed, want string
		files               map[string]string
	}{
		{"swift", "Sources/Core/Ledger.swift", "Sources/Core/Audit.swift", swiftPackageFixture},
		{"kotlin", "src/main/kotlin/com/acme/ledger/Ledger.kt", "src/main/kotlin/com/acme/ledger/Audit.kt", kotlinFixture},
		{"csharp", "src/Ledger/LedgerBook.cs", "src/Ledger/Checks/Audit.cs", csharpFixture},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, _ := moduleImpact(t, test.files, test.changed)
			if _, ok := moduleImpactRows(result, "reference")[test.want]; !ok {
				t.Fatalf("references = %v, want %s", moduleImpactRows(result, "reference"), test.want)
			}
		})
	}
}

// TestNGIV0984UnmappedModulePathKeepsProfileGap: a Swift file no SwiftPM
// target claims has no module, so the profile gap stays; a Kotlin build
// script carries no reading (NGI-V0-984).
func TestNGIV0984UnmappedModulePathKeepsProfileGap(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, changed string
		files         map[string]string
	}{
		{"swift-no-target", "Scripts/Loose.swift", map[string]string{"Package.swift": swiftPackageFixture["Package.swift"], "Sources/Core/Ledger.swift": "struct Ledger {}\n", "Scripts/Loose.swift": "struct Loose {}\n"}},
		{"kotlin-script", "build.gradle.kts", map[string]string{"build.gradle.kts": "plugins {}\n", "src/main/kotlin/App.kt": "class App\n"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, uncertainty := moduleImpact(t, test.files, test.changed)
			if !anyContains(uncertainty, moduleProfileGap) {
				t.Fatalf("uncertainty = %v, want the profile gap for an unmapped path", uncertainty)
			}
			if len(moduleImpactRows(result, "reverse-import")) != 0 {
				t.Fatalf("an unmapped path reported reverse imports: %v", result["results"])
			}
		})
	}
}

// TestNGIV0986CapabilitiesListModuleReferenceSuffixes: the capability listing
// names the module-reading suffixes, none of which is a GPK-V0-027 rule
// (NGI-V0-986).
func TestNGIV0986CapabilitiesListModuleReferenceSuffixes(t *testing.T) {
	t.Parallel()
	suffixes := ImpactCapabilities(10)["suffixes"].(map[string]any)
	moduled := suffixes["withModuleReferenceRule"].([]string)
	if len(moduled) != 3 || moduled[0] != ".cs" || moduled[1] != ".kt" || moduled[2] != ".swift" {
		t.Fatalf("withModuleReferenceRule = %v, want [.cs .kt .swift]", moduled)
	}
	for _, suffix := range moduled {
		if ImpactRuleNamed("a"+suffix) || !stringIn(suffixes["admittedWithoutRule"].([]string), suffix) {
			t.Fatalf("%s is a GPK-V0-027 rule or not admitted without one", suffix)
		}
	}
}
