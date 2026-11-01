// Package bridge exposes the currently qualified Corvint read surfaces without
// coupling them to an MCP transport implementation.
package bridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/contextsummary"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/lspevidence"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/plansnapshot"
	"github.com/Beamfall/corvint/internal/projectprofile"
	"github.com/Beamfall/corvint/internal/rootalias"
)

const (
	ToolQuery  = "corvint.query"
	ToolImpact = "corvint.impact"
	ToolStatus = "corvint.status"
	// ToolContext and ToolCEMReport project `corvint context` and `corvint cem
	// report` (MCPV0-024, MCPV0-025). Only the opt-in task-review descendant
	// profile advertises them (MCPV0-026, decision 0374).
	ToolContext   = "corvint.context"
	ToolCEMReport = "corvint.cem.report"

	resultSchema    = "corvint-mcp-bridge-result/0"
	maxArgumentSize = 512 * 1024
	maxResultBytes  = 384 * 1024
	maxQueryRunes   = 2_000
	maxImpactPaths  = 100
	maxPathRunes    = 1_024
	maxImpactLimit  = 50
	maxMCPLineBytes = 1 << 20
	mcpFrameReserve = 8 * 1024

	maxContextTaskBytes = 32_000
	maxContextLimit     = 50
	defaultContextLimit = 20
)

type ToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema map[string]any  `json:"inputSchema"`
	Annotations ToolAnnotations `json:"annotations"`
}

type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

type RepositoryBinding struct {
	CommitRevision   string `json:"commitRevision"`
	DirtyPathCount   int    `json:"dirtyPathCount"`
	DirtyPathsSHA256 string `json:"dirtyPathsSha256"`
	ObjectFormat     string `json:"objectFormat"`
	ProfileID        string `json:"profileId"`
	TreeRevision     string `json:"treeRevision"`
	WorktreeState    string `json:"worktreeState"`
}

type repositoryOperations struct {
	build      func(context.Context, string) (*contextindex.Index, error)
	buildQuery func(context.Context, string, string) (*contextindex.Index, error)
	context    func(context.Context, string, contextInput) (*contextindex.Index, map[string]any, error)
	// cemReport receives the resolved root and the root as the caller named
	// it; only a cem/1.0 read admits the latter (MCPV0-036, CEM-V1-017).
	cemReport func(ctx context.Context, root, callerRoot string, options workflow.ReadOptions) (map[string]any, error)
	platform  string
	probe     func(context.Context, string) (gokernel.Repository, error)
}

func productionRepositoryOperations() repositoryOperations {
	return repositoryOperations{
		build: contextindex.Build, buildQuery: contextindex.BuildQuery,
		context: compileContext, cemReport: previewCEMReport,
		platform: runtime.GOOS, probe: gokernel.ProbeRepositoryContext,
	}
}

// compileContext is `corvint context` without its operator-only members: the
// snapshot read, the observed build on a miss, and the admitted slot weights,
// retried through the eager loader when a deferred body is refused. The
// gopls attachment is omitted because it would spawn a second executable.
func compileContext(ctx context.Context, root string, input contextInput) (*contextindex.Index, map[string]any, error) {
	index, packet, err := compileContextWith(ctx, root, input, contextindex.LoadContextSnapshotDeferred)
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		if index != nil {
			index.Release()
		}
		return compileContextWith(ctx, root, input, contextindex.LoadContextSnapshot)
	}
	return index, packet, err
}

