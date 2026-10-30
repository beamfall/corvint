package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/procprofile"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/rootalias"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const (
	serverName    = "corvint-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = "corvint-mcp-tool-error/0"

	// toolErrorReasonClass is the tool-error object under the closed
	// --error-profile reason-class selector (MCPV0-027, decision 0383).
	toolErrorReasonClass = "corvint-mcp-tool-error/1"
	errorProfileReason   = "reason-class"
	unclassified         = "unclassified"

	// toolProfileTaskReview and toolProfileFlows are the values of the closed
	// --tool-profile selector (MCPV0-026, decision 0374, amended by AFU-V1-034).
	toolProfileTaskReview    = "task-review"
	toolProfileTaskReviewLSP = "task-review-lsp"
	toolProfileFlows         = "flows"
	toolProfileImportGraph   = "import-graph"
	// toolProfileTestValidity, toolProfileDocs and toolProfileCorpus are the
	// tool groups a --tool-profile list may add (MMR-V0-012).
	toolProfileTestValidity = "test-validity"
	toolProfileDocs         = "docs"
	toolProfileCorpus       = "corpus"

	// untrustedDataPrefix and untrustedDataSuffix are the internal/repoenvelope
	// envelope the host adapters apply to repository-authored free text.
	// Repository-authored title, summary, and evidence-reason fields reach a
	// model through this "text" content block, so it is framed by the same
	// builder (AHI-004); structuredContent stays unwrapped for programmatic callers.
	untrustedDataPrefix = repoenvelope.Prefix
	untrustedDataSuffix = repoenvelope.Suffix
)

// usage is the --help text (MCPV0-113).
const usage = "usage: corvint-mcp --root ABSOLUTE_ROOT [--protocol-version 2026-07-28|2025-11-25]\n" +
	"       corvint-mcp --root ALIAS=ABSOLUTE_ROOT [--root ALIAS=ABSOLUTE_ROOT ...]\n" +
	"       corvint-mcp --root ABSOLUTE_ROOT --tool-profile import-graph\n" +
	"       corvint-mcp --root ABSOLUTE_ROOT --follow-ref REF\n" +
	"       corvint-mcp --root ALIAS=ABSOLUTE_ROOT ... --follow-ref ALIAS=REF [--follow-ref ALIAS=REF ...]\n" +
	"       corvint-mcp --workspace FILE [--follow-ref ALIAS=REF ...]\n" +
	"       corvint-mcp ROOTS... --tool-profile LIST [--corpus-artifact [ALIAS=]PATH]\n" +
	"       corvint-mcp --version\n" +
	"A local read-only stdio MCP server. Each --root must be an absolute path to a Git worktree.\n" +
	"The import-graph profile adds corvint.graph.imports; calls require a full commit revision.\n" +
	"--follow-ref keeps a detached linked worktree on the commit REF names, moving it before each call;\n" +
	"with aliased roots, ALIAS=REF selects the root that follows and the others serve their checkout.\n" +
	"--workspace reads the corvint-workspace/0 declaration FILE and serves each alias as --root ALIAS=ROOT.\n" +
	"--tool-profile LIST is a comma list of at most one of task-review, task-review-lsp, flows or import-graph\n" +
	"and the groups test-validity, docs and corpus; corpus needs --corpus-artifact, a path relative to the root\n" +
	"(ALIAS=PATH with aliased roots). Without --tool-profile the default tools are listed.\n" +
	"Example: corvint-mcp --root /absolute/repo --tool-profile import-graph"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(runProfiled(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv))
}

