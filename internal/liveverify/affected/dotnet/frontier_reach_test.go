// SPDX-License-Identifier: Apache-2.0
package dotnet

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-511: an unparsed source binds only `.cs` paths, so a dirty path no plugin owns drops
// it; dynamic test discovery binds the runner, so a Makefile keeps it while a Go-hosted testdata
// file drops it; a dirty project file, which the plugin reads, keeps every entry.
func TestFrontierReachHoldsForUnownedPaths_AFPV0511(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "Tests/Tests.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="NUnit" /><PackageReference Include="NUnit3TestAdapter" /><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>`)
	write(t, root, "Tests/DynamicTests.cs", "namespace Tests; public class DynamicTests { [Test, TestCaseSource(nameof(Cases))] public void Dynamic(int value) {} }\n")
	write(t, root, "Tests/EscapedNamespaceTests.cs", "namespace @class; public class EscapedNamespaceTests { [Test] public void Works() {} }\n")
	before, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(before.Frontier) != fmt.Sprint([]string{FrontierDynamicDiscovery, FrontierUnparsedSource}) {
		t.Fatalf("plugin frontier = %v", before.Frontier)
	}
	write(t, root, "Makefile", "all:\n\tdotnet test\n")
	write(t, root, "pkg/testdata/cases.json", `[{"name": "a"}]`)
	after, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unowned paths changed the observation:\n%+v\n%+v", before, after)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  []string
	}{
		{"Makefile", []string{FrontierDynamicDiscovery}},
		{"pkg/testdata/cases.json", []string{}},
		{"Tests/Tests.csproj", []string{FrontierDynamicDiscovery, FrontierUnparsedSource}},
		{"Tests/DynamicTests.cs", []string{FrontierDynamicDiscovery, FrontierUnparsedSource}},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		got := []string{}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownLanguageFrontier {
				got = append(got, unknown.Detail)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("dirty %s: frontier %v, want %v", tc.dirty, got, tc.want)
		}
	}
}