func compileContextWith(ctx context.Context, root string, input contextInput, load func(context.Context, string) (*contextindex.Index, bool, *contextindex.LoaderObservation, error)) (*contextindex.Index, map[string]any, error) {
	index, hit, opening, err := load(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	if !hit {
		index, err = contextindex.BuildContextObserved(ctx, root, input.Subject, opening)
	}
	if err != nil {
		return nil, nil, err
	}
	admitted, err := contextindex.LoadAdmittedSlotWeights(root)
	if err != nil {
		// The caller releases only an index it receives, so a refused
		// slot-weights file must not keep the mapping leased (MCPV0-090).
		index.Release()
		return nil, nil, err
	}
	packet, err := contextindex.TaskContextWeighted(ctx, index, input.Task, input.Subject, input.Limit, admitted)
	return index, packet, err
}

// previewCEMReport is `corvint cem report` rendered without publication. A
// 0.x map reads the resolved root; a cem/1.0 map is admitted against
// callerRoot, so a symlinked --root is refused exactly as
// `cem verify-stable --repository` refuses it (MCPV0-036, CEM-V1-017).
func previewCEMReport(ctx context.Context, root, callerRoot string, options workflow.ReadOptions) (map[string]any, error) {
	session, err := workflow.Open(root)
	if err != nil {
		return nil, err
	}
	session.SetStableRepository(callerRoot)
	return session.Read(ctx, workflow.ActionReportPreview, options)
}

// repositorySnapshot is one immutable context-index capture for a read
// operation. Its receipt and binding must describe the same stable revision.
type repositorySnapshot struct {
	index   *contextindex.Index
	binding RepositoryBinding
}

type Abstention struct {
	Active bool   `json:"active"`
	Reason string `json:"reason"`
}

type Result struct {
	Schema         string             `json:"schema"`
	Tool           string             `json:"tool"`
	Mutates        bool               `json:"mutates"`
	State          string             `json:"state"`
	EpistemicClass string             `json:"epistemicClass"`
	AuthorityClass string             `json:"authorityClass"`
	Repository     *RepositoryBinding `json:"repository"`
	Receipt        map[string]any     `json:"receipt"`
	Abstention     Abstention         `json:"abstention"`
}

// Object returns the closed representation consumed by the MCP handler.
// Receipt is passed through without flattening or reinterpreting its fields.
func (result Result) Object() (map[string]any, *Error) {
	if !validResult(result) {
		return nil, failure("internal-error")
	}
	var repository any
	if result.Repository != nil {
		repository = map[string]any{
			"commitRevision": result.Repository.CommitRevision, "treeRevision": result.Repository.TreeRevision,
			"objectFormat": result.Repository.ObjectFormat, "profileId": result.Repository.ProfileID,
			"worktreeState": result.Repository.WorktreeState, "dirtyPathCount": result.Repository.DirtyPathCount,
			"dirtyPathsSha256": result.Repository.DirtyPathsSHA256,
		}
	}
	return map[string]any{
		"schema": result.Schema, "tool": result.Tool, "mutates": result.Mutates,
		"state": result.State, "epistemicClass": result.EpistemicClass,
		"authorityClass": result.AuthorityClass, "repository": repository,
		"receipt":    result.Receipt,
		"abstention": map[string]any{"active": result.Abstention.Active, "reason": result.Abstention.Reason},
	}, nil
}

// CanonicalJSON returns deterministic compact bytes for the MCP text content.
func (result Result) CanonicalJSON() ([]byte, *Error) {
	object, err := result.Object()
	if err != nil {
		return nil, err
	}
	encoded, encodeErr := gokernel.CanonicalJSON(object)
	if encodeErr != nil {
		return nil, failure("internal-error")
	}
	return encoded, nil
}

// TextSummaryProfile names the MCP text block that summarizes a receipt whose
// full rows travel in structuredContent (MCPV0-033, proposed).
const TextSummaryProfile = "corvint-mcp-text-summary/0"

// TextJSON returns the canonical bytes for the MCP text content block. A
// receipt carrying a results list is summarized: each row keeps only kind, id
// and score, and the object names TextSummaryProfile, while every other member,
// coverage and uncertainty included, is the structuredContent value unchanged.
// Any other result is the full canonical object, as CanonicalJSON returns.
func (result Result) TextJSON() ([]byte, *Error) {
	object, err := result.Object()
	if err != nil {
		return nil, err
	}
	if rows, ok := receiptRows(result.Receipt["results"]); ok {
		receipt := make(map[string]any, len(result.Receipt))
		for key, value := range result.Receipt {
			receipt[key] = value
		}
		summary := make([]any, len(rows))
		for position, row := range rows {
			projected := make(map[string]any, 3)
			for _, key := range []string{"kind", "id", "score"} {
				if value, present := row[key]; present {
					projected[key] = value
				}
			}
			summary[position] = projected
		}
		receipt["results"] = summary
		object["receipt"] = receipt
		object["textProfile"] = TextSummaryProfile
	}
	encoded, encodeErr := gokernel.CanonicalJSON(object)
	if encodeErr != nil {
		return nil, failure("internal-error")
	}
	return encoded, nil
}

func receiptRows(value any) ([]map[string]any, bool) {
	switch typed := value.(type) {
	case []map[string]any:
		return typed, true
	case []any:
		rows := make([]map[string]any, len(typed))
		for position, raw := range typed {
			rows[position], _ = raw.(map[string]any)
		}
		return rows, true
	}
	return nil, false
}

// Error is a closed, sanitized failure. It never contains repository paths,
// tool output, or underlying process errors. ReasonClass is the closed status
// refusal class of decision 0383, empty when none is known.
type Error struct{ Code, ReasonClass string }

func (failure *Error) Error() string { return failure.Code }

// taskReviewTools exist only in the task-review descendant profile; the
// default V0 registry neither advertises nor dispatches them (decision 0103).
var taskReviewTools = map[string]bool{ToolContext: true, ToolCEMReport: true}

// Registry binds every call to one canonical local repository root.
type Registry struct {
	root          string
	callerRoot    string
	rootIdentity  os.FileInfo
	git           *rootbind.Binding
	operations    repositoryOperations
	taskReview    bool
	lsp           bool
	lspExecutable string
	importGraph   bool
	flows         bool
}

// New binds the default V0 registry: exactly query, impact, and status.
func New(root string) (*Registry, *Error) {
	if !validRoot(root) {
		return nil, failure("invalid-root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
		return nil, failure("invalid-root")
	}
	rootIdentity, err := os.Stat(resolved)
	if err != nil || !rootIdentity.IsDir() {
		return nil, failure("invalid-root")
	}
	git, ok := rootbind.Bind(resolved)
	if !ok {
		return nil, failure("invalid-root")
	}
	registry := &Registry{root: resolved, callerRoot: root, rootIdentity: rootIdentity, git: git, operations: productionRepositoryOperations()}
	return registry, nil
}

// NewTaskReview binds the opt-in task-review descendant profile, which adds
// the context and CEM report tools to the V0 three (MCPV0-026).
func NewTaskReview(root string) (*Registry, *Error) {
	registry, err := New(root)
	if err != nil {
		return nil, err
	}
	registry.taskReview = true
	return registry, nil
}

// NewTaskReviewLSP authorizes the optional executable at startup. Individual
// calls must still request lsp:gopls; legacy environment knobs have no effect.
func NewTaskReviewLSP(root string) (*Registry, *Error) {
	registry, err := NewTaskReview(root)
	if err != nil {
		return nil, err
	}
	registry.lsp = true
	registry.lspExecutable = lspevidence.Executable()
	return registry, nil
}

// Profile names one registry profile's tool surface without a root.
type Profile struct{ TaskReview, LSP, Flows, ImportGraph bool }

// Tools lists what the profile's registry advertises, for a server that
// starts with no usable root (MCPV0-950): it lists the tools it would serve and
// refuses every repository call.
func (profile Profile) Tools() []ToolDescriptor {
	return (&Registry{taskReview: profile.TaskReview || profile.LSP, lsp: profile.LSP, flows: profile.Flows, importGraph: profile.ImportGraph}).Tools()
}

func (registry *Registry) advertises(name string) bool {
	if name == ToolGraphImports {
		return registry.importGraph
	}
	if taskReviewTools[name] {
		return registry.taskReview
	}
	if flowsTools[name] {
		return registry.flows
	}
	return true
}

// Tools is the exact delivered surface of the registry's profile. Standalone
// evidence and dashboard snapshot tools are omitted; query and impact receipts
// carry their evidence.
func (registry *Registry) Tools() []ToolDescriptor {
	if registry == nil {
		return nil
	}
	all := registry.allTools()
	advertised := make([]ToolDescriptor, 0, len(all))
	for _, tool := range all {
		if registry.advertises(tool.Name) {
			if registry.lsp && tool.Name == ToolContext {
				tool.InputSchema["properties"].(map[string]any)["lsp"] = map[string]any{"type": "string", "enum": []any{"off", "gopls"}, "default": "off"}
				tool.Description += " Optional lsp:gopls starts the operator-selected local Go language server for bounded definition/reference evidence; requires installed gopls and cached dependencies, may write Go caches, and does not change rankings."
			}
			advertised = append(advertised, tool)
		}
	}
	return advertised
}

func (registry *Registry) allTools() []ToolDescriptor {
	// Byte bounds are advertised as code-point bounds divided by UTFMax, and
	// the task pattern excludes U+0085 (Go trims it, ECMA \s does not), so
	// every schema-valid argument is also runtime-valid.
	oid := map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"}
	ceiling := map[string]any{"type": "integer", "minimum": 0}
	// The map and the cem/1.0 artifacts directory share one closed
	// repository-relative path rule (MCPV0-025, MCPV0-036).
	relativePath := func() map[string]any {
		return map[string]any{
			"type": "string", "minLength": 1, "maxLength": wire.MaxPathBytes / utf8.UTFMax,
			"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\)(?!(?:.*/)?[.][Gg][Ii][Tt](?:/|$))[^\x00-\x1f\x7f]+$`,
		}
	}
	return append([]ToolDescriptor{
		{
			Name:        ToolCEMReport,
			Description: "Render the CEM reviewer report for a repository-relative canonical map against two full revision IDs, without writing it.",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"map": relativePath(), "artifacts": relativePath(),
				"expectedBase": oid, "target": oid,
				"maxUnknown": ceiling, "maxMechanical": ceiling,
			}, []any{"map", "expectedBase", "target"}),
		},
		{
			Name:        ToolContext,
			Description: "Compile the task-context packet: the files to read for one task, each with the relation that admitted it.",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"task": map[string]any{"type": "string", "minLength": 1, "maxLength": maxContextTaskBytes / utf8.UTFMax, "pattern": `[^\s\x85]`},
				"subject": map[string]any{
					"type": "string", "minLength": 1, "maxLength": maxPathRunes,
					"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\).+$`,
				},
				"limit":        map[string]any{"type": "integer", "minimum": 1, "maximum": maxContextLimit, "default": defaultContextLimit},
				"summaryBytes": map[string]any{"type": "integer", "minimum": contextsummary.MinBytes, "maximum": contextsummary.MaxBytes},
			}, []any{"task"}),
		},
		helpDescriptor(),
		{
			Name:        ToolImpact,
			Description: "Compile revision-bound impact evidence without source bodies. Capabilities: Go (.go): path, package imports; Ruby (.rb): path, markers, advisory autoload constant references, reverse imports unknown; JavaScript (.js/.jsx/.mjs/.cjs) and TypeScript (.ts/.tsx): path, literal relative/profile/tsconfig-alias imports, unresolved bare imports unknown. Non-Go paths name experimental non-go-syntax-v0. Dynamic dispatch and test closure remain unknown. The receipt equals CLI corvint impact with the syntax profile for the same paths, revision and limit; the CLI admits more suffixes and --base (corvint impact --capabilities). MCP accepts paths only.",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"snapshot": snapshotSchema(),
				"paths": map[string]any{
					"type": "array", "minItems": 1, "maxItems": maxImpactPaths, "uniqueItems": true,
					"items": map[string]any{
						"type": "string", "minLength": 1, "maxLength": maxPathRunes,
						"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\).+[.](?:go|rb|js|jsx|mjs|cjs|ts|tsx)$`,
					},
				},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": maxImpactLimit, "default": 10},
				"budgetBytes": map[string]any{"type": "integer", "minimum": contextindex.MinPacketBytes, "maximum": contextindex.MaxPacketBytes},
			}, []any{"paths"}),
		},
		{
			Name:        ToolQuery,
			Description: "Compile the narrow project-operations authority-start receipt (limit is fixed at one).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"snapshot": snapshotSchema(),
				"task":     map[string]any{"type": "string", "minLength": 1, "maxLength": maxQueryRunes, "pattern": `^[ -~]*[!-~][ -~]*$`},
			}, []any{"task"}),
		},
		{
			Name:        ToolStatus,
			Description: "Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest. Gitlinks bind commit IDs; nested submodule contents are outside coverage.",
			InputSchema: objectSchema(map[string]any{}, []any{}),
			Annotations: readAnnotations(),
		},
		importGraphDescriptor(),
	}, flowsToolDescriptors()...)
}

