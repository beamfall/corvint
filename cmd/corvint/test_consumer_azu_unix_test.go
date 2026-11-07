// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// AZU-V0-007: both consumer argv survive affected -> session planning ->
// execution, with root-relative paths and literal arguments. CI retains them
// as required checks, since that profile cannot run declared commands.
func TestAZUV0007CLIExecutesDeclaredConsumers(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	runs := map[string][]string{
		"web/e2e.mjs": {"bash", "web/e2e.mjs", "arg with space", "literal;$value"},
		"web/nav.mjs": {"bash", "web/nav.mjs", "navigation"},
	}
	consumers := map[string]any{}
	for file, argv := range runs {
		consumers[file] = map[string]any{"commands": []string{"cmd/tool"}, "run": argv}
	}
	declaration, err := json.Marshal(map[string]any{"profile": "corvint-test-binary-consumers/0", "consumers": consumers})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"cmd/tool/main.go":           "package main\nimport \"example.test/m/a\"\nfunc main() { _ = a.A() }\n",
		"web/e2e.mjs":                "printf '%s\\n' \"$PWD\" \"$@\" >> \"$SESSION_FAKE_LOG.e2e\"\n",
		"web/nav.mjs":                "printf '%s\\n' \"$PWD\" \"$@\" >> \"$SESSION_FAKE_LOG.nav\"\n",
		affected.BinaryConsumersPath: string(declaration),
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Keep Go execution a recording fixture while running the declared bash
	// commands themselves. Discovery must also confirm the testless command.
	bin := t.TempDir()
	goScript := strings.Replace(sessionFakeGo, "list) echo example.test/m/a;", "list) echo example.test/m/cmd/tool; echo example.test/m/a;", 1)
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(goScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare runnable consumers")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\nfunc A() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change command dependency")

	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 {
		t.Fatalf("run code %d stderr %s", code, stderr)
	}
	receipt := readSessionReceipt(t, filepath.Join(root, ".git", "corvint", "test-run.json"))
	if receipt.Result != sessiontest.ResultPass {
		t.Fatalf("receipt %+v", receipt)
	}
	found := 0
	for _, invocation := range receipt.Invocations {
		if invocation.Adapter != sessiontest.AdapterDeclared {
			continue
		}
		found++
		argv := runs[invocation.Argv[1]]
		if !slices.Equal(invocation.Argv, argv) || invocation.Exit != 0 || len(invocation.Outcomes) != 1 || invocation.Outcomes[0].Outcome != sessiontest.OutcomePass {
			t.Fatalf("declared invocation %+v", invocation)
		}
	}
	if found != len(runs) {
		t.Fatalf("executed %d declared consumers, want %d", found, len(runs))
	}
	for suffix, file := range map[string]string{"e2e": "web/e2e.mjs", "nav": "web/nav.mjs"} {
		data := mustRead(t, log+"."+suffix)
		if want := root + "\n" + strings.Join(runs[file][2:], "\n") + "\n"; string(data) != want {
			t.Fatalf("consumer output %q, want %q", data, want)
		}
	}
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--profile", "ci", "--confine", "/unused/confine", "--dry-run")
	if code != 0 {
		t.Fatalf("CI dry run code %d stderr %s", code, stderr)
	}
	var manifest sessiontest.Manifest
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatal(err)
	}
	for _, argv := range runs {
		if !slices.Contains(manifest.Checks, strings.Join(argv, " ")) {
			t.Fatalf("CI lost required consumer command: %+v", manifest)
		}
	}
	for _, unit := range manifest.Units {
		if unit.Adapter == sessiontest.AdapterDeclared {
			t.Fatalf("CI planned an unsupported command: %+v", unit)
		}
	}
}

// AZU-V0-007 / STR-V0-750: a complete CI manifest for a runnable browser
// consumer must still require the browser job under the existing workflow.
func TestAZUV0007CIRunnableBrowserConsumerRequired(t *testing.T) {
	root, _, _ := sessionFixture(t, "pass")
	const consumer = "tools/web-flows/test/e2e.mjs"
	for name, body := range map[string]string{
		"cmd/corvint-web-flows/main.go": "package main\nimport \"example.test/m/a\"\nfunc main() { _ = a.A() }\n",
		consumer:                        "// browser consumer\n",
		affected.BinaryConsumersPath:    `{"profile":"corvint-test-binary-consumers/0","consumers":{"tools/web-flows/test/e2e.mjs":{"commands":["cmd/corvint-web-flows"],"run":["bash","script/web-flows-gate"]}}}` + "\n",
	} {
		cemWrite(t, root, name, body)
	}
	bin := t.TempDir()
	goScript := strings.Replace(sessionFakeGo, "list) echo example.test/m/a;", "list) echo example.test/m/cmd/corvint-web-flows; echo example.test/m/a;", 1)
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(goScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare browser consumer")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	cemWrite(t, root, "a/a.go", "package a\nfunc A() int { return 4 }\n")
	affectedGit(t, root, "commit", "-qam", "change browser command dependency")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--profile", "ci", "--confine", "/unused/confine", "--dry-run")
	if code != 0 {
		t.Fatalf("complete CI dry run code %d stderr %s", code, stderr)
	}
	var manifest sessiontest.Manifest
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Scope != sessiontest.ScopeNarrowed || len(manifest.Incomplete) != 0 || !slices.Contains(manifest.Checks, "bash script/web-flows-gate") || !slices.Contains(manifest.Checks, sessiontest.BinaryConsumerCheck(consumer)) {
		t.Fatalf("CI lost a required runnable browser check: %+v", manifest)
	}
	// This is the existing jq predicate's check branch; FULL and exit 3 must
	// not mask its result. Pin the predicate to the workflow's actual bytes.
	workflow := string(mustRead(t, filepath.Join("..", "..", ".github", "workflows", "ci.yml")))
	if !strings.Contains(workflow, `startswith("tools/web-flows/test/") and contains("UNSELECTED_BINARY_CONSUMER")`) {
		t.Fatal("workflow consumer predicate changed; update the regression with it")
	}
	required := false
	for _, check := range manifest.Checks {
		required = required || strings.HasPrefix(check, "tools/web-flows/test/") && strings.Contains(check, "UNSELECTED_BINARY_CONSUMER")
	}
	if !required {
		t.Fatalf("workflow would skip the required browser job: %s", stdout)
	}
}
