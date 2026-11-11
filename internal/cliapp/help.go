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
	return AnswerStamped(binary, "", "", args, stdout, stderr)
}

// AnswerStamped is Answer for a binary whose main package passes its
// `-X main.build=N` and `-X main.sourceCommit=SHA` stamps through, so a release
// build reports the build and commit on --version (ARTIFACT-BIN-V0-508,
// PUB-V0-052); an unstamped build passes empty values and reports as before.
func AnswerStamped(binary, build, sourceCommit string, args []string, stdout, stderr io.Writer) (int, bool) {
	return AnswerReleased(binary, "", build, sourceCommit, args, stdout, stderr)
}

// AnswerReleased is AnswerStamped for an independently released binary whose
// main package also passes its `-X main.version=X.Y.Z` stamp through, so a
// release build reports the version the release builder named rather than the
// repository's compiled release line (ARTIFACT-BIN-V0-002). An empty version
// is the compiled development line, as for a core-set binary.
func AnswerReleased(binary, version, build, sourceCommit string, args []string, stdout, stderr io.Writer) (int, bool) {
	if version == "" {
		version = companioncli.ExperimentalVersion
	}
	// Preserve authored test-command help, including provider flag details.
	// These binaries can still expose their offline command-tree catalog.
	switch binary {
	case "corvint-test-runner", "corvint-js-test-provider":
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
		return companioncli.Answer(companioncli.Spec{Binary: binary, Version: version, Build: build, Commit: sourceCommit}, args, stdout)
	}
	tree, ok := capcatalog.TreeSnapshot(binary)
	if !ok {
		return 0, false
	}
	// ACP-011 / CAP-V0-8800: analyzer node argv belongs to the native
	// protocol dispatcher, even when a discovery-tree node matches exactly.
	if tree.Source == "companion root contract" && strings.HasPrefix(binary, "corvint-analyzer-") && len(args) > 1 && (args[len(args)-1] == "--help" || args[len(args)-1] == "-h") {
		return 0, false
	}
	// CAP-V0-9300 / CAP-V0-8800: an exact node owns its help page even
	// when its name is "help". Preambles still reach authored dispatch.
	if binary != "corvint-tasks" && len(args) > 1 && (args[len(args)-1] == "--help" || args[len(args)-1] == "-h") {
		words := args[:len(args)-1]
		if node, found := tree.Find(words); found && len(node.Path) == len(words) {
			if _, err := io.WriteString(stdout, clitree.Render(tree, node)); err != nil {
				return 1, true
			}
			return 0, true
		}
	}
	if tree.Source != "companion root contract" {
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