func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (Result, *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.git == nil ||
		registry.operations.build == nil || registry.operations.buildQuery == nil || registry.operations.probe == nil ||
		registry.operations.context == nil || registry.operations.cemReport == nil {
		return Result{}, failure("invalid-registry")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, failure("cancelled")
	}
	if len(arguments) == 0 || len(arguments) > maxArgumentSize || !utf8.Valid(arguments) {
		return Result{}, failure("invalid-arguments")
	}
	if !registry.advertises(name) {
		return Result{}, failure("unsupported-tool")
	}
	if name == ToolHelp {
		return callHelp(ctx, arguments) // MCPV0-100: binary-only, no root identity check.
	}
	ctx = gitstatus.WithIsolation(ctx)
	var result Result
	var callErr *Error
	switch name {
	case ToolQuery:
		result, callErr = registry.callQuery(ctx, arguments)
	case ToolGraphImports:
		result, callErr = registry.callGraphImports(ctx, arguments)
	case ToolImpact:
		result, callErr = registry.callImpact(ctx, arguments)
	case ToolStatus:
		result, callErr = registry.callStatus(ctx, arguments)
	case ToolContext:
		result, callErr = registry.callContext(ctx, arguments)
	case ToolCEMReport:
		result, callErr = registry.callCEMReport(ctx, arguments)
	case ToolFlowsMap:
		result, callErr = registry.callFlowsMap(ctx, arguments)
	case ToolFlowsGaps:
		result, callErr = registry.callFlowsGaps(ctx, arguments)
	case ToolFlowsImpact:
		result, callErr = registry.callFlowsImpact(ctx, arguments)
	case ToolFlowsCoverage:
		result, callErr = registry.callFlowsCoverage(ctx, arguments)
	case ToolFlowsNavigate:
		result, callErr = registry.callFlowsNavigate(ctx, arguments)
	default:
		return Result{}, failure("unsupported-tool")
	}
	if callErr != nil && (callErr.Code == "invalid-arguments" || callErr.Code == "cancelled") {
		return Result{}, callErr
	}
	// MCPV0-034: a request whose context ended before its result is returned
	// is cancelled, whatever the operation produced first.
	if ctx.Err() != nil {
		return Result{}, failure("cancelled")
	}
	if !registry.sameRoot() {
		return abstained(name, "ROOT_IDENTITY_CHANGED", nil), nil
	}
	return result, callErr
}

