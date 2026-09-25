package bridge

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"errors"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/appflows"
)

// The flows descendant profile projects `corvint flows map|gaps|impact|navigate` (AFU-V1-034).
const (
	ToolFlowsMap      = "corvint.flows.map"
	ToolFlowsGaps     = "corvint.flows.gaps"
	ToolFlowsImpact   = "corvint.flows.impact"
	ToolFlowsNavigate = "corvint.flows.navigate"

	maxFlowInputs    = 100
	maxFlowGoalRunes = 64
)

// flowsTools exist only in the flows descendant profile; the default V0 and task-review registries
// neither advertise nor dispatch them.
var flowsTools = map[string]bool{ToolFlowsMap: true, ToolFlowsGaps: true, ToolFlowsImpact: true, ToolFlowsNavigate: true}

// effectClasses is the closed maxEffect ladder of `flows navigate --max-effect`.
var effectClasses = []string{appflows.EffectRead, appflows.EffectWriteReversible, appflows.EffectWriteIrreversible, appflows.EffectExternal}

// flowsImpactSerial runs one flows impact at a time, probes included. affected.Build shares a
// root's file walk among the Builds that overlap in time, so an impact that overlapped another could
// reuse a walk taken before its own first probe. In this process no other tool calls Build.
var flowsImpactSerial sync.Mutex

// flowsSchemas are the receipt schemas each flows tool may return.
var flowsSchemas = map[string][]string{
	ToolFlowsMap:      {appflows.MapSchema, appflows.LookupSchema},
	ToolFlowsGaps:     {appflows.GapsSchema},
	ToolFlowsImpact:   {appflows.ImpactSchema},
	ToolFlowsNavigate: {appflows.NavigationMapSchema, appflows.NavigationPacketSchema},
}

// NewFlows binds the opt-in flows descendant profile, which adds the four read-only flows tools to
// the V0 three (AFU-V1-034, amending MCPV0-026).
func NewFlows(root string) (*Registry, *Error) {
	registry, err := New(root)
	if err != nil {
		return nil, err
	}
	registry.flows = true
	return registry, nil
}

// EnvelopeOnly reports whether a tool's result carries repository-authored flow text, which is
// returned only inside the untrusted-data envelope and never as structuredContent (AFU-V1-035).
func EnvelopeOnly(name string) bool { return flowsTools[name] }

func flowsToolDescriptors() []ToolDescriptor {
	relative := map[string]any{
		"type": "string", "minLength": 1, "maxLength": maxPathRunes,
		"pattern": `^(?!/)(?!.*/$)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\)(?!(?:.*/)?[.][Gg][Ii][Tt](?:/|$))[^\x00-\x1f\x7f-\x9f]+$`,
	}
	files := map[string]any{"type": "array", "maxItems": maxFlowInputs, "items": relative}
	return []ToolDescriptor{
		{
			Name:        ToolFlowsGaps,
			Description: "Report the flows at HEAD without passing test evidence and the tests no flow names (application-flow-gaps/1).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{"flows": relative, "evidence": files}, []any{"flows"}),
		},
		{
			Name:        ToolFlowsImpact,
			Description: "Report the flows the base..HEAD changed paths reach through links and the impact graph (application-flow-impact/1).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"flows": relative,
				"base":  map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"},
			}, []any{"flows", "base"}),
		},
		{
			Name:        ToolFlowsMap,
			Description: "Map the flow intents at HEAD to their links and test evidence, or look up the flows one path or test key reaches (application-flow-map/1, application-flow-lookup/1).",
			Annotations: readAnnotations(),
			InputSchema: constrained(objectSchema(map[string]any{
				"flows": relative, "evidence": files, "path": relative,
				"testKey": map[string]any{"type": "string", "minLength": 1, "maxLength": maxPathRunes, "pattern": `^[^\x00-\x1f\x7f-\x9f]+$`},
			}, []any{"flows"}), map[string]any{"not": map[string]any{"anyOf": []any{
				map[string]any{"required": []any{"path", "testKey"}},
				map[string]any{"required": []any{"path", "evidence"}, "properties": map[string]any{"evidence": map[string]any{"minItems": 1}}},
				map[string]any{"required": []any{"testKey", "evidence"}, "properties": map[string]any{"evidence": map[string]any{"minItems": 1}}},
			}}}),
		},
		{
			Name:        ToolFlowsNavigate,
			Description: "Compile the navigation map of the flow intents at HEAD, or the ordered packet that reaches one goal flow with each step above maxEffect marked requires-grant (application-navigation-map/0, application-navigation-packet/0).",
			Annotations: readAnnotations(),
			InputSchema: constrained(objectSchema(map[string]any{
				"flows": relative, "evidence": files, "traffic": files,
				"goal":      map[string]any{"type": "string", "minLength": 1, "maxLength": maxFlowGoalRunes},
				"maxEffect": map[string]any{"type": "string", "enum": []any{effectClasses[0], effectClasses[1], effectClasses[2], effectClasses[3]}},
			}, []any{"flows"}), map[string]any{"dependentRequired": map[string]any{"maxEffect": []any{"goal"}}}),
		},
	}
}

