package dotnet

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestUnitsDetectsEverySupportedFrameworkAndProducesRunnableFilters(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" />
    <PackageReference Include="xunit" />
	<PackageReference Include="xunit.runner.visualstudio" />
    <PackageReference Include="NUnit" />
	<PackageReference Include="NUnit3TestAdapter" />
    <PackageReference Include="MSTest.TestFramework" />
	<PackageReference Include="MSTest.TestAdapter" />
    <PackageReference Include="Microsoft.Playwright" />
    <PackageReference Include="Selenium.WebDriver" />
  </ItemGroup>
</Project>`)
	write(t, root, "Tests/XunitTests.cs", "namespace Example;\npublic class XunitTests { [Fact] public void FactCase() {} [Theory] public void TheoryCase() {} }\n")
	write(t, root, "Tests/NUnitTests.cs", "namespace Example;\n[TestFixture] public class NUnitTests { [Test] public void TestCase() {} [TestCase(1)] public void RowCase(int value) {} [TestCaseSource(nameof(Cases))] public void SourceCase(int value) {} }\n")
	write(t, root, "Tests/MSTests.cs", "namespace Example;\n[TestClass] public class MSTests { [TestMethod] public void MethodCase() {} [DataTestMethod] public void DataCase() {} }\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	filters := make([]string, 0, 3)
	for _, unit := range result.Units {
		project, filter, ok := Address(unit.ID)
		if !ok {
			continue
		}
		if project != "Tests/Tests.csproj" {
			t.Fatalf("project=%q", project)
		}
		filters = append(filters, filter)
	}
	joined := strings.Join(filters, "|")
	for _, method := range []string{
		"Example.XunitTests.FactCase", "Example.XunitTests.TheoryCase",
		"Example.NUnitTests.TestCase", "Example.NUnitTests.RowCase", "Example.NUnitTests.SourceCase",
		"Example.MSTests.MethodCase", "Example.MSTests.DataCase",
	} {
		if !strings.Contains(joined, "FullyQualifiedName="+method) {
			t.Fatalf("missing %s in %q", method, joined)
		}
	}
}

func TestProjectReferencesSelectTheDependentTestClosure(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Core/Core.csproj", `<Project Sdk="Microsoft.NET.Sdk" />`)
	write(t, root, "Core/Core.cs", "namespace Core; public class Value {}\n")
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup>
	<ItemGroup><PackageReference Include="xunit" /><PackageReference Include="xunit.runner.visualstudio" /><PackageReference Include="Microsoft.NET.Test.Sdk" />
    <ProjectReference Include="../Core/Core.csproj" /></ItemGroup>
</Project>`)
	write(t, root, "Tests/CoreTests.cs", "namespace Tests; public class CoreTests { [Fact] public void CoversCore() {} }\n")
	write(t, root, "Tests/OtherTests.cs", "namespace Tests; public class OtherTests { [Fact] public void CoversHelper() {} }\n")
	write(t, root, "Solo/Solo.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /><PackageReference Include="NUnit" /><PackageReference Include="NUnit3TestAdapter" /></ItemGroup></Project>`)
	write(t, root, "Solo/SoloTests.cs", "namespace Solo; public class SoloTests { [Test] public void Alone() {} }\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"Core/Core.cs"})
	if got := plan.SelectedTests(); !slices.Equal(got, []string{"Tests/CoreTests.cs", "Tests/OtherTests.cs"}) {
		t.Fatalf("selected=%v", got)
	}
	if plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	testEdit := affected.Select(graph, []string{"Tests/CoreTests.cs"})
	if got := testEdit.SelectedTests(); !slices.Equal(got, []string{"Tests/CoreTests.cs", "Tests/OtherTests.cs"}) {
		t.Fatalf("test-file helper change selected=%v", got)
	}
}

func TestAmbiguousAndDynamicDiscoveryRaiseFrontiers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="NUnit" /><PackageReference Include="GdUnit4" /><PackageReference Include="Reqnroll.NUnit" /></ItemGroup></Project>`)
	write(t, root, "Tests/GodotTests.cs", "using GdUnit4; namespace Tests; public class GodotTests { [TestCase] public void NotNUnit() {} }\n")
	write(t, root, "Tests/DynamicTests.cs", "namespace Tests; public class DynamicTests { [Test, TestCaseSource(nameof(Cases))] public void Dynamic(int value) {} }\n")
	write(t, root, "Tests/CustomTests.cs", "namespace Tests; public class CustomTests { [RetryFact] void DiscoveredByExtension() {} }\n")
	write(t, root, "Tests/EscapedNamespaceTests.cs", "namespace @class; public class EscapedNamespaceTests { [Test] public void Works() {} }\n")
	write(t, root, "Tests/Login.feature", "Feature: Login\n  Scenario: success\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{FrontierUnsupportedRunner, FrontierDynamicDiscovery, FrontierFeatureOwnership, FrontierUnparsedSource} {
		if !slices.Contains(result.Frontier, want) {
			t.Fatalf("missing frontier %s in %v", want, result.Frontier)
		}
	}
	for _, unit := range result.Units {
		if slices.Contains(unit.Tests, "Tests/GodotTests.cs") {
			t.Fatal("GdUnit4 TestCase was misclassified as NUnit")
		}
	}
}

func TestMissingVSTestSdkRaisesRunnerFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="xunit" /><PackageReference Include="xunit.runner.visualstudio" /></ItemGroup></Project>`)
	write(t, root, "Tests/Tests.cs", "namespace Tests; public class Tests { [Fact] public void Works() {} }\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Frontier, FrontierRunner) {
		t.Fatalf("missing runner frontier: %v", result.Frontier)
	}
}

