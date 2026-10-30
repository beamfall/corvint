package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const ToolGraphImports = "corvint.graph.imports"

// importGraphKindsPattern admits one to five comma-separated kind names;
// uniqueness is checked by contextindex.ImportGraphKinds (NGI-V0-308).
const importGraphKindsPattern = "^(import|type-import|angular-template|angular-style|ruby-constant)(,(import|type-import|angular-template|angular-style|ruby-constant)){0,4}$"

// NewImportGraph selects the additive import-graph profile. Existing profiles
// retain their exact tool lists and continue to refuse this tool.
func NewImportGraph(root string) (*Registry, *Error) {
	registry, err := New(root)
	if err != nil {
		return nil, err
	}
	registry.importGraph = true
	return registry, nil
}

func importGraphDescriptor() ToolDescriptor {
	return ToolDescriptor{
		Name: ToolGraphImports, Annotations: readAnnotations(),
		Description: "Read pinned whole-tree file import edges and language uncertainties as graph-imports/1 JSONL. Required: revision (full commit ID). Optional kinds: comma-separated import,type-import,angular-template,angular-style,ruby-constant (filters edges only). Pages preserve CLI byte order; repeat with nextOffset and identical revision/kinds until done. No total result cap. Syntax and single-candidate Zeitwerk evidence only; non-literal or ambiguous references, selectors and string-keyed DI are uncertain. Example: {revision: FULL_COMMIT, offset: 0}.",
		InputSchema: objectSchema(map[string]any{
			"revision": map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"},
			"kinds":    map[string]any{"type": "string", "pattern": importGraphKindsPattern},
			"offset":   map[string]any{"type": "integer", "minimum": 0, "maximum": 9007199254740991},
		}, []any{"revision"}),
	}
}

func (registry *Registry) callGraphImports(ctx context.Context, arguments []byte) (Result, *Error) {
	var input struct {
		Revision string          `json:"revision"`
		Kinds    json.RawMessage `json:"kinds"`
		Offset   json.RawMessage `json:"offset"`
	}
	if decodeClosed(arguments, &input) != nil {
		return Result{}, failure("invalid-arguments")
	}
	value := ""
	if len(input.Kinds) != 0 {
		if json.Unmarshal(input.Kinds, &value) != nil || value == "" {
			return Result{}, failure("invalid-arguments")
		}
	}
	var offset int64
	if len(input.Offset) != 0 {
		if bytes.Equal(bytes.TrimSpace(input.Offset), []byte("null")) || json.Unmarshal(input.Offset, &offset) != nil {
			return Result{}, failure("invalid-arguments")
		}
	}
	if offset < 0 || offset > 9007199254740991 {
		return Result{}, failure("invalid-arguments")
	}
	kinds, err := contextindex.ImportGraphKinds(value)
	if err != nil {
		return Result{}, failure("graph-invalid-kinds")
	}
	index, err := contextindex.BuildImportGraph(ctx, registry.root, input.Revision)
	if err != nil {
		var detail *contextindex.Error
		if errors.As(err, &detail) && (detail.Code == "graph-invalid-revision" || detail.Code == "graph-unknown-revision") {
			return Result{}, failure(detail.Code)
		}
		return Result{}, normalizeFailure(ctx, err)
	}
	snapshot, bridgeErr := registry.snapshotIndex(ctx, index, nil)
	if bridgeErr != nil {
		return Result{}, bridgeErr
	}
	// The object-only build never observes the checkout: its dirty count
	// and digest describe no observation, so the state says so (NGI-V0-305).
	snapshot.binding.WorktreeState = "NOT_OBSERVED"
	var page bytes.Buffer
	var position int64
	count := 0
	pageFull := errors.New("graph page full")
	err = contextindex.WalkImportGraph(ctx, index, kinds, func(row contextindex.ImportGraphRecord) error {
		if position < offset {
			position++
			return nil
		}
		data, err := json.Marshal(row)
		if err != nil {
			return err
		}
		// Bound the raw JSONL well below the bridge's escaped transport cap.
		if count == 128 || page.Len()+len(data)+1 > 48*1024 {
			return pageFull
		}
		page.Write(data)
		page.WriteByte('\n')
		count++
		position++
		return nil
	})
	done := err == nil
	if err != nil && !errors.Is(err, pageFull) {
		return Result{}, normalizeFailure(ctx, err)
	}
	if count == 0 && !done {
		return abstained(ToolGraphImports, "OUTPUT_BUDGET_EXCEEDED", &snapshot.binding), nil
	}
	if done && position < offset {
		return Result{}, failure("invalid-arguments")
	}
	receipt := map[string]any{"tool": "graph-imports", "schema": "graph-imports/1", "revision": input.Revision,
		"format": "jsonl", "jsonl": page.String(), "offset": offset, "done": done}
	if !done {
		receipt["nextOffset"] = position
	}
	return boundedObserved(ToolGraphImports, snapshot.binding, receipt)
}
