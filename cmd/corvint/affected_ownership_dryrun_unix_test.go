//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// AZU-V0-9900: exercise the public dry-run through manifest projection and argv,
// rather than relying only on the affected selection's package membership.
func TestOwnershipNamedDryRun_AZU_V0_9900(t *testing.T) {
	root, _ := ratchetSelectionRepo(t)
	affectedGit(t, root, "rm", "web/test.ts")
	cemWrite(t, root, sessiontest.DeclarationPath, sessionDeclaration(""))
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "Go-only dry-run fixture")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	cemWrite(t, root, "data/new.txt", "new\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "tracked addition")
	bin := t.TempDir()
	fake := strings.Replace(sessionFakeGo, "list) echo example.test/m/a; echo example.test/m/b; exit 0;;", "list) echo example.test/m/internal/liveverify/affected; echo example.test/m/tools/ci-reuse-plan; exit 0;;", 1)
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "go.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--format", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var output struct {
		Invocations []struct {
			Argv  []string           `json:"argv"`
			Units []sessiontest.Unit `json:"units"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	for _, inv := range output.Invocations {
		for _, unit := range inv.Units {
			if unit.Unit != "go:example.test/m/internal/liveverify/affected" {
				continue
			}
			if unit.Slice != sessiontest.SliceSelected || unit.Pattern != "^("+affected.OwnershipAuditTest+")$" || !strings.Contains(unit.Reason, "AZU-V0-9900") || !strings.Contains(strings.Join(inv.Argv, " "), affected.OwnershipAuditTest) {
				t.Fatalf("gate unit=%+v argv=%q", unit, inv.Argv)
			}
			return
		}
	}
	t.Fatalf("ratchet absent from dry-run: %s", stdout)
}

// AZU-V0-9900: deleting the last source of another package forces FULL.
// The ratchet still runs whole with its name and triggering path explained.
func TestOwnershipFullDryRun_AZU_V0_9900(t *testing.T) {
	root, _ := ratchetSelectionRepo(t)
	affectedGit(t, root, "rm", "web/test.ts")
	cemWrite(t, root, "gone/only.go", "package gone\n")
	// Keep the directory present: an entirely removed directory can be
	// attributed to its importers without a FULL fallback (AFP-V0-821).
	cemWrite(t, root, "gone/keep.txt", "package notes\n")
	cemWrite(t, root, sessiontest.DeclarationPath, sessionDeclaration(""))
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "Go-only FULL fixture")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "rm", "gone/only.go")
	affectedGit(t, root, "commit", "-qm", "delete last source")
	bin := t.TempDir()
	fake := strings.Replace(sessionFakeGo, "list) echo example.test/m/a; echo example.test/m/b; exit 0;;", "list) echo example.test/m/internal/liveverify/affected; echo example.test/m/tools/ci-reuse-plan; exit 0;;", 1)
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "go.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--format", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var output struct {
		Invocations []struct {
			Argv  []string           `json:"argv"`
			Units []sessiontest.Unit `json:"units"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "FULL") {
		t.Fatalf("no FULL fallback: %s", stderr)
	}
	for _, inv := range output.Invocations {
		for _, unit := range inv.Units {
			if unit.Unit != "go:example.test/m/internal/liveverify/affected" {
				continue
			}
			if unit.Slice != sessiontest.SliceWhole || unit.Pattern != "" || unit.Flag != "" || strings.Contains(strings.Join(inv.Argv, " "), "-run") || !strings.Contains(unit.Reason, affected.OwnershipAuditTest) || !strings.Contains(unit.Reason, "gone/only.go") || !strings.Contains(unit.Reason, "AZU-V0-9900") {
				t.Fatalf("FULL lost obligation or narrowed coverage: unit=%+v argv=%q", unit, inv.Argv)
			}
			return
		}
	}
	t.Fatalf("ratchet absent from FULL dry-run: %s", stdout)
}
