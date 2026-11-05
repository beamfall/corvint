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
	// Preserve the native help of the folded test command and its compatibility
	// shims. The shims can still expose their offline command-tree catalog.
	if binary == "corvint-test" {
		return 0, false
	}
	switch binary {
	case "corvint-test-runner", "corvint-behavior-falsify", "corvint-playwright-minimize", "corvint-tests-accept":
		catalog := len(args) == 2 && args[0] == "help" && (args[1] == "--json" || args[1] == "--tree")
		version := len(args) > 0 && args[0] == "--version"
		if !catalog && !version {
			return 0, false
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
	rest := args

	// Only the shared root-directory preamble consumes a following argument.
	for len(rest) > 0 {
		if rest[0] == "--root" && len(rest) > 1 {
			rest = rest[2:]
			continue
		}
		if strings.HasPrefix(rest[0], "--root=") {
			rest = rest[1:]
			continue
		}
		break
	}
	if tree.Source != "companion root contract" {
		// Preserve authored topic detail after a uniform node synopsis. Only exact
		// command paths followed by a help flag qualify; flag values are never help.
		if binary != "corvint-tasks" && len(rest) > 1 && (rest[len(rest)-1] == "--help" || rest[len(rest)-1] == "-h") {
			words := rest[:len(rest)-1]
			if node, found := tree.Find(words); found && len(node.Path) == len(words) {
				io.WriteString(stdout, clitree.Render(tree, node))
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
	return (clitree.Help{Tree: tree, Title: "Experimental local companion", RootForms: nil, Intro: func(w io.Writer) { io.WriteString(w, root.String()) }}).Serve(args, stdout, stderr)
}
