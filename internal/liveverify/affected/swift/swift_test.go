package swift

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsOnlySwiftSource(t *testing.T) {
	language := New()
	for _, relative := range []string{"Sources/App.swift", "Package.swift", "Tests/AppTests.swift"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{".maestro/login.yaml", "features/login.feature", "package.json"} {
		if language.Owns(relative) {
			t.Fatalf("%s must remain cross-language", relative)
		}
	}
}

func TestSwiftPMTargetIsTheRunnableMixedFrameworkUnit(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Package.swift", `
import PackageDescription
let package = Package(name: "Fixture", targets: [
    .target(name: "Core"),
    .target(name: "Solo"),
    .testTarget(name: "CoreTests", dependencies: ["Core"]),
])
`)
	write(t, root, "Sources/Core/Core.swift", "public struct Core {}\n")
	write(t, root, "Sources/Solo/Solo.swift", "public struct Solo {}\n")
	write(t, root, "Tests/CoreTests/LegacyTests.swift", `
import XCTest
@testable import Core
final class LegacyTests: XCTestCase { func testCore() {} }
`)
	write(t, root, "Tests/CoreTests/ModernTests.swift", `
import Testing
@Test func coreWorks() { #expect(true) }
`)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	if got := graph.Frontier(); !equal(got, []string{FrontierExclusionContract}) {
		t.Fatalf("frontier=%v", got)
	}
	const testID = "swift:spm:./CoreTests"
	unit, ok := graph.Unit(testID)
	if !ok {
		t.Fatalf("missing %s; units=%v", testID, graph.UnitIDs())
	}
	wantTests := []string{"Tests/CoreTests/LegacyTests.swift", "Tests/CoreTests/ModernTests.swift"}
	if !equal(unit.Tests, wantTests) {
		t.Fatalf("tests=%v want=%v", unit.Tests, wantTests)
	}
	if !equal(unit.Imports, []string{"swift:spm:./Core"}) {
		t.Fatalf("imports=%v", unit.Imports)
	}
	plan := affected.Select(graph, []string{"Sources/Core/Core.swift"})
	if plan.Scope != affected.ScopeUnknown || !equal(plan.SelectedTests(), wantTests) {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestSwiftPMReadsExplicitPathsSourcesExcludesAndDependencies(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Nested/Package.swift", `
import PackageDescription
let package = Package(name: "Fixture", targets: [
    .target(name: "Library", path: "Code", exclude: ["Generated"], sources: ["Live.swift"]),
    .testTarget(name: "LibraryTests", dependencies: ["Library"], path: "Checks"),
])
`)
	write(t, root, "Nested/Code/Live.swift", "public struct Live {}\n")
	write(t, root, "Nested/Code/Generated/Omitted.swift", "public struct Omitted {}\n")
	write(t, root, "Nested/Checks/LibraryTests.swift", "import XCTest\n@testable import Library\nfinal class LibraryTests: XCTestCase { func testLive() {} }\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	units := unitMap(result.Units)
	if !equal(units["swift:spm:Nested/Library"].Sources, []string{"Nested/Code/Live.swift"}) {
		t.Fatalf("library=%+v", units["swift:spm:Nested/Library"])
	}
	if !contains(result.Frontier, FrontierUnmappedSource) {
		t.Fatalf("excluded owned source must be explicit unknown: %v", result.Frontier)
	}
}

func TestPackageScannerIgnoresCommentsAndClosesNestedDependencyLists(t *testing.T) {
	targets, complete := scanPackageTargets(`
// .testTarget(name: "Phantom")
/* .target(name: "AlsoPhantom") */
.testTarget(
    name: "RealTests",
    dependencies: [
        .product(name: "External", package: "external", condition: .when(platforms: [.macOS])),
        "LocalAfterNestedList",
    ]
)
`)
	if !complete || len(targets) != 1 || targets[0].name != "RealTests" {
		t.Fatalf("targets=%+v complete=%v", targets, complete)
	}
	if !contains(targets[0].dependencies, "LocalAfterNestedList") {
		t.Fatalf("dependencies=%v", targets[0].dependencies)
	}
}

func TestPackageScannerRejectsComputedLists(t *testing.T) {
	for name, manifest := range map[string]string{
		"bare dependency":           `.testTarget(name: "Tests", dependencies: [computedDependency])`,
		"computed dependency name":  `.testTarget(name: "Tests", dependencies: [.target(name: computedName)])`,
		"interpolated dependency":   `.testTarget(name: "Tests", dependencies: ["Core\(suffix)"])`,
		"interpolated target dep":   `.testTarget(name: "Tests", dependencies: [.target(name: "Core\(suffix)")])`,
		"concatenated target name":  `.testTarget(name: "Tests" + suffix)`,
		"concatenated target path":  `.testTarget(name: "Tests", path: "Checks" + suffix)`,
		"dependency prefix literal": `.testTarget(name: "Tests", dependencies: [.target(name: "Core" + suffix)])`,
		"concatenated dependencies": `.testTarget(name: "Tests", dependencies: ["Core"] + optionalDependencies)`,
		"computed sources":          `.testTarget(name: "Tests", sources: ["Known.swift", computedSource])`,
		"interpolated source":       `.testTarget(name: "Tests", sources: ["Known\(suffix).swift"])`,
	} {
		t.Run(name, func(t *testing.T) {
			_, complete := scanPackageTargets(manifest)
			if complete {
				t.Fatalf("computed manifest was certified complete: %s", manifest)
			}
		})
	}
}

func TestUnresolvedAndDynamicDiscoveryRaiseFrontiers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Package.swift", `
import PackageDescription
let testName = "CoreTests"
let package = Package(name: "Fixture", targets: [.testTarget(name: testName)])
`)
	write(t, root, "Tests/CoreTests/CoreTests.swift", `
import XCTest
final class DynamicTests: XCTestCase {
    override class var defaultTestSuite: XCTestSuite { XCTestSuite(name: "generated") }
}
`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{FrontierManifestUnresolved, FrontierUnmappedSource} {
		if !contains(result.Frontier, want) {
			t.Fatalf("frontier=%v missing %s", result.Frontier, want)
		}
	}

	write(t, root, "Package.swift", `
import PackageDescription
let package = Package(name: "Fixture", targets: [.testTarget(name: "CoreTests")])
`)
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}

	write(t, root, "Tests/CoreTests/CoreTests.swift", `
import Testing
@Test(arguments: runtimeArguments()) func generated(value: Int) {}
`)
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("parameterized frontier=%v", result.Frontier)
	}

	write(t, root, "Tests/CoreTests/CoreTests.swift", `
import Testing
@Testing.Test(arguments: [1], runtimeArguments()) func generated(value: Int) {}
`)
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("qualified parameterized frontier=%v", result.Frontier)
	}

	write(t, root, "Tests/CoreTests/CoreTests.swift", `
import Testing
@Test(arguments: [1, 2, 3]) func literal(value: Int) {}
`)
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("literal parameter inventory raised dynamic frontier: %v", result.Frontier)
	}

	write(t, root, "Tests/CoreTests/CoreTests.swift", `
import Testing
@Test(arguments: [1, 2], [3, 4]) func literalPair(left: Int, right: Int) {}
`)
	result, err = New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("multiple literal inventories raised dynamic frontier: %v", result.Frontier)
	}
}

func TestXcodeUnitAndUITestTargetsStaySeparatelyAddressable(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\n@testable import Core\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\n@testable import Core\nfinal class AppUITests: XCTestCase { func testLaunch() { _ = XCUIApplication() } }\n")
	write(t, root, "Fixture.xcodeproj/project.pbxproj", pbxFixture)
	write(t, root, "Fixture.xcodeproj/xcshareddata/xcschemes/Core.xcscheme", schemeFixture("BBBBBBBBBBBBBBBBBBBBBBBB"))
	write(t, root, "Fixture.xcodeproj/xcshareddata/xcschemes/UI.xcscheme", schemeFixture("CCCCCCCCCCCCCCCCCCCCCCCC"))
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	if got := graph.Frontier(); !equal(got, []string{FrontierExclusionContract}) {
		t.Fatalf("frontier=%v", got)
	}
	wantIDs := []string{
		"swift:xcode:Fixture.xcodeproj/Core",
		"swift:xcode:Fixture.xcodeproj/Core/CoreTests",
		"swift:xcode:Fixture.xcodeproj/UI/AppUITests",
	}
	if !equal(graph.UnitIDs(), wantIDs) {
		t.Fatalf("units=%v want=%v", graph.UnitIDs(), wantIDs)
	}
	plan := affected.Select(graph, []string{"App/Core.swift"})
	wantTests := []string{"Tests/CoreTests.swift", "UITests/AppUITests.swift"}
	if !equal(plan.SelectedTests(), wantTests) {
		t.Fatalf("selected=%v want=%v", plan.SelectedTests(), wantTests)
	}
}

func TestXcodeTestWithoutSharedSchemeIsUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase {}\n")
	write(t, root, "Fixture.xcodeproj/project.pbxproj", pbxFixture)
	write(t, root, "Fixture.xcodeproj/xcshareddata/xcschemes/Core.xcscheme", strings.ReplaceAll(schemeFixture("BBBBBBBBBBBBBBBBBBBBBBBB"), `skipped="NO"`, `skipped="YES"`))
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierXcodeSchemeUnresolved) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestXcodeDoesNotGuessAUniqueBasenameOutsideItsGroup(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Elsewhere/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase { func testLaunch() {} }\n")
	body := strings.ReplaceAll(pbxFixture, "path = App;", "path = MissingApp;")
	write(t, root, "Fixture.xcodeproj/project.pbxproj", body)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierXcodeProjectUnresolved) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	for _, unit := range result.Units {
		if contains(unit.Sources, "Elsewhere/Core.swift") {
			t.Fatalf("stale PBX group silently claimed unique basename: %+v", unit)
		}
	}
}

func TestXcodeRejectsAGroupDetachedFromTheProjectMainGroup(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase { func testLaunch() {} }\n")
	body := strings.Replace(pbxFixture, "mainGroup = 444444444444444444444444;", "mainGroup = 411111111111111111111111;", 1)
	write(t, root, "Fixture.xcodeproj/project.pbxproj", body)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierXcodeProjectUnresolved) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	for _, unit := range result.Units {
		if contains(unit.Tests, "Tests/CoreTests.swift") || contains(unit.Tests, "UITests/AppUITests.swift") {
			t.Fatalf("detached group was treated as project-owned: %+v", unit)
		}
	}
}

func TestXCUITestRequiresAnApplicationHostDependency(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase { func testLaunch() {} }\n")
	body := strings.ReplaceAll(pbxFixture, "com.apple.product-type.application", "com.apple.product-type.framework")
	write(t, root, "Fixture.xcodeproj/project.pbxproj", body)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierXcodeUIHostUnresolved) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestXcodeTestShellPhaseMakesGeneratedDiscoveryUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase { func testLaunch() {} }\n")
	body := strings.Replace(pbxFixture, "333333333333333333333333 = {isa = PBXSourcesBuildPhase;", "555555555555555555555555 = {isa = PBXShellScriptBuildPhase; shellScript = generate; };\n333333333333333333333333 = {isa = PBXSourcesBuildPhase;", 1)
	body = strings.Replace(body, "buildPhases = (322222222222222222222222,);", "buildPhases = (322222222222222222222222, 555555555555555555555555,);", 1)
	write(t, root, "Fixture.xcodeproj/project.pbxproj", body)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestXcodeTestDependencyShellPhaseMakesGeneratedDiscoveryUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Tests/CoreTests.swift", "import XCTest\nfinal class CoreTests: XCTestCase { func testCore() {} }\n")
	write(t, root, "UITests/AppUITests.swift", "import XCTest\nfinal class AppUITests: XCTestCase { func testLaunch() {} }\n")
	body := strings.Replace(pbxFixture, "311111111111111111111111 = {isa = PBXSourcesBuildPhase;", "555555555555555555555555 = {isa = PBXShellScriptBuildPhase; shellScript = generate; };\n311111111111111111111111 = {isa = PBXSourcesBuildPhase;", 1)
	body = strings.Replace(body, "buildPhases = (311111111111111111111111,);", "buildPhases = (311111111111111111111111, 555555555555555555555555,);", 1)
	body = strings.Replace(body, "dependencies = ();\nname = CoreTests;", "dependencies = (DDDDDDDDDDDDDDDDDDDDDDDD,);\nname = CoreTests;", 1)
	write(t, root, "Fixture.xcodeproj/project.pbxproj", body)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDiscovery) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
}

func TestCrossLanguageHarnessesAreObservedButNotOwned(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Package.swift", `
import PackageDescription
let package = Package(name: "Fixture", targets: [.target(name: "Core")])
`)
	write(t, root, "Sources/Core/Core.swift", "public struct Core {}\n")
	write(t, root, ".maestro/login.yaml", "appId: example.app\n---\n- launchApp\n")
	write(t, root, "package.json", `{"devDependencies":{"appium":"latest"}}`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{FrontierMaestroExternal, FrontierAppiumExternal} {
		if !contains(result.Frontier, want) {
			t.Fatalf("frontier=%v missing %s", result.Frontier, want)
		}
	}
	if New().Owns(".maestro/login.yaml") || New().Owns("package.json") {
		t.Fatal("external harness files must not be claimed")
	}
}

func TestOverlappingBuildMembershipIsUnknownNotDuplicateOwnership(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Package.swift", `
import PackageDescription
let package = Package(name: "Fixture", targets: [.target(name: "Core", path: "App")])
`)
	write(t, root, "App/Core.swift", "public struct Core {}\n")
	write(t, root, "Fixture.xcodeproj/project.pbxproj", pbxFixture)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierOverlappingMembership) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	if _, err := affected.Build(root, New()); err != nil {
		t.Fatalf("overlap must widen, not fail graph admission: %v", err)
	}
}

func TestRealSwiftRepositoryObservation(t *testing.T) {
	root := os.Getenv("CORVINT_SWIFT_REAL_REPO")
	if root == "" {
		t.Skip("CORVINT_SWIFT_REAL_REPO is not set")
	}
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	testUnits := 0
	testFiles := 0
	uiUnits := 0
	for _, unit := range result.Units {
		if len(unit.Tests) == 0 {
			continue
		}
		testUnits++
		testFiles += len(unit.Tests)
		ui := false
		for _, test := range unit.Tests {
			if strings.Contains(strings.ToLower(test), "e2e") || strings.Contains(strings.ToLower(test), "uitest") {
				ui = true
			}
		}
		if ui {
			uiUnits++
		}
	}
	expected := 0
	if value := os.Getenv("CORVINT_SWIFT_EXPECT_TEST_FILES"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			t.Fatalf("CORVINT_SWIFT_EXPECT_TEST_FILES: %v", parseErr)
		}
		expected = parsed
		if testFiles != expected {
			t.Fatalf("discovered %d test files, independently audited ground truth is %d", testFiles, expected)
		}
	}
	t.Logf("real Swift repository: units=%d testUnits=%d testFiles=%d expected=%d uiUnits=%d frontier=%v", len(result.Units), testUnits, testFiles, expected, uiUnits, result.Frontier)
	for _, unit := range result.Units {
		if len(unit.Tests) != 0 {
			t.Logf("real Swift test unit: %s tests=%d sources=%d", unit.ID, len(unit.Tests), len(unit.Sources))
		}
	}
	if testUnits == 0 || testFiles == 0 {
		t.Fatal("real repository observation found no tests")
	}
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

func unitMap(units []affected.Unit) map[string]affected.Unit {
	result := make(map[string]affected.Unit, len(units))
	for _, unit := range units {
		result[unit.ID] = unit
	}
	return result
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

func schemeFixture(target string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<Scheme><TestAction><Testables><TestableReference skipped="NO"><BuildableReference
BlueprintIdentifier="` + target + `" ReferencedContainer="container:Fixture.xcodeproj">
</BuildableReference></TestableReference></Testables></TestAction></Scheme>`
}

const pbxFixture = `// !$*UTF8*$!
{
objects = {
111111111111111111111111 = {isa = PBXBuildFile; fileRef = 211111111111111111111111; };
122222222222222222222222 = {isa = PBXBuildFile; fileRef = 222222222222222222222222; };
133333333333333333333333 = {isa = PBXBuildFile; fileRef = 233333333333333333333333; };
211111111111111111111111 = {isa = PBXFileReference; path = Core.swift; sourceTree = "<group>"; };
222222222222222222222222 = {isa = PBXFileReference; path = CoreTests.swift; sourceTree = "<group>"; };
233333333333333333333333 = {isa = PBXFileReference; path = AppUITests.swift; sourceTree = "<group>"; };
411111111111111111111111 = {isa = PBXGroup; children = (211111111111111111111111,); path = App; sourceTree = "<group>"; };
422222222222222222222222 = {isa = PBXGroup; children = (222222222222222222222222,); path = Tests; sourceTree = "<group>"; };
433333333333333333333333 = {isa = PBXGroup; children = (233333333333333333333333,); path = UITests; sourceTree = "<group>"; };
444444444444444444444444 = {isa = PBXGroup; children = (411111111111111111111111, 422222222222222222222222, 433333333333333333333333,); sourceTree = "<group>"; };
311111111111111111111111 = {isa = PBXSourcesBuildPhase; files = (111111111111111111111111,); };
322222222222222222222222 = {isa = PBXSourcesBuildPhase; files = (122222222222222222222222,); };
333333333333333333333333 = {isa = PBXSourcesBuildPhase; files = (133333333333333333333333,); };
DDDDDDDDDDDDDDDDDDDDDDDD = {isa = PBXTargetDependency; target = AAAAAAAAAAAAAAAAAAAAAAAA; };
AAAAAAAAAAAAAAAAAAAAAAAA = {
isa = PBXNativeTarget;
buildPhases = (311111111111111111111111,);
dependencies = ();
name = Core;
productType = "com.apple.product-type.application";
};
BBBBBBBBBBBBBBBBBBBBBBBB = {
isa = PBXNativeTarget;
buildPhases = (322222222222222222222222,);
dependencies = ();
name = CoreTests;
productType = "com.apple.product-type.bundle.unit-test";
};
CCCCCCCCCCCCCCCCCCCCCCCC = {
isa = PBXNativeTarget;
buildPhases = (333333333333333333333333,);
dependencies = (DDDDDDDDDDDDDDDDDDDDDDDD,);
name = AppUITests;
productType = "com.apple.product-type.bundle.ui-testing";
};
EEEEEEEEEEEEEEEEEEEEEEEE = {
isa = PBXProject;
mainGroup = 444444444444444444444444;
TargetAttributes = {
AAAAAAAAAAAAAAAAAAAAAAAA = { DevelopmentTeam = ""; };
};
};
};
}`
