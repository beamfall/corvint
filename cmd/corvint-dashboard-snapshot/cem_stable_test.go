package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/adapters"
)

// TestCEMV1168DashboardRefusesStableBundle (CEM-V1-168, CEM-V1-217): the
// hash-pinned adapter registry has no CEM profile. The never-delivered
// cem-ocm-bundle-v0 adapter and its five bundle flags are retired with the
// cem/0.x wire, so any bundle flag, for any profile, is a
// DASHBOARD_INVALID_ARGUMENT before any path opens, with no stdout.
func TestCEMV1168DashboardRefusesStableBundle(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{"cem.json": `{"spec":"cem/1.0"}`, "ocm.json": `{"spec":"ocm/0.1"}`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	invalid := `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}` + "\n"
	for _, profile := range []string{"cem/1.0+ocm/0.1", "cem/1.0", "cem/0.2+ocm/0.1", "cem/0.1+ocm/0.1"} {
		arguments := []string{"snapshot", "--root", root, "--cem", "cem.json", "--ocm", "ocm.json",
			"--cem-ocm-profile", profile, "--expected-base", testBase, "--target", testTarget}
		if _, err := parseArguments(arguments); err == nil {
			t.Fatalf("%s: parseArguments admitted a bundle profile", profile)
		}
		stdout, stderr := runCLIHelper(t, arguments, nil)
		if len(stdout) != 0 || string(stderr) != invalid {
			t.Fatalf("%s: stdout=%q stderr=%q", profile, stdout, stderr)
		}
	}
	for _, descriptor := range adapters.Registry() {
		if descriptor.AdapterID == "cem-ocm-bundle-v0" || strings.Contains(strings.Join(descriptor.AcceptedProfiles, ","), "cem/") {
			t.Fatalf("registry still declares a CEM bundle: %+v", descriptor)
		}
	}
}
