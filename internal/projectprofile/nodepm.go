package projectprofile

import (
	"encoding/json"
	"strings"
)

// nodeManagerFiles maps each root lockfile or workspace file to the Node package manager it
// signals (NGI-V0-780).
var nodeManagerFiles = map[string]string{
	"package-lock.json":   "npm",
	"npm-shrinkwrap.json": "npm",
	"pnpm-lock.yaml":      "pnpm",
	"pnpm-workspace.yaml": "pnpm",
	"yarn.lock":           "yarn",
	"bun.lock":            "bun",
	"bun.lockb":           "bun",
}

// NodePackageManager names a Node repository's package manager from its root package.json bytes
// and which root files it tracks (NGI-V0-780): the `packageManager` member's name when it is npm,
// pnpm, yarn or bun; else the one manager whose lockfile or workspace file is present; else npm,
// the Node default. known is false when present files name more than one manager and no
// `packageManager` member settles it, so no manager-specific command is certain.
func NodePackageManager(packageJSON []byte, has func(name string) bool) (manager string, known bool) {
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if json.Unmarshal(packageJSON, &manifest) == nil {
		name, _, _ := strings.Cut(strings.TrimSpace(manifest.PackageManager), "@")
		switch name {
		case "npm", "pnpm", "yarn", "bun":
			return name, true
		}
	}
	found := map[string]bool{}
	for file, name := range nodeManagerFiles {
		if has(file) {
			found[name] = true
		}
	}
	switch len(found) {
	case 0:
		return "npm", true
	case 1:
		for name := range found {
			return name, true
		}
	}
	return "", false
}

// NodeTestCommand is the command that runs a package's `test` script under manager: `bun test`
// would run Bun's own runner, not the script.
func NodeTestCommand(manager string) string {
	if manager == "bun" {
		return "bun run test"
	}
	return manager + " test"
}

// NodeExec is the argv prefix that runs a locally installed package binary under manager.
func NodeExec(manager string) []string {
	switch manager {
	case "pnpm":
		return []string{"pnpm", "exec"}
	case "yarn":
		return []string{"yarn"}
	case "bun":
		return []string{"bunx"}
	}
	return []string{"npx"}
}
