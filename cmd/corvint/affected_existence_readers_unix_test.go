//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
)

// AFP-V0-980 (V1-3772): deleting a file a docs/specs/INDEX.json-style manifest
// names selects the declared existence reader in `corvint affected`, in the
// gate selector the CI driver runs on that receipt, and in the shadow
// `corvint test --dry-run` plan. Before the change all three omitted it.
func TestDeletedManifestNameSelectsExistenceReaderCLI_AFPV0980(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	for name, body := range map[string]string{
		"index/index.go":                  "package index\n",
		"index/index_test.go":             "package index\n\nimport \"testing\"\n\nfunc TestPaths(t *testing.T) {}\n",
		"specs/INDEX.json":                `[{"implementation":["skills/x/SKILL.md"]}]`,
		"skills/x/SKILL.md":               "skill\n",
		".corvint/existence-readers.json": `{"profile":"corvint-existence-readers/0","entries":[{"package":"index","manifests":["specs/INDEX.json"],"reason":"stats implementation paths"}]}`,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add index")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "rm", "-q", "skills/x/SKILL.md")
	affectedGit(t, root, "commit", "-qm", "delete skill")

	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--format", "json")
	if code != 0 {
		t.Fatalf("affected code %d stderr %q", code, stderr)
	}
	var receipt struct {
		Plan struct {
			Selected []struct {
				UnitID  string `json:"unitId"`
				Witness struct {
					Kind string `json:"kind"`
				} `json:"witness"`
			} `json:"selected"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(receipt.Plan.Selected, func(s struct {
		UnitID  string `json:"unitId"`
		Witness struct {
			Kind string `json:"kind"`
		} `json:"witness"`
	}) bool {
		return s.UnitID == "go:example.test/m/index" && s.Witness.Kind == "DECLARED_EXISTENCE_READER"
	}) {
		t.Fatalf("affected omitted the existence reader: %s", stdout)
	}

	var plan gateselect.Receipt
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatal(err)
	}
	if lines := gateselect.Select(plan, "example.test/m", root); !slices.Contains(lines, "selected example.test/m/index") {
		t.Fatalf("gate selector omitted the existence reader:\n%s", strings.Join(lines, "\n"))
	}

	code, stdout, stderr = runCLI(t, "--root", root, "test", "--base", base, "--dry-run")
	if code != 0 || !strings.Contains(stdout, `"go:example.test/m/index"`) || !strings.Contains(stdout, `"scope":"NARROWED"`) {
		t.Fatalf("shadow dry run code %d omitted the existence reader: stdout %s stderr %q", code, stdout, stderr)
	}
	if got := sessionRunnerLog(t, log); strings.Contains(got, "go test") {
		t.Fatalf("a dry run launched tests:\n%s", got)
	}
}
