package main

import "testing"

// VSC-V0-080 (V1-0437): the VS Code extension workflow installs from the lockfile with install
// scripts off, then runs lint, typecheck and the unit tests, on every pull request that touches
// the extension or the CLI, MCP and live test-provider wire contracts it consumes.
func TestVSCV0080ExtensionWorkflowRunsOnContractChanges(t *testing.T) {
	const name = "vscode-extension.yml"
	w := readWorkflow(t, name)
	for _, path := range []string{
		"extensions/vscode/**",
		"docs/specs/vscode-extension-v0.md",
		"cmd/corvint/main.go",
		"cmd/corvint/query*.go",
		"cmd/corvint/impact*.go",
		"cmd/corvint-mcp/**",
		"internal/mcp/**",
		"cmd/corvint-go-test-provider/**",
		"internal/liveverify/**",
	} {
		wantLine(t, name, w, `      - "`+path+`"`)
	}
	job := jobBlock(t, w, "vscode-extension")
	wantLine(t, name, job, "        working-directory: extensions/vscode")
	for _, run := range []string{
		"npm ci --ignore-scripts --no-audit --no-fund",
		"npm run lint",
		"npm run typecheck",
		"npm test",
	} {
		wantLine(t, name, job, "        run: "+run)
	}
}
