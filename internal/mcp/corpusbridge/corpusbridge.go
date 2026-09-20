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

var tools = map[string]string{"corvint.docs_info": "info", "corvint.docs_search": "search", "corvint.docs_get": "get", "corvint.docs_locate": "locate", "corvint.docs_find_related": "related", "corvint.docs_coverage": "coverage", "corvint.docs_gaps": "gaps", "corvint.docs_get_journey": "journey", "corvint.docs_get_stability": "stability", "corvint.docs_trace": "trace"}

func New(root, path string) (*Registry, *Error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, &Error{"invalid-root"}
	}
	identity, err := os.Lstat(absolute)
	if err != nil || !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, &Error{"invalid-root"}
	}
	raw, err := doccorpus.ReadFile(absolute, path)
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
		if !r.artifact.HasCapability(doccorpus.CapabilityFor(op)) {
			continue
		}
		properties := map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxResults}}
		required := []any{}
		if op == "search" {
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "query")
		}
		if op == "get" || op == "related" || op == "journey" || op == "stability" || op == "trace" {
			properties["id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "id")
		}
		if op == "locate" {
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
	if !ok || !r.artifact.HasCapability(doccorpus.CapabilityFor(op)) {
		return nil, "", nil, &Error{"unsupported-tool"}
	}
	if ctx.Err() != nil {
		return nil, "", nil, &Error{"cancelled"}
	}
	var input struct {
		Query string `json:"query"`
		ID    string `json:"id"`
		Path  string `json:"path"`
		Limit *int   `json:"limit"`
	}
	if len(arguments) > 16<<10 || json.Unmarshal(arguments, &input, json.RejectUnknownMembers(true)) != nil {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	var members map[string]any
	if json.Unmarshal(arguments, &members) != nil || members == nil {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	for key, value := range members {
		allowed := key == "limit" || key == "query" && op == "search" || key == "path" && op == "locate" || key == "id" && (op == "get" || op == "trace" || op == "related" || op == "journey" || op == "stability" || op == "gaps")
		if !allowed || value == nil {
			return nil, "", nil, &Error{"invalid-arguments"}
		}
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
		if limit < 1 || limit > doccorpus.MaxResults {
			return nil, "", nil, &Error{"invalid-arguments"}
		}
	}
	if op != "get" && op != "related" && op != "journey" && op != "stability" && op != "trace" && op != "gaps" && input.ID != "" {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	if op == "search" && input.Query == "" || op == "locate" && input.Path == "" || (op == "get" || op == "related" || op == "journey" || op == "stability" || op == "trace") && input.ID == "" {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	if op != "search" && input.Query != "" || op != "locate" && input.Path != "" {
		return nil, "", nil, &Error{"invalid-arguments"}
	}
	identity, err := os.Lstat(r.root)
	if err != nil || !os.SameFile(identity, r.identity) {
		return nil, "", &ToolFailure{"corpus-input-unavailable", "repository root changed"}, nil
	}
	raw, err := doccorpus.ReadFile(r.root, r.path)
	if err != nil {
		return failure(err)
	}
	if doccorpus.Digest(raw) != r.digest {
		return nil, "", &ToolFailure{"corpus-refused", "configured artifact changed; restart with the new artifact"}, nil
	}
	receipt, err := doccorpus.ReadQuery(ctx, r.root, raw, doccorpus.Request{Operation: op, Query: input.Query, ID: input.ID, Path: input.Path, Limit: limit})
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
