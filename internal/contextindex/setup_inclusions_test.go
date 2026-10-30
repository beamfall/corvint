package contextindex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestSelectSetupInclusions proves every setup file the closure proved carries the edge chain
// from a named spec to it: a static import, a require(), a registry key and its seeder, an import
// from a seeder and a shared-constant key; and that a mode `all` widening lists only the files it
// proved, so a widened file is never given an invented reason (TSA-V0-647).
func TestSelectSetupInclusions(t *testing.T) {
	t.Parallel()
	files := setupSelectFiles()
	files["setup/alpha.setup.ts"] = "import { AlphaContext } from '../src/contexts/alpha';\nimport './base.setup';\nexport default new AlphaContext();\n"
	files["setup/base.setup.ts"] = "export const base = 1;\n"
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	spec := func(path string) SetupLink { return SetupLink{Path: path, Edge: "spec"} }
	alpha := []SetupLink{spec("tests/alpha.spec.ts"),
		{Path: "src/contexts/alpha.ts", Edge: "registry-key", Detail: "createTestData(ctx:alpha)"},
		{Path: "setup/alpha.setup.ts", Edge: "registry-seeder", Detail: "../src/contexts/alpha"}}
	cases := []struct {
		specs []string
		want  []SetupInclusion
	}{
		{[]string{"tests/users.spec.ts"}, []SetupInclusion{{Path: "setup/users.seed.ts", Chain: []SetupLink{spec("tests/users.spec.ts"),
			{Path: "setup/users.seed.ts", Edge: "import", Detail: "../setup/users.seed"}}}}},
		{[]string{"tests/required.spec.ts"}, []SetupInclusion{{Path: "setup/users.seed.ts", Chain: []SetupLink{spec("tests/required.spec.ts"),
			{Path: "setup/users.seed.ts", Edge: "require", Detail: "../setup/users.seed"}}}}},
		{[]string{"tests/alpha.spec.ts"}, []SetupInclusion{{Path: "setup/alpha.setup.ts", Chain: alpha},
			{Path: "setup/base.setup.ts", Chain: append(append([]SetupLink{}, alpha...), SetupLink{Path: "setup/base.setup.ts", Edge: "import", Detail: "./base.setup"})}}},
		{[]string{"tests/computed.spec.ts"}, []SetupInclusion{}},
	}
	for _, c := range cases {
		selection := SelectSetup(index, c.specs)
		if !reflect.DeepEqual(selection.Inclusions, c.want) {
			t.Errorf("%v: mode=%s inclusions=%+v, want %+v", c.specs, selection.Mode, selection.Inclusions, c.want)
		}
	}
	// The mode `all` widening still names every setup file; only the proved ones get a chain.
	if widened := SelectSetup(index, []string{"tests/computed.spec.ts"}); widened.Mode != "all" || len(widened.SetupFiles) != 5 {
		t.Fatalf("widened = %+v", widened)
	}

	root = impactRepositoryWithFiles(t, sharedConstantFiles())
	if index, err = BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))); err != nil {
		t.Fatal(err)
	}
	selection := SelectSetup(index, []string{"tests/fixtured.spec.ts"})
	want := []SetupInclusion{{Path: "setup/scenario_a.setup.ts", Chain: []SetupLink{spec("tests/fixtured.spec.ts"),
		{Path: "tests/fixture.ts", Edge: "import", Detail: "./fixture"},
		{Path: "lib/data/contexts/a_context.ts", Edge: "import", Detail: "../lib/data/contexts/a_context"},
		{Path: "setup/scenario_a.setup.ts", Edge: "shared-constant-key", Detail: "lib/data/shared_map.ts#SHARED.scenario_a"}}}}
	if !reflect.DeepEqual(selection.Inclusions, want) {
		t.Fatalf("shared inclusions = %+v, want %+v", selection.Inclusions, want)
	}
}
