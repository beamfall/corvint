// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AZU-V1-960 / V1-4683: a source lookup is not a complete foreign-read
// proof. Directory entries are candidates; unsupported package resolution and
// unknown path forms must keep uncertainty for arbitrary redirected reads.
func TestForeignModuleResolutionBoundary_AZU_V1_960(t *testing.T) {
	for _, mode := range []string{"relative", "baseUrl", "paths"} {
		for _, tc := range []struct {
			name, ref, dirty, manifest string
			unproven                   bool
		}{
			{name: "extensionless JSON", ref: "settings", dirty: "settings.json"},
			{name: "directory index", ref: "cfg", dirty: "cfg/index.json"},
			{name: "JSON suffix fallback", ref: "cfg.json", dirty: "cfg.json.json"},
			{name: "dotted directory index", ref: "cfg.json", dirty: "cfg.json/index.json"},
			{name: "package main", ref: "cfg", dirty: "elsewhere/payload.json", manifest: `{"main":"../elsewhere/payload.json"}`, unproven: true},
			{name: "unknown manifest form", ref: "cfg", dirty: "elsewhere/payload.json", manifest: `{"main":{"future":"../elsewhere/payload.json"}}`, unproven: true},
			{name: "unknown suffix directory", ref: "cfg.future", dirty: "cfg.future/index.json"},
			{name: "unknown path syntax", ref: "cfg%2Ffuture", dirty: "elsewhere/payload.json", unproven: true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
				write(t, root, "playwright.config.ts", "import {defineConfig} from '@playwright/test';\nexport default defineConfig({});\n")
				ref := tc.ref
				switch mode {
				case "relative":
					ref = "./" + ref
				case "baseUrl":
					write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":"."}}`)
				case "paths":
					write(t, root, "tsconfig.json", fmt.Sprintf(`{"compilerOptions":{"baseUrl":".","paths":{"@cfg":[%q]}}}`, ref))
					ref = "@cfg"
				}
				write(t, root, tc.dirty, `{"value":1}`)
				if tc.manifest != "" {
					write(t, root, "cfg/package.json", tc.manifest)
				}
				write(t, root, "helper.ts", "export const helper = 1;\n")
				write(t, root, "read.spec.ts", "import {test} from '@playwright/test';\nimport {helper} from './helper';\nconst data = require("+fmt.Sprintf("%q", ref)+");\ntest('read', () => data.value + helper);\n")
				result, err := New().Units(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				reader := unitByID(result, unitID(runnerPlaywright, "read.spec.ts"))
				if reader.PathTokensBounded != tc.unproven {
					t.Fatalf("read-set proof bounded=%v, want %v: %+v", reader.PathTokensBounded, tc.unproven, reader)
				}
				graph, err := affected.Build(context.Background(), root, New())
				if err != nil {
					t.Fatal(err)
				}
				plan := affected.Select(graph, []string{tc.dirty})
				if tc.unproven {
					if len(frontierDetails(plan)) == 0 {
						t.Fatalf("unproved resolution lost uncertainty: %+v", plan)
					}
				} else if !slices.Contains(plan.SelectedTests(), "read.spec.ts") {
					t.Fatalf("foreign module reader omitted: %+v", plan)
				}
			})
		}
	}
}

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

// AZU-V1-960: resolving a source fallback proves nothing about the other
// candidates, including file types the source resolver does not understand.
func TestMixedAliasFallbackReaders_AZU_V1_960(t *testing.T) {
	for _, candidate := range []string{"data.json", "data.future-format"} {
		t.Run(candidate, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
			write(t, root, "playwright.config.ts", "import { defineConfig } from '@playwright/test';\nexport default defineConfig({});\n")
			write(t, root, "tsconfig.json", fmt.Sprintf(`{"compilerOptions":{"baseUrl":".","paths":{"@cfg":[%q,"fallback.ts"]}}}`, candidate))
			write(t, root, candidate, `{"value":1}`)
			write(t, root, "fallback.ts", "export default {value: 2};\n")
			write(t, root, "read.spec.ts", "import {test} from '@playwright/test';\nimport data from '@cfg';\ntest('read', () => { console.log(data); });\n")
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if frontierHas(result, FrontierPathAlias) {
				t.Fatal("fixture must resolve under the alias gate")
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{candidate})
			if !slices.Contains(plan.SelectedTests(), "read.spec.ts") {
				t.Fatalf("lost fallback candidate reader: %+v", plan)
			}
		})
	}
}

