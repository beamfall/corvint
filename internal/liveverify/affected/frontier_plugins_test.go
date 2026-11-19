// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// AZU-V0-003, AFP-V0-381: independently count the occurrences each registered
// plugin reports, both at plugin scope and per unit, including +N entries.
// A declaration absent from a plugin remains INBOUND, never inferred outbound.
func TestFrontierCountsEveryPlugin_AZU_V0_003(t *testing.T) {
	files := map[string]string{
		"go.mod":               "module example.com/frontier\n\ngo 1.22\n",
		"app.go":               "package app\n",
		"app_test.go":          "package app\n",
		"Makefile":             "test:\n\t$(COMMAND)\n",
		"app.c":                "#include UNKNOWN\nint main(void) { return 0; }\n",
		"app.py":               "import importlib\nx = importlib.import_module(name)\n",
		"test_app.py":          "import app\ndef test_app(): pass\n",
		"lib/app.rb":           "require name\n",
		"spec/app_spec.rb":     "require '../lib/app'\nRSpec.describe 'x' do\n it('works') {}\nend\n",
		"app.sh":               "#!/bin/sh\neval \"$COMMAND\"\n",
		"App.java":             "class App { void run() { Class.forName(name); } }\n",
		"App.kt":               "fun run() = Class.forName(name)\n",
		"Cargo.toml":           "[package]\nname = \"frontier\"\nversion = \"0.1.0\"\n",
		"src/lib.rs":           "#[cfg(test)] mod tests { #[test] fn works() {} }\n",
		"Tests/Tests.csproj":   `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><IsTestProject>true</IsTestProject></PropertyGroup><ItemGroup><PackageReference Include="NUnit" /></ItemGroup></Project>`,
		"Tests/AppTests.cs":    "class AppTests { [Test, TestCaseSource(nameof(Cases))] public void Run(int n) {} }\n",
		"Package.swift":        "// swift-tools-version: 5.9\nimport PackageDescription\nlet package = Package(name: \"Frontier\", targets: [.testTarget(name: \"Tests\")])\n",
		"Tests/AppTests.swift": "import XCTest\n#if DEBUG\nfinal class AppTests: XCTestCase { func testRun() {} }\n#endif\n",
		"package.json":         `{"devDependencies":{"vitest":"1"}}`,
		"app.test.ts":          "import {test} from 'vitest';\nimport(name);\n",
	}
	root := affprecFiles(t, files)
	for _, language := range languages.All() {
		t.Run(language.Name(), func(t *testing.T) {
			result, err := language.Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]int
			count := func(entries []string) {
				for _, entry := range entries {
					fields := strings.SplitN(entry, ":", 3)
					reason := entry
					n := 1
					if len(fields) >= 2 {
						reason = fields[0] + ":" + fields[1]
					}
					if len(fields) == 3 && strings.HasPrefix(fields[2], "+") {
						if parsed, err := strconv.Atoi(fields[2][1:]); err == nil && parsed > 0 {
							n = parsed
						}
					}
					if want == nil {
						want = map[string]int{}
					}
					want[reason] += n
					if direction, ok := language.(affected.FrontierDirection); ok {
						switch got := direction.FrontierDirection(reason); got {
						case affected.FrontierInbound, affected.FrontierOutbound, affected.FrontierDiscovery, affected.FrontierPackageRunnable:
						default:
							t.Errorf("unclassified %s: %q", reason, got)
						}
					}
				}
			}
			count(result.Frontier)
			var dirty []string
			for file := range files {
				if language.Owns(file) {
					dirty = append(dirty, file)
				}
			}
			for _, unit := range result.Units {
				count(unit.Frontier)
				dirty = append(dirty, unit.Sources...)
				dirty = append(dirty, unit.Tests...)
			}
			graph, err := affected.Build(context.Background(), root, language)
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, dirty)
			if !reflect.DeepEqual(plan.FrontierSites, want) {
				t.Fatalf("counts=%v want observed occurrences %v", plan.FrontierSites, want)
			}
		})
	}
}