// runProfiled is run under the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE bracket (PRB-V0-010):
// one profile per server process, written when the server exits and on SIGINT or SIGTERM.
func runProfiled(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer, lookup runtimeenv.Lookup) int {
	return procprofile.Run(ctx, lookup, procprofile.Reporter(stderr, "corvint-mcp"), func() int {
		return run(ctx, arguments, stdin, stdout, stderr)
	})
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h") { // MCPV0-113
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	}
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	arguments, profile, profileOK := extractToolProfile(arguments)
	arguments, reasonClass, errorProfileOK := extractErrorProfile(arguments)
	arguments, followSelectors, followReason := extractFollowRef(arguments)
	arguments, corpusArtifact, corpusReason := extractCorpusArtifact(arguments)
	arguments, workspaceReason := expandWorkspace(arguments, stderr)
	roots, versionOnly, ok := parseArguments(arguments)
	if !protocolOK || !profileOK || !errorProfileOK || (cmp.Or(followReason, corpusReason, workspaceReason) == "" && !ok) {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: invalid arguments")
		return 2
	}
	profile, groups, _ := splitToolProfile(profile)
	followRefs, bindReason := bindFollowSelectors(roots, followSelectors)
	corpusRoot, corpusPath, corpusBindReason := bindCorpusArtifact(roots, groups, corpusArtifact)
	if reason := cmp.Or(followReason, corpusReason, workspaceReason, bindReason, corpusBindReason); reason != "" { // MMR-V0-011: name the reason.
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: invalid arguments: "+reason)
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	for _, declared := range roots {
		if !filepath.IsAbs(declared.root) { // MCPV0-113: name the requirement, still fail closed.
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: repository unavailable: --root must be an absolute path")
			return 2
		}
	}
	git, err := gitstatus.Pin()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: git unavailable")
		return 2
	}
	// The CEM seams spawn Git through their own runner; pin it to the same path.
	gitrun.PinBinary(git)
	handler := &toolHandler{reasonClass: reasonClass}
	bindings := map[*bridge.Registry]string{}
	if len(roots) == 1 && roots[0].alias == "" {
		registry, registryErr := profileRegistries[profile](roots[0].root)
		if registryErr != nil {
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: repository unavailable")
			return 2
		}
		handler.registry = registry
		bindings[registry] = roots[0].root
		handler.repositoryRoots = map[string]string{"": roots[0].root}
		if followRef, follows := followRefs[""]; follows {
			var refusal string
			handler.follow, refusal = newFollowRef(ctx, git, registry, followRef)
			if refusal != "" {
				_, _ = fmt.Fprintln(stderr, "corvint-mcp: "+refusal)
				return 2
			}
		}
	} else {
		var refusal string
		handler.repositories, handler.aliases, refusal = openRepositories(roots, profileRegistries[profile])
		if refusal != "" {
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: "+refusal)
			return 2
		}
		handler.repositoryRoots = map[string]string{}
		for _, declared := range roots {
			bindings[handler.repositories[declared.alias]] = declared.root
			handler.repositoryRoots[declared.alias] = declared.root
		}
		if handler.follows, refusal = openFollowRefs(ctx, git, handler.repositories, handler.aliases, followRefs); refusal != "" {
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: "+refusal)
			return 2
		}
	}
	if refusal := handler.openGroups(groups, corpusRoot, corpusPath, stderr); refusal != "" {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: "+refusal)
		return 2
	}
	// The launch directory is observed once and only labels results (MMR-V0-009).
	if launch, err := os.Getwd(); err == nil {
		handler.notices = launchWorktreeNotices(launch, bindings)
	}
	instance, err := server.New(server.Config{
		ProtocolVersion: protocolVersion,
		Name:            serverName, Version: serverVersion,
		Description:  "Local read-only Corvint context and repository evidence server.",
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: transport failed")
		return 2
	}
	return 0
}

// profileRegistries binds each --tool-profile value; the empty value is the
// default V0 registry.
var profileRegistries = map[string]func(string) (*bridge.Registry, *bridge.Error){
	"": bridge.New, toolProfileTaskReview: bridge.NewTaskReview, toolProfileTaskReviewLSP: bridge.NewTaskReviewLSP, toolProfileFlows: bridge.NewFlows, toolProfileImportGraph: bridge.NewImportGraph,
}

// extractToolProfile removes the optional, closed descendant-profile selector
// (MCPV0-026), whose value may be a list of tool groups (MMR-V0-012). A
// missing, duplicate, or unknown value, or the selector beside --version,
// fails before repository startup; omission keeps the V0 tools.
func extractToolProfile(arguments []string) (remaining []string, profile string, ok bool) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--tool-profile" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if profile != "" || index+1 == len(arguments) || arguments[index+1] == "" {
			return nil, "", false
		}
		if _, _, valid := splitToolProfile(arguments[index+1]); !valid {
			return nil, "", false
		}
		profile = arguments[index+1]
		index++
	}
	if profile != "" && slices.Contains(remaining, "--version") {
		return nil, "", false
	}
	return remaining, profile, true
}

// extractErrorProfile removes the optional, closed tool-error selector
// (MCPV0-027) under the same rules as extractToolProfile; omission keeps
// corvint-mcp-tool-error/0.
func extractErrorProfile(arguments []string) (remaining []string, reasonClass bool, ok bool) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--error-profile" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if reasonClass || index+1 == len(arguments) || arguments[index+1] != errorProfileReason {
			return nil, false, false
		}
		reasonClass = true
		index++
	}
	if reasonClass && slices.Contains(remaining, "--version") {
		return nil, false, false
	}
	return remaining, reasonClass, true
}

