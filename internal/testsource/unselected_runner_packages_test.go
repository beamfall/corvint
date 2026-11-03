package testsource

import (
	"maps"
	"testing"
)

// TestRunnerPackagesFromPlugin_LGPV0017 pins the npm runner tables derived
// from the TypeScript plugin to the sets this reading used before.
func TestRunnerPackagesFromPlugin_LGPV0017(t *testing.T) {
	wantModelled := map[string]string{"jest": RunnerJest, "vitest": RunnerVitest, "@playwright/test": RunnerPlaywright, "mocha": RunnerMocha}
	wantUnmodelled := map[string]bool{"cypress": true, "karma": true, "jasmine": true, "ava": true, "@web/test-runner": true,
		"@wdio/cli": true, "tap": true, "uvu": true, "@japa/runner": true, "@storybook/test-runner": true, "testcafe": true}
	if !maps.Equal(runnerPackages, wantModelled) {
		t.Errorf("runnerPackages = %v, want %v", runnerPackages, wantModelled)
	}
	if !maps.Equal(unmodelledRunnerPackages, wantUnmodelled) {
		t.Errorf("unmodelledRunnerPackages = %v, want %v", unmodelledRunnerPackages, wantUnmodelled)
	}
}
