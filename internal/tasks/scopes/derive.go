// Package scopes derives experimental claim scopes from an explicitly enabled
// Corvint index pack. It never builds an index or writes repository state.
package scopes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/runtimeenv"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Derive implements CAL-V0-022. Default snapshots are executable-specific;
// cross-binary reuse requires the operator's existing pack-format opt-in.
func Derive(ctx context.Context, root, baseTree, title, body string) ([]string, string, bool) {
	if runtimeenv.Value("SNAPSHOT_FORMAT") != "pack" {
		return nil, "", false
	}
	index, hit, _, err := contextindex.LoadContextPackSnapshotDeferred(ctx, root)
	if err != nil || !hit || index.Revision != baseTree || len(index.DirtyPaths) != 0 {
		return nil, "", false
	}
	packet, err := contextindex.TaskContext(ctx, index, title+"\n"+body, "", min(wire.MaxTouchPaths, 50))
	if err != nil {
		return nil, "", false
	}
	paths := packetPaths(packet, index)
	if len(paths) == 0 {
		return nil, "", false
	}
	raw, err := json.Marshal(struct {
		Profile, Schema, Tree, Title, Body string
		Packet                             map[string]any
	}{"corvint-task-scope/0", contextindex.AnalyzerSchemaID(), baseTree, title, body, packet})
	if err != nil {
		return nil, "", false
	}
	sum := sha256.Sum256(raw)
	return paths, hex.EncodeToString(sum[:]), true
}

func packetPaths(packet map[string]any, index *contextindex.Index) []string {
	if packet["state"] != "READY" || packet["revision"] != index.Revision {
		return nil
	}
	coverage, ok := packet["coverage"].(map[string]any)
	if !ok || coverage["omitted_results"] != 0 || coverage["budget_shortage"] != "none" {
		return nil
	}
	for _, key := range []string{"critical_missing", "governance_refused"} {
		raw, err := json.Marshal(coverage[key])
		if err != nil || string(raw) != "[]" {
			return nil
		}
	}
	answer, ok := coverage["answerability"].(map[string]any)
	if !ok || (answer["verdict"] != "supported" && answer["verdict"] != "relations-answer") {
		return nil
	}
	unexamined, ok := coverage["unexamined"].([]any)
	if !ok {
		return nil
	}
	for _, raw := range unexamined {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		withheld := row["withheld"]
		if withheld != nil && withheld != 0 {
			return nil
		}
		switch row["state"] {
		case "examined", "not-applicable", "subject-absent":
		default:
			return nil
		}
	}
	rows, ok := packet["results"].([]any)
	if !ok {
		return nil
	}
	set := map[string]bool{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		switch row["kind"] {
		case "governing", "spec-mentioned", "instruction-routed":
			continue
		}
		path, ok := row["id"].(string)
		if !ok {
			return nil
		}
		source, ok := index.Sources[path]
		if !ok || source.BlobHash == "" {
			return nil
		}
		if _, err := wire.ParsePath("scope", path); err != nil {
			return nil
		}
		if _, err := wire.ParseIdentifier("scope", path); err != nil {
			return nil
		}
		set[path] = true
	}
	if len(set) > wire.MaxTouchPaths {
		return nil
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
