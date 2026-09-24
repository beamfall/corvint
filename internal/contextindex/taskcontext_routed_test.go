package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"slices"
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
	index := routedIndex(t, map[string]string{
		"go.mod":         "module example.test/routed\n\ngo 1.27.0\n",
		"AGENTS.md":      "# Rules\n\n- Backlog ledger questions: follow `docs/ROUTES.md`.\n",
		"docs/ROUTES.md": "# Routes\n\nMove the backlog ledger into the store here.\n",
	})
	packet := spanPacket(t, index, "on", "move the backlog ledger into the store")
	if pairs := contextPairs(t, packet); !slices.Contains(pairs, "instruction-routed docs/ROUTES.md") {
		t.Fatalf("rows = %v, want the routed row", pairs)
	}
	_, rows := spanRowsOf(t, packet)
	if core := findSpan(rows, "core", "docs/ROUTES.md"); core != nil {
		t.Fatalf("routed row took a core span: %#v", core)
	}
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

func routedPacket(t *testing.T, index *Index, task, subject string) map[string]any {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, subject, 20)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

// TestTaskContextRoutingIsNotApplicableWithoutAGoverningRow is TCP-V0-047:
// with no governing instructions there is nothing to route from.
func TestTaskContextRoutingIsNotApplicableWithoutAGoverningRow(t *testing.T) {
	index := routedIndex(t, map[string]string{
		"go.mod":           "module example.test/routed\n\ngo 1.27.0\n",
		"docs/ROUTES.md":   "# Routes\n\nBacklog ledger questions go here.\n",
		"widget/ledger.go": "package widget\n\nfunc Ledger() {}\n",
	})
	packet := routedPacket(t, index, routedTask, "")
	states, _ := contextUnexamined(t, contextCoverage(t, packet))
	if states["instruction-routed"] != "not-applicable" {
		t.Fatalf("instruction-routed state = %q, want not-applicable", states["instruction-routed"])
	}
	if routed := routedPaths(t, packet); len(routed) != 0 {
		t.Fatalf("routed = %v, want none", routed)
	}
}

// TestTaskContextRoutingSkipsTheSubject is TCP-V0-047: the subject is never a
// routing candidate, so the next named path takes its place and nothing is
// withheld.
func TestTaskContextRoutingSkipsTheSubject(t *testing.T) {
	index := routedFixture(t, "# Rules\n\n- Backlog ledger questions: follow `docs/ROUTES.md` and `docs/archive.md`.\n")
	packet := routedPacket(t, index, routedTask, "docs/ROUTES.md")
	if routed := routedPaths(t, packet); !slices.Equal(routed, []string{"docs/archive.md"}) {
		t.Fatalf("routed = %v, want [docs/archive.md]", routed)
	}
	_, withheld := contextUnexamined(t, contextCoverage(t, packet))
	if contextIntValue(withheld["instruction-routed"]) != 0 {
		t.Fatalf("withheld = %v, want 0", withheld["instruction-routed"])
	}
}

// TestTaskContextOrdersRoutingPassagesByIDF is TCP-V0-047's order: the passage
// whose shared terms carry more summed body idf routes first, whatever its line.
// `store` is in three sources and `backlog` in one, so the later passage wins.
func TestTaskContextOrdersRoutingPassagesByIDF(t *testing.T) {
	index := routedIndex(t, map[string]string{
		"go.mod": "module example.test/routed\n\ngo 1.27.0\n",
		"AGENTS.md": "# Rules\n\n- Ledger store layout: `docs/archive.md`.\n" +
			"- Backlog ledger questions: `docs/ROUTES.md`.\n",
		"docs/ROUTES.md":   "# Routes\n\nOne row per question.\n",
		"docs/archive.md":  "# Archive\n\nOld notes.\n",
		"docs/depot.md":    "# Depot\n\nThe store opens at nine.\n",
		"docs/shop.md":     "# Shop\n\nThe store closes at five.\n",
		"widget/ledger.go": "package widget\n\nfunc Ledger() {}\n",
	})
	packet := routedPacket(t, index, routedTask, "")
	want := []string{"docs/ROUTES.md", "docs/archive.md"}
	if routed := routedPaths(t, packet); !slices.Equal(routed, want) {
		t.Fatalf("routed = %v, want %v", routed, want)
	}
}