// AZU-V1-960: package targets denote URLs. Only understood local paths can
// prove a bounded read set; unknown target forms must keep uncertainty.
func TestPackageImportURLReaders_AZU_V1_960(t *testing.T) {
	for _, tc := range []struct {
		target, dirty string
		bounded       bool
	}{
		{"./data.json?variant=one", "data.json", false},
		{"./data.json#fragment", "data.json", false},
		{"./%64ata.json", "data.json", false},
		{"./data%20one.json?variant=one", "data one.json", false},
		{"./dir%2fdata.json", "dir/data.json", true},
		{"./data%ZZ.json", "data.json", true},
		{"./data%00.json", "data.json", true},
		{"./data.json ", "data.json", true},
		{"./data%3Bfuture.json", "data;future.json", true},
		{"future:payload", "data.json", true},
	} {
		t.Run(tc.target, func(t *testing.T) {
			body := fmt.Sprintf(`{"imports":{"#cfg":%q},"devDependencies":{"vitest":"1"}}`, tc.target)
			targets, bounded := packageImportTargets("tests/read.test.ts", "#cfg", map[string]string{"package.json": body})
			if bounded != tc.bounded || !bounded && !slices.Contains(targets, tc.dirty) {
				t.Fatalf("targets=%v bounded=%v", targets, bounded)
			}
			root := t.TempDir()
			write(t, root, "package.json", body)
			write(t, root, tc.dirty, `{"value":1}`)
			write(t, root, "read.test.ts", "import {test} from 'vitest';\nimport data from '#cfg';\ntest('read', () => data.value);\n")
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{tc.dirty})
			if !tc.bounded && !slices.Contains(plan.SelectedTests(), "read.test.ts") {
				t.Fatalf("URL reader omitted: %+v", plan)
			}
			if tc.bounded && len(frontierDetails(plan)) == 0 {
				t.Fatalf("unknown URL form lost uncertainty: %+v", plan)
			}
		})
	}
}

// AZU-V1-960: source resolution cannot certify a complete runtime read set.
func TestResolvedRelativeForeignReaders_AZU_V1_960(t *testing.T) {
	for _, tc := range []struct {
		name, candidate, dirty string
		unproven               bool
	}{
		{"extensionless JSON beside source", "settings", "settings.json", false},
		{"unknown path beside source", "settings%future", "elsewhere/payload.json", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tc.candidate+".cjs", "module.exports = {value: 2};\n")
			write(t, root, tc.dirty, `{"value":1}`)
			write(t, root, "read.test.cjs", "const {test} = require('node:test');\nconst data = require("+fmt.Sprintf("%q", "./"+tc.candidate)+");\ntest('read', () => data.value);\n")
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			reader := unitByID(result, unitID(runnerNode, "read.test.cjs"))
			if !slices.Contains(reader.Imports, unitID("", tc.candidate+".cjs")) {
				t.Fatalf("fixture must resolve its source candidate: %+v", reader)
			}
			if reader.PathTokensBounded != tc.unproven {
				t.Fatalf("read-set proof bounded=%v, want %v: %+v", reader.PathTokensBounded, tc.unproven, reader)
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{tc.dirty})
			if tc.unproven {
				want := "typescript:path-token-bound:" + reader.ID
				if !slices.Contains(frontierDetails(plan), want) {
					t.Fatalf("unknown path form lost its read-set frontier: %+v", plan)
				}
			} else if !slices.Contains(plan.SelectedTests(), "read.test.cjs") {
				t.Fatalf("foreign reader omitted: %+v", plan)
			}
		})
	}
}

// AFP-V0-391/392, AZU-V1-960: regular-file imports must not poison observation
// recording with an ENOTDIR manifest probe; a warm cache must preserve selection.
func TestForeignRegularFileGraphCache_AZU_V1_960(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("graph cache change-time pins require linux or darwin")
	}
	saved := affected.GraphCacheRacyWindow
	affected.GraphCacheRacyWindow = 0
	t.Cleanup(func() { affected.GraphCacheRacyWindow = saved })
	for _, mode := range []string{"relative", "alias"} {
		t.Run(mode, func(t *testing.T) {
			root, cache := t.TempDir(), t.TempDir()
			if err := os.Chmod(cache, 0o700); err != nil {
				t.Fatal(err)
			}
			ref := "./settings.json"
			if mode == "alias" {
				ref = "@cfg"
				write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@cfg":["settings.json"]}}}`)
			}
			write(t, root, "settings.json", `{"value":1}`)
			write(t, root, "read.test.cjs", "const {test} = require('node:test');\nconst data = require("+fmt.Sprintf("%q", ref)+");\ntest('read', () => data.value);\n")
			cold, outcome, err := affected.BuildCached(context.Background(), root, cache, "sha256:foreign-file-test", New())
			if err != nil || !outcome.Stored {
				t.Fatalf("cold build must store: %+v, %v", outcome, err)
			}
			warm, outcome, err := affected.BuildCached(context.Background(), root, cache, "sha256:foreign-file-test", New())
			if err != nil || !outcome.Hit {
				t.Fatalf("warm build must hit: %+v, %v", outcome, err)
			}
			coldPlan := affected.Select(cold, []string{"settings.json"})
			warmPlan := affected.Select(warm, []string{"settings.json"})
			if cold.Digest() != warm.Digest() || !reflect.DeepEqual(coldPlan, warmPlan) {
				t.Fatal("cached graph or selection differs from fresh")
			}
			if !slices.Contains(warmPlan.SelectedTests(), "read.test.cjs") {
				t.Fatalf("cached foreign reader omitted: %+v", warmPlan)
			}
		})
	}
}
