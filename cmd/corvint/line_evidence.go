// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/lineevidence"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

const lineEvidenceHelp = `Collect private, tree-bound line evidence (experimental, LEV-V1).

Usage:
  corvint [--root PATH] line-evidence refresh [--runner RUNNER] [--root DIR]

refresh runs the tests of each matching declared Go, Vitest, Jest, RSpec or
Minitest instance. --root after refresh is a repository-relative instance
root; --runner limits collection to that runner. A clean committed tree is
required. The result is stored per instance under the Git directory at
corvint/line-evidence/<runner>@<root-digest>.linecov (linecov-index/4), with
canonical test IDs. A failed collection keeps the previous instance file.

Only refresh collects or writes evidence. affected, test, watch and MCP read
it. Missing, stale, damaged or unqualified evidence widens with a named
reason; evidence never removes tests from the file-level plan. All runners
remain unqualified until their FIH records are admitted. pytest selects
whole files in 1.0; Playwright line evidence only adds or ranks tests.

Output: a JSON array of refreshed instance keys and private file paths.
Exit status: 0 on success, 2 on invalid arguments or failed collection.
Tests and their hooks execute project code and may write their own outputs.

Example:
  corvint line-evidence refresh --runner go --root .
`

func lineEvidenceInvocation(args []string) (string, []string, bool) {
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
	if len(args) == 0 || args[0] != "line-evidence" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runLineEvidence(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	refuse := func(err error) int {
		return inventoryRefuse(diagnostic, "line-evidence-invalid-arguments", err.Error())
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") || len(args) == 2 && args[0] == "refresh" && (args[1] == "--help" || args[1] == "-h") {
		_, _ = io.WriteString(out, lineEvidenceHelp)
		return 0
	}
	if len(args) == 0 || args[0] != "refresh" {
		return refuse(fmt.Errorf("usage: corvint line-evidence refresh [--runner RUNNER] [--root DIR]"))
	}
	f := flag.NewFlagSet("line-evidence refresh", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	runner := f.String("runner", "", "")
	instanceRoot := f.String("root", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return refuse(err)
	}
	if f.NArg() != 0 || *runner != "" && !testid.KnownRunner(*runner) || *instanceRoot != "" && !testid.CleanFile(*instanceRoot) {
		return refuse(fmt.Errorf("invalid runner, instance root or positional argument"))
	}
	if root == "" {
		root = "."
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		return refuse(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return refuse(err)
	}
	result, err := lineevidence.Refresh(ctx, lineevidence.Options{Git: testinventory.Git{Executable: git, Root: resolved}, Runner: *runner, Root: *instanceRoot})
	if err != nil {
		return inventoryRefuse(diagnostic, "line-evidence-unavailable", err.Error())
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return 2
	}
	return 0
}

const e2eLineExperimental = "e2e-line-experimental"
