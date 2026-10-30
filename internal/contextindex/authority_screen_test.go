package contextindex

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// TestHiddenUnicodeClassesAreExactlyTheSpecSets is TCP-V0-055: every code
// point of each named set is flagged under its class, the neighbours of each
// range are not, and a byte order mark is exempt only at byte offset 0.
func TestHiddenUnicodeClassesAreExactlyTheSpecSets(t *testing.T) {
	t.Parallel()
	want := map[string][]rune{
		hiddenZeroWidth:   {0x200B, 0x200C, 0x200D, 0x2060, 0x2061, 0x2062, 0x2063, 0x2064},
		hiddenBidiControl: {0x061C, 0x200E, 0x200F},
		hiddenTag:         {},
	}
	for r := rune(0x202A); r <= 0x202E; r++ {
		want[hiddenBidiControl] = append(want[hiddenBidiControl], r)
	}
	for r := rune(0x2066); r <= 0x2069; r++ {
		want[hiddenBidiControl] = append(want[hiddenBidiControl], r)
	}
	for r := rune(0xE0000); r <= 0xE007F; r++ {
		want[hiddenTag] = append(want[hiddenTag], r)
	}
	for class, runes := range want {
		for _, r := range runes {
			if got := hiddenUnicodeClass(r, 5); got != class {
				t.Fatalf("U+%04X = %q, want %q", r, got, class)
			}
		}
	}
	for _, r := range []rune{'a', '\t', 0x061B, 0x061D, 0x200A, 0x2010, 0x2029, 0x202F, 0x205F, 0x2065, 0x206A, 0xDFFFF, 0xE0080, 0xFEFE, 0xFF00} {
		if got := hiddenUnicodeClass(r, 5); got != "" {
			t.Fatalf("visible U+%04X classed %q", r, got)
		}
	}
	if got := hiddenUnicodeClass(0xFEFF, 0); got != "" {
		t.Fatalf("leading BOM classed %q, want exempt", got)
	}
	if got := hiddenUnicodeClass(0xFEFF, 3); got != hiddenZeroWidth {
		t.Fatalf("mid-file U+FEFF classed %q, want %q", got, hiddenZeroWidth)
	}
}

// TestScreenHiddenUnicodeReportsCountClassesAndFirstSite: the report counts
// every hidden code point, lists classes in the fixed order, and names the
// first one's line and code point; a leading BOM alone is clean.
func TestScreenHiddenUnicodeReportsCountClassesAndFirstSite(t *testing.T) {
	t.Parallel()
	report := screenHiddenUnicode("clean\nrule\u202Eevil\u2069\n\U000E0041\u200B\n")
	if report.count != 4 || report.firstLine != 2 || report.firstCodePoint != "U+202E" {
		t.Fatalf("report = %+v", report)
	}
	if !slices.Equal(report.classes, []string{hiddenZeroWidth, hiddenBidiControl, hiddenTag}) {
		t.Fatalf("classes = %v", report.classes)
	}
	if clean := screenHiddenUnicode("\uFEFFStanding instructions.\n"); clean.count != 0 || len(clean.classes) != 0 {
		t.Fatalf("leading BOM report = %+v, want clean", clean)
	}
	if mid := screenHiddenUnicode("a\n\uFEFFb"); mid.count != 1 || mid.firstLine != 2 {
		t.Fatalf("mid-file BOM report = %+v", mid)
	}
}

func screenFixture(t *testing.T, agents string, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":         "module example.test/screen\n\ngo 1.27.0\n",
		"AGENTS.md":      agents,
		"docs/STYLE.md":  "# Style\n\nWrap at 100 columns.\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
	}
	for path, content := range extra {
		files[path] = content
	}
	return routedIndex(t, files)
}

func packetRow(t *testing.T, packet map[string]any, kind, path string) (map[string]any, map[string]any) {
	t.Helper()
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] == kind && row["id"] == path {
			return row, mapsFromAny(row["evidence"])[0]
		}
	}
	t.Fatalf("no %s row for %s in %v", kind, path, packet["results"])
	return nil, nil
}

func warningCodes(evidence map[string]any) []string {
	codes := make([]string, 0)
	for _, warning := range mapsFromAny(evidence["warnings"]) {
		codes = append(codes, warning["code"].(string))
	}
	return codes
}