// constrained adds the cross-member rules the runtime enforces, so a schema-valid argument object
// is runtime-valid.
func constrained(schema, rules map[string]any) map[string]any {
	maps.Copy(schema, rules)
	return schema
}

type flowsMapInput struct {
	Flows    string   `json:"flows"`
	Evidence []string `json:"evidence"`
	Path     *string  `json:"path"`
	TestKey  *string  `json:"testKey"`
}

type flowsGapsInput struct {
	Flows    string   `json:"flows"`
	Evidence []string `json:"evidence"`
}

type flowsImpactInput struct {
	Flows string `json:"flows"`
	Base  string `json:"base"`
}

type flowsNavigateInput struct {
	Flows     string   `json:"flows"`
	Evidence  []string `json:"evidence"`
	Traffic   []string `json:"traffic"`
	Goal      *string  `json:"goal"`
	MaxEffect *string  `json:"maxEffect"`
}

// flowsOperation is one flows verb over the intents loaded at HEAD.
type flowsOperation func(context.Context, string, appflows.IntentSet) ([]byte, error)

func (registry *Registry) callFlowsMap(ctx context.Context, arguments []byte) (Result, *Error) {
	var input flowsMapInput
	if decodeClosed(arguments, &input) != nil || !validFlowsMap(input) {
		return Result{}, failure("invalid-arguments")
	}
	return registry.runFlows(ctx, ToolFlowsMap, input.Flows, func(ctx context.Context, root string, set appflows.IntentSet) ([]byte, error) {
		if input.Path != nil || input.TestKey != nil {
			return appflows.FlowLookup(ctx, root, set, deref(input.Path), deref(input.TestKey))
		}
		records, err := readFlowInputs(root, input.Evidence, appflows.ReadRunEvidence)
		if err != nil {
			return nil, err
		}
		return appflows.FlowMap(ctx, root, set, records)
	})
}

func (registry *Registry) callFlowsGaps(ctx context.Context, arguments []byte) (Result, *Error) {
	var input flowsGapsInput
	if decodeClosed(arguments, &input) != nil || !validFlowFiles(input.Flows, input.Evidence) {
		return Result{}, failure("invalid-arguments")
	}
	return registry.runFlows(ctx, ToolFlowsGaps, input.Flows, func(ctx context.Context, root string, set appflows.IntentSet) ([]byte, error) {
		records, err := readFlowInputs(root, input.Evidence, appflows.ReadRunEvidence)
		if err != nil {
			return nil, err
		}
		return appflows.FlowGaps(ctx, root, set, records)
	})
}

func (registry *Registry) callFlowsImpact(ctx context.Context, arguments []byte) (Result, *Error) {
	var input flowsImpactInput
	if decodeClosed(arguments, &input) != nil || !validFlowFiles(input.Flows, nil) || !fullObjectID(input.Base) {
		return Result{}, failure("invalid-arguments")
	}
	flowsImpactSerial.Lock()
	defer flowsImpactSerial.Unlock()
	return registry.runFlows(ctx, ToolFlowsImpact, input.Flows, func(ctx context.Context, root string, set appflows.IntentSet) ([]byte, error) {
		resolved, err := appflows.ResolveRevision(ctx, root, input.Base)
		if err != nil {
			return nil, err
		}
		return appflows.FlowImpactAt(ctx, root, set, resolved)
	})
}

func (registry *Registry) callFlowsNavigate(ctx context.Context, arguments []byte) (Result, *Error) {
	var input flowsNavigateInput
	if decodeClosed(arguments, &input) != nil || !validFlowsNavigate(input) {
		return Result{}, failure("invalid-arguments")
	}
	maxEffect := appflows.EffectRead
	if input.MaxEffect != nil {
		maxEffect = *input.MaxEffect
	}
	return registry.runFlows(ctx, ToolFlowsNavigate, input.Flows, func(ctx context.Context, root string, set appflows.IntentSet) ([]byte, error) {
		records, err := readFlowInputs(root, input.Evidence, appflows.ReadRunEvidence)
		if err != nil {
			return nil, err
		}
		observed, err := readFlowInputs(root, input.Traffic, appflows.ReadTraffic)
		if err != nil {
			return nil, err
		}
		if input.Goal == nil {
			return appflows.FlowNavigationMap(ctx, root, set, records, observed)
		}
		return appflows.FlowNavigationPacket(ctx, root, set, records, observed, *input.Goal, maxEffect)
	})
}