func (registry *Registry) sameRoot() bool {
	current, err := os.Stat(registry.root)
	if err != nil || !current.IsDir() || !os.SameFile(registry.rootIdentity, current) {
		return false
	}
	return registry.git.Same(registry.root)
}

type queryInput struct {
	Snapshot jsonv1.RawMessage `json:"snapshot"`
	Task     string            `json:"task"`
}

func (registry *Registry) callQuery(ctx context.Context, arguments []byte) (Result, *Error) {
	var input queryInput
	if err := decodeClosed(arguments, &input); err != nil || !validTask(input.Task) {
		return Result{}, failure("invalid-arguments")
	}
	if !nativePlatformQualified(registry.operations.platform) {
		return abstained(ToolQuery, "UNSUPPORTED_PLATFORM", nil), nil
	}
	if err := contextindex.ValidateQueryAuthorityStart(input.Task, 1); err != nil {
		if isUnsupported(err, "unsupported-query-") {
			return abstained(ToolQuery, abstentionReason(err), nil), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	snapshot, scope, bridgeErr := registry.planningSnapshot(ctx, input.Snapshot, input.Task)
	if bridgeErr != nil {
		if repositoryDrift(bridgeErr) {
			return abstained(ToolQuery, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, bridgeErr
	}
	var receipt map[string]any
	var err error
	if scope != nil {
		receipt, err = contextindex.QuerySnapshotAuthority(snapshot.index, input.Task)
	} else {
		receipt, err = contextindex.QueryAuthorityStart(ctx, snapshot.index, input.Task, 1)
	}
	if err != nil {
		if isUnsupported(err, "unsupported-query-") {
			return abstained(ToolQuery, abstentionReason(err), &snapshot.binding), nil
		}
		if repositoryDrift(err) {
			return abstained(ToolQuery, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	return registry.boundedPlanning(ctx, ToolQuery, snapshot, scope, receipt)
}

type impactInput struct {
	Snapshot jsonv1.RawMessage `json:"snapshot"`
	Paths    []string          `json:"paths"`
	Limit    int               `json:"limit"`
	// BudgetBytes is CLI path impact's --budget-bytes (MCPV0-110): set, the
	// receipt is budget-compiled exactly as the CLI compiles it; omitted, the
	// unbudgeted wire is unchanged.
	BudgetBytes jsonv1.RawMessage `json:"budgetBytes,omitempty"`
}

func (registry *Registry) callImpact(ctx context.Context, arguments []byte) (Result, *Error) {
	// JSON v2 preserves an omitted member but zeroes explicit null, which the
	// integer range below rejects before any repository operation.
	input := impactInput{Limit: 10}
	if err := decodeClosed(arguments, &input); err != nil || !validImpact(input) {
		return Result{}, failure("invalid-arguments")
	}
	if !nativePlatformQualified(registry.operations.platform) {
		return abstained(ToolImpact, "UNSUPPORTED_PLATFORM", nil), nil
	}
	snapshot, scope, bridgeErr := registry.planningSnapshot(ctx, input.Snapshot, "")
	if bridgeErr != nil {
		if repositoryDrift(bridgeErr) {
			return abstained(ToolImpact, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, bridgeErr
	}
	budget, _ := optionalBytes(input.BudgetBytes, contextindex.MinPacketBytes, contextindex.MaxPacketBytes)
	receipt, err := contextindex.PathImpactSyntax(snapshot.index, input.Paths, input.Limit, budget)
	if err != nil {
		if isUnsupported(err, "unsupported-impact-") {
			return abstained(ToolImpact, abstentionReason(err), &snapshot.binding), nil
		}
		if strings.HasPrefix(err.Error(), "impact paths are not tracked at revision ") {
			return abstained(ToolImpact, "NOT_TRACKED_AT_REVISION", &snapshot.binding), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	return registry.boundedPlanning(ctx, ToolImpact, snapshot, scope, receipt)
}

type contextInput struct {
	LSP     jsonv1.RawMessage `json:"lsp,omitempty"`
	Task    string            `json:"task"`
	Subject string            `json:"subject"`
	Limit   int               `json:"limit"`
	// SummaryBytes is `corvint context --summary-bytes` (MCPV0-111): set, the
	// result is the evidence summary of the canonical receipt, not the receipt.
	SummaryBytes jsonv1.RawMessage `json:"summaryBytes,omitempty"`
}

func (registry *Registry) callContext(ctx context.Context, arguments []byte) (Result, *Error) {
	input := contextInput{Limit: defaultContextLimit}
	if err := decodeClosed(arguments, &input); err != nil || !validContext(input) {
		return Result{}, failure("invalid-arguments")
	}
	selection := "off"
	if input.LSP != nil {
		if !registry.lsp || jsonv1.Unmarshal(input.LSP, &selection) != nil || string(input.LSP) == "null" || (selection != "off" && selection != "gopls") {
			return Result{}, failure("invalid-arguments")
		}
	}
	if !nativePlatformQualified(registry.operations.platform) {
		return abstained(ToolContext, "UNSUPPORTED_PLATFORM", nil), nil
	}
	index, packet, err := registry.operations.context(ctx, registry.root, input)
	if index != nil {
		// A mapped snapshot read leases its mapping until released; the
		// result is detached from it below, so the lease ends with the call
		// and a long-lived bridge does not pin every mapping it read
		// (MCPV0-090, V1-1004).
		defer index.Release()
	}
	if err != nil && index == nil {
		if repositoryDrift(err) {
			return abstained(ToolContext, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	snapshot, bridgeErr := registry.snapshotIndex(ctx, index, nil)
	if bridgeErr != nil {
		return Result{}, bridgeErr
	}
	if err != nil {
		if strings.HasPrefix(err.Error(), "subject path is not tracked at revision ") {
			return abstained(ToolContext, "NOT_TRACKED_AT_REVISION", &snapshot.binding), nil
		}
		if repositoryDrift(err) {
			return abstained(ToolContext, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	if selection == "gopls" {
		if err := lspevidence.Attach(ctx, index, input.Subject, packet, input.Limit, registry.lspExecutable); err != nil {
			if errors.Is(err, lspevidence.ErrRepositoryChanged) || repositoryDrift(err) {
				return abstained(ToolContext, "REPOSITORY_STATE_UNSTABLE", nil), nil
			}
			return Result{}, normalizeFailure(ctx, err)
		}
	}
	receipt, binding, detachErr := detachedContext(packet, snapshot.binding)
	if detachErr != nil {
		return Result{}, failure("internal-error")
	}
	if budget, _ := optionalBytes(input.SummaryBytes, contextsummary.MinBytes, contextsummary.MaxBytes); budget != nil {
		summary, err := summarizedContext(receipt, *budget)
		if err != nil {
			if code := errorCode(err); code == "summary-budget" || code == "summary-shape" {
				return abstained(ToolContext, abstentionReason(err), &binding), nil
			}
			return Result{}, failure("internal-error")
		}
		receipt = summary
	}
	return boundedObserved(ToolContext, binding, receipt)
}

// summarizedContext projects a detached context receipt through the CLI's
// summary (MCPV0-111). The input is the canonical receipt plus LF, the bytes
// `corvint context` writes, so packet_sha256 and packet_bytes name the same
// bytes a caller reads back from corvint.context without summaryBytes.
func summarizedContext(receipt map[string]any, budget int) (map[string]any, error) {
	encoded, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		return nil, err
	}
	raw, err := contextsummary.Summarize(append(encoded, '\n'), budget, contextsummary.MCPContinuation)
	if err != nil {
		return nil, err
	}
	decoder := jsonv1.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var summary map[string]any
	if err := decoder.Decode(&summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// detachedContext copies a context packet and its binding onto the heap so
// nothing the result returns aliases the snapshot mapping its Index leased
// (MCPV0-090, V1-1004). The packet is copied through its canonical bytes,
// decoding numbers as their literals, and the copy must encode to the same
// bytes; any difference refuses rather than changing the wire result.
func detachedContext(packet map[string]any, binding RepositoryBinding) (map[string]any, RepositoryBinding, error) {
	encoded, err := gokernel.CanonicalJSON(packet)
	if err != nil {
		return nil, RepositoryBinding{}, err
	}
	decoder := jsonv1.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var copied map[string]any
	if err := decoder.Decode(&copied); err != nil {
		return nil, RepositoryBinding{}, err
	}
	again, err := gokernel.CanonicalJSON(copied)
	if err != nil {
		return nil, RepositoryBinding{}, err
	}
	if !bytes.Equal(again, encoded) {
		return nil, RepositoryBinding{}, errors.New("context packet does not survive its canonical copy")
	}
	binding.CommitRevision, binding.TreeRevision = strings.Clone(binding.CommitRevision), strings.Clone(binding.TreeRevision)
	binding.ObjectFormat, binding.ProfileID = strings.Clone(binding.ObjectFormat), strings.Clone(binding.ProfileID)
	return copied, binding, nil
}

type cemReportInput struct {
	Map           string  `json:"map"`
	Artifacts     *string `json:"artifacts"`
	ExpectedBase  string  `json:"expectedBase"`
	Target        string  `json:"target"`
	MaxUnknown    *int    `json:"maxUnknown"`
	MaxMechanical *int    `json:"maxMechanical"`
}

func (registry *Registry) callCEMReport(ctx context.Context, arguments []byte) (Result, *Error) {
	var input cemReportInput
	if err := decodeClosed(arguments, &input); err != nil || !validCEMReport(input) {
		return Result{}, failure("invalid-arguments")
	}
	before, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		return Result{}, normalizeFailure(ctx, err)
	}
	options := workflow.ReadOptions{
		MapPath: input.Map, ExpectedBase: input.ExpectedBase, Target: input.Target,
		Limits: workflow.PolicyLimits{MaxUnknown: input.MaxUnknown, MaxMechanical: input.MaxMechanical},
	}
	// MCPV0-036: the stable verifier needs an absolute artifact root; the
	// caller names it relative to the bound root.
	if input.Artifacts != nil {
		options.ArtifactRoot = filepath.Join(registry.callerRoot, filepath.FromSlash(*input.Artifacts))
		options.ArtifactsGiven = true
	}
	receipt, err := registry.operations.cemReport(ctx, registry.root, registry.callerRoot, options)
	if err != nil {
		return Result{}, cemFailure(ctx, err)
	}
	after, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		return Result{}, normalizeFailure(ctx, err)
	}
	if before != after {
		return abstained(ToolCEMReport, "REPOSITORY_STATE_UNSTABLE", nil), nil
	}
	return boundedObserved(ToolCEMReport, bindingFromRepository(after), receipt)
}

// validCEMPath is the closed repository-relative path rule shared by the map
// and the artifacts directory: no traversal and no .git segment in any case.
func validCEMPath(path string) bool {
	if wire.ValidatePath(path) != nil || !validRelativePath(path, wire.MaxPathBytes) {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

// cemFailures closes CEM's registered codes onto the bridge's sanitized
// failure set; an unlisted code is internal-error. A Git read that ran out of
// its own bound is the MCPV0-038 timeout code (decision 0533); one that
// exhausted its operation or output bound is the capacity code (V1-1992).
var cemFailures = map[string]string{
	cemcode.MapUnavailable: "cem-map-unavailable", cemcode.InvalidArguments: "cem-map-unsupported",
	cemcode.InvalidJSON: "cem-map-invalid", cemcode.NotAnObject: "cem-map-invalid",
	cemcode.MissingField: "cem-map-invalid", cemcode.UnknownField: "cem-map-invalid",
	cemcode.UnsupportedSpec: "cem-map-invalid", cemcode.InvalidField: "cem-map-invalid",
	cemcode.InvalidExcludedPath: "cem-map-invalid", cemcode.PathTraversal: "cem-map-invalid",
	cemcode.GitReadFailed: "repository-unavailable", cemcode.GitDiffFailed: "repository-unavailable",
	cemcode.GitDiffTimeout: "repository-probe-timeout", cemcode.GitStartFailed: "repository-unavailable",
	cemcode.GitExitFailure: "repository-unavailable", cemcode.GitTimeout: "repository-probe-timeout",
	cemcode.GitOutputExceeded: "corpus-capacity-exceeded", cemcode.GitBudgetExceeded: "corpus-capacity-exceeded",
	cemcode.RepositoryObjectUnavailable:     "repository-unavailable",
	cemcode.UnsupportedObjectAlternates:     "repository-unavailable",
	cemcode.UnsupportedRepositoryAttributes: "repository-unavailable",
}

func cemFailure(ctx context.Context, err error) *Error {
	code := cemcode.CodeOf(err)
	if ctx.Err() != nil || code == cemcode.GitCancelled {
		return failure("cancelled")
	}
	// MCPV0-037: a cem/1.0 map refused before its verifier ran is named
	// apart from a malformed or unsupported 0.x map.
	if workflow.IsStableRefusal(err) {
		return failure("cem-map-stable-refused")
	}
	if mapped, known := cemFailures[code]; known {
		return failure(mapped)
	}
	return failure("internal-error")
}

func nativePlatformQualified(platform string) bool {
	return platform == "darwin" || platform == "linux"
}

func (registry *Registry) callStatus(ctx context.Context, arguments []byte) (Result, *Error) {
	var input struct{}
	if err := decodeClosed(arguments, &input); err != nil {
		return Result{}, failure("invalid-arguments")
	}
	repository, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		if repositoryDrift(err) {
			return abstained(ToolStatus, "REPOSITORY_STATE_UNSTABLE", nil), nil
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	binding := bindingFromRepository(repository)
	return Result{
		Schema: resultSchema, Tool: ToolStatus, Mutates: false, State: "READY",
		EpistemicClass: "OBSERVED", AuthorityClass: "GIT_REPOSITORY",
		Repository: &binding, Receipt: nil, Abstention: Abstention{Active: false, Reason: "NONE"},
	}, nil
}

func (registry *Registry) snapshot(ctx context.Context) (repositorySnapshot, *Error) {
	index, err := registry.operations.build(ctx, registry.root)
	return registry.snapshotIndex(ctx, index, err)
}

func (registry *Registry) querySnapshot(ctx context.Context, task string) (repositorySnapshot, *Error) {
	index, err := registry.operations.buildQuery(ctx, registry.root, task)
	return registry.snapshotIndex(ctx, index, err)
}

func (registry *Registry) snapshotIndex(ctx context.Context, index *contextindex.Index, err error) (repositorySnapshot, *Error) {
	if err != nil {
		return repositorySnapshot{}, normalizeFailure(ctx, err)
	}
	dirtyBytes, err := gokernel.CanonicalJSON(index.DirtyPaths)
	if err != nil {
		return repositorySnapshot{}, failure("internal-error")
	}
	dirtyDigest := sha256.Sum256(dirtyBytes)
	state := "CLEAN"
	if len(index.DirtyPaths) != 0 {
		state = "MIXED"
	}
	return repositorySnapshot{
		index: index,
		binding: RepositoryBinding{
			CommitRevision: index.CommitRevision, DirtyPathCount: len(index.DirtyPaths),
			DirtyPathsSHA256: hex.EncodeToString(dirtyDigest[:]), ObjectFormat: index.ObjectFormat,
			ProfileID: index.ProfileID, TreeRevision: index.Revision, WorktreeState: state,
		},
	}, nil
}

func bindingFromRepository(repository gokernel.Repository) RepositoryBinding {
	state := "CLEAN"
	if repository.WorktreeState != "clean" {
		state = "MIXED"
	}
	return RepositoryBinding{
		CommitRevision: repository.CommitRevision, DirtyPathCount: repository.DirtyPathCount,
		DirtyPathsSHA256: repository.DirtyPathsSHA, ObjectFormat: repository.ObjectFormat,
		ProfileID: repository.ProfileID, TreeRevision: repository.TreeRevision, WorktreeState: state,
	}
}

func observed(tool string, binding RepositoryBinding, receipt map[string]any) Result {
	return Result{
		Schema: resultSchema, Tool: tool, Mutates: false, State: "READY",
		EpistemicClass: "OBSERVED", AuthorityClass: "REPOSITORY_EVIDENCE",
		Repository: &binding, Receipt: receipt, Abstention: Abstention{Active: false, Reason: "NONE"},
	}
}

func boundedObserved(tool string, binding RepositoryBinding, receipt map[string]any) (Result, *Error) {
	result := observed(tool, binding, receipt)
	if receipt["state"] == "OUT_OF_SCOPE" {
		result = abstained(tool, "OUT_OF_SCOPE", &binding)
		result.Receipt = receipt
	}
	encoded, err := result.CanonicalJSON()
	if err != nil {
		return Result{}, err
	}
	if len(encoded) > maxResultBytes || !fitsMCPFrame(encoded, result) {
		return abstained(tool, "OUTPUT_BUDGET_EXCEEDED", &binding), nil
	}
	return result, nil
}

func abstained(tool, reason string, binding *RepositoryBinding) Result {
	return Result{
		Schema: resultSchema, Tool: tool, Mutates: false, State: "ABSTAINED",
		EpistemicClass: "NOT_OBSERVED", AuthorityClass: "NONE",
		Repository: binding, Receipt: nil, Abstention: Abstention{Active: true, Reason: reason},
	}
}

func decodeClosed(arguments []byte, target any) error {
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return errors.New("arguments must be one JSON object")
	}
	return json.Unmarshal(arguments, target, json.RejectUnknownMembers(true))
}

func validTask(task string) bool {
	if strings.TrimSpace(task) == "" || utf8.RuneCountInString(task) > maxQueryRunes {
		return false
	}
	for _, character := range task {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}

func validImpact(input impactInput) bool {
	if len(input.Paths) == 0 || len(input.Paths) > maxImpactPaths {
		return false
	}
	if input.Limit < 1 || input.Limit > maxImpactLimit {
		return false
	}
	if _, ok := optionalBytes(input.BudgetBytes, contextindex.MinPacketBytes, contextindex.MaxPacketBytes); !ok {
		return false
	}
	seen := make(map[string]struct{}, len(input.Paths))
	for _, value := range input.Paths {
		if value == "" || utf8.RuneCountInString(value) > maxPathRunes || filepath.IsAbs(value) ||
			filepath.ToSlash(filepath.Clean(value)) != value || !contextindex.MCPImpactPathAdmitted(value) || strings.Contains(value, "\\") {
			return false
		}
		for _, part := range strings.Split(value, "/") {
			if part == "" || part == "." || part == ".." {
				return false
			}
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

// optionalBytes reads an optional byte-budget argument: omitted is nil and
// admitted, and a present member must be one integer in [low, high]; explicit
// null, a fraction or any other JSON value is refused.
func optionalBytes(raw jsonv1.RawMessage, low, high int) (*int, bool) {
	if raw == nil {
		return nil, true
	}
	var value int
	if string(raw) == "null" || json.Unmarshal(raw, &value) != nil || value < low || value > high {
		return nil, false
	}
	return &value, true
}

func validContext(input contextInput) bool {
	if strings.TrimSpace(input.Task) == "" || len(input.Task) > maxContextTaskBytes {
		return false
	}
	if input.Limit < 1 || input.Limit > maxContextLimit {
		return false
	}
	if _, ok := optionalBytes(input.SummaryBytes, contextsummary.MinBytes, contextsummary.MaxBytes); !ok {
		return false
	}
	return input.Subject == "" || validRelativePath(input.Subject, maxPathRunes)
}

// validCEMReport admits a repository-relative map path outside Git metadata
// and two full object IDs. A symlink at or above the map is refused by the
// CEM read itself (publish.Root.ReadBounded), as cem-map-unavailable.
func validCEMReport(input cemReportInput) bool {
	if !validCEMPath(input.Map) || (input.Artifacts != nil && !validCEMPath(*input.Artifacts)) {
		return false
	}
	if !fullObjectID(input.ExpectedBase) || !fullObjectID(input.Target) {
		return false
	}
	return nonNegative(input.MaxUnknown) && nonNegative(input.MaxMechanical)
}

func fullObjectID(value string) bool { return lowerHex(value, 40) || lowerHex(value, 64) }

func nonNegative(value *int) bool { return value == nil || *value >= 0 }

// validRelativePath is the impact path rule without the Go suffix.
func validRelativePath(value string, maxRunes int) bool {
	if value == "" || utf8.RuneCountInString(value) > maxRunes || filepath.IsAbs(value) ||
		filepath.ToSlash(filepath.Clean(value)) != value || strings.Contains(value, "\\") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// validRoot applies the MCPV0-001 root bounds, shared with multi-root and appmap roots.
func validRoot(root string) bool { return rootalias.ValidRoot(root) }

func objectSchema(properties map[string]any, required []any) map[string]any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object", "properties": properties, "required": required, "additionalProperties": false,
	}
}

func fitsMCPFrame(encoded []byte, result Result) bool {
	object, err := result.Object()
	if err != nil {
		return false
	}
	// This is the exact duplicated CallToolResult shape plus a conservative
	// reserve for JSON-RPC ID, server metadata, and transport decorations.
	frame, marshalErr := jsonv1.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": strings.Repeat("x", 256),
		"result": map[string]any{
			"content":           []any{map[string]any{"type": "text", "text": string(encoded)}},
			"structuredContent": object, "isError": false, "resultType": "complete",
		},
	})
	return marshalErr == nil && len(frame)+mcpFrameReserve <= maxMCPLineBytes
}

func readAnnotations() ToolAnnotations {
	return ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false}
}

func validResult(result Result) bool {
	if result.Schema != resultSchema || result.Mutates ||
		!knownTool(result.Tool) ||
		// Only the object-only graph read leaves the checkout unobserved.
		(result.Repository != nil && result.Repository.WorktreeState == "NOT_OBSERVED" && result.Tool != ToolGraphImports) ||
		(result.State != "READY" && result.State != "ABSTAINED") ||
		(result.EpistemicClass != "OBSERVED" && result.EpistemicClass != "NOT_OBSERVED") ||
		(result.AuthorityClass != "REPOSITORY_EVIDENCE" && result.AuthorityClass != "GIT_REPOSITORY" && result.AuthorityClass != "NONE" && result.AuthorityClass != "CORVINT_BINARY") {
		return false
	}
	if (result.State == "READY" && result.Tool == ToolHelp) || result.AuthorityClass == "CORVINT_BINARY" {
		return validHelpResult(result)
	}
	if result.State == "READY" {
		if !validBinding(result.Repository) || result.Abstention.Active || result.Abstention.Reason != "NONE" || result.EpistemicClass != "OBSERVED" {
			return false
		}
		if result.Tool == ToolStatus {
			return result.AuthorityClass == "GIT_REPOSITORY" && result.Receipt == nil
		}
		return result.AuthorityClass == "REPOSITORY_EVIDENCE" && validReceiptBinding(result)
	}
	if result.EpistemicClass != "NOT_OBSERVED" || result.AuthorityClass != "NONE" ||
		!result.Abstention.Active || result.Abstention.Reason == "" {
		return false
	}
	if result.Repository != nil && !validBinding(result.Repository) {
		return false
	}
	return result.Receipt == nil || (result.Abstention.Reason == "OUT_OF_SCOPE" && validReceiptBinding(result))
}

func knownTool(tool string) bool {
	switch tool {
	case ToolQuery, ToolImpact, ToolStatus, ToolContext, ToolCEMReport, ToolHelp, ToolGraphImports:
		return true
	}
	return flowsTools[tool]
}

func validBinding(binding *RepositoryBinding) bool {
	if binding == nil || binding.DirtyPathCount < 0 || !lowerHex(binding.DirtyPathsSHA256, 64) ||
		(binding.WorktreeState != "CLEAN" && binding.WorktreeState != "MIXED" && binding.WorktreeState != "NOT_OBSERVED") ||
		(binding.WorktreeState == "NOT_OBSERVED" && binding.DirtyPathCount != 0) ||
		!projectprofile.Known(binding.ProfileID) {
		return false
	}
	length := 0
	switch binding.ObjectFormat {
	case "sha1":
		length = 40
	case "sha256":
		length = 64
	default:
		return false
	}
	return lowerHex(binding.CommitRevision, length) && lowerHex(binding.TreeRevision, length)
}

func validReceiptBinding(result Result) bool {
	if result.Repository == nil || result.Receipt == nil {
		return false
	}
	switch result.Tool {
	case ToolGraphImports:
		return result.Receipt["tool"] == "graph-imports" && result.Receipt["revision"] == result.Repository.CommitRevision
	case ToolContext:
		revision, revisionOK := result.Receipt["revision"].(string)
		return result.Receipt["tool"] == "context" && revisionOK && revision == result.Repository.TreeRevision
	case ToolCEMReport:
		// A CEM receipt binds to the caller's two object IDs, not the checkout.
		return result.Receipt["tool"] == "cem-report" && result.Receipt["mutates"] == false
	}
	if flowsTools[result.Tool] {
		return validFlowsReceipt(result)
	}
	mode, modeOK := result.Receipt["mode"].(string)
	revision, revisionOK := result.Receipt["revision"].(string)
	wantMode := "query"
	if result.Tool == ToolImpact {
		wantMode = "impact"
	}
	return modeOK && revisionOK && mode == wantMode && revision == result.Repository.TreeRevision
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func repositoryDrift(err error) bool {
	code := errorCode(err)
	return code == "repository-state-unstable" || code == "unsupported-query-drift" || err.Error() == "repository-state-unstable" ||
		strings.Contains(err.Error(), "repository revision changed") ||
		strings.Contains(err.Error(), "repository HEAD changed")
}

func isUnsupported(err error, prefix string) bool { return strings.HasPrefix(errorCode(err), prefix) }

func abstentionReason(err error) string {
	code := errorCode(err)
	if code == "" {
		return "UNSUPPORTED"
	}
	return strings.ToUpper(strings.ReplaceAll(code, "-", "_"))
}

func errorCode(err error) string {
	var contextFailure *contextindex.Error
	if errors.As(err, &contextFailure) {
		return contextFailure.Code
	}
	var kernelFailure *gokernel.Error
	if errors.As(err, &kernelFailure) {
		return kernelFailure.Code
	}
	return ""
}

func normalizeFailure(ctx context.Context, err error) *Error {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return failure("cancelled")
	}
	code := errorCode(err)
	if code == "repository-probe-timeout" || indexDeadline(err) { // MCPV0-038: a bounded overrun is not a missing repository
		return &Error{Code: "repository-probe-timeout", ReasonClass: reasonClass(err)}
	}
	if strings.HasPrefix(code, "repository-") || code == "unsupported-git-object-format" || strings.HasPrefix(err.Error(), "Git ") {
		return &Error{Code: "repository-unavailable", ReasonClass: reasonClass(err)}
	}
	return failure("internal-error")
}

// reasonClass is the typed status refusal class a kernel error carries
// (decision 0383); it is never derived from message text.
func reasonClass(err error) string {
	var contextFailure *contextindex.Error
	if errors.As(err, &contextFailure) {
		return contextFailure.ReasonClass
	}
	var kernelFailure *gokernel.Error
	if errors.As(err, &kernelFailure) {
		return kernelFailure.ReasonClass
	}
	return ""
}

func failure(code string) *Error { return &Error{Code: code} }

func snapshotSchema() map[string]any {
	oid := map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"}
	return objectSchema(map[string]any{
		"schema":         map[string]any{"const": plansnapshot.Schema},
		"commitRevision": oid, "treeRevision": oid, "baseRevision": oid,
		"changedPaths":       map[string]any{"type": "array", "maxItems": 4096, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1}},
		"changedPathsSha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
	}, []any{"schema", "commitRevision", "treeRevision", "baseRevision", "changedPaths", "changedPathsSha256"})
}

func (registry *Registry) planningSnapshot(ctx context.Context, raw []byte, task string) (repositorySnapshot, *plansnapshot.Receipt, *Error) {
	if len(raw) == 0 {
		if task != "" {
			snapshot, err := registry.querySnapshot(ctx, task)
			return snapshot, nil, err
		}
		snapshot, err := registry.snapshot(ctx)
		return snapshot, nil, err
	}
	receipt, err := plansnapshot.Decode(raw)
	if err != nil {
		return repositorySnapshot{}, nil, failure("invalid-arguments")
	}
	if err = receipt.Validate(ctx, registry.root); err != nil {
		return repositorySnapshot{}, nil, failure("repository-unavailable")
	}
	index, err := contextindex.BuildRevisionContext(ctx, registry.root, receipt.Commit)
	snapshot, bridgeErr := registry.snapshotIndex(ctx, index, err)
	if bridgeErr != nil {
		return repositorySnapshot{}, nil, bridgeErr
	}
	if index.Revision != receipt.Tree || index.CommitRevision != receipt.Commit {
		return repositorySnapshot{}, nil, failure("repository-unavailable")
	}
	// The binding describes the observed checkout; only the nested receipt uses
	// immutable snapshot freshness. Never relabel a dirty checkout as clean.
	observed, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		return repositorySnapshot{}, nil, normalizeFailure(ctx, err)
	}
	snapshot.binding = bindingFromRepository(observed)
	if snapshot.binding.CommitRevision != receipt.Commit || snapshot.binding.TreeRevision != receipt.Tree {
		return repositorySnapshot{}, nil, failure("repository-unavailable")
	}
	return snapshot, &receipt, nil
}

func (registry *Registry) boundedPlanning(ctx context.Context, tool string, snapshot repositorySnapshot, scope *plansnapshot.Receipt, receipt map[string]any) (Result, *Error) {
	if scope != nil {
		if err := scope.Validate(ctx, registry.root); err != nil {
			return Result{}, failure("repository-unavailable")
		}
		receipt["snapshot"] = scope.Scope()
	}
	return boundedObserved(tool, snapshot.binding, receipt)
}

// SharesRoot reports whether two registries pinned the same root directory, so
// one multi-root process never binds a repository under two aliases (MMR-V0-003).
func (registry *Registry) SharesRoot(other *Registry) bool {
	return registry != nil && other != nil && registry.rootIdentity != nil && other.rootIdentity != nil &&
		os.SameFile(registry.rootIdentity, other.rootIdentity)
}

// indexDeadline reports the context index's own Git deadline refusal, which carries no code; only
// its typed cause is read (MCPV0-038, decision 0533). The caller's own end is checked first.
func indexDeadline(err error) bool {
	var contextFailure *contextindex.Error
	return errors.As(err, &contextFailure) && errors.Is(contextFailure.Cause, context.DeadlineExceeded)
}

// Root is the canonical root the registry is bound to.
func (registry *Registry) Root() string { return registry.root }

// SameRoot reports whether the bound root, its Git marker and its Git
// directories still have the identities observed at binding.
func (registry *Registry) SameRoot() bool { return registry.sameRoot() }