func TestTestProjectWithoutRecognizedFrameworkWidensScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Lib/Lib.csproj", `<Project Sdk="Microsoft.NET.Sdk" />`)
	write(t, root, "Lib/Calc.cs", "namespace Lib; public static class Calc { public static int Add(int a, int b) => a + b; }\n")
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /><PackageReference Include="xunit.core" /><PackageReference Include="xunit.runner.visualstudio" /><ProjectReference Include="..\Lib\Lib.csproj" /></ItemGroup></Project>`)
	write(t, root, "Tests/CalcTests.cs", "namespace Tests; public class CalcTests { [Fact] public void Adds() {} }\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"Lib/Calc.cs"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestAttributesWithoutProjectEvidenceAreNotClaimedAsTests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "App/App.csproj", `<Project Sdk="Microsoft.NET.Sdk" />`)
	write(t, root, "App/Fact.cs", "namespace App; public class FactAttribute : System.Attribute {} public class Example { [Fact] public void Ordinary() {} }\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || len(result.Units[0].Tests) != 0 {
		t.Fatalf("units=%+v", result.Units)
	}
}

func TestOwnsOnlyCSharpAndAddressRejectsCompilationUnits(t *testing.T) {
	language := New()
	if !language.Owns("src/A.CS") || language.Owns("spec.feature") || language.Owns("flow.yaml") {
		t.Fatal("ownership does not preserve the C#-only boundary")
	}
	if _, _, ok := Address("dotnet:Tests/Tests.csproj"); ok {
		t.Fatal("a compilation unit has no test filter")
	}
	project, filter, ok := Address("dotnet:Tests/Tests.csproj::FullyQualifiedName=Tests.A.Works")
	if !ok || project != "Tests/Tests.csproj" || filter != "FullyQualifiedName=Tests.A.Works" {
		t.Fatalf("address=(%q,%q,%v)", project, filter, ok)
	}
}

func TestNonCSharpProjectReferencingCSharpWidensInsteadOfExcluding(t *testing.T) {
	for _, extension := range []string{"fsproj", "VBPROJ"} {
		root := t.TempDir()
		write(t, root, "Core/Core.csproj", `<Project Sdk="Microsoft.NET.Sdk" />`)
		write(t, root, "Core/Core.cs", "namespace Core; public class Value {}\n")
		write(t, root, "Tests/Tests."+extension, `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="xunit" /><PackageReference Include="Microsoft.NET.Test.Sdk" /><ProjectReference Include="../Core/Core.csproj" /></ItemGroup></Project>`)
		write(t, root, "Tests/Tests.fs", "module Tests\n")

		graph, err := affected.Build(root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"Core/Core.cs"})
		want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierProjectReference}
		if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 1 || plan.Unknown[0] != want {
			t.Errorf("%s: scope=%s unknown=%v", extension, plan.Scope, plan.Unknown)
		}
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