// parseArguments reads `--version` alone, one plain `--root ABSOLUTE_ROOT`
// (MCPV0-001), or 1 to maxRoots `--root ALIAS=ABSOLUTE_ROOT` declarations with
// distinct aliases (MMR-V0-001, MMR-V0-002). A plain root never shares argv
// with another root.
func parseArguments(arguments []string) (roots []rootDeclaration, versionOnly bool, ok bool) {
	if len(arguments) == 1 && arguments[0] == "--version" {
		return nil, true, true
	}
	if len(arguments) == 0 || len(arguments)%2 != 0 || len(arguments) > 2*maxRoots {
		return nil, false, false
	}
	aliases := make(map[string]bool, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		if arguments[index] != "--root" || arguments[index+1] == "" {
			return nil, false, false
		}
		alias, root, aliased := rootalias.Split(arguments[index+1])
		if (!aliased && len(arguments) != 2) || (aliased && (root == "" || aliases[alias])) {
			return nil, false, false
		}
		aliases[alias] = true
		roots = append(roots, rootDeclaration{alias: alias, root: root})
	}
	return roots, false, true
}

type toolHandler struct {
	registry    *bridge.Registry
	reasonClass bool
	// repositories and aliases bind the declared roots in multi-root mode
	// (MMR-V0); both are nil under the single-root contract.
	repositories map[string]*bridge.Registry
	aliases      []string
	// notices holds the launch-worktree notice per binding (MMR-V0-009).
	notices map[*bridge.Registry]string
	// follow binds the single root to --follow-ref (MCPV0-039); nil otherwise.
	follow *followRef
	// follows binds each multi-root alias that follows a ref (MMR-V0-010).
	follows map[string]*followRef
	// repositoryRoots holds each declared root by alias, "" for the single root.
	repositoryRoots map[string]string
	// groups holds the selected per-root tool groups by alias, "" for the
	// single root, in one order for every alias; groupTools maps each of their
	// tool names to its group's index (MMR-V0-013).
	groups     map[string][]toolgroup.Group
	groupTools map[string]int
	// corpus is the workspace-level corpus group and corpusTools every corpus
	// tool name, listed or not, so none can reach a followed ref; nil unless
	// the corpus group is selected (MMR-V0-014, MMR-V0-017).
	corpus      *toolgroup.CorpusArtifact
	corpusTools map[string]bool
}

