// SPDX-License-Identifier: Apache-2.0

// Package docsserver is the `corvint-docs mcp` local read-only stdio MCP
// server (MTT-V1-004, owner decision R30): the corvint.docs_* corpus tools,
// corvint.docs_draft, corvint.docs_consume and corvint.help, served from the
// corvint-docs executable alone. It reuses the shared transport
// (internal/mcp/server), the docs tool group (internal/mcp/toolgroup over
// internal/mcp/docsbridge) and the corpus artifact group
// (internal/mcp/corpusbridge), so each tool answers exactly as it did on
// corvint mcp's former docs and corpus groups, which it replaces
// (MTT-V1-005).
// It lives in the docs import area because it serves the corpus group; the
// tests-area `corvint mcp` server cannot be imported here (decision 0594
// item 8).
package docsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/projectpath"
)

const (
	// Name is serverInfo.name and the stderr prefix's binary.
	Name        = "corvint-docs"
	prefix      = "corvint-docs mcp: "
	description = "Local read-only Corvint documentation server."

	// corpusRefused is the corpus group's own refusal code (MMR-V0-014).
	corpusRefused = "corpus-refused"
	// noArtifact is the corpus refusal while no --corpus-artifact was given.
	noArtifact = "corvint-docs mcp was started without --corpus-artifact; restart it with --corpus-artifact PATH, a corpus artifact path relative to --root"
)

// Usage is the `corvint-docs mcp --help` text.
const Usage = `Serve the documentation tools over the local read-only stdio MCP transport (MTT-V1-004, experimental).

Usage:
  corvint-docs mcp --root ABSOLUTE_ROOT [--corpus-artifact PATH] [--protocol-version 2026-07-28|2025-11-25]

Required: --root, an absolute path inside a Git worktree; the server binds its toplevel.
It lists corvint.help, corvint.docs_draft, corvint.docs_consume and every
corvint.docs_* corpus tool, and needs only the corvint-docs executable. --corpus-artifact
PATH binds a corpus artifact, a path relative to the root: one that opens at startup
lists exactly the tools it declares; without it, or while it cannot be opened, each
corpus call answers the tool error corpus-refused naming the fix. It speaks JSON-RPC on
stdin and stdout until stdin closes or SIGINT/SIGTERM arrives, and exits 2 with
"corvint-docs mcp: invalid arguments: REASON" on stderr for an argument it does not
accept. The testing and flow tools are on corvint mcp; docs/MCP-SERVER.md lists every tool.

Example:
  corvint-docs mcp --root /absolute/repo --corpus-artifact corpus.json
`

// Serve runs one `corvint-docs mcp` server until stdin closes or ctx ends;
// version is serverInfo.version. It returns the process exit code.
func Serve(ctx context.Context, version string, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h") {
		_, _ = io.WriteString(stdout, Usage)
		return 0
	}
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	declared, artifact, reason := parseArguments(arguments)
	if !protocolOK {
		reason = "--protocol-version must name a supported protocol version once"
	}
	if reason != "" {
		_, _ = fmt.Fprintln(stderr, prefix+"invalid arguments: "+reason)
		return 2
	}
	root, unusable := rootbind.Resolve(declared) // MCPV0-950: a root inside a worktree binds its toplevel.
	if unusable != "" {
		_, _ = fmt.Fprintln(stderr, prefix+"repository unavailable: "+unusable)
		return 2
	}
	registry, registryErr := docsbridge.New(root)
	if registryErr != nil {
		_, _ = fmt.Fprintln(stderr, prefix+"repository unavailable")
		return 2
	}
	handler := &handler{docs: toolgroup.Docs{Registry: registry}, corpusTools: map[string]bool{}}
	for _, tool := range corpusbridge.AllTools() {
		handler.corpusTools[tool.Name] = true
	}
	if artifact != "" {
		path, pathReason := artifactPath(root, declared, artifact)
		if pathReason != "" {
			_, _ = fmt.Fprintln(stderr, prefix+"invalid arguments: --corpus-artifact: "+pathReason)
			return 2
		}
		opened, code := corpusbridge.OpenArtifact(root, path)
		if code != "" {
			_, _ = fmt.Fprintf(stderr, "%snote: --corpus-artifact unavailable (%s); corpus tools answer %s until it opens\n", prefix, code, corpusRefused)
		}
		handler.corpus = opened
	}
	// Every Git runner is pinned to one start-time executable (MCPV0-016).
	git, err := gitstatus.Pin()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, prefix+"git unavailable")
		return 2
	}
	gitrun.PinBinary(git)
	instance, err := server.New(server.Config{
		ProtocolVersion: protocolVersion,
		Name:            Name, Version: version,
		Description:  description,
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
		// A 2025-11-25 initialize is told to restart with --protocol-version (MCPV0-820).
		NameLegacyFlag: true,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, prefix+"startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, prefix+"transport failed")
		return 2
	}
	return 0
}

