// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

const inventoryHelp = `Refresh the committed test inventory (experimental, TIV-V1).

Usage:
  corvint [--root PATH] inventory refresh [--runner RUNNER] [--root DIR]

refresh lists every test of the seven blocking runners (go, vitest, jest,
playwright, pytest, rspec, minitest) by canonical test ID into
.corvint/test-inventory.json (corvint-test-inventory/0), one record per
runner instance: runner, root directory and config file, keyed
<runner>@<root>. An instance is declared by its config file (go.mod,
vitest.config.*, jest.config.*, playwright.config.*, pytest.ini or a
pyproject.toml, tox.ini or setup.cfg pytest section, .rspec or
spec/spec_helper.rb, test/test_helper.rb) outside node_modules, vendor,
testdata and hidden directories. Each record binds HEAD, the config and
lockfile blobs, the lister name, version and kind, and every test file's
blob (TIV-V1-005).

Listers: go, rspec and minitest are static (read source only); vitest
(vitest list --json), jest (jest --listTests), playwright (playwright test
--list --reporter=json, through playwright-discovery/0) and pytest
(--collect-only -q) are executing: they run project code from the
project's own node_modules/.bin or .venv, and only this command runs them
(TIV-V1-004). A lister that fails leaves its instance out with the failure
recorded, so every reader runs that instance in full with
inventory-absent (TIV-V1-011).

--runner RUNNER and --root DIR (repository-relative, after the verb)
refresh only the matching instances and keep the other records of a valid
current inventory.

Refusals (exit 2, nothing written, the previous file kept):
inventory-refresh-drift when HEAD or a source moved while a lister ran
(TIV-V1-010); inventory-too-large when the file would exceed 8 MiB
(TIV-V1-011); inventory-invalid-arguments; inventory-unavailable when Git
cannot read the repository.

Readers: corvint affected --summary, corvint test, corvint watch and the
corvint.affected MCP tool only read the file and print its reasons
(inventory-absent, inventory-stale, unassigned, inventory-invalid); they
never run a lister or write it (TIV-V1-002).

Output: one JSON object on stdout: path, commit, bytes, changed,
instances (key, runner, root, kind, tests, files) and failures (key,
error).
Writes: .corvint/test-inventory.json only, replaced through a temporary
file in the same directory; listers write what they write.

Example:
  corvint inventory refresh --runner go
`

// inventoryInvocation recognises `[--root PATH] inventory ARGS...`.
func inventoryInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "inventory" {
		return "", nil, false
	}
	return root, args[1:], true
}

func inventoryRefuse(diagnostic io.Writer, code, message string) int {
	_ = json.NewEncoder(diagnostic).Encode(map[string]any{"code": code, "error": message, "ok": false})
	return 2
}

// inventoryRefreshExec runs executing listers; tests replace it.
var inventoryRefreshExec testinventory.Exec = testinventory.RunLister

// runInventory is `corvint inventory refresh` (TIV-V1-002): the only writer
// of the committed test inventory.
func runInventory(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	const usage = "usage: corvint [--root PATH] inventory refresh [--runner RUNNER] [--root DIR]"
	if len(args) == 0 {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", "a subcommand is required; "+usage)
	}
	if args[0] == "--help" || args[0] == "-h" || (len(args) == 2 && args[0] == "refresh" && (args[1] == "--help" || args[1] == "-h")) {
		_, _ = io.WriteString(out, inventoryHelp)
		return 0
	}
	if args[0] != "refresh" {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", fmt.Sprintf("unknown subcommand %q; %s", args[0], usage))
	}
	f := flag.NewFlagSet("inventory refresh", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	runner := f.String("runner", "", "")
	instanceRoot := f.String("root", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", err.Error()+"; "+usage)
	}
	if f.NArg() != 0 {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", fmt.Sprintf("unexpected argument %q; %s", f.Arg(0), usage))
	}
	if *runner != "" && !testid.KnownRunner(*runner) {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", fmt.Sprintf("--runner %q is not one of %s", *runner, strings.Join(testid.Runners, ", ")))
	}
	if *instanceRoot != "" && !testid.CleanFile(*instanceRoot) {
		return inventoryRefuse(diagnostic, "inventory-invalid-arguments", fmt.Sprintf("--root %q is not a clean repository-relative directory", *instanceRoot))
	}
	if root == "" {
		root = "."
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		emitError(diagnostic, err)
		return 2
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return inventoryRefuse(diagnostic, "inventory-unavailable", "git is not on PATH")
	}
	result, err := testinventory.Refresh(ctx, testinventory.Options{
		Git:    testinventory.Git{Executable: gitExecutable, Root: resolved},
		Exec:   inventoryRefreshExec,
		Runner: *runner, Root: *instanceRoot,
	})
	var refusal *testinventory.Refusal
	if errors.As(err, &refusal) {
		return inventoryRefuse(diagnostic, refusal.Code, refusal.Message)
	}
	if err != nil {
		return inventoryRefuse(diagnostic, "inventory-unavailable", err.Error())
	}
	type instanceSummary struct {
		Key    string `json:"key"`
		Runner string `json:"runner"`
		Root   string `json:"root"`
		Kind   string `json:"kind"`
		Tests  int    `json:"tests"`
		Files  int    `json:"files"`
	}
	type failureSummary struct {
		Key   string `json:"key"`
		Error string `json:"error"`
	}
	summary := struct {
		OK        bool              `json:"ok"`
		Path      string            `json:"path"`
		Profile   string            `json:"profile"`
		Commit    string            `json:"commit,omitempty"`
		Bytes     int               `json:"bytes"`
		Changed   bool              `json:"changed"`
		Instances []instanceSummary `json:"instances"`
		Failures  []failureSummary  `json:"failures"`
	}{OK: true, Path: testinventory.Path, Profile: testinventory.Profile, Bytes: result.Bytes, Changed: result.Changed,
		Commit: result.Commit, Instances: []instanceSummary{}, Failures: []failureSummary{}}
	for _, instance := range result.Inventory.Instances {
		summary.Instances = append(summary.Instances, instanceSummary{Key: instance.Key, Runner: instance.Runner, Root: instance.Root,
			Kind: instance.Lister.Kind, Tests: len(instance.Tests), Files: len(instance.Files)})
	}
	for _, failure := range result.Inventory.Failures {
		summary.Failures = append(summary.Failures, failureSummary{Key: failure.Key, Error: failure.Error})
	}
	if err := emit(out, summary); err != nil {
		return 2
	}
	return 0
}
