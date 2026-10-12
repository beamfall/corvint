package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/adapters"
)

// TestCEMV1168DashboardRefusesStableBundle (CEM-V1-168): the hash-pinned
// adapter registry has no cem/1.0 profile, so a stable bundle profile is a
// DASHBOARD_INVALID_ARGUMENT before any path opens, with no stdout, and the
// registered bundle adapter still accepts only the 0.x profiles.
func TestCEMV1168DashboardRefusesStableBundle(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{"cem.json": `{"spec":"cem/1.0"}`, "ocm.json": `{"spec":"ocm/0.1"}`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	invalid := `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}` + "\n"
	for _, profile := range []string{"cem/1.0+ocm/0.1", "cem/1.0+ocm/1.0", "cem/1.0"} {
		arguments := []string{"snapshot", "--root", root, "--cem", "cem.json", "--ocm", "ocm.json",
			"--cem-ocm-profile", profile, "--expected-base", testBase, "--target", testTarget}
		if _, err := parseArguments(arguments); err == nil {
			t.Fatalf("%s: parseArguments admitted a stable bundle profile", profile)
		}
		stdout, stderr := runCLIHelper(t, arguments, nil)
		if len(stdout) != 0 || string(stderr) != invalid {
			t.Fatalf("%s: stdout=%q stderr=%q", profile, stdout, stderr)
		}
	}
	descriptor, ok := adapters.Lookup(adapters.AdapterCEMOCMBundle)
	if !ok || strings.Contains(strings.Join(descriptor.AcceptedProfiles, ","), "cem/1.0") {
		t.Fatalf("bundle adapter profiles = %v", descriptor.AcceptedProfiles)
	}
}
