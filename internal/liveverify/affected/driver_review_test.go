// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestDriverTestOnlyPackageChange_AZU_V0_001(t *testing.T) {
	graph := driverGraph(t, map[string]string{
		"Makefile":                 "check:; MODE=integration go test ./core\nall: check\nother:; echo other\n",
		".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make all\n  build:\n    needs: unit\n    steps:\n      - run: echo done\n  direct:\n    steps:\n      - run: MODE=integration go test ./core\n",
		"importer/value.go":        "package importer\nimport \"example.test/m/core\"\nvar Value = core.Value\n",
		"importer/value_test.go":   "package importer\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
		"testuser/value.go":        "package testuser\n",
		"testuser/value_test.go":   "package testuser\nimport (\"testing\"; \"example.test/m/core\")\nfunc TestValue(t *testing.T) { _ = core.Value() }\n",
	}, nil)
	plan := affected.Select(graph, []string{"core/core_test.go"})
	wantDriverSelection(t, driverSelected(t, plan), []string{
		"driver:make:Makefile#check", makeAll, jobUnit, jobBuild,
		"driver:ci:.github/workflows/ci.yml#direct",
	}, []string{"driver:make:Makefile#other"})
	for _, selection := range plan.Selected {
		if selection.UnitID == "go:example.test/m/importer" || selection.UnitID == "go:example.test/m/testuser" {
			t.Errorf("test-only change escaped containment to %s", selection.UnitID)
		}
		if selection.UnitID == "driver:make:Makefile#check" {
			want := []string{"go:example.test/m/core", selection.UnitID}
			if selection.Witness.DirtyPath != "core/core_test.go" || !slices.Equal(selection.Witness.Chain(), want) {
				t.Errorf("driver witness = %+v, want test path through %v", selection.Witness, want)
			}
		}
	}
}

func TestDriverGoGlobComputed_AZU_V0_001(t *testing.T) {
	for _, pattern := range []string{"./pkgs/*", "./pkgs/?", "./pkgs/[ab]", "pkgs/*"} {
		t.Run(pattern, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:; go test " + pattern + "\nall: check\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make all\n  build:\n    needs: unit\n    steps:\n      - run: echo done\n",
				"pkgs/a/value.go":          "package a\n",
				"pkgs/a/value_test.go":     "package a\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
			}, nil)
			for _, dirty := range []string{"pkgs/a/value.go", "pkgs/a/value_test.go"} {
				wantDriverSelection(t, driverSelected(t, affected.Select(graph, []string{dirty})),
					[]string{"driver:make:Makefile#check", makeAll, jobUnit, jobBuild}, nil)
			}
			unit, found := graph.Unit("driver:make:Makefile#check")
			if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
				t.Error("glob did not retain computed invocation")
			}
		})
	}
}

func TestDriverGoEqualsDirectory_AZU_V0_001(t *testing.T) {
	for _, invocation := range []string{"-C=service test", "-C service test", "test -C=service", "test -C service"} {
		t.Run(invocation, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                  "check:; go " + invocation + " ./pkg\nall: check\n",
				".github/workflows/ci.yml":  "jobs:\n  unit:\n    steps:\n      - run: make all\n  build:\n    needs: unit\n    steps:\n      - run: echo done\n  direct:\n    steps:\n      - run: go " + invocation + " ./pkg\n",
				"service/pkg/value.go":      "package pkg\n",
				"service/pkg/value_test.go": "package pkg\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
			}, nil)
			for _, dirty := range []string{"service/pkg/value.go", "service/pkg/value_test.go"} {
				wantDriverSelection(t, driverSelected(t, affected.Select(graph, []string{dirty})),
					[]string{"driver:make:Makefile#check", makeAll, jobUnit, jobBuild, "driver:ci:.github/workflows/ci.yml#direct"}, nil)
			}
			selected := driverSelected(t, affected.Select(graph, []string{"core/core.go"}))
			wantDriverSelection(t, selected, nil, []string{"driver:make:Makefile#check", makeAll, jobUnit, jobBuild, "driver:ci:.github/workflows/ci.yml#direct"})
		})
	}
}

func TestDriverInlineInterpreterComputed_AZU_V0_001(t *testing.T) {
	for _, command := range []string{
		`node -e 'require("child_process").execFileSync("go", ["test", "./core"])'`,
		`python3 -c 'import subprocess; subprocess.run(["go", "test", "./core"])'`,
		`node --eval 'require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`node --eval='require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`node -e'require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`node --print 'require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`node --print='require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`node -p'require("child_process").execFileSync("go", ["test", "." + "/core"])'`,
		`python3 -c'import subprocess; subprocess.run(["go", "test", "." + "/core"])'`,
		`python3 -B -c 'import subprocess; subprocess.run(["go", "test", "." + "/core"])'`,
		`ruby -e'exec("go", "test", "." + "/core")'`,
		`perl -E'exec "go", "test", "." . "/core"'`,
		`pwsh -Command '& go test ("." + "/core")'`,
	} {
		t.Run(command, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:; " + command + "\nall: check\nother:; echo other\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make all\n  build:\n    needs: unit\n    steps:\n      - run: echo done\n  direct:\n    steps:\n      - run: " + command + "\n",
			}, nil)
			for _, dirty := range []string{"core/core.go", "core/core_test.go"} {
				wantDriverSelection(t, driverSelected(t, affected.Select(graph, []string{dirty})),
					[]string{"driver:make:Makefile#check", makeAll, jobUnit, jobBuild, "driver:ci:.github/workflows/ci.yml#direct"}, []string{"driver:make:Makefile#other"})
			}
			for _, id := range []string{"driver:make:Makefile#check", "driver:ci:.github/workflows/ci.yml#direct"} {
				unit, found := graph.Unit(id)
				if !found || !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) {
					t.Errorf("inline interpreter did not retain computed invocation at %s", id)
				}
			}
		})
	}
}
