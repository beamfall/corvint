package cli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/tasksbridge"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/rootalias"
	"github.com/Beamfall/corvint/internal/workspace"
)

// The refusal reasons the group, corpus and workspace arguments name (MMR-V0-012, MMR-V0-014, MMR-V0-015).
const (
	corpusReasonUnpaired     = "--corpus-artifact requires --tool-profile with the corpus group"
	corpusReasonMissing      = "the corpus group requires --corpus-artifact"
	corpusReasonAlias        = "--corpus-artifact must be ALIAS=PATH naming a declared --root alias"
	corpusReasonRepeated     = "--corpus-artifact must be given once with a value"
	corpusReasonVersion      = "--corpus-artifact cannot be combined with --version"
	workspaceReasonRepeated  = "--workspace must be given once with a value"
	workspaceReasonWithRoots = "--workspace cannot be combined with --root or --version"
)

// toolGroups are the per-root and workspace-level groups a --tool-profile list
// may add beside at most one code profile (MMR-V0-012).
var toolGroups = []string{toolProfileTestValidity, toolProfileDocs, toolProfileCorpus, toolProfileTasks}

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

// extractCorpusArtifact removes the optional --corpus-artifact VALUE
// (MMR-V0-014): given at most once, with a non-empty value, never beside --version.
func extractCorpusArtifact(arguments []string) (remaining []string, value, reason string) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--corpus-artifact" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if value != "" || index+1 == len(arguments) || arguments[index+1] == "" {
			return nil, "", corpusReasonRepeated
		}
		value = arguments[index+1]
		index++
	}
	if value != "" && slices.Contains(remaining, "--version") {
		return nil, "", corpusReasonVersion
	}
	return remaining, value, ""
}

// bindCorpusArtifact pairs --corpus-artifact with the corpus group and binds
// it to one declared root (MMR-V0-014): the plain root's PATH, or ALIAS=PATH
// with aliased roots. PATH follows the corvint-corpus-mcp --artifact rules: an
// absolute path must lie below that root, and a relative one must keep the local
// input path rule.
func bindCorpusArtifact(roots []rootDeclaration, groups []string, value string) (root, path, reason string) {
	selected := slices.Contains(groups, toolProfileCorpus)
	switch {
	case value == "" && !selected:
		return "", "", ""
	case value == "":
		return "", "", corpusReasonMissing
	case !selected:
		return "", "", corpusReasonUnpaired
	case len(roots) == 0:
		return "", "", ""
	}
	if len(roots) == 1 && roots[0].alias == "" {
		root, path = roots[0].root, value
	} else {
		alias, aliasedPath, aliased := rootalias.Split(value)
		index := slices.IndexFunc(roots, func(declared rootDeclaration) bool { return declared.alias == alias })
		if !aliased || aliasedPath == "" || index < 0 {
			return "", "", corpusReasonAlias
		}
		root, path = roots[index].root, aliasedPath
	}
	path, pathReason := toolgroup.RootRelativePath(root, path)
	if pathReason != "" {
		return "", "", "--corpus-artifact: " + pathReason
	}
	if rule := doccorpus.LocalInputPathRule(path); rule != "" {
		return "", "", "--corpus-artifact: local input paths must be canonical and relative to --root (" + rule + ")"
	}
	return root, path, ""
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

// openGroups opens each selected per-root group for every declared root, and
// the corpus artifact (MMR-V0-013, MMR-V0-014). A root a group cannot open is
// "repository unavailable", as for the code tools. A corpus artifact that
// cannot be opened yet is noted on stderr and refused per call, so the server
// and its other tools keep serving.
func (handler *toolHandler) openGroups(groups []string, corpusRoot, corpusPath string, stderr io.Writer) string {
	handler.groups = map[string][]toolgroup.Group{}
	handler.groupTools = map[string]int{}
	for alias, root := range handler.repositoryRoots {
		for _, name := range groups {
			var group toolgroup.Group
			switch name {
			case toolProfileTestValidity:
				registry, err := testvaliditybridge.New(root)
				if err != nil {
					return "repository unavailable"
				}
				group = toolgroup.TestValidity{Registry: registry}
			case toolProfileDocs:
				registry, err := docsbridge.New(root)
				if err != nil {
					return "repository unavailable"
				}
				group = toolgroup.Docs{Registry: registry}
			case toolProfileTasks: // TAW-V0-015
				group = tasksbridge.Group{Root: root, Class: handler.tasksRole}
			default:
				continue
			}
			for _, tool := range group.Tools() {
				handler.groupTools[tool.Name] = len(handler.groups[alias])
			}
			handler.groups[alias] = append(handler.groups[alias], group)
		}
	}
	if corpusRoot == "" {
		return ""
	}
	corpus, code := toolgroup.OpenCorpusArtifact(corpusRoot, corpusPath)
	if code != "" {
		_, _ = fmt.Fprintf(stderr, "corvint-mcp: note: --corpus-artifact unavailable (%s); corpus tools answer corpus-refused until it opens\n", code)
	}
	handler.corpus, handler.corpusTools = corpus, map[string]bool{}
	for _, tool := range corpusbridge.AllTools() {
		handler.corpusTools[tool.Name] = true
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
