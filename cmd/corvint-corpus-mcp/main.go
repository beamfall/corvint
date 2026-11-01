package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/procprofile"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const (
	serverName    = "corvint-corpus-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = toolgroup.CorpusErrorProfile
	callDeadline  = toolgroup.CallDeadline
	timingMeta    = toolgroup.TimingMeta
)

// usage is the --help text (MMR-V0-016). The unified corvint-mcp server serves
// the artifact's corpus tools; this binary stays for one release.
const usage = "usage: corvint-corpus-mcp --root ROOT --artifact ARTIFACT [--map FILE]... [--consolidation] [--legacy-cases]\n" +
	"           [--outcome-log FILE] [--protocol-version 2026-07-28|2025-11-25]\n" +
	"       corvint-corpus-mcp --outcome-summary LOG [--floor FLOOR]\n" +
	"       corvint-corpus-mcp --version\n" +
	"A local read-only stdio MCP server for the documentation corpus ARTIFACT of the Git worktree at ROOT.\n" +
	"Deprecated alias for the corpus tools, kept for one release:\n" +
	"corvint-mcp --root ROOT --tool-profile corpus --corpus-artifact ARTIFACT serves the same corpus tools\n" +
	"with the same results; --map, --consolidation, --legacy-cases and --outcome-log stay here."

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(runProfiled(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv))
}

// runProfiled is run under the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE bracket (PRB-V0-010):
// one profile per server process, written when the server exits and on SIGINT or SIGTERM.
func runProfiled(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer, lookup runtimeenv.Lookup) int {
	return procprofile.Run(ctx, lookup, procprofile.Reporter(stderr, "corvint-corpus-mcp"), func() int {
		return run(ctx, arguments, stdin, stdout, stderr)
	})
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h") { // MMR-V0-016
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	}
	if len(arguments) > 0 && arguments[0] == "--outcome-summary" {
		return runOutcomeSummary(arguments, stdout, stderr) // DCP-V1-286
	}
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	arguments, legacyCases, legacyOK := extractLegacyCasesFlag(arguments)
	arguments, outcomePath, outcomeOK := extractOutcomeLogFlag(arguments)
	root, artifact, maps, consolidation, versionOnly, ok := parseArguments(arguments)
	if !ok || !protocolOK || !legacyOK || !outcomeOK || versionOnly && legacyCases {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: invalid arguments")
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	artifact, artifactErr := rootRelativePath(root, artifact)
	if artifactErr != "" {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --artifact: "+artifactErr)
		return 2
	}
	// DCP-V1-242: a relative --artifact that breaks the local input path rule is named with the rule,
	// not the generic startup refusal; the bounded reader would refuse the same path.
	if rule := doccorpus.LocalInputPathRule(artifact); rule != "" {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --artifact: local input paths must be canonical and relative to --root ("+rule+")")
		return 2
	}
	for i, name := range maps { // V1-1998: --map takes the same absolute spellings
		var mapErr string
		if maps[i], mapErr = rootRelativePath(root, name); mapErr != "" {
			_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --map: "+mapErr)
			return 2
		}
	}
	registry, registryErr := corpusbridge.New(root, artifact)
	if registryErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: repository unavailable")
		return 2
	}
	planner, plannerErr := newMapPlanner(root, maps)
	if plannerErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: application map unavailable")
		return 2
	}
	handler := &toolHandler{registry: registry, planner: planner}
	var err error
	if consolidation {
		absolute, err := filepath.Abs(root)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: repository unavailable")
			return 2
		}
		handler.consolidator = &consolidator{root: absolute}
	}
	if legacyCases {
		handler.legacyCases = &legacyCaseSearcher{root: root}
	}
	if outcomePath != "" { // DCP-V1-285: opt-in, off by default
		listed := []string{}
		for _, tool := range handler.listedTools() {
			listed = append(listed, tool.Name)
		}
		handler.outcomes, err = openOutcomeLog(outcomePath, registry.ArtifactSHA256(), listed, stderr)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --outcome-log: "+err.Error())
			return 2
		}
	}
	instance, err := server.New(server.Config{
		ProtocolVersion: protocolVersion,
		Name:            serverName, Version: serverVersion,
		Description:  "Local read-only Corvint experimental documentation corpus server.",
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
		// DCP-V1-652: a 2025-11-25 initialize learns the flag it needs.
		NameLegacyFlag: true,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: transport failed")
		return 2
	}
	return 0
}

