package main

import (
	"strings"
	"testing"
)

// VSC-V0-080 (V1-0437): the VS Code extension workflow installs from the lockfile with install
// scripts off, then runs lint, typecheck and the unit tests, on every pull request that reaches
// the extension or the CLI, MCP and live test-provider binaries it drives. V1-5200: the select
// job decides that from the Go dependency closure of those binaries (.github/consumers/gate.py)
// instead of a hand-written path filter, the suite runs unless that job succeeded and printed
// SKIP, and the same job runs the interactive-alpha process harness tests.
func TestVSCV0080ExtensionWorkflowRunsOnContractChanges(t *testing.T) {
	const name = "vscode-extension.yml"
	w := readWorkflow(t, name)
	if strings.Contains(w, "\n    paths:\n") {
		t.Errorf("%s keeps a path filter; the select job decides instead (V1-5200)", name)
	}
	wantLine(t, name, w, "  pull_request:")
	gate := stepScript(t, jobBlock(t, w, "select"), "gate")
	for _, arg := range []string{
		"python3 -I .github/consumers/gate.py",
		"--path .github/workflows/vscode-extension.yml",
		"--path .github/consumers/gate.py",
		"--path extensions/vscode",
		"--path conformance/vscode-extension-v0",
		"--path conformance/interactive-alpha",
		"--path Makefile",
		"--path docs/specs/vscode-extension-v0.md",
		"--path internal/mcp",
		"--path internal/liveverify",
		// cmd/corvint/main.go and cmd/corvint/impact*.go lie in the closure of ./cmd/corvint.
		"--go ./cmd/corvint ",
		"--go ./cmd/corvint-mcp ",
		"--go ./cmd/corvint-go-test-provider)",
		"SKIP:*) echo 'run=SKIP' >> \"$GITHUB_OUTPUT\" ;;",
		"*) echo 'run=RUN' >> \"$GITHUB_OUTPUT\" ;;",
	} {
		if strings.Count(gate, arg) != 1 {
			t.Errorf("%s select step does not pass %q once", name, arg)
		}
	}
	job := jobBlock(t, w, "vscode-extension")
	wantLine(t, name, job, "    needs: select")
	wantLine(t, name, job, "    if: ${{ always() && !cancelled() && (needs.select.result != 'success' || needs.select.outputs.run != 'SKIP') }}")
	wantLine(t, name, job, "        working-directory: extensions/vscode")
	for _, run := range []string{
		"npm ci --ignore-scripts --no-audit --no-fund",
		"npm run lint",
		"npm run typecheck",
		"npm test",
		"make interactive-alpha-test",
	} {
		wantLine(t, name, job, "        run: "+run)
	}
}