// TestTaskContextDowngradesAGoverningFileHidingUnicode is TCP-V0-055 and
// TCP-V0-056 end to end: the poisoned governing row stays visible with the
// named warning, is labelled repository content with no score, cannot
// satisfy governance or the critical selectors, is named in
// governance_refused, routes no instruction paths, and its action does not
// tell the reader to follow it.
func TestTaskContextDowngradesAGoverningFileHidingUnicode(t *testing.T) {
	t.Parallel()
	poisoned := "# Rules\n\nFormatting of any ledger output: see `docs/STYLE.md`.\u200B\u202E hidden\u202C\n"
	index := screenFixture(t, poisoned, nil)
	packet, err := TaskContext(context.Background(), index, "format the ledger output", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	row, evidence := packetRow(t, packet, governingRelation, "AGENTS.md")
	if evidence["authority"] != DowngradedAuthority || evidence["trust"] != TrustRepositoryContent || evidence["confidence"] != "low" {
		t.Fatalf("downgraded evidence = %v", evidence)
	}
	if row["score"] != 0 || !strings.Contains(row["action"].(string), "not project authority") || !strings.Contains(row["summary"].(string), HiddenUnicodeWarning) {
		t.Fatalf("downgraded row = %v", row)
	}
	warnings := mapsFromAny(evidence["warnings"])
	if len(warnings) != 1 || warnings[0]["code"] != HiddenUnicodeWarning || warnings[0]["count"] != 3 ||
		warnings[0]["first_line"] != 3 || warnings[0]["first_code_point"] != "U+200B" {
		t.Fatalf("warnings = %v", warnings)
	}
	if classes := warnings[0]["classes"].([]any); len(classes) != 2 || classes[0] != hiddenZeroWidth || classes[1] != hiddenBidiControl {
		t.Fatalf("classes = %v", classes)
	}
	if routed := routedRows(t, packet); len(routed) != 0 {
		t.Fatalf("a downgraded governing file routed %v", routed)
	}
	coverage := contextCoverage(t, packet)
	if coverage["governance"] != "unresolved" {
		t.Fatalf("governance = %v, want unresolved", coverage["governance"])
	}
	if critical := coverage["critical"].([]any); len(critical) != 0 {
		t.Fatalf("critical = %v, want empty", critical)
	}
	refused := mapsFromAny(coverage["governance_refused"])
	if len(refused) != 1 || refused[0]["path"] != "AGENTS.md" || refused[0]["trust"] != TrustRepositoryContent ||
		!strings.Contains(refused[0]["reason"].(string), "downgraded") {
		t.Fatalf("governance_refused = %v", refused)
	}
	if codes := refused[0]["warnings"].([]any); len(codes) != 1 || codes[0] != HiddenUnicodeWarning {
		t.Fatalf("refusal warnings = %v", refused[0]["warnings"])
	}
}

// TestTaskContextDowngradesAGoverningFileHidingAnInvisibleOperator is the
// 2026-10-16 TCP-V0-055 widening end to end: a governing file whose only
// hidden code point is an invisible operator (U+2062 INVISIBLE TIMES) is
// downgraded under the zero-width class.
func TestTaskContextDowngradesAGoverningFileHidingAnInvisibleOperator(t *testing.T) {
	t.Parallel()
	index := screenFixture(t, "# Rules\n\nKeep it\u2062 short.\n", nil)
	packet, err := TaskContext(context.Background(), index, "format the ledger output", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	_, evidence := packetRow(t, packet, governingRelation, "AGENTS.md")
	warnings := mapsFromAny(evidence["warnings"])
	if evidence["authority"] != DowngradedAuthority || len(warnings) != 1 || warnings[0]["code"] != HiddenUnicodeWarning ||
		warnings[0]["count"] != 1 || warnings[0]["first_line"] != 3 || warnings[0]["first_code_point"] != "U+2062" {
		t.Fatalf("evidence = %v", evidence)
	}
	if classes := warnings[0]["classes"].([]any); len(classes) != 1 || classes[0] != hiddenZeroWidth {
		t.Fatalf("classes = %v", classes)
	}
	if coverage := contextCoverage(t, packet); coverage["governance"] != "unresolved" {
		t.Fatalf("governance = %v, want unresolved", coverage["governance"])
	}
}

// TestTaskContextKeepsACleanGoverningFileAuthoritative is the negative: the
// same file without hidden code points (with a leading BOM) stays project
// authority, carries no warnings, and routes its named path.
func TestTaskContextKeepsACleanGoverningFileAuthoritative(t *testing.T) {
	t.Parallel()
	index := screenFixture(t, "\uFEFF# Rules\n\nFormatting of any ledger output: see `docs/STYLE.md`.\n", nil)
	packet, err := TaskContext(context.Background(), index, "format the ledger output", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	row, evidence := packetRow(t, packet, governingRelation, "AGENTS.md")
	if evidence["authority"] != "project-instructions" || row["score"] != 1000 {
		t.Fatalf("clean governing row = %v / %v", row, evidence)
	}
	if _, ok := evidence["warnings"]; ok {
		t.Fatalf("clean row carries warnings %v", evidence["warnings"])
	}
	coverage := contextCoverage(t, packet)
	if coverage["governance"] != "reserved" || len(coverage["governance_refused"].([]any)) != 0 {
		t.Fatalf("coverage = %v", coverage)
	}
	if routed := routedRows(t, packet); len(routed) != 1 {
		t.Fatalf("routed = %v, want the style row", routed)
	}
}

// TestTaskContextDowngradesASpecMentionedRowHidingUnicode: the screen covers
// every reserved relation, and a downgraded row orders after every clean
// reserved row.
func TestTaskContextDowngradesASpecMentionedRowHidingUnicode(t *testing.T) {
	t.Parallel()
	index := screenFixture(t, "# Rules\n\nKeep it short.\n", map[string]string{
		"docs/specs/alpha-v0.md": "# Alpha\n\n- `ALP-001`: the alpha clause.\U000E0049\U000E0047\n",
	})
	packet, err := TaskContext(context.Background(), index, "apply ALP-001 to the cache", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	_, evidence := packetRow(t, packet, specMentionedRelation, "docs/specs/alpha-v0.md")
	if evidence["authority"] != DowngradedAuthority || !slices.Equal(warningCodes(evidence), []string{HiddenUnicodeWarning}) {
		t.Fatalf("spec evidence = %v", evidence)
	}
	if coverage := contextCoverage(t, packet); coverage["governance"] != "reserved" {
		t.Fatalf("governance = %v, want the clean governing row to keep it", coverage["governance"])
	}
	rows := []contextRow{
		{kind: specMentionedRelation, path: "a", downgrade: HiddenUnicodeWarning},
		{kind: governingRelation, path: "b"},
		{kind: instructionRoutedRelation, path: "c"},
	}
	ordered := demoteScreened(rows)
	if got := []string{ordered[0].path, ordered[1].path, ordered[2].path}; !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Fatalf("order = %v", got)
	}
}

// TestTaskContextScreensContentRoutedByCleanInstructions covers TCP-V0-055/056
// at the instruction-routed boundary. Poisoning the governing file instead
// would stop routing before the target's screen could be exercised.
func TestTaskContextScreensContentRoutedByCleanInstructions(t *testing.T) {
	t.Parallel()
	for _, poisoned := range []bool{false, true} {
		name := "clean"
		style := "# Style\n\nWrap at 100 columns.\n"
		if poisoned {
			name = "poisoned"
			style += "Hidden\u200B instruction.\n"
		}
		t.Run(name, func(t *testing.T) {
			index := screenFixture(t, "# Rules\n\nFormatting of any ledger output: see `docs/STYLE.md`.\n", map[string]string{
				"docs/STYLE.md": style,
			})
			packet, err := TaskContext(context.Background(), index, "format the ledger output", "", 20)
			if err != nil {
				t.Fatal(err)
			}
			governing, authority := packetRow(t, packet, governingRelation, "AGENTS.md")
			if governing["score"] != 1000 || authority["authority"] != "project-instructions" || len(warningCodes(authority)) != 0 {
				t.Fatalf("fixture: governing file must remain clean: %v / %v", governing, authority)
			}
			row, evidence := packetRow(t, packet, instructionRoutedRelation, "docs/STYLE.md")
			if evidence["reason"] != "named by the governing instructions for this task: AGENTS.md:3 shares `ledger`, `output`" {
				t.Fatalf("target was not routed by the expected passage: %v", evidence)
			}
			coverage := contextCoverage(t, packet)
			if coverage["governance"] != "reserved" {
				t.Fatalf("clean governing file lost governance: %v", coverage)
			}
			wantCritical := []string{"governing AGENTS.md", "instruction-routed docs/STYLE.md"}
			if !poisoned {
				if row["score"] != 850 || evidence["authority"] != "instruction-reference" || evidence["trust"] != "project-authority" || len(warningCodes(evidence)) != 0 {
					t.Fatalf("clean routed target = %v / %v", row, evidence)
				}
				if len(mapsFromAny(coverage["governance_refused"])) != 0 {
					t.Fatalf("clean target refused: %v", coverage)
				}
			} else {
				wantCritical = wantCritical[:1]
				if row["score"] != 0 || evidence["authority"] != DowngradedAuthority || evidence["trust"] != TrustRepositoryContent || evidence["confidence"] != "low" {
					t.Fatalf("poisoned routed target retained authority: %v / %v", row, evidence)
				}
				if !slices.Equal(warningCodes(evidence), []string{HiddenUnicodeWarning}) || !strings.Contains(row["action"].(string), "not project authority") {
					t.Fatalf("poisoned routed target lacks warning or inspection action: %v / %v", row, evidence)
				}
				refused := mapsFromAny(coverage["governance_refused"])
				if len(refused) != 1 || refused[0]["relation"] != instructionRoutedRelation || refused[0]["path"] != "docs/STYLE.md" || refused[0]["trust"] != TrustRepositoryContent {
					t.Fatalf("poisoned routed target not refused: %v", refused)
				}
			}
			if got := contextSelectors(t, coverage, "critical"); !slices.Equal(got, wantCritical) {
				t.Fatalf("critical = %v, want %v", got, wantCritical)
			}
			if got := contextSelectors(t, coverage, "critical_missing"); len(got) != 0 {
				t.Fatalf("unexpected critical_missing = %v", got)
			}
		})
	}
}

// TestTaskContextOrdersScreenedReservationsEndToEnd is TCP-V0-056: a poisoned
// governing reservation starts ahead of clean specs before demotion. Inspect
// the emitted packet, not demoteScreened directly, so removing its reservedRows
// call loses this witness. Each partition must retain reservation order.
func TestTaskContextOrdersScreenedReservationsEndToEnd(t *testing.T) {
	t.Parallel()
	index := screenFixture(t, "# Rules\n\nKeep it short.\u200B\n", map[string]string{
		"docs/specs/alpha-v0.md": "# Alpha\n\n- `ALP-001`: the alpha clause.\n",
		"docs/specs/beta-v0.md":  "# Beta\n\n- `BET-001`: the beta clause.\u200B\n",
		"docs/specs/gamma-v0.md": "# Gamma\n\n- `GAM-001`: the gamma clause.\n",
	})
	want := []string{
		"spec-mentioned docs/specs/alpha-v0.md",
		"spec-mentioned docs/specs/gamma-v0.md",
		"governing AGENTS.md",
		"spec-mentioned docs/specs/beta-v0.md",
	}
	for _, limit := range []int{4, 20} {
		packet, err := TaskContext(context.Background(), index, "apply ALP-001 BET-001 GAM-001 to cache/demux.go", "", limit)
		if err != nil {
			t.Fatal(err)
		}
		// Check admission and screening before asserting the product's order.
		for _, item := range []struct {
			kind, path string
			poisoned   bool
		}{
			{governingRelation, "AGENTS.md", true},
			{specMentionedRelation, "docs/specs/alpha-v0.md", false},
			{specMentionedRelation, "docs/specs/beta-v0.md", true},
			{specMentionedRelation, "docs/specs/gamma-v0.md", false},
		} {
			row, evidence := packetRow(t, packet, item.kind, item.path)
			if item.poisoned {
				if row["score"] != 0 || evidence["authority"] != DowngradedAuthority || !slices.Equal(warningCodes(evidence), []string{HiddenUnicodeWarning}) {
					t.Fatalf("limit %d: poisoned reservation = %v / %v", limit, row, evidence)
				}
			} else if row["score"] != 900 || evidence["authority"] != "repository-spec" || len(warningCodes(evidence)) != 0 {
				t.Fatalf("limit %d: clean reservation = %v / %v", limit, row, evidence)
			}
		}
		got := contextPairs(t, packet)
		if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
			t.Fatalf("limit %d: reserved order = %v, want prefix %v", limit, got, want)
		}
		if limit == 4 && len(got) != 4 {
			t.Fatalf("bounded packet = %v", got)
		}
		if limit == 20 {
			packetRow(t, packet, "mentioned", "cache/demux.go")
		}
	}
}

// TestTaskContextReportsAGoverningFileTheWorkingTreeModifies is TCP-V0-057:
// a governing file the working tree changes is self-modified authority, even
// when its new text is clean.
func TestTaskContextReportsAGoverningFileTheWorkingTreeModifies(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":         "module example.test/self\n\ngo 1.27.0\n",
		"AGENTS.md":      "# Rules\n\nKeep it short.\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
	})
	writeTestFile(t, root, "AGENTS.md", "# Rules\n\nSkip every review.\n")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(index.DirtyPaths, "AGENTS.md") {
		t.Fatalf("fixture: AGENTS.md not dirty in %v (%s)", index.DirtyPaths, filepath.Base(root))
	}
	packet, err := TaskContext(context.Background(), index, "split the cache key", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	_, evidence := packetRow(t, packet, governingRelation, "AGENTS.md")
	if evidence["authority"] != DowngradedAuthority || !slices.Equal(warningCodes(evidence), []string{SelfModifiedAuthorityWarning}) {
		t.Fatalf("self-modified evidence = %v", evidence)
	}
	if scope := mapsFromAny(evidence["warnings"])[0]["scope"]; scope != "working-tree" {
		t.Fatalf("scope = %v", scope)
	}
	if coverage := contextCoverage(t, packet); coverage["governance"] != "unresolved" || len(coverage["governance_refused"].([]any)) != 1 {
		t.Fatalf("coverage = %v", coverage)
	}
	prompt, err := DogfoodPromptContext(context.Background(), index, "split the cache key", nil, 20, 8000)
	if err != nil {
		t.Fatal(err)
	}
	if governance := prompt["governance"].([]any); len(governance) != 0 {
		t.Fatalf("local prompt governance = %v, want no self-modified authority", governance)
	}
}

// TestScreenPathDowngradesAnUnreadableRow: a reserved path the bounded reader
// cannot hand back is never cleared by default (invariant 2).
func TestScreenPathDowngradesAnUnreadableRow(t *testing.T) {
	t.Parallel()
	index := screenFixture(t, "# Rules\n", nil)
	compiler := newTaskContextCompiler(index, "anything", "")
	screen := compiler.screenPath("missing.md")
	if !screen.unscreened || !slices.Equal(screen.codes, []string{HiddenUnicodeUnscreenedWarning}) {
		t.Fatalf("screen = %+v", screen)
	}
	rows := compiler.screenAuthority([]contextRow{{kind: governingRelation, path: "missing.md", authority: "project-instructions", score: 1000}})
	if rows[0].authority != DowngradedAuthority || rows[0].score != 0 {
		t.Fatalf("row = %+v", rows[0])
	}
	if TrustClass(DowngradedAuthority) != TrustRepositoryContent {
		t.Fatalf("TrustClass(%s) = %s", DowngradedAuthority, TrustClass(DowngradedAuthority))
	}
}

// TestCEMGoverningInstructionRuleMatchesThePacket: cem status names the same
// governing instruction files the packet reserves (CEM-V1-270, TCP-V0-057).
func TestCEMGoverningInstructionRuleMatchesThePacket(t *testing.T) {
	t.Parallel()
	for _, candidate := range []string{
		"AGENTS.md", "pkg/AGENTS.md", "Claude.md", "GEMINI.md", ".github/copilot-instructions.md", "copilot-instructions.md",
		".github/instructions/go.instructions.md", ".github/instructions/go.md", "docs/instructions/go.instructions.md",
		"AGENTS.txt", "README.md", "docs/specs/task-context-packet-v0.md", "AGENTS.override.md",
	} {
		if got, want := workflow.GoverningInstructionPath(candidate), documentKind(candidate) == "instructions"; got != want {
			t.Fatalf("%s: cem rule %v, packet rule %v", candidate, got, want)
		}
	}
}
