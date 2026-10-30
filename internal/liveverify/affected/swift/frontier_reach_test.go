// SPDX-License-Identifier: Apache-2.0
package swift

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-511: a conditional compilation block binds only `.swift` paths, so a dirty path no
// plugin owns drops it; dynamic test discovery binds the runner, so a Makefile keeps it while a
// Go-hosted testdata file drops it; the undeclared exclusion-evidence reason stays everywhere;
// a dirty YAML file, which the plugin reads as metadata, keeps every entry.
func TestFrontierReachHoldsForUnownedPaths_AFPV0511(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "Package.swift", "import PackageDescription\nlet package = Package(name: \"Fixture\", targets: [.target(name: \"Core\"), .testTarget(name: \"CoreTests\", dependencies: [\"Core\"])])\n")
	write(t, root, "Sources/Core/Core.swift", "#if DEBUG\nimport Foundation\n#endif\npublic let core = 1\n")
	write(t, root, "Tests/CoreTests/CoreTests.swift", "import XCTest\n@testable import Core\nfinal class DynamicTests: XCTestCase {\n    override class var defaultTestSuite: XCTestSuite { XCTestSuite(name: \"generated\") }\n}\n")
	before, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	all := []string{FrontierConditionalCompilation, FrontierDynamicTestDiscovery, FrontierExclusionContract}
	if fmt.Sprint(before.Frontier) != fmt.Sprint(all) {
		t.Fatalf("plugin frontier = %v", before.Frontier)
	}
	write(t, root, "Makefile", "all:\n\tswift test\n")
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
		{"Makefile", []string{FrontierDynamicTestDiscovery, FrontierExclusionContract}},
		{"pkg/testdata/cases.json", []string{FrontierExclusionContract}},
		{"ci/pipeline.yml", all},
		{"Sources/Core/Core.swift", all},
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
