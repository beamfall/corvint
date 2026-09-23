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
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/plansnapshot"
	"github.com/Beamfall/corvint/internal/projectprofile"
)

const (
	ToolQuery  = "corvint.query"
	ToolImpact = "corvint.impact"
	ToolStatus = "corvint.status"
	// ToolContext and ToolCEMReport project `corvint context` and `corvint cem
	// report` (MCPV0-024, MCPV0-025).
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
	cemReport  func(context.Context, string, workflow.ReadOptions) (map[string]any, error)
	platform   string
	probe      func(context.Context, string) (gokernel.Repository, error)
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
		return nil, nil, err
	}
	packet, err := contextindex.TaskContextWeighted(ctx, index, input.Task, input.Subject, input.Limit, admitted)
	return index, packet, err
}

// previewCEMReport is `corvint cem report` rendered without publication.
func previewCEMReport(ctx context.Context, root string, options workflow.ReadOptions) (map[string]any, error) {
	session, err := workflow.Open(root)
	if err != nil {
		return nil, err
	}
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

// Error is a closed, sanitized failure. It never contains repository paths,
// tool output, or underlying process errors.
type Error struct{ Code string }

func (failure *Error) Error() string { return failure.Code }

// Registry binds every call to one canonical local repository root.
type Registry struct {
	root         string
	rootIdentity os.FileInfo
	gitIdentity  os.FileInfo
	operations   repositoryOperations
}

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
	gitMarker, err := os.Lstat(filepath.Join(resolved, ".git"))
	if err != nil || !gitMarker.IsDir() || gitMarker.Mode()&os.ModeSymlink != 0 {
		return nil, failure("invalid-root")
	}
	return &Registry{root: resolved, rootIdentity: rootIdentity, gitIdentity: gitMarker, operations: productionRepositoryOperations()}, nil
}

// Tools is the exact delivered surface. Standalone evidence and dashboard
// snapshot tools are omitted; query and impact receipts carry their evidence.
func (registry *Registry) Tools() []ToolDescriptor {
	if registry == nil {
		return nil
	}
	// Byte bounds are advertised as code-point bounds divided by UTFMax, and
	// the task pattern excludes U+0085 (Go trims it, ECMA \s does not), so
	// every schema-valid argument is also runtime-valid.
	oid := map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"}
	ceiling := map[string]any{"type": "integer", "minimum": 0}
	return []ToolDescriptor{
		{
			Name:        ToolCEMReport,
			Description: "Render the CEM reviewer report for a repository-relative canonical map against two full revision IDs, without writing it.",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"map": map[string]any{
					"type": "string", "minLength": 1, "maxLength": wire.MaxPathBytes / utf8.UTFMax,
					"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\)(?!(?:.*/)?[.][Gg][Ii][Tt](?:/|$))[^\x00-\x1f\x7f]+$`,
				},
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
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": maxContextLimit, "default": defaultContextLimit},
			}, []any{"task"}),
		},
		{
			Name:        ToolImpact,
			Description: "Compile revision-bound impact evidence for tracked Go files without returning source bodies.",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"snapshot": snapshotSchema(),
				"paths": map[string]any{
					"type": "array", "minItems": 1, "maxItems": maxImpactPaths, "uniqueItems": true,
					"items": map[string]any{
						"type": "string", "minLength": 1, "maxLength": maxPathRunes,
						"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\).+[.]go$`,
					},
				},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": maxImpactLimit, "default": 10},
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
			Description: "Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest.",
			InputSchema: objectSchema(map[string]any{}, []any{}),
			Annotations: readAnnotations(),
		},
	}
}

