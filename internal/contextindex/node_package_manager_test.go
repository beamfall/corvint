package contextindex

import (
	"reflect"
	"testing"
)

// TestNGIV0780NodeTestCommandFollowsPackageManager: NGI-V0-780 (V1-3498). A web
// path's test command runs the root test script under the repository's package
// manager: the packageManager member, else the one signalled lockfile, else npm.
// A pnpm workspace whose AGENTS.md declares `pnpm check` never reads `npm test`,
// and conflicting lockfiles name no Node command.
func TestNGIV0780NodeTestCommandFollowsPackageManager(t *testing.T) {
	t.Parallel()
	pnpmPackage := "{\"name\":\"vt\",\"packageManager\":\"pnpm@11.28.5\",\"scripts\":{\"test\":\"vitest\",\"check\":\"tsc\"}}\n"
	for name, c := range map[string]struct {
		files map[string]string
		want  []any
	}{
		"pnpm workspace with declared gate": {map[string]string{
			"package.json": pnpmPackage, "pnpm-lock.yaml": "lockfileVersion: '9.0'\n", "pnpm-workspace.yaml": "packages:\n  - apps/*\n",
			"AGENTS.md": "# Agents\n\n## Verify\n\n```sh\npnpm check\n```\n",
		}, []any{"pnpm test", "pnpm check"}},
		"packageManager outranks lockfiles": {map[string]string{"package.json": pnpmPackage, "package-lock.json": "{}\n", "yarn.lock": "\n"}, []any{"pnpm test", "git diff --check"}},
		"pnpm lockfile":                     {map[string]string{"pnpm-lock.yaml": "lockfileVersion: '9.0'\n"}, []any{"pnpm test", "git diff --check"}},
		"yarn lockfile":                     {map[string]string{"yarn.lock": "\n"}, []any{"yarn test", "git diff --check"}},
		"bun lockfile":                      {map[string]string{"bun.lock": "{}\n"}, []any{"bun run test", "git diff --check"}},
		"npm lockfile":                      {map[string]string{"package-lock.json": "{}\n"}, []any{"npm test", "git diff --check"}},
		"conflicting lockfiles":             {map[string]string{"package-lock.json": "{}\n", "pnpm-lock.yaml": "lockfileVersion: '9.0'\n"}, []any{"git diff --check"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path, _ := declaredReceipts(t, npmRepository(c.files), "src/a.ts", "export const a = 2;\n")
			if got := anySlice(path["verification"]); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("verification = %v, want %v", got, c.want)
			}
		})
	}
}
