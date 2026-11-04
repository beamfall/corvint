// SPDX-License-Identifier: Apache-2.0
// Package cliapp serves offline process entrypoint contracts shared by shipped binaries.
package cliapp

import (
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clierror"
	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/companioncli"
)

// Answer serves migrated companion help before any protocol, repository or server starts.
// Existing command-tree producers continue to use their authored help implementations.
func Answer(binary string, args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) > 0 && args[0] == "--version" && binary != "corvint" && binary != "corvint-tasks" {
		return companioncli.Answer(companioncli.Spec{Binary: binary, Version: companioncli.ExperimentalVersion}, args, stdout)
	}
	tree, ok := capcatalog.TreeSnapshot(binary)
	if !ok {
		return 0, false
	}
	rest := args
	if len(rest) > 0 && rest[0] == "--root" && (len(rest) < 2 || strings.HasPrefix(rest[1], "-")) {
		clierror.Write(stderr, binary, "invalid-arguments", "--root needs a directory value; use --root DIR.", nil)
		return 2, true
	}

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
	if len(rest) > 1 && rest[0] == "help" && clitree.OperationWord(rest[1]) && rest[1] != "guide" {
		if _, found := tree.Find([]string{rest[1]}); !found {
			clierror.Write(stderr, binary, "unknown-help-topic", "Unknown help topic "+rest[1]+"; run "+binary+" --help.", nil)
			return 2, true
		}
	}
	if len(rest) > 0 && strings.HasPrefix(rest[0], "-") && rest[0] != "--help" && rest[0] != "-h" {
		flag := strings.SplitN(rest[0], "=", 2)[0]
		known := tree.AllFlags()
		if !known[flag] && !(strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") && known["-"+flag]) {
			clierror.Write(stderr, binary, "invalid-arguments", "Unknown option "+flag+"; run "+binary+" --help.", nil)
			return 2, true
		}
	}
	if len(rest) > 0 && rest[0] != "help" {
		if exit, handled := clitree.RefuseIfUnknownVerb(tree, rest, stderr); handled {
			return exit, true
		}
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
