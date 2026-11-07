// SPDX-License-Identifier: Apache-2.0
// Package cliapp serves offline process entrypoint contracts shared by shipped binaries.
package cliapp

import (
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/companioncli"
)

// Answer serves migrated companion help before any protocol, repository or server starts.
// Existing command-tree producers continue to use their authored help implementations.
func Answer(binary string, args []string, stdout, stderr io.Writer) (int, bool) {
	// Preserve authored test-command help, including provider flag details.
	// These binaries can still expose their offline command-tree catalog.
	if binary == "corvint-test" {
		return 0, false
	}
	switch binary {
	case "corvint-test-runner", "corvint-behavior-falsify", "corvint-playwright-minimize", "corvint-tests-accept", "corvint-js-test-provider":
		catalog := len(args) == 2 && args[0] == "help" && (args[1] == "--json" || args[1] == "--tree")
		version := len(args) > 0 && args[0] == "--version"
		if !catalog && !version {
			return 0, false
		}
		if catalog && args[1] == "--tree" {
			args = []string{"help", "--json"}
		}
	}
	// Analyzer versions are protocol receipts or native refusal frames, with
	// exact bytes and argv rules owned by each analyzer.
	if strings.HasPrefix(binary, "corvint-analyzer-") && len(args) > 0 && args[0] == "--version" {
		return 0, false
	}
	if len(args) > 0 && args[0] == "--version" && binary != "corvint" && binary != "corvint-tasks" {
		return companioncli.Answer(companioncli.Spec{Binary: binary, Version: companioncli.ExperimentalVersion}, args, stdout)
	}
	tree, ok := capcatalog.TreeSnapshot(binary)
	if !ok {
		return 0, false
	}
	if tree.Source != "companion root contract" {
		// CAP-V0-9300 / ACX-V0-9301: only a complete, exact node help
		// invocation is ours. Preambles and all other layouts must reach
		// authored dispatch with untouched streams.
		if binary != "corvint-tasks" && len(args) > 1 && (args[len(args)-1] == "--help" || args[len(args)-1] == "-h") {
			words := args[:len(args)-1]
			if node, found := tree.Find(words); found && len(node.Path) == len(words) {
				if _, err := io.WriteString(stdout, clitree.Render(tree, node)); err != nil {
					return 1, true
				}
				return 0, true
			}
		}
		return 0, false
	}
	var root strings.Builder
	for _, node := range tree.Nodes {
		if len(node.Path) == 0 {
			root.WriteString(clitree.Render(tree, node))
		}
	}
	if strings.HasPrefix(binary, "corvint-analyzer-") {
		// ACP-011 / ACX-V0-7707: only supported help belongs here. All
		// other argv must reach the analyzer's native protocol dispatcher.
		var answer strings.Builder
		handled, err := clitree.Answer(tree, args, root.String(), &answer)
		if !handled || err != nil {
			return 0, false
		}
		if _, err := io.WriteString(stdout, answer.String()); err != nil {
			return 1, true
		}
		return 0, true
	}
	// CAP-V0-9300 / ACX-V0-9301: discovery is additive. A new help
	// interceptor must never replace a shipped native refusal on other argv.
	var answer strings.Builder
	handled, err := clitree.Answer(tree, args, root.String(), &answer)
	if !handled || err != nil {
		return 0, false
	}
	if _, err := io.WriteString(stdout, answer.String()); err != nil {
		return 1, true
	}
	return 0, true
}
