package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/projectprofile"
)

// playwrightPackageManagerIdentity is the source-digest entry that binds a non-npm manager
// (TJAA-V0-780).
const playwrightPackageManagerIdentity = "corvint:node-package-manager"

// playwrightPackageManager names the Node package manager that launches Playwright: the
// NGI-V0-780 rule over the root package.json and the root lockfiles and workspace files in the
// worktree. Files that name more than one manager with no packageManager member keep npm's npx.
func playwrightPackageManager(root string) string {
	raw, err := affected.ReadSource(root, "package.json")
	if err != nil {
		raw = nil
	}
	manager, known := projectprofile.NodePackageManager(raw, func(name string) bool {
		_, err := os.Lstat(filepath.Join(root, name))
		return err == nil
	})
	if !known {
		return "npm"
	}
	return manager
}

// playwrightPackageManagerSum is the digest identity of a non-npm manager, or "" for npm, whose
// plans keep their bytes.
func playwrightPackageManagerSum(root string) string {
	manager := playwrightPackageManager(root)
	if manager == "npm" {
		return ""
	}
	sum := sha256.Sum256([]byte(manager))
	return hex.EncodeToString(sum[:])
}

// applyPlaywrightPackageManager rewrites the `npx` launcher of every argv the plan emits to the
// repository's package manager (TJAA-V0-780).
func applyPlaywrightPackageManager(plan *PlaywrightPlan, root string) {
	manager := playwrightPackageManager(root)
	if manager == "npm" {
		return
	}
	exec := projectprofile.NodeExec(manager)
	launch := func(argv []string) []string {
		if len(argv) == 0 || argv[0] != "npx" {
			return argv
		}
		return append(append([]string{}, exec...), argv[1:]...)
	}
	plan.FallbackArgv = launch(plan.FallbackArgv)
	for i := range plan.Selected {
		plan.Selected[i].Argv = launch(plan.Selected[i].Argv)
	}
	for i := range plan.Projects {
		plan.Projects[i].Argv = launch(plan.Projects[i].Argv)
	}
	if plan.Narrowed != nil {
		for i := range plan.Narrowed.Candidates {
			plan.Narrowed.Candidates[i].Argv = launch(plan.Narrowed.Candidates[i].Argv)
		}
		for i := range plan.Narrowed.Projects {
			plan.Narrowed.Projects[i].Argv = launch(plan.Narrowed.Projects[i].Argv)
		}
	}
}
