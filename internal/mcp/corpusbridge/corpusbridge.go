// Package corpusbridge exposes only capabilities declared by a revalidated
// documentation corpus. It never invokes a provider, test or shell command.
package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

type ToolFailure struct{ Code, Message string }
type Registry struct {
	root, path, digest string
	identity           os.FileInfo
	artifact           *doccorpus.Artifact
}

var tools = map[string]string{"corvint.docs_inventory": "inventory", "corvint.docs_info": "info", "corvint.docs_validate": "validate", "corvint.docs_concept": "concept", "corvint.docs_claims": "claims", "corvint.docs_flow": "flow", "corvint.docs_dependencies": "dependencies", "corvint.docs_recommend_tests": "recommend-tests", "corvint.docs_navigation": "navigation", "corvint.docs_vocabulary": "vocabulary", "corvint.docs_intent": "intent", "corvint.docs_search": "search", "corvint.docs_get": "get", "corvint.docs_locate": "locate", "corvint.docs_find_related": "related", "corvint.docs_coverage": "coverage", "corvint.docs_gaps": "gaps", "corvint.docs_get_journey": "journey", "corvint.docs_get_stability": "stability", "corvint.docs_trace": "trace"}

func New(root, path string) (*Registry, *Error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, &Error{"invalid-root"}
	}
	identity, err := os.Lstat(absolute)
	if err != nil || !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, &Error{"invalid-root"}
	}
	raw, err := doccorpus.ReadCorpusFile(absolute, path)
	if err != nil {
		return nil, &Error{"corpus-unavailable"}
	}
	a, err := doccorpus.Open(context.Background(), absolute, raw)
	if err != nil {
		return nil, &Error{"corpus-unavailable"}
	}
	return &Registry{root: absolute, path: path, digest: doccorpus.Digest(raw), identity: identity, artifact: a}, nil
}
func (r *Registry) Tools() []bridge.ToolDescriptor {
	result := []bridge.ToolDescriptor{}
	for name, op := range tools {
		if op == "inventory" && r.artifact.Schema != doccorpus.SchemaV2 || !r.artifact.HasCapability(doccorpus.CapabilityFor(op)) {
			continue
		}
		properties := map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxResults}}
		if r.artifact.Schema == doccorpus.SchemaV2 {
			properties["offset"] = map[string]any{"type": "integer", "minimum": 0, "maximum": doccorpus.MaxCorpusRecords * 4}
		}
		properties["retirement"] = doccorpus.RetirementInputSchema()
		required := []any{}
		if doccorpus.OperationInput(op) == "query" {
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "query")
		}
		if doccorpus.OperationInput(op) == "id" {
			properties["id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "id")
		}
		if doccorpus.OperationInput(op) == "path" {
			properties["path"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "path")
		}
		if op == "gaps" {
			properties["id"] = map[string]any{"type": "string", "maxLength": 1024}
		}
		result = append(result, bridge.ToolDescriptor{Name: name, Description: "Read source-revalidated experimental documentation corpus evidence; generated authority and explicit limitations remain.", Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: false, DestructiveHint: false}, InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func (r *Registry) Call(ctx context.Context, name string, arguments []byte) (map[string]any, string, *ToolFailure, *Error) {
	op, ok := tools[name]
	if !ok || op == "inventory" && r.artifact.Schema != doccorpus.SchemaV2 || !r.artifact.HasCapability(doccorpus.CapabilityFor(op)) {
		return nil, "", nil, &Error{"unsupported-tool"}
	}
	if ctx.Err() != nil {
		return nil, "", nil, &Error{"cancelled"}
	}
	var input struct {
		Retirement *doccorpus.RetirementPolicy `json:"retirement,omitempty"`
		Query      string                      `json:"query"`
		ID         string                      `json:"id"`
		Path       string                      `json:"path"`
		Limit      *int                        `json:"limit"`
		Offset     int                         `json:"offset"`
	}
	if len(arguments) > 16<<10 || json.Unmarshal(arguments, &input, json.RejectUnknownMembers(true)) != nil {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	var members map[string]any
	if json.Unmarshal(arguments, &members) != nil || members == nil {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	for key, value := range members {
		allowed := key == "retirement" || key == "offset" && r.artifact.Schema == doccorpus.SchemaV2 || key == "limit" || key == "query" && doccorpus.OperationInput(op) == "query" || key == "path" && doccorpus.OperationInput(op) == "path" || key == "id" && (doccorpus.OperationInput(op) == "id" || op == "gaps")
		if !allowed || value == nil {
			return nil, "", nil, &Error{"invalid-arguments"}
		}
	}
	if input.Offset < 0 || input.Offset > doccorpus.MaxCorpusRecords*4 {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
		if limit < 1 || limit > doccorpus.MaxResults {
			return nil, "", nil, &Error{"invalid-arguments"}
		}
	}
	kind := doccorpus.OperationInput(op)
	if kind == "query" && input.Query == "" || kind == "path" && input.Path == "" || kind == "id" && input.ID == "" {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	if kind != "query" && input.Query != "" || kind != "path" && input.Path != "" || kind != "id" && op != "gaps" && input.ID != "" {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	identity, err := os.Lstat(r.root)
	if err != nil || !os.SameFile(identity, r.identity) {
		return nil, "", &ToolFailure{"corpus-input-unavailable", "repository root changed"}, nil
	}
	raw, err := doccorpus.ReadCorpusFile(r.root, r.path)
	if err != nil {
		return failure(err)
	}
	if doccorpus.Digest(raw) != r.digest {
		return nil, "", &ToolFailure{"corpus-refused", "configured artifact changed; restart with the new artifact"}, nil
	}
	receipt, err := doccorpus.ReadQuery(ctx, r.root, raw, doccorpus.Request{Operation: op, Retirement: input.Retirement, Query: input.Query, ID: input.ID, Path: input.Path, Limit: limit, Offset: input.Offset})
	if err != nil {
		return failure(err)
	}
	data, err := doccorpus.Encode(receipt)
	if err != nil {
		return failure(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return failure(err)
	}
	return result, string(data), nil, nil
}
func failure(err error) (map[string]any, string, *ToolFailure, *Error) {
	var e *doccorpus.Error
	if errors.As(err, &e) {
		return nil, "", &ToolFailure{e.Code, e.Message}, nil
	}
	return nil, "", &ToolFailure{"corpus-refused", "native corpus validation failed"}, nil
}