func (handler *toolHandler) Handle(ctx context.Context, request protocol.Request, _ server.Notifier) (map[string]any, *protocol.RPCError) {
	switch request.Method {
	case "tools/list":
		if !onlyKeys(request.Params, "_meta", "cursor") {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if cursor, present := request.Params["cursor"]; present {
			if value, valid := cursor.(string); !valid || value != "" {
				return nil, protocol.InvalidParams("Invalid params")
			}
		}
		var tools []bridge.ToolDescriptor
		if handler.repositories != nil {
			tools = handler.multiRootTools()
		} else {
			tools = append(handler.registry.Tools(), handler.groupDescriptors("")...)
		}
		if handler.follow != nil {
			tools = followRefTools(tools)
		}
		if len(handler.follows) != 0 {
			tools = followRefRootsTools(tools, slices.Sorted(maps.Keys(handler.follows)))
		}
		if handler.corpus != nil { // MMR-V0-014: workspace-level, so never alias-routed or follow-marked.
			tools = append(tools, handler.corpus.Tools()...)
		}
		sort.Slice(tools, func(left, right int) bool { return tools[left].Name < tools[right].Name })
		return map[string]any{
			"cacheScope": "private", "tools": tools, "ttlMs": 300_000,
		}, nil
	case "tools/call":
		return handler.call(ctx, request.Params)
	default:
		return nil, protocol.MethodNotFound()
	}
}

func (handler *toolHandler) call(ctx context.Context, params map[string]any) (map[string]any, *protocol.RPCError) {
	if !onlyKeys(params, "_meta", "name", "arguments") {
		return nil, protocol.InvalidParams("Invalid params")
	}
	name, nameOK := params["name"].(string)
	arguments := map[string]any{}
	if rawArguments, present := params["arguments"]; present {
		var argumentsOK bool
		arguments, argumentsOK = rawArguments.(map[string]any)
		if !argumentsOK {
			return nil, protocol.InvalidParams("Invalid params")
		}
	}
	if !nameOK || name == "" {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if handler.corpusTools[name] { // MMR-V0-014: the corpus artifact never binds a followed ref.
		return handler.corpus.Call(ctx, name, arguments)
	}
	if follow, registry := handler.followFor(arguments); follow != nil && name != bridge.ToolHelp { // MCPV0-100: help reads only the binary, so it never binds the ref.
		return follow.bound(ctx, func(state followState, code string) (map[string]any, *protocol.RPCError) {
			echo := withFollowRef(&state, follow)
			if code != "" {
				return echo(handler.withWorktreeNotice(registry)(handler.toolFailure(name, code, "")))
			}
			return echo(handler.answer(ctx, name, arguments))
		})
	}
	return handler.answer(ctx, name, arguments)
}

// followFor returns the ref binding a call is answered under and the
// registry it binds: the single root's, or the one the call's repository alias
// names when that alias follows a ref (MMR-V0-010). An undeclared or
// non-following alias binds nothing and is routed as before.
func (handler *toolHandler) followFor(arguments map[string]any) (*followRef, *bridge.Registry) {
	if handler.follow != nil {
		return handler.follow, handler.registry
	}
	alias, _ := arguments[repositoryArgument].(string)
	if follow := handler.follows[alias]; follow != nil {
		return follow, handler.repositories[alias]
	}
	return nil, nil
}

// answer routes and runs one validated tool call.
func (handler *toolHandler) answer(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	if index, grouped := handler.groupTools[name]; grouped {
		return handler.answerGroup(ctx, index, name, arguments)
	}
	registry := handler.registry
	if handler.repositories != nil {
		var routed bool
		if registry, arguments, routed = handler.route(name, arguments); !routed {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if registry == nil {
			return handler.statusAll(ctx)
		}
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	result, bridgeErr := registry.Call(ctx, name, raw)
	if bridgeErr != nil {
		if bridgeErr.Code == "invalid-arguments" || bridgeErr.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if bridgeErr.Code == "cancelled" || ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return handler.withWorktreeNotice(registry)(handler.toolFailure(name, bridgeErr.Code, bridgeErr.ReasonClass))
	}
	structured, full, text, err := structuredResult(result, bridge.EnvelopeOnly(name))
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	// The summary can drop the field that carries the terminator, so the full
	// object is checked too: such a receipt stays refused, never emitted.
	if frameErr != nil || strings.Contains(full, repoenvelope.Terminator) {
		return handler.withWorktreeNotice(registry)(handler.toolFailure(name, repoenvelope.CollisionCode, ""))
	}
	response := map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}
	// Flow intents carry repository-authored step text, which reaches the
	// caller only inside the envelope (AFU-V1-035).
	if bridge.EnvelopeOnly(name) {
		delete(response, "structuredContent")
	}
	return handler.withWorktreeNotice(registry)(response, nil)
}

// toolFailure is the closed tool-error object: profile /0 by default, or /1
// with a required reasonClass under the reason-class selector (MCPV0-028).
func (handler *toolHandler) toolFailure(name, code, reasonClass string) (map[string]any, *protocol.RPCError) {
	value := handler.toolErrorObject(name, code, reasonClass)
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(raw)}},
		"isError":           true,
		"structuredContent": value,
	}, nil
}

func (handler *toolHandler) toolErrorObject(name, code, reasonClass string) map[string]any {
	value := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "mutates": false, "profile": toolError, "tool": name,
	}
	if handler.reasonClass {
		value["profile"] = toolErrorReasonClass
		value["reasonClass"] = cmp.Or(reasonClass, unclassified)
	}
	return value
}

// structuredResult returns the structuredContent object, its full canonical
// JSON, and the text block payload: the MCPV0-033 summary when
// structuredContent is emitted, and the full canonical JSON when the result
// travels in the envelope alone.
func structuredResult(result bridge.Result, envelopeOnly bool) (map[string]any, string, string, error) {
	structured, objectErr := result.Object()
	if objectErr != nil {
		return nil, "", "", objectErr
	}
	full, canonicalErr := result.CanonicalJSON()
	if canonicalErr != nil {
		return nil, "", "", canonicalErr
	}
	if envelopeOnly {
		return structured, string(full), string(full), nil
	}
	text, textErr := result.TextJSON()
	if textErr != nil {
		return nil, "", "", textErr
	}
	return structured, string(full), string(text), nil
}

func onlyKeys(params map[string]any, allowed ...string) bool {
	if params == nil {
		return false
	}
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range params {
		if _, exists := set[key]; !exists {
			return false
		}
	}
	return true
}