// parseArguments accepts --version, or --root R --artifact A followed by up to eight --map FILE
// pairs naming root-relative application maps for corvint.map_plan (AMSP-V0-010) and at most one
// --consolidation, which lists corvint.consolidate_tests (TCN-V0-012). run removes one
// --legacy-cases first, which lists corvint.docs_search_cases (DCP-V1-077).
func parseArguments(arguments []string) (root, artifact string, maps []string, consolidation, versionOnly, ok bool) {
	if len(arguments) == 1 && arguments[0] == "--version" {
		return "", "", nil, false, true, true
	}
	if len(arguments) < 4 || arguments[0] != "--root" || arguments[1] == "" || arguments[2] != "--artifact" || arguments[3] == "" {
		return "", "", nil, false, false, false
	}
	for i := 4; i < len(arguments); i++ {
		switch {
		case arguments[i] == "--consolidation" && !consolidation:
			consolidation = true
		case arguments[i] == "--map" && i+1 < len(arguments) && arguments[i+1] != "":
			maps = append(maps, arguments[i+1])
			i++
		default:
			return "", "", nil, false, false, false
		}
	}
	if len(maps) > maxConfiguredMaps {
		return "", "", nil, false, false, false
	}
	return arguments[1], arguments[3], maps, consolidation, false, true
}

// rootRelativePath is toolgroup.RootRelativePath (DCP-V1-205 to DCP-V1-207).
func rootRelativePath(root, artifact string) (string, string) {
	return toolgroup.RootRelativePath(root, artifact)
}

type toolHandler struct {
	registry     *corpusbridge.Registry
	planner      *mapPlanner
	consolidator *consolidator
	legacyCases  *legacyCaseSearcher
	outcomes     *outcomeLog   // nil unless --outcome-log (DCP-V1-285)
	deadline     time.Duration // zero means callDeadline
}

func (handler *toolHandler) listedTools() []bridge.ToolDescriptor {
	tools := append(handler.registry.Tools(), bridge.HelpDescriptor()) // MCPV0-102
	if handler.planner != nil {
		tools = append(tools, handler.planner.descriptor(), handler.planner.knowledgeDescriptor())
	}
	if handler.consolidator != nil {
		tools = append(tools, handler.consolidator.descriptor())
	}
	if handler.legacyCases != nil {
		tools = append(tools, handler.legacyCases.descriptor())
	}
	sort.Slice(tools, func(left, right int) bool { return tools[left].Name < tools[right].Name })
	return bridge.WithOutputSchemas(tools) // MCPV0-953
}

func (handler *toolHandler) Handle(ctx context.Context, request protocol.Request, _ server.Notifier) (map[string]any, *protocol.RPCError) {
	switch request.Method {
	case "tools/list":
		if refusal := toolgroup.ListParamsRefusal(request.Params); refusal != nil { // MCPV0-952
			return nil, refusal
		}
		tools := handler.listedTools()
		return map[string]any{
			"cacheScope": "private", "tools": tools, "ttlMs": 300_000,
		}, nil
	case "tools/call":
		if handler.outcomes == nil {
			result, rpcErr := handler.call(ctx, request.Params)
			return result, toolgroup.WithInvalidParamsData(handler.listedTools, request.Params, rpcErr) // MCPV0-952
		}
		started := time.Now()
		result, rpcErr := handler.call(ctx, request.Params)
		rpcErr = toolgroup.WithInvalidParamsData(handler.listedTools, request.Params, rpcErr)
		name, _ := request.Params["name"].(string)
		handler.outcomes.record(name, classifyOutcome(result, rpcErr), time.Since(started))
		return result, rpcErr
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
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if name == bridge.ToolHelp { // MCPV0-102: reads only the binary, so no repository or corpus check applies.
		return helpResult(ctx, name, raw)
	}
	if name == mapPlanTool && handler.planner != nil {
		return handler.callMapTool(ctx, mapPlanTool, handler.planner.call, raw)
	}
	if name == mapKnowledgeTool && handler.planner != nil {
		return handler.callMapTool(ctx, mapKnowledgeTool, handler.planner.knowledge, raw)
	}
	if name == consolidateTool && handler.consolidator != nil {
		return handler.callConsolidate(ctx, raw)
	}
	if name == legacyCasesTool && handler.legacyCases != nil {
		return handler.callLegacyCases(ctx, raw)
	}
	return toolgroup.Corpus{Registry: handler.registry, Deadline: handler.deadline}.Call(ctx, name, arguments)
}

// callMapTool frames a map tool's report (the plan, or a site-knowledge projection) like every
// other tool result: the model-facing text in the untrusted-data envelope, the report itself as
// structured content.
func (handler *toolHandler) callMapTool(ctx context.Context, name string,
	run func(context.Context, []byte) (string, string, string, bool), raw []byte) (map[string]any, *protocol.RPCError) {
	text, code, message, valid := run(ctx, raw)
	if !valid {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	if code != "" {
		return toolFailureResult(name, code, message)
	}
	if text == "" {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	var structured map[string]any
	if json.Unmarshal([]byte(text), &structured) != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return toolFailureResult(name, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

// helpResult answers corvint.help through the shared bridge path (MCPV0-104).
func helpResult(ctx context.Context, name string, raw []byte) (map[string]any, *protocol.RPCError) {
	return toolgroup.Help(ctx, name, raw, toolFailureResult)
}

// toolFailureResult is toolgroup.CorpusFailure: the closed tool-error object
// with an all-unknown trust envelope; it always sets isError: true.
func toolFailureResult(name, code, message string) (map[string]any, *protocol.RPCError) {
	return toolgroup.CorpusFailure(name, code, message)
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
