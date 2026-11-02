// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tasksTree writes a minimal internal/tasks tree: LiveFormats names one
// literal and one constant of another package, and output lists the
// output-only profiles.
func tasksTree(t *testing.T, output string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"internal/tasks/snapshot/snapshot.go": "package snapshot\n\nconst ProfileHead = \"taskman-head/0\"\n",
		"internal/tasks/cli/cli.go": "package cli\n\nimport \"github.com/Beamfall/corvint/internal/tasks/snapshot\"\n\n" +
			"func LiveFormats() []string {\n\treturn []string{snapshot.ProfileHead, \"taskman-stage/0\", doctorCacheProfile}\n}\n",
		"internal/tasks/cli/doctor.go": "package cli\n\nconst doctorCacheProfile = \"taskman-doctor-cache/0\"\n\nvar report = \"taskman-doctor/0\"\n",
		// The 46e9ee1fd shape: a new profile constant no list registers.
		"internal/tasks/cli/doctor_closed_index.go": "package cli\n\nconst profileClosedIndexRebuild = \"taskman-closed-index-rebuild/0\"\n",
		"internal/tasks/cli/live_upgrade_test.go": "package cli_test\n\nvar outputOnlyProfiles = []string{" + output + "}\n\n" +
			"var adoptOnlyProfiles = []string{\"taskman-dispatch-state/1\"}\n",
		"internal/tasks/dispatch/state.go":      "package dispatch\n\nvar old = \"taskman-dispatch-state/1\"\n",
		"internal/tasks/dispatch/state_test.go": "package dispatch\n\nvar fixture = \"taskman-test-only/0\"\n",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TCL-V0-815: an unregistered output-only profile literal fails the check,
// naming the profile and file, and the same tree passes once it is listed.
func TestTCLV0815_UnregisteredProfileFailsUntilRegistered(t *testing.T) {
	problems, err := check(tasksTree(t, `"taskman-doctor/0"`))
	if err != nil {
		t.Fatal(err)
	}
	want := "taskman-closed-index-rebuild/0 (internal/tasks/cli/doctor_closed_index.go) must be in exactly one of"
	if len(problems) != 1 || !strings.HasPrefix(problems[0], want) {
		t.Fatalf("problems = %q, want one starting %q", problems, want)
	}
	problems, err = check(tasksTree(t, `"taskman-doctor/0", "taskman-closed-index-rebuild/0"`))
	if err != nil || len(problems) != 0 {
		t.Fatalf("registered tree: problems = %q, err = %v", problems, err)
	}
}

// TCL-V0-815: a profile in two lists also fails, as the runtime test does.
func TestTCLV0815_ProfileInTwoListsFails(t *testing.T) {
	problems, err := check(tasksTree(t, `"taskman-doctor/0", "taskman-closed-index-rebuild/0", "taskman-stage/0"`))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("taskman-stage/0 is named only inside LiveFormats, so no source names it twice: %q", problems)
	}
	root := tasksTree(t, `"taskman-doctor/0", "taskman-closed-index-rebuild/0", "taskman-head/0"`)
	problems, err = check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "taskman-head/0 (internal/tasks/snapshot/snapshot.go)") {
		t.Fatalf("problems = %q", problems)
	}
}

func TestRunExitCodes(t *testing.T) {
	var out, errs strings.Builder
	if code := run([]string{"--root", tasksTree(t, `"taskman-doctor/0"`)}, &out, &errs); code != 1 || !strings.Contains(errs.String(), "taskman-closed-index-rebuild/0") {
		t.Fatalf("code = %d, stderr = %q", code, errs.String())
	}
	if code := run([]string{"--root", t.TempDir()}, &out, &errs); code != 2 {
		t.Fatalf("missing tree: code = %d", code)
	}
}
