// SPDX-License-Identifier: Apache-2.0
// Package cliprofile owns the explicitly selected CLI adapter, outside native help closures.
package cliprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clierror"
	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/companioncli"
)

// Profile implements CAP-V0-9300 / ACX-V0-9301's explicitly selected CLI
// profile. The native executables, streams and protocol frames are untouched.
// binDir is the adapter's own directory, never a PATH lookup or caller path.
func Profile(ctx context.Context, binDir string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(binary, code, message string, details any) int {
		clierror.Write(stderr, binary, code, message, details)
		return 2
	}
	if len(args) < 2 || args[0] != "--binary" {
		return fail("corvint-cli", "invalid-arguments", "Required: --binary BINARY; example: corvint-cli --binary corvint --help", nil)
	}
	binary, rest := args[1], args[2:]
	tree, ok := capcatalog.TreeSnapshot(binary)
	if !ok || binary == "corvint-cli" {
		return fail("corvint-cli", "unknown-binary", fmt.Sprintf("No native CLI profile for %q; inspect corvint help --json", binary), nil)
	}
	// Inspect adapter controls on the original argv. The suffix is later
	// forwarded to a native child; CAP's scanner correctly excludes child-only
	// argv, so these parent-owned controls must remain visible independently.
	if len(rest) > 0 && args[2] == "--version" {
		if code, handled := companioncli.Answer(companioncli.Spec{Binary: binary, Version: companioncli.ExperimentalVersion}, rest, stdout); handled {
			return code
		}
		return fail(binary, "invalid-arguments", "Version accepts only --version [--json]", nil)
	}
	// Do not call native help: some isolated analyzers intentionally reject all
	// argv, and several compatibility shims own nonzero help exits.
	if len(rest) == 2 && args[2] == "help" && (args[3] == "--json" || args[3] == "--tree") {
		encoded, err := clitree.Encode(tree)
		if err != nil {
			return fail(binary, "catalog-unavailable", err.Error(), nil)
		}
		_, err = stdout.Write(encoded)
		if err != nil {
			return 1
		}
		return 0
	}
	words, help := helpPath(tree, rest)
	if help {
		if len(words) > 0 && (words[len(words)-1] == "--help" || words[len(words)-1] == "-h") {
			words = words[:len(words)-1]
		}
		node, found := tree.Find(words)
		if len(words) == 0 {
			for _, root := range tree.Nodes {
				if len(root.Path) == 0 {
					node, found = root, true
					break
				}
			}
		}
		if found && len(node.Path) == len(words) {
			_, err := io.WriteString(stdout, clitree.Render(tree, node))
			if err != nil {
				return 1
			}
			return 0
		}
		// Core's authored tree has no root node. Its offline root page lists
		// every node and still supplies the same required-input vocabulary.
		if len(words) == 0 {
			fmt.Fprintf(stdout, "Usage: %s COMMAND [FLAGS]\nRequired inputs: COMMAND; use COMMAND --help for flags and inputs.\nExample: %s help --json\n", binary, binary)
			for _, node := range tree.Nodes {
				fmt.Fprintln(stdout, node.Command)
			}
			return 0
		}
		return fail(binary, "unknown-verb", "Unknown help command "+strings.Join(words, " ")+"; inspect help --json", nil)
	}
	// The inventory includes protocol-only roots. Unknown operation words must
	// still get a coded refusal here, without starting a protocol process.
	if len(rest) > 0 && clitree.OperationWord(rest[0]) && !rootPathInput(tree, rest) {
		words = clitree.LeadingWords(rest, nil)
		node, found := tree.Find(words)
		if !found || len(node.Path) == 0 {
			verb := clitree.Suggestion(tree, rest[0])
			word, hint, _ := strings.Cut(verb, "\n")
			return fail(binary, "unknown-verb", strings.TrimSuffix(fmt.Sprintf("unknown command %q; %s", word, hint), "; "), nil)
		}
	}
	operation := ""
	if len(rest) > 0 {
		operation = rest[0]
	}
	if executionMode(binary, operation) == discoveryOnly {
		return fail(binary, "native-protocol-required", "Use the native binary for protocol traffic; this adapter serves CLI discovery only", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(binDir, binary), rest...)
	cmd.WaitDelay = time.Second // A descendant retaining a pipe cannot defeat the CLI deadline.
	cmd.Stdin = stdin
	var out, errout boundedOutput
	cmd.Stdout, cmd.Stderr = &out, &errout
	err := cmd.Run()
	if out.truncated || errout.truncated {
		return fail(binary, "output-limit", "CLI output exceeded 1 MiB; use the native binary for this operation", nil)
	}
	if ctx.Err() != nil {
		return fail(binary, "cli-deadline", "CLI operation exceeded its deadline; use the native binary for long operations", nil)
	}
	if err == nil {
		if _, err := stdout.Write(out.Bytes()); err != nil {
			return 1
		}
		if _, err := stderr.Write(errout.Bytes()); err != nil {
			return 1
		}
		return 0
	}
	code, message := refusal(out.Bytes(), errout.Bytes())
	if _, ok := err.(*exec.ExitError); !ok {
		code, message = "binary-unavailable", "Cannot launch the installed native binary"
	}
	nativeExit := -1
	if cmd.ProcessState != nil {
		nativeExit = cmd.ProcessState.ExitCode()
	}
	return fail(binary, code, message, map[string]any{"stdout": out.String(), "stderr": errout.String(), "nativeExit": nativeExit})
}

// helpPath separates command words from options using the selected tree's
// authored value placeholders. A consumed option value is never a help flag;
// -- ends option interpretation, including help detection.
func helpPath(tree clitree.Tree, args []string) ([]string, bool) {
	values := make(map[string]bool)
	for _, node := range tree.Nodes {
		for _, form := range node.Usage {
			// Preserve | as a boundary: [--full | --summary] has no value.
			fields := strings.Fields(strings.NewReplacer("[", " ", "]", " ", "(", " ", ")", " ").Replace(form))
			for i := 0; i+1 < len(fields); i++ {
				flag, next := fields[i], fields[i+1]
				if strings.HasPrefix(flag, "-") && !strings.Contains(flag, "=") && next != "|" && !strings.HasPrefix(next, "-") && (next[0] >= 'A' && next[0] <= 'Z' || strings.HasPrefix(next, "<") || strings.Contains(next, "|")) {
					values[flag] = true
				}
			}
		}
	}
	var words []string
	help := len(args) > 0 && args[0] == "help"
	if help {
		args = args[1:]
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" {
			help = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			flag, _, inline := strings.Cut(arg, "=")
			if values[flag] && !inline && i+1 < len(args) {
				i++
			}
			continue
		}
		words = append(words, arg)
	}
	return words, help
}

