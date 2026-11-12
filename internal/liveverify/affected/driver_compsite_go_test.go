// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3601: a Go toolchain selection assignment is no computed invocation.
// A change to a file only the selected variant compiles still selects the
// target through the Go claimant's union of variants, and a path nothing
// names selects neither the target nor the job.
func TestDriverGoSelectionAssignments_AFP_V0_3601(t *testing.T) {
	for _, recipe := range []string{
		"GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOENV=off GOWORK=off GOPROXY=off go vet ./...",
		"env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...",
		`GOOS="darwin" GOARCH='arm64' go vet ./core`,
		"GOWORK=off; go vet ./...",
	} {
		t.Run(recipe, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":                 "check:\n\t" + recipe + "\nall: check\nother:\n\techo done\n",
				".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: |\n          " + strings.ReplaceAll(recipe, "'", "") + "\n  caller:\n    steps:\n      - run: make all\n",
				"core/core_windows.go":     "package core\n\nfunc windowsValue() int { return 2 }\n",
				"core/core_darwin.go":      "package core\n\nfunc darwinValue() int { return 3 }\n",
			}, nil)
			for _, id := range []string{"driver:make:Makefile#check", jobUnit} {
				unit, _ := graph.Unit(id)
				if len(unit.Frontier) != 0 {
					t.Fatalf("%s is still a computed site: %+v", id, unit)
				}
			}
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit, "driver:ci:.github/workflows/ci.yml#caller"}
			for _, dirty := range []string{"core/core_windows.go", "core/core_darwin.go"} {
				wantDriverSelection(t, driverSelections(affected.Select(graph, []string{dirty})), units, nil)
			}
			wantDriverSelection(t, driverSelections(affected.Select(graph, []string{"docs/build-log/2026-11-11-probe.md"})), nil, units)
		})
	}
}
