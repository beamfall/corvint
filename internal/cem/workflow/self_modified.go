package workflow

import (
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// GoverningInstructionPath reports whether a repository path is a governing
// instruction file under the same rule the context packet uses to reserve its
// governing row: AGENTS.md, CLAUDE.md, GEMINI.md or copilot-instructions.md
// in any directory, or a .github/instructions/*.instructions.md file.
func GoverningInstructionPath(value string) bool {
	if strings.ToLower(path.Ext(value)) != ".md" {
		return false
	}
	name := strings.ToLower(path.Base(value))
	parts := strings.Split(strings.ToLower(path.Dir(value)), "/")
	switch name {
	case "agents.md", "claude.md", "gemini.md", "copilot-instructions.md":
		return true
	}
	return len(parts) >= 2 && parts[0] == ".github" && parts[1] == "instructions" && strings.HasSuffix(name, ".instructions.md")
}

// selfModifiedAuthority lists every governing instruction file the map's own
// patch changes (CEM-CB-026): the hunks that change it, and every hunk basis
// that cites it, which is a change citing the authority it rewrites. Status
// reports these and does not change its state, so every map that was
// ready-for-ci stays ready-for-ci. Empty when the patch changes no governing
// file; the caller then omits the member.
func selfModifiedAuthority(document *wire.Map) []any {
	changed := map[string][]any{}
	for _, hunk := range document.Hunks {
		if GoverningInstructionPath(hunk.Path) {
			changed[hunk.Path] = append(changed[hunk.Path], hunk.ID)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	evidencePath := make(map[string]string, len(document.Evidence))
	for _, evidence := range document.Evidence {
		evidencePath[evidence.ID] = evidence.Path
	}
	cited := map[string][]any{}
	for _, hunk := range document.Hunks {
		for _, basis := range hunk.Basis {
			target := evidencePath[basis.EvidenceID]
			if _, ok := changed[target]; !ok {
				continue
			}
			cited[target] = append(cited[target], map[string]any{
				"hunk": hunk.ID, "evidenceId": basis.EvidenceID, "relation": basis.Relation,
			})
		}
	}
	paths := make([]string, 0, len(changed))
	for changedPath := range changed {
		paths = append(paths, changedPath)
	}
	sort.Strings(paths)
	rows := make([]any, 0, len(paths))
	for _, changedPath := range paths {
		citedBy := cited[changedPath]
		if citedBy == nil {
			citedBy = []any{}
		}
		rows = append(rows, map[string]any{
			"path": changedPath, "hunks": changed[changedPath], "citedBy": citedBy,
			"reason": "this change modifies a governing instruction file; it is not authority for this change's own hunks",
		})
	}
	return rows
}
