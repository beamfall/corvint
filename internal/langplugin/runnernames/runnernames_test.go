// SPDX-License-Identifier: Apache-2.0
package runnernames_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
)

// TestRunnerNameConstantsAreManifestNames keeps the exported names core
// selection plugins compare against equal to a declared manifest or runner
// name (LGP-V0-009): a stale or mistyped constant fails here.
func TestRunnerNameConstantsAreManifestNames(t *testing.T) {
	declared := map[string]bool{}
	for _, n := range langplugin.Names() {
		declared[n] = true
	}
	for _, n := range []string{
		runnernames.RunnerBats, runnernames.RunnerCargoTest,
		runnernames.RunnerDotnetMtpMstest, runnernames.RunnerDotnetMtpNunit, runnernames.RunnerDotnetMtpXunit,
		runnernames.RunnerDotnetVstestMstest, runnernames.RunnerDotnetVstestNunit, runnernames.RunnerDotnetVstestXunit,
		runnernames.RunnerGradleJunit, runnernames.LanguageJava, runnernames.RunnerJest,
		runnernames.RunnerJunitPlatform, runnernames.RunnerMavenSurefire, runnernames.RunnerMinitest,
		runnernames.RunnerNextest, runnernames.RunnerNodeTest, runnernames.RunnerPlaywright,
		runnernames.RunnerPytest, runnernames.RunnerRailsTest, runnernames.RunnerRspec,
		runnernames.RunnerShellspec, runnernames.RunnerTestUnit, runnernames.RunnerUnittest,
		runnernames.RunnerVitest,
	} {
		if !declared[n] {
			t.Errorf("%q is not a declared plugin or runner name", n)
		}
	}
}