// TestTaskContextRoutingPromotesAnExistingRow is TCP-V0-047: a path a slot
// would admit anyway is reserved as instruction-routed instead, and the packet
// carries it once.
func TestTaskContextRoutingPromotesAnExistingRow(t *testing.T) {
	files := func(instructions string) map[string]string {
		return map[string]string{
			"go.mod":         "module example.test/routed\n\ngo 1.27.0\n",
			"AGENTS.md":      instructions,
			"docs/ROUTES.md": "# Routes\n\nMove the backlog ledger into the store here.\n",
		}
	}
	unrouted := routedPacket(t, routedIndex(t, files("# Rules\n\nBe brief.\n")), routedTask, "")
	if pairs := contextPairs(t, unrouted); !slices.Contains(pairs, "documentation docs/ROUTES.md") {
		t.Fatalf("control rows = %v, want docs/ROUTES.md admitted by a slot", pairs)
	}
	packet := routedPacket(t, routedIndex(t, files("# Rules\n\n- Backlog ledger questions: follow `docs/ROUTES.md`.\n")), routedTask, "")
	count := 0
	for _, row := range mapsFromAny(packet["results"]) {
		if row["id"] == "docs/ROUTES.md" {
			count++
		}
	}
	if pairs := contextPairs(t, packet); count != 1 || !slices.Contains(pairs, "instruction-routed docs/ROUTES.md") {
		t.Fatalf("rows = %v, want docs/ROUTES.md once, as instruction-routed", pairs)
	}
}

// TestTaskContextKeepsRoutedRowsWhenResultsAreWithheld is TCP-V0-047 under
// TCP-V0-016: an unsupported conjunction withdraws slot rows but keeps the
// reserved ones, routed rows included.
func TestTaskContextKeepsRoutedRowsWhenResultsAreWithheld(t *testing.T) {
	index := routedFixture(t, "# Rules\n\n- Backlog ledger questions: follow `docs/ROUTES.md`.\n")
	packet := routedPacket(t, index, routedTask+" with `absentOne` and `absentTwo`", "")
	answer := contextCoverage(t, packet)["answerability"].(map[string]any)
	if answer["verdict"] != "unsupported-conjunction" {
		t.Fatalf("verdict = %v, want unsupported-conjunction", answer["verdict"])
	}
	want := []string{"governing AGENTS.md", "instruction-routed docs/ROUTES.md"}
	if pairs := contextPairs(t, packet); !slices.Equal(pairs, want) {
		t.Fatalf("rows = %v, want %v", pairs, want)
	}
}

// TestTaskContextRoutingIgnoresCommonTerms is TCP-V0-047's idf floor: `note`
// is in most sources, so a passage sharing it and one rarer task term shares
// one qualifying term and routes nothing.
func TestTaskContextRoutingIgnoresCommonTerms(t *testing.T) {
	index := routedFixture(t, "# Rules\n\n- Ledger note: follow `docs/STYLE.md`.\n")
	packet := routedPacket(t, index, "change the ledger note format", "")
	if routed := routedPaths(t, packet); len(routed) != 0 {
		t.Fatalf("routed = %v, want none: `note` is below the idf floor", routed)
	}
}

// TestTaskContextRoutingSkipsFencedCodeBlocks is TCP-V0-047: fenced code is
// literal text, so a path backticked inside a fence routes nothing, even after
// a blank line inside the fence.
func TestTaskContextRoutingSkipsFencedCodeBlocks(t *testing.T) {
	index := routedFixture(t, "# Rules\n\nBacklog ledger commands:\n"+
		"```sh\ncat `docs/ROUTES.md`\n\nbacklog ledger `docs/archive.md`\n```\n\n"+
		"- Backlog ledger questions: `docs/history.md`.\n")
	packet := routedPacket(t, index, routedTask, "")
	if routed := routedPaths(t, packet); !slices.Equal(routed, []string{"docs/history.md"}) {
		t.Fatalf("routed = %v, want [docs/history.md]", routed)
	}
}

// TestTaskContextRoutingClosesFencesAsCommonMark is TCP-V0-047: only a fence
// of the opening character, at least as long and with no info string, closes
// a block, so a nested fence of the other kind or a shorter one stays literal
// text and the prose after the real close still routes.
func TestTaskContextRoutingClosesFencesAsCommonMark(t *testing.T) {
	for name, block := range map[string]string{
		"other character": "~~~md\n```sh\nbacklog ledger `docs/archive.md`\n~~~\n",
		"shorter fence":   "````\n```\nbacklog ledger `docs/archive.md`\n```\n````\n",
		"info string":     "```\n```sh\nbacklog ledger `docs/archive.md`\n```\n",
	} {
		index := routedFixture(t, "# Rules\n\n"+block+"\n- Backlog ledger questions: `docs/history.md`.\n")
		packet := routedPacket(t, index, routedTask, "")
		if routed := routedPaths(t, packet); !slices.Equal(routed, []string{"docs/history.md"}) {
			t.Fatalf("%s: routed = %v, want [docs/history.md]", name, routed)
		}
	}
}

// TestTaskContextRoutingSkipsCompoundsTheTableSplits is TCP-V0-047: the body
// term table splits a camelCase identifier, so its lowered compound has no
// document frequency; it must not count at the highest idf, and the parts
// most sources hold fall below the floor.
func TestTaskContextRoutingSkipsCompoundsTheTableSplits(t *testing.T) {
	index := routedFixture(t, "# Rules\n\n- noteRecord and noteKept: follow `docs/STYLE.md`.\n")
	packet := routedPacket(t, index, "change noteRecord and noteKept", "")
	if routed := routedPaths(t, packet); len(routed) != 0 {
		t.Fatalf("routed = %v, want none: the compounds are not in the body table", routed)
	}
}