// parseArguments reads `--root ROOT` once and `--corpus-artifact PATH` at
// most once, in either order; anything else is refused with its reason.
func parseArguments(arguments []string) (root, artifact, reason string) {
	values := map[string]*string{"--root": &root, "--corpus-artifact": &artifact}
	for index := 0; index < len(arguments); index += 2 {
		target, known := values[arguments[index]]
		switch {
		case !known:
			return "", "", fmt.Sprintf("unknown argument %q; corvint-docs mcp takes --root, --corpus-artifact and --protocol-version (see --help)", arguments[index])
		case index+1 == len(arguments) || arguments[index+1] == "":
			return "", "", arguments[index] + " needs a value"
		case *target != "":
			return "", "", arguments[index] + " must be given once"
		}
		*target = arguments[index+1]
	}
	if root == "" {
		return "", "", "--root ABSOLUTE_ROOT is required"
	}
	return root, artifact, ""
}

// artifactPath applies the corpus --artifact rules (MMR-V0-014): an absolute
// path must lie below the root, a relative one must keep the local input path
// rule and names a file below the declared root, which may itself lie below
// the toplevel the root resolved to.
func artifactPath(toplevel, declared, value string) (string, string) {
	path, reason := toolgroup.RootRelativePath(toplevel, value)
	if reason != "" {
		return "", reason
	}
	if rule := projectpath.LocalInputRule(path); rule != "" {
		return "", "local input paths must be canonical and relative to --root (" + rule + ")"
	}
	if below := belowToplevel(declared, toplevel); below != "" && !filepath.IsAbs(value) {
		path = below + "/" + path
	}
	return path, ""
}

type handler struct {
	docs toolgroup.Docs
	// corpus is the bound artifact, nil without --corpus-artifact;
	// corpusTools holds every corpus tool name, listed or not.
	corpus      *corpusbridge.Artifact
	corpusTools map[string]bool
}

func (handler *handler) Handle(ctx context.Context, request protocol.Request, _ server.Notifier) (map[string]any, *protocol.RPCError) {
	switch request.Method {
	case "tools/list":
		if refusal := toolgroup.ListParamsRefusal(request.Params); refusal != nil { // MCPV0-952
			return nil, refusal
		}
		return map[string]any{"cacheScope": "private", "tools": handler.listTools(), "ttlMs": 300_000}, nil
	case "tools/call":
		result, failure := handler.call(ctx, request.Params)
		return result, toolgroup.WithInvalidParamsData(handler.listTools, request.Params, failure) // MCPV0-952
	default:
		return nil, protocol.MethodNotFound()
	}
}

// listTools is the tools/list surface, sorted by name.
func (handler *handler) listTools() []bridge.ToolDescriptor {
	tools := append(handler.docs.Tools(), bridge.HelpDescriptor())
	if handler.corpus != nil {
		tools = append(tools, handler.corpus.Tools()...)
	} else {
		tools = append(tools, corpusbridge.AllTools()...)
	}
	sort.Slice(tools, func(left, right int) bool { return tools[left].Name < tools[right].Name })
	return bridge.WithOutputSchemas(tools)
}

func (handler *handler) call(ctx context.Context, params map[string]any) (map[string]any, *protocol.RPCError) {
	for key := range params {
		if key != "_meta" && key != "name" && key != "arguments" {
			return nil, protocol.InvalidParams("Invalid params")
		}
	}
	name, _ := params["name"].(string)
	arguments := map[string]any{}
	if raw, present := params["arguments"]; present {
		var ok bool
		if arguments, ok = raw.(map[string]any); !ok {
			return nil, protocol.InvalidParams("Invalid params")
		}
	}
	switch {
	case name == bridge.ToolHelp: // MCPV0-101: reads only the binary.
		raw, err := json.Marshal(arguments)
		if err != nil {
			return nil, protocol.InvalidParams("Invalid params")
		}
		return toolgroup.Help(ctx, name, raw, func(name, code, message string) (map[string]any, *protocol.RPCError) {
			return toolgroup.Failure(toolgroup.DocsErrorProfile, name, code, message)
		})
	case slices.ContainsFunc(handler.docs.Tools(), func(tool bridge.ToolDescriptor) bool { return tool.Name == name }):
		return handler.docs.Call(ctx, name, arguments)
	case handler.corpusTools[name] && handler.corpus != nil:
		return handler.corpus.Call(ctx, name, arguments)
	case handler.corpusTools[name]:
		return corpusbridge.Failure(name, corpusRefused, noArtifact)
	}
	return nil, protocol.InvalidParams("Invalid params")
}

// belowToplevel is the slash path of a declared root below the toplevel
// rootbind.Resolve bound, "" when the root is that toplevel.
func belowToplevel(declared, toplevel string) string {
	if toplevel == "" || toplevel == declared {
		return ""
	}
	physical, err := filepath.EvalSymlinks(declared)
	if err != nil {
		return ""
	}
	below, err := filepath.Rel(toplevel, physical)
	if err != nil || below == "." {
		return ""
	}
	return filepath.ToSlash(below)
}
