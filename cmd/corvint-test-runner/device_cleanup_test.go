package main

import (
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// TestPreCleanupDevicePlanRefusedForExecution binds TRE-V0-087(6): a
// device-bound plan without its cleanup steps is refused, never run without
// cleanup; the current plan runs. A historical Android plan differs only by
// the steps and gets the re-plan diagnosis. A historical Appium plan also
// lacks the session hook in its wrapper, so it gets the generic refusal.
func TestPreCleanupDevicePlanRefusedForExecution(t *testing.T) {
	sum := strings.Repeat("a", 64)
	for _, r := range []tr.Request{
		{Runner: "android-junit", Root: "/source", Executable: "/sdk/platform-tools/adb", ExecutableSha256: sum, Target: "emulator-5554", Project: "local.proof.test/androidx.test.runner.AndroidJUnitRunner", Selectors: []string{"local.proof.Case#test"}, InputFiles: map[string]string{"src/Case.kt": sum, "app-debug-androidTest.apk": sum}, ReportDir: "/fresh", TimeoutSeconds: 60},
		{Runner: "appium-uiautomator2-wdio", Root: "/source", Executable: "/tools/wdio", ExecutableSha256: sum, Config: "config.mjs", ConfigSha256: sum, Reporter: "/tools/reporter.js", ReporterSha256: sum, ReportDir: "/fresh", Project: "http://127.0.0.1:4723/", Target: "emulator-5580", Selectors: []string{"test.cjs"}, InputFiles: map[string]string{"test.cjs": sum}, TimeoutSeconds: 60},
	} {
		current, e := registry.Build(r)
		if e != nil || len(current.DeviceCleanup) == 0 {
			t.Fatalf("%s: %+v %v", r.Runner, current, e)
		}
		if _, e := admittedInvocation(plan{Request: r, Invocation: current}); e != nil {
			t.Fatalf("%s: current plan refused: %v", r.Runner, e)
		}
		historical := current
		historical.DeviceCleanup = nil
		if _, e := admittedInvocation(plan{Request: r, Invocation: historical}); e == nil || e.Error() != "plan predates device cleanup; re-plan it" {
			t.Fatalf("%s: plan without cleanup lacks the re-plan diagnosis: %v", r.Runner, e)
		}
		if wrapper, ok := current.Files["appium.config.mjs"]; ok {
			old := historical
			old.Files = map[string][]byte{"appium.config.mjs": []byte(strings.Replace(string(wrapper), "before:", "unhooked:", 1))}
			if _, e := admittedInvocation(plan{Request: r, Invocation: old}); e == nil || e.Error() != "plan does not match current fixed runner profile" {
				t.Fatalf("historical Appium wrapper: %v", e)
			}
		}
		drifted := current
		drifted.DeviceCleanup = append([]tr.CleanupStep{}, current.DeviceCleanup...)
		drifted.DeviceCleanup[0].Name = "other"
		if _, e := admittedInvocation(plan{Request: r, Invocation: drifted}); e == nil || e.Error() != "plan does not match current fixed runner profile" {
			t.Fatalf("%s: drifted cleanup admitted: %v", r.Runner, e)
		}
	}
}