// A bare word can be either a verb or a positional path. Admit existing paths
// only where the catalog declares a positional root input; retain the existing
// unknown-verb refusal for unrecognized bare words. Explicit ./ paths already
// bypass operation-word discovery and reach the native parser.
func rootPathInput(tree clitree.Tree, args []string) bool {
	if _, unknown := clitree.UnknownVerb(tree, args); unknown {
		return false
	}
	for _, node := range tree.Nodes {
		if len(node.Path) == 0 {
			for _, input := range node.Required {
				if !strings.HasPrefix(input, "-") && !strings.HasPrefix(input, "stdin:") {
					_, err := os.Stat(args[0])
					return err == nil
				}
			}
		}
	}
	return false
}

type mode uint8

const (
	nativeCLI mode = iota
	discoveryOnly
)

// These are binary selector values, not adapter subcommands. Mixed-mode
// binaries classify the selected operation rather than every invocation.
func executionMode(name, operation string) mode {
	if strings.HasPrefix(name, "corvint-analyzer-") || strings.Contains(name, "mcp") {
		return discoveryOnly
	}
	switch name {
	case "corvint-lsp", "corvint-console", "corvint-corpus-http", "corvint-pulse":
		return discoveryOnly
	case "corvint-gateway":
		if operation == "serve" {
			return discoveryOnly
		}
	}
	return nativeCLI
}

type boundedOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *boundedOutput) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedOutput) String() string { return b.buffer.String() }

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > (1<<20)-b.buffer.Len() {
		p = p[:(1<<20)-b.buffer.Len()]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

// refusal understands native Core string/object errors and Tasks results. A
// codeless refusal gets a profile-owned code; no native serializer is rewritten.
func refusal(out, errout []byte) (string, string) {
	code, message := "command-failed", "Native command refused; inspect retained details and command help"
	for _, raw := range [][]byte{errout, out} {
		var v struct {
			Code     string          `json:"code"`
			Error    json.RawMessage `json:"error"`
			Codes    []string        `json:"codes"`
			Warnings []string        `json:"warnings"`
		}
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		if v.Code != "" {
			code = v.Code
		}
		if len(v.Codes) > 0 {
			code = v.Codes[0]
		}
		if len(v.Warnings) > 0 {
			message = strings.Join(v.Warnings, "; ")
		}
		if len(v.Error) > 0 {
			var nested struct{ Code, Message string }
			if json.Unmarshal(v.Error, &nested) == nil {
				if nested.Code != "" {
					code = nested.Code
				}
				if nested.Message != "" {
					message = nested.Message
				}
			} else {
				_ = json.Unmarshal(v.Error, &message)
			}
		}
	}
	if code == "command-failed" && (strings.Contains(message, "does not exist") || strings.Contains(message, "not found")) {
		code = "not-found"
	}
	return code, message
}
