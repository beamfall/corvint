// SPDX-License-Identifier: Apache-2.0
package webdialect

import (
	"github.com/Beamfall/corvint/internal/browsercov"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// E2EInventory is the AFU e2e-safe discovery entry point. It delegates to the
// runner piece, so titles, projects, source paths, caps and refusals cannot
// diverge into a second Playwright inventory model (RSP-V0-052).
func E2EInventory(d jslinecov.Discovery, report string) (testrunner.RunnerInventory, error) {
	return browsercov.PlaywrightInventory(d, report)
}
