package contextindex

import (
	"bytes"
	"context"
	"slices"
	"testing"
)

func routedFixture(t *testing.T, instructions string) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/routed\n\ngo 1.27.0\n",
		"AGENTS.md":        instructions,
		"docs/ROUTES.md":   "# Routes\n\nOne row per question.\n",
		"docs/STYLE.md":    "# Style\n\nWrap at 100 columns.\n",
		"docs/archive.md":  "# Archive\n\nOld notes.\n",
		"docs/history.md":  "# History\n\nOlder notes.\n",
		"widget/ledger.go": "package widget\n\nfunc Ledger() {}\n",
	})
	index, err := Build(context.Background(), root)
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

// TestTaskContextRoutesPathsTheGoverningInstructionsNameForTheTask is
// TCP-V0-047: a tracked path the governing file names in a passage sharing
// two task terms is reserved after the governing row, with its reason; a path
// named where the passage shares fewer terms is not.
func TestTaskContextRoutesPathsTheGoverningInstructionsNameForTheTask(t *testing.T) {
	index := routedFixture(t, "# Rules\n\nFormatting of any ledger: see `docs/STYLE.md`.\n"+
		"- Backlog ledger questions: follow `docs/ROUTES.md` and `docs/missing.md`.\n")
	task := "move the backlog ledger into the store"
	packet, err := TaskContext(context.Background(), index, task, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("TCP-V0-047 a task-matching passage routes its named path", func(t *testing.T) {
		pairs := contextPairs(t, packet)
		if len(pairs) < 2 || pairs[0] != "governing AGENTS.md" || pairs[1] != "instruction-routed docs/ROUTES.md" {
			t.Fatalf("rows = %v, want the routed row right after the governing row", pairs)
		}
		want := []string{"docs/ROUTES.md | named by the governing instructions for this task: AGENTS.md:4 shares `backlog`, `ledger`"}
		if got := routedRows(t, packet); !slices.Equal(got, want) {
			t.Fatalf("routed = %v, want %v", got, want)
		}
		row := mapsFromAny(packet["results"])[1]
		evidence := mapsFromAny(row["evidence"])[0]
		if evidence["authority"] != "instruction-reference" || evidence["trust"] != "project-authority" {
			t.Fatalf("routed evidence = %v", evidence)
		}
		critical := contextSelectors(t, contextCoverage(t, packet), "critical")
		if !slices.Equal(critical, []string{"governing AGENTS.md", "instruction-routed docs/ROUTES.md"}) {
			t.Fatalf("critical = %v", critical)
		}
	})
	t.Run("TCP-V0-047 a passage sharing one task term routes nothing", func(t *testing.T) {
		for _, row := range mapsFromAny(packet["results"]) {
			if row["id"] == "docs/STYLE.md" && row["kind"] == "instruction-routed" {
				t.Fatalf("docs/STYLE.md was routed from a passage sharing only `ledger`: %v", row)
			}
		}
	})
	t.Run("TCP-V0-047 identical inputs render identical bytes", func(t *testing.T) {
		again, err := TaskContext(context.Background(), index, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		first, err := CanonicalJSON(packet)
		if err != nil {
			t.Fatal(err)
		}
		second, err := CanonicalJSON(again)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("packets differ:\n%s\n%s", first, second)
		}
	})
}

// TestTaskContextCapsInstructionRoutedRows is TCP-V0-047's bound: at most two
// routed rows; a third named path is withheld and reported as a slot shortage.
func TestTaskContextCapsInstructionRoutedRows(t *testing.T) {
	index := routedFixture(t, "# Rules\n\n"+
		"- Backlog ledger questions: follow `docs/ROUTES.md`, `docs/archive.md` and `docs/history.md`.\n")
	packet, err := TaskContext(context.Background(), index, "move the backlog ledger into the store", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	pairs := contextPairs(t, packet)
	want := []string{"governing AGENTS.md", "instruction-routed docs/ROUTES.md", "instruction-routed docs/archive.md"}
	if len(pairs) < 3 || !slices.Equal(pairs[:3], want) {
		t.Fatalf("rows = %v, want prefix %v", pairs, want)
	}
	coverage := contextCoverage(t, packet)
	states, withheld := contextUnexamined(t, coverage)
	if states["instruction-routed"] != "examined" || contextIntValue(withheld["instruction-routed"]) != 1 {
		t.Fatalf("instruction-routed scope = %v / %v", states["instruction-routed"], withheld["instruction-routed"])
	}
	if coverage["budget_shortage"] != "slots" {
		t.Fatalf("budget_shortage = %v, want slots", coverage["budget_shortage"])
	}
}

// TestContextSpansSkipInstructionRoutedRows is TCP-V0-047's "reserved in every
// other respect" under TCP-V0-025: a routed row, like the governing row, takes
// no core span even when its text carries the task's terms.
func TestContextSpansSkipInstructionRoutedRows(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":         "module example.test/routed\n\ngo 1.27.0\n",
		"AGENTS.md":      "# Rules\n\n- Backlog ledger questions: follow `docs/ROUTES.md`.\n",
		"docs/ROUTES.md": "# Routes\n\nMove the backlog ledger into the store here.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet := spanPacket(t, index, "on", "move the backlog ledger into the store")
	if pairs := contextPairs(t, packet); !slices.Contains(pairs, "instruction-routed docs/ROUTES.md") {
		t.Fatalf("rows = %v, want the routed row", pairs)
	}
	_, rows := spanRowsOf(t, packet)
	if core := findSpan(rows, "core", "docs/ROUTES.md"); core != nil {
		t.Fatalf("routed row took a core span: %#v", core)
	}
}
