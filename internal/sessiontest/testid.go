// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testid"
)

// GoUnitDir resolves the import path of a root Go universe unit to its
// package directory: the path under the module the root go.mod declares
// (CTI-V1-003). It is nil when root has no readable module directive, and
// then no Go outcome carries a testId.
func GoUnitDir(root string) func(importPath string) (string, bool) {
	if root == "" {
		return nil
	}
	module, err := linecov.ModulePath(root)
	if err != nil {
		return nil
	}
	return testid.GoModuleDir(module, ".")
}

// UnitTestID is the canonical test ID an execution unit names, or "" when
// it names none (CTI-V1-009, CTI-V1-012): the package-level ID of a Go
// unit, the file-level ID of a Vitest, Jest, Playwright, RSpec or Minitest
// unit. A selected or sharded Go slice still names its package; the
// outcome's slice says which of its tests ran.
func UnitTestID(unit string, goDir func(string) (string, bool)) string {
	id, ok := testid.FromUnit(unit, goDir)
	if !ok {
		return ""
	}
	return id.String()
}

// stampTestIDs adds the testId of every outcome of a shard's invocations
// whose unit names one, as the added corvint-test-run/0 field (CTI-V1-009,
// settled D25). It is part of Runner.Execute's receipt writing, not a
// receipt reader (STR-V0-011).
func stampTestIDs(invocations []Invocation, goDir func(string) (string, bool)) {
	for index := range invocations {
		outcomes := invocations[index].Outcomes
		for at := range outcomes {
			outcomes[at].TestID = UnitTestID(outcomes[at].Unit, goDir)
		}
	}
}

// ReadTestID reads an outcome at the granularity it was recorded
// (CTI-V1-012): its testId when it carries one that parses, otherwise the
// ID its unit names, so a receipt written before testId existed joins on
// the same IDs and is never rewritten. It returns false when neither names
// a test ID; the reader then keeps the unit key.
func (o Outcome) ReadTestID(goDir func(string) (string, bool)) (testid.ID, bool) {
	if o.TestID != "" {
		id, err := testid.Parse(o.TestID)
		return id, err == nil
	}
	return testid.FromUnit(o.Unit, goDir)
}
