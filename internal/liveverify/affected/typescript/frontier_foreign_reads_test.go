// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AZU-V1-960, V1-4683: normal resolved imports must not erase JSON alias
// targets. Extensionless imports and every hidden overflow site remain readers.
func TestForeignJSONFrontierReaders_AZU_V1_960(t *testing.T) {
	for _, tc := range []struct{ name, specifier, config string }{
		{"exact alias", "@cfg", `{"compilerOptions":{"baseUrl":".","paths":{"@cfg":["data.json"]}}}`},
		{"extensionless alias", "@cfg", `{"compilerOptions":{"baseUrl":".","paths":{"@cfg":["data"]}}}`},
		{"relative", "../data", "{}"},
		{"package imports", "#cfg", "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", `{"imports":{"#cfg":"./data.json"},"devDependencies":{"vitest":"1"}}`)
			write(t, root, "tsconfig.json", tc.config)
			write(t, root, "data.json", `{"value":1}`)
			write(t, root, "helper.ts", "export const helper = 1\n")
			var want []string
			for i := 0; i < frontierSiteBound+2; i++ {
				file := fmt.Sprintf("tests/read%03d.test.ts", i)
				want = append(want, file)
				write(t, root, file, "import { test } from 'vitest';\nimport { helper } from '../helper';\nimport data from '"+tc.specifier+"';\ntest('read', () => data.value + helper);\n")
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"data.json"})
			for _, file := range want {
				if !slices.Contains(plan.SelectedTests(), file) {
					t.Errorf("missing importer %s: %+v", file, plan.Unknown)
				}
			}
			other := affected.Select(graph, []string{"unrelated.json"})
			if len(other.SelectedTests()) != 0 {
				t.Errorf("resolved alias tables should exclude unrelated data: %v", other.SelectedTests())
			}
		})
	}
}

// AZU-V1-961: scanned YAML is a resolution input only when its path denotes
// a runner configuration or a supported test asset; arbitrary CI/data YAML
// must not steer every TypeScript frontier site.
func TestYAMLResolutionInputs_AZU_V1_961(t *testing.T) {
	for file, want := range map[string]bool{
		".github/workflows/ci.yml": false, "data/items.yaml": false, "pnpm-lock.yaml": false,
		"tests/.mocharc.yaml": true, "e2e/flow.yml": true, "maestro/login.yaml": true,
		"config/package.json": true, "tsconfig.base.json": true,
	} {
		if got := New().ResolutionInput(file); got != want {
			t.Errorf("%s: input=%v want %v", file, got, want)
		}
	}
}

// Unsupported mappings must never become a proof of no foreign read.
func TestPackageImportTargetFailures_AZU_V1_960(t *testing.T) {
	for _, body := range []string{`{}`, `{"imports":{"#cfg":null}}`, `{"imports":{"#cfg":["./data.json"]}}`, `{"imports":{"#cfg":"external"}}`, `{"imports":{"#cfg":"./../../data.json"}}`, `{"imports":{"#cfg":{"default":{"default":{"default":[]}}}}}`} {
		_, bounded := packageImportTargets("tests/a.test.ts", "#cfg", map[string]string{"package.json": body})
		if !bounded {
			t.Errorf("unsupported mapping narrowed: %s", body)
		}
	}
	targets, bounded := packageImportTargets("tests/a.test.ts", "#cfg/a", map[string]string{"package.json": `{"imports":{"#cfg/*":{"node":"./data/*.json","default":"./fallback/*.json"}}}`})
	if bounded || !slices.Contains(targets, "data/a.json") || !slices.Contains(targets, "fallback/a.json") {
		t.Fatalf("lost conditional alternatives: %v bounded=%v", targets, bounded)
	}
	root := t.TempDir()
	write(t, root, "package.json", `{"imports":{"#cfg":"external"},"devDependencies":{"vitest":"1"}}`)
	write(t, root, "test.test.ts", "import { test } from 'vitest';\nimport data from '#cfg';\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"data.json"})
	if len(frontierDetails(plan)) == 0 {
		t.Fatalf("unsupported mapping lost unknown: %+v", plan)
	}
}

// AZU-V0-003: every TypeScript OUTBOUND label starts at the importing file;
// changing another source selects that site and its consumer, closing it.
func TestOutboundKindsSelectTheirSites_AZU_V0_003(t *testing.T) {
	for reason, statement := range map[string]string{
		FrontierUnresolvedImport:     "import './missing';",
		FrontierBareImportUndeclared: "import 'undeclared-package';",
		FrontierDynamicImport:        "import(name);",
	} {
		t.Run(reason, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
			write(t, root, "changed.ts", "export const changed = 1;\n")
			write(t, root, "site.ts", statement+"\nexport const site = 1;\n")
			write(t, root, "site.test.ts", "import {test} from 'vitest';\nimport {site} from './site';\n")
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.FrontierSites[reason]) != 1 {
				t.Fatalf("missing actual site for %s: %+v", reason, result)
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"changed.ts"})
			if !slices.Contains(plan.SelectedTests(), "site.test.ts") || len(frontierDetails(plan)) != 0 {
				t.Fatalf("site consumer omitted or unknown retained: %+v", plan)
			}
		})
	}
}