// runFlows loads the intents at HEAD and runs one verb between two repository probes, as the CEM
// report does: a probe that changes across the verb abstains rather than binding the receipt to a
// revision it may not describe.
func (registry *Registry) runFlows(ctx context.Context, tool, dir string, operation flowsOperation) (Result, *Error) {
	before, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		return Result{}, normalizeFailure(ctx, err)
	}
	set, err := appflows.LoadIntentsAt(ctx, registry.root, dir, "HEAD")
	if err != nil {
		return Result{}, flowsFailure(ctx, err)
	}
	data, err := operation(ctx, registry.root, set)
	if err != nil {
		return Result{}, flowsFailure(ctx, err)
	}
	after, err := registry.operations.probe(ctx, registry.root)
	if err != nil {
		return Result{}, normalizeFailure(ctx, err)
	}
	if before != after {
		return abstained(tool, "REPOSITORY_STATE_UNSTABLE", nil), nil
	}
	receipt := map[string]any{}
	decoder := jsonv1.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&receipt) != nil {
		return Result{}, failure("internal-error")
	}
	return boundedObserved(tool, bindingFromRepository(after), receipt)
}

// flowsFailure closes every appflows refusal onto one sanitized code; the refusal text can name
// repository content, so it never reaches the caller.
func flowsFailure(ctx context.Context, err error) *Error {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return failure("cancelled")
	}
	return failure("flows-refused")
}

// readFlowInputs resolves repository-relative input files under root, refusing a symlinked parent
// directory; the reader refuses a file that is not regular (AFU-V1-036).
func readFlowInputs[T any](root string, files []string, read func([]string) ([]T, error)) ([]T, error) {
	resolved := make([]string, 0, len(files))
	for _, file := range files {
		if parent := path.Dir(file); parent != "." {
			if _, err := appflows.FlowsDir(root, parent); err != nil {
				return nil, err
			}
		}
		resolved = append(resolved, filepath.Join(root, filepath.FromSlash(file)))
	}
	return read(resolved)
}

func validFlowFiles(dir string, files []string) bool {
	if !validFlowPath(dir) || len(files) > maxFlowInputs {
		return false
	}
	for _, file := range files {
		if !validFlowPath(file) {
			return false
		}
	}
	return true
}

// validFlowPath is a repository-relative path outside Git metadata with no control character.
func validFlowPath(value string) bool {
	if !validRelativePath(value, maxPathRunes) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

// validFlowsMap is the `flows map` rule: evidence files, or one of path and testKey without evidence.
func validFlowsMap(input flowsMapInput) bool {
	lookup := input.Path != nil || input.TestKey != nil
	if !validFlowFiles(input.Flows, input.Evidence) || (input.Path != nil && input.TestKey != nil) || (lookup && len(input.Evidence) != 0) {
		return false
	}
	if input.Path != nil && !validFlowPath(*input.Path) {
		return false
	}
	return input.TestKey == nil || validTestKey(*input.TestKey)
}

// validFlowsNavigate is the `flows navigate` rule: maxEffect only beside a goal.
func validFlowsNavigate(input flowsNavigateInput) bool {
	if !validFlowFiles(input.Flows, input.Evidence) || !validFlowFiles(input.Flows, input.Traffic) {
		return false
	}
	if input.Goal == nil {
		return input.MaxEffect == nil
	}
	if *input.Goal == "" || utf8.RuneCountInString(*input.Goal) > maxFlowGoalRunes {
		return false
	}
	return input.MaxEffect == nil || slices.Contains(effectClasses, *input.MaxEffect)
}

func validTestKey(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxPathRunes && !strings.ContainsFunc(value, unicode.IsControl)
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// validFlowsReceipt binds a flows receipt to the probed HEAD commit and its tool's schemas.
func validFlowsReceipt(result Result) bool {
	schema, schemaOK := result.Receipt["schema"].(string)
	revision, revisionOK := result.Receipt["revision"].(string)
	known := false
	for _, allowed := range flowsSchemas[result.Tool] {
		known = known || schema == allowed
	}
	return schemaOK && revisionOK && known && revision == result.Repository.CommitRevision
}
