package contextindex

import (
	"context"
	"fmt"
	"testing"
)

func routedFixture(t *testing.T, instructions string) *Index {
	t.Helper()
	return routedIndex(t, map[string]string{
		"go.mod":           "module example.test/routed\n\ngo 1.27.0\n",
		"AGENTS.md":        instructions,
		"docs/ROUTES.md":   "# Routes\n\nOne row per question.\n",
		"docs/STYLE.md":    "# Style\n\nWrap at 100 columns.\n",
		"docs/archive.md":  "# Archive\n\nOld notes.\n",
		"docs/history.md":  "# History\n\nOlder notes.\n",
		"widget/ledger.go": "package widget\n\nfunc Ledger() {}\n",
	})
}

// routedIndex indexes files plus two dozen unrelated notes, so a word two or
// three files share clears TCP-V0-047's idf floor as it would in a real
// repository, while the notes' own shared words fall below it.
func routedIndex(t *testing.T, files map[string]string) *Index {
	t.Helper()
	for number := range 24 {
		files[fmt.Sprintf("notes/note%02d.md", number)] = fmt.Sprintf("# Note %d\n\nA note kept for the record.\n", number)
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// routedRows renders the instruction-routed rows as "path | reason".
func routedRows(t *testing.T, packet map[string]any) []string {
	t.Helper()
	rendered := make([]string, 0)
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] != "instruction-routed" {
			continue
		}
		evidence := mapsFromAny(row["evidence"])[0]
		rendered = append(rendered, row["id"].(string)+" | "+evidence["reason"].(string))
	}
	return rendered
}

// routedTask is the task every routing fixture below is asked.
const routedTask = "move the backlog ledger into the store"

func routedPaths(t *testing.T, packet map[string]any) []string {
	t.Helper()
	paths := make([]string, 0)
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] == "instruction-routed" {
			paths = append(paths, row["id"].(string))
		}
	}
	return paths
}
