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

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/plansnapshot"
	"github.com/Beamfall/corvint/internal/projectprofile"
	"github.com/Beamfall/corvint/internal/rootalias"
)

const (
	ToolImpact = "corvint.impact"
	ToolStatus = "corvint.status"

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
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  map[string]any  `json:"inputSchema"`
	OutputSchema map[string]any  `json:"outputSchema,omitempty"`
	Annotations  ToolAnnotations `json:"annotations"`
}

// WithOutputSchemas declares an outputSchema on every listed tool whose
// result carries structuredContent (MCPV0-953), so a client may read the
// structured object instead of parsing the text block that repeats it. Every
// such object is a read-only result or tool-error object; the schema states
// only what all of them share. A flows tool returns its result in the
// envelope alone (AFU-V1-035) and declares none.
func WithOutputSchemas(tools []ToolDescriptor) []ToolDescriptor {
	for index := range tools {
		if !EnvelopeOnly(tools[index].Name) {
			tools[index].OutputSchema = map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool": map[string]any{"type": "string"}, "mutates": map[string]any{"const": false},
				},
			}
		}
	}
	return tools
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
	build    func(context.Context, string) (*contextindex.Index, error)
	platform string
	probe    func(context.Context, string) (gokernel.Repository, error)
}

func productionRepositoryOperations() repositoryOperations {
	return repositoryOperations{
		build:    contextindex.Build,
		platform: runtime.GOOS, probe: gokernel.ProbeRepositoryContext,
	}
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

const unsupportedQueryIntent = "UNSUPPORTED_QUERY_INTENT"

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

// Registry binds every call to one canonical local repository root.
type Registry struct {
	root         string
	callerRoot   string
	rootIdentity os.FileInfo
	git          *rootbind.Binding
	operations   repositoryOperations
	flows        bool
}

// New binds the default V0 registry: exactly query, impact, and status.
func New(root string) (*Registry, error) {
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

// Profile names one registry profile's tool surface without a root.
type Profile struct{ Flows bool }

// Tools lists what the profile's registry advertises, for a server that
// starts with no usable root (MCPV0-950): it lists the tools it would serve and
// refuses every repository call.
func (profile Profile) Tools() []ToolDescriptor {
	return (&Registry{flows: profile.Flows}).Tools()
}

func (registry *Registry) advertises(name string) bool {
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
			advertised = append(advertised, tool)
		}
	}
	return advertised
}

func (registry *Registry) allTools() []ToolDescriptor {
	// Byte bounds are advertised as code-point bounds divided by UTFMax, and
	// the task pattern excludes U+0085 (Go trims it, ECMA \s does not), so
	// every schema-valid argument is also runtime-valid.
	return append([]ToolDescriptor{
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
						"pattern": impactPathRule.Pattern(),
					},
				},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": maxImpactLimit, "default": 10},
				"budgetBytes": map[string]any{"type": "integer", "minimum": contextindex.MinPacketBytes, "maximum": contextindex.MaxPacketBytes},
			}, []any{"paths"}),
		},
		{
			Name:        ToolStatus,
			Description: "Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest. Gitlinks bind commit IDs; nested submodule contents are outside coverage.",
			InputSchema: objectSchema(map[string]any{}, []any{}),
			Annotations: readAnnotations(),
		},
	}, flowsToolDescriptors()...)
}

func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (Result, *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.git == nil ||
		registry.operations.build == nil || registry.operations.probe == nil {
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
	case ToolImpact:
		result, callErr = registry.callImpact(ctx, arguments)
	case ToolStatus:
		result, callErr = registry.callStatus(ctx, arguments)
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

func fullObjectID(value string) bool { return lowerHex(value, 40) || lowerHex(value, 64) }

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

// nestedObjectSchema is objectSchema below a schema's root, which repeats no
// $schema (MCPV0-954).
func nestedObjectSchema(properties map[string]any, required []any) map[string]any {
	schema := objectSchema(properties, required)
	delete(schema, "$schema")
	return schema
}

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
		(result.Repository != nil && result.Repository.WorktreeState == "NOT_OBSERVED") ||
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
	case ToolImpact, ToolStatus, ToolHelp:
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
	return nestedObjectSchema(map[string]any{
		"schema":         map[string]any{"const": plansnapshot.Schema},
		"commitRevision": oid, "treeRevision": oid, "baseRevision": oid,
		"changedPaths":       map[string]any{"type": "array", "maxItems": 4096, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1}},
		"changedPathsSha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
	}, []any{"schema", "commitRevision", "treeRevision", "baseRevision", "changedPaths", "changedPathsSha256"})
}

func (registry *Registry) planningSnapshot(ctx context.Context, raw []byte, task string) (repositorySnapshot, *plansnapshot.Receipt, *Error) {
	if len(raw) == 0 {
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
