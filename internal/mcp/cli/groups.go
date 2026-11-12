package cli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/workspace"
)

// The refusal reasons the workspace argument names (MMR-V0-015).
const (
	workspaceReasonRepeated  = "--workspace must be given once with a value"
	workspaceReasonWithRoots = "--workspace cannot be combined with --root or --version"
)

// removedGroups are the tool groups corvint mcp no longer serves, each with
// its typed refusal (MTT-V1-005): docs and corpus moved to corvint-docs mcp,
// and test-validity was removed with the G24 batch (M12).
var removedGroups = map[string]string{
	"docs":          "profile-moved: the docs group is served by corvint-docs mcp (MTT-V1-005); start corvint-docs mcp --root ROOT",
	"corpus":        "profile-moved: the corpus group is served by corvint-docs mcp (MTT-V1-005); start corvint-docs mcp --root ROOT --corpus-artifact PATH",
	"test-validity": "group-removed: corvint mcp no longer serves the test-validity group (MTT-V1-005); corvint test-validity answers on the command line",
}

// valueFlags are the corvint mcp flags that take the next argument as their
// value, with --corpus-artifact, which took one before the corpus group moved.
var valueFlags = []string{"--corpus-artifact", "--error-profile", "--follow-ref", "--protocol-version", "--root", "--tool-profile", "--workspace"}

// removedGroupReason names the refusal for a --tool-profile list that asks for
// a removed group, or for --corpus-artifact, which only the corpus group read;
// "" when neither appears. Only an argument in flag position counts: the value
// of another flag, such as `--root --corpus-artifact`, is that flag's to refuse.
func removedGroupReason(arguments []string) string {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--corpus-artifact" {
			return removedGroups["corpus"]
		}
		if !slices.Contains(valueFlags, argument) || index+1 == len(arguments) {
			continue
		}
		index++
		if argument != "--tool-profile" {
			continue
		}
		for _, token := range strings.Split(arguments[index], ",") {
			if reason := removedGroups[token]; reason != "" {
				return reason
			}
		}
	}
	return ""
}

// toolGroups are the per-root and workspace-level groups a --tool-profile list
// may add beside at most one code profile (MMR-V0-012).
var toolGroups = []string{toolProfileTestEvidence, toolProfileAffected}

// splitToolProfile reads one --tool-profile value: a single code profile, as
// MCPV0-026 has always admitted, or a comma list of at most one code profile
// and distinct tool groups (MMR-V0-012). Any empty, unknown, differently cased or
// repeated token, or two code profiles, is not ok. The code tools are always
// served; the groups are added only when named.
func splitToolProfile(value string) (code string, groups []string, ok bool) {
	seen := map[string]bool{}
	for _, token := range strings.Split(value, ",") {
		switch {
		case token == "" || seen[token]:
			return "", nil, false
		case profileRegistries[token] != nil:
			if code != "" {
				return "", nil, false
			}
			code = token
		case slices.Contains(toolGroups, token):
			groups = append(groups, token)
		default:
			return "", nil, false
		}
		seen[token] = true
	}
	return code, groups, true
}

// expandWorkspace replaces --workspace FILE with one --root ALIAS=ROOT per
// alias the corvint-workspace/0 declaration names, in alias order
// (MMR-V0-015). It cannot be combined with --root or --version. A declared
// default revision is not followed: the server notes it on stderr, and
// --follow-ref ALIAS=REF follows a ref explicitly.
func expandWorkspace(arguments []string, stderr io.Writer) (remaining []string, reason string) {
	var file string
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--workspace" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if file != "" || index+1 == len(arguments) || arguments[index+1] == "" {
			return nil, workspaceReasonRepeated
		}
		file = arguments[index+1]
		index++
	}
	if file == "" {
		return remaining, ""
	}
	if slices.Contains(remaining, "--root") || slices.Contains(remaining, "--version") {
		return nil, workspaceReasonWithRoots
	}
	declared, err := workspace.Load(file)
	if err != nil {
		return nil, "--workspace: " + err.Error()
	}
	for _, alias := range declared.Aliases() {
		repository := declared.Repos[alias]
		remaining = append(remaining, "--root", alias+"="+repository.Root)
		if repository.Revision != "" {
			_, _ = fmt.Fprintf(stderr, "corvint-mcp: note: workspace alias %s declares revision %q, which is not followed; --follow-ref %s=REF follows a ref\n", alias, repository.Revision, alias)
		}
	}
	return remaining, ""
}

// openGroups opens each selected per-root group for every declared root
// (MMR-V0-013). A root a group cannot open is "repository unavailable", as
// for the code tools.
func (handler *toolHandler) openGroups(groups []string) string {
	handler.groups = map[string][]toolgroup.Group{}
	handler.groupTools = map[string]int{}
	for alias, root := range handler.repositoryRoots {
		for _, name := range groups {
			var group toolgroup.Group
			switch name {
			case toolProfileTestEvidence:
				binding, ok := rootbind.Bind(root)
				if !ok {
					return "repository unavailable"
				}
				group = toolgroup.TestEvidence{Root: root, Binding: binding, Parse: parseTestEvidence}
			case toolProfileAffected:
				binding, ok := rootbind.Bind(root)
				if !ok {
					return "repository unavailable"
				}
				group = toolgroup.NewAffected(root, binding, handler.affected)
			default:
				continue
			}
			for _, tool := range group.Tools() {
				handler.groupTools[tool.Name] = len(handler.groups[alias])
			}
			handler.groups[alias] = append(handler.groups[alias], group)
		}
	}
	return ""
}

// groupDescriptors lists the per-root group tools of one alias, "" for the
// single root; each call builds fresh schemas.
func (handler *toolHandler) groupDescriptors(alias string) []bridge.ToolDescriptor {
	var tools []bridge.ToolDescriptor
	for _, group := range handler.groups[alias] {
		tools = append(tools, group.Tools()...)
	}
	return tools
}

// answerGroup routes a per-root group call like a code tool (MMR-V0-013): the
// required repository alias selects the root and is removed before the
// group's closed decode, and the launch-worktree notice labels the result.
func (handler *toolHandler) answerGroup(ctx context.Context, index int, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	registry, alias := handler.registry, ""
	if handler.repositories != nil {
		value, isString := arguments[repositoryArgument].(string)
		if !isString || handler.repositories[value] == nil {
			return nil, protocol.InvalidParams("Invalid params")
		}
		registry, alias = handler.repositories[value], value
		arguments = maps.Clone(arguments)
		delete(arguments, repositoryArgument)
	}
	return handler.withWorktreeNotice(registry)(handler.groups[alias][index].Call(ctx, name, arguments))
}