func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (Result, *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.gitIdentity == nil ||
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
	ctx = gitstatus.WithIsolation(ctx)
	var result Result
	var callErr *Error
	switch name {
	case ToolQuery:
		result, callErr = registry.callQuery(ctx, arguments)
	case ToolImpact:
		result, callErr = registry.callImpact(ctx, arguments)
	case ToolStatus:
		result, callErr = registry.callStatus(ctx, arguments)
	case ToolContext:
		result, callErr = registry.callContext(ctx, arguments)
	case ToolCEMReport:
		result, callErr = registry.callCEMReport(ctx, arguments)
	default:
		return Result{}, failure("unsupported-tool")
	}
	if callErr != nil && (callErr.Code == "invalid-arguments" || callErr.Code == "cancelled") {
		return Result{}, callErr
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
	gitMarker, err := os.Lstat(filepath.Join(registry.root, ".git"))
	return err == nil && gitMarker.IsDir() && gitMarker.Mode()&os.ModeSymlink == 0 && os.SameFile(registry.gitIdentity, gitMarker)
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
	receipt, err := contextindex.Impact(snapshot.index, input.Paths, input.Limit)
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
	Task    string `json:"task"`
	Subject string `json:"subject"`
	Limit   int    `json:"limit"`
}

func (registry *Registry) callContext(ctx context.Context, arguments []byte) (Result, *Error) {
	input := contextInput{Limit: defaultContextLimit}
	if err := decodeClosed(arguments, &input); err != nil || !validContext(input) {
		return Result{}, failure("invalid-arguments")
	}
	if !nativePlatformQualified(registry.operations.platform) {
		return abstained(ToolContext, "UNSUPPORTED_PLATFORM", nil), nil
	}
	index, packet, err := registry.operations.context(ctx, registry.root, input)
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
	return boundedObserved(ToolContext, snapshot.binding, packet)
}

type cemReportInput struct {
	Map           string `json:"map"`
	ExpectedBase  string `json:"expectedBase"`
	Target        string `json:"target"`
	MaxUnknown    *int   `json:"maxUnknown"`
	MaxMechanical *int   `json:"maxMechanical"`
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
	receipt, err := registry.operations.cemReport(ctx, registry.root, workflow.ReadOptions{
		MapPath: input.Map, ExpectedBase: input.ExpectedBase, Target: input.Target,
		Limits: workflow.PolicyLimits{MaxUnknown: input.MaxUnknown, MaxMechanical: input.MaxMechanical},
	})
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

// cemFailures closes CEM's registered codes onto the bridge's sanitized
// failure set; an unlisted code is internal-error.
var cemFailures = map[string]string{
	cemcode.MapUnavailable: "cem-map-unavailable", cemcode.InvalidArguments: "cem-map-unsupported",
	cemcode.InvalidJSON: "cem-map-invalid", cemcode.NotAnObject: "cem-map-invalid",
	cemcode.MissingField: "cem-map-invalid", cemcode.UnknownField: "cem-map-invalid",
	cemcode.UnsupportedSpec: "cem-map-invalid", cemcode.InvalidField: "cem-map-invalid",
	cemcode.InvalidExcludedPath: "cem-map-invalid", cemcode.PathTraversal: "cem-map-invalid",
	cemcode.GitReadFailed: "repository-unavailable", cemcode.GitDiffFailed: "repository-unavailable",
	cemcode.GitDiffTimeout: "repository-unavailable", cemcode.GitStartFailed: "repository-unavailable",
	cemcode.GitExitFailure: "repository-unavailable", cemcode.GitTimeout: "repository-unavailable",
	cemcode.GitOutputExceeded: "repository-unavailable", cemcode.GitBudgetExceeded: "repository-unavailable",
	cemcode.RepositoryObjectUnavailable:     "repository-unavailable",
	cemcode.UnsupportedObjectAlternates:     "repository-unavailable",
	cemcode.UnsupportedRepositoryAttributes: "repository-unavailable",
}

func cemFailure(ctx context.Context, err error) *Error {
	code := cemcode.CodeOf(err)
	if ctx.Err() != nil || code == cemcode.GitCancelled {
		return failure("cancelled")
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
	seen := make(map[string]struct{}, len(input.Paths))
	for _, value := range input.Paths {
		if value == "" || utf8.RuneCountInString(value) > maxPathRunes || filepath.IsAbs(value) ||
			filepath.ToSlash(filepath.Clean(value)) != value || !strings.HasSuffix(value, ".go") || strings.Contains(value, "\\") {
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

func validContext(input contextInput) bool {
	if strings.TrimSpace(input.Task) == "" || len(input.Task) > maxContextTaskBytes {
		return false
	}
	if input.Limit < 1 || input.Limit > maxContextLimit {
		return false
	}
	return input.Subject == "" || validRelativePath(input.Subject, maxPathRunes)
}

// validCEMReport admits a repository-relative map path outside Git metadata
// and two full object IDs. A symlink at or above the map is refused by the
// CEM read itself (publish.Root.ReadBounded), as cem-map-unavailable.
func validCEMReport(input cemReportInput) bool {
	if wire.ValidatePath(input.Map) != nil || !validRelativePath(input.Map, wire.MaxPathBytes) {
		return false
	}
	for _, part := range strings.Split(input.Map, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
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

func validRoot(root string) bool {
	if root == "" || len(root) > 4096 || !utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	for _, character := range root {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
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
		(result.State != "READY" && result.State != "ABSTAINED") ||
		(result.EpistemicClass != "OBSERVED" && result.EpistemicClass != "NOT_OBSERVED") ||
		(result.AuthorityClass != "REPOSITORY_EVIDENCE" && result.AuthorityClass != "GIT_REPOSITORY" && result.AuthorityClass != "NONE") {
		return false
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
	case ToolQuery, ToolImpact, ToolStatus, ToolContext, ToolCEMReport:
		return true
	}
	return false
}

func validBinding(binding *RepositoryBinding) bool {
	if binding == nil || binding.DirtyPathCount < 0 || !lowerHex(binding.DirtyPathsSHA256, 64) ||
		(binding.WorktreeState != "CLEAN" && binding.WorktreeState != "MIXED") ||
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
	case ToolContext:
		revision, revisionOK := result.Receipt["revision"].(string)
		return result.Receipt["tool"] == "context" && revisionOK && revision == result.Repository.TreeRevision
	case ToolCEMReport:
		// A CEM receipt binds to the caller's two object IDs, not the checkout.
		return result.Receipt["tool"] == "cem-report" && result.Receipt["mutates"] == false
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
	if strings.HasPrefix(code, "repository-") || code == "unsupported-git-object-format" || strings.HasPrefix(err.Error(), "Git ") {
		return failure("repository-unavailable")
	}
	return failure("internal-error")
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
