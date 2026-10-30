package contextindex

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// evalFloorQuery runs one query against the shared evaluation repository and
// returns the packet state, the number of rendered results, and the abstention
// reason.
func evalFloorQuery(t *testing.T, index *Index, text string) (string, int, string) {
	t.Helper()
	packet, err := EvalQuery(context.Background(), index, text, 10, nil)
	if err != nil {
		t.Fatalf("EvalQuery(%q): %v", text, err)
	}
	results := mapsFromAny(packet["results"])
	reason := ""
	if abstention, ok := packet["abstention"].(map[string]any); ok {
		reason = stringValue(abstention["reason"])
	}
	return stringValue(packet["state"]), len(results), reason
}

// TestEvalQueryRelevanceFloor pins the floor in both directions: a task the
// repository cannot answer is withdrawn even though ranking produced results
// for it, and a task it can answer is untouched.
func TestEvalQueryRelevanceFloor(t *testing.T) {
	t.Parallel()
	index, err := Build(context.Background(), evalQueryRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name, text, state string
		results           string
		reason            string
	}{{
		// "expired" matches the session-revocation summary and nothing else
		// does. One word of six is the background rate of English in a code
		// repository, not evidence that this repository answers the task.
		name: "one-word accident abstains", text: "renew my expired passport before the embassy appointment",
		state: "OUT_OF_SCOPE", results: "empty", reason: "below-relevance-floor",
	}, {
		name: "unrelated task abstains", text: "how do I bake sourdough bread at high altitude",
		state: "OUT_OF_SCOPE", results: "empty",
	}, {
		name: "supported task answers", text: "session expiry device revocation enforcement",
		state: "READY", results: "present", reason: "none",
	}, {
		// A query offering fewer than two words to match is held to what it
		// offers, so a bare identifier lookup still answers.
		name: "single-word query answers", text: "revocation",
		state: "READY", results: "present", reason: "none",
	}} {
		t.Run(testCase.name, func(t *testing.T) {
			state, count, reason := evalFloorQuery(t, index, testCase.text)
			if state != testCase.state {
				t.Fatalf("state = %q, want %q (results=%d reason=%q)", state, testCase.state, count, reason)
			}
			if testCase.results == "empty" && count != 0 {
				t.Fatalf("results = %d, want 0", count)
			}
			if testCase.results == "present" && count == 0 {
				t.Fatal("results = 0, want at least one")
			}
			if testCase.reason != "" && reason != testCase.reason {
				t.Fatalf("abstention reason = %q, want %q", reason, testCase.reason)
			}
		})
	}
}

// TestEvalStrongestSupportCountsQueryWords pins the two properties the floor
// depends on: support is counted in words of the query as written rather than
// in the stems terms() derives from them, and a result carried into the packet
// behind another result contributes no support of its own.
func TestEvalStrongestSupportCountsQueryWords(t *testing.T) {
	ordered := evalOrderedTerms("autumn leaves rendering parity")
	supportIndex := map[string]map[string]struct{}{
		"decision:docs/adr/0001.md": intersectionSet([]string{"leaves", "leave"}),
		"feature:parity-render":     intersectionSet([]string{"rendering", "parity"}),
	}
	stemOnly := []map[string]any{{"kind": "decision", "id": "docs/adr/0001.md"}}
	if support := evalStrongestSupport(stemOnly, supportIndex, ordered); support != 1 {
		t.Fatalf("stem variants counted as %d words, want 1", support)
	}
	carried := []map[string]any{{"kind": "symbol", "id": "internal/render/render.go:Draw"}}
	if support := evalStrongestSupport(carried, supportIndex, ordered); support != 0 {
		t.Fatalf("unscored carried result counted as %d words, want 0", support)
	}
	twoWords := []map[string]any{{"kind": "feature", "id": "parity-render"}}
	if support := evalStrongestSupport(twoWords, supportIndex, ordered); support != 2 {
		t.Fatalf("two matched words counted as %d, want 2", support)
	}
}

// GPK-V0-039, GPK-V0-044.
func TestEvalQueryLearnedPathCannotLiftRelevanceFloorAbstention(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/learnedfloor\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/dashboard/authority/types.go", "package authority\n\ntype TraceCode string\n")
	path := "internal/trace/codec.go"
	writeTestFile(t, root, path, "package trace\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	query := "trace codec zebra quokka"
	absent, err := NewQueryTraceSnapshot("absent", nil)
	if err != nil {
		t.Fatal(err)
	}
	state, count, reason, _ := evalFloorQueryWithSnapshot(t, index, query, absent)
	if state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without trace: state=%q results=%d reason=%q", state, count, reason)
	}
	ready, err := NewQueryTraceSnapshot("ready", []QueryTrace{{
		TraceID: strings.Repeat("a", 64), Task: "trace codec zebra quokka cleanup", Outcome: "passed", ChangedPaths: []string{path},
	}})
	if err != nil {
		t.Fatal(err)
	}
	state, count, reason, matched := evalFloorQueryWithSnapshot(t, index, query, ready)
	if matched != 1 {
		t.Fatalf("matched local traces=%d, want 1", matched)
	}
	if state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("with trace: state=%q results=%d reason=%q", state, count, reason)
	}
}

func evalFloorQueryWithSnapshot(t *testing.T, index *Index, text string, snapshot QueryTraceSnapshot) (string, int, string, int) {
	t.Helper()
	packet, err := EvalQuery(context.Background(), index, text, 10, nil, snapshot)
	if err != nil {
		t.Fatalf("EvalQuery(%q): %v", text, err)
	}
	results := mapsFromAny(packet["results"])
	reason := ""
	if abstention, ok := packet["abstention"].(map[string]any); ok {
		reason = stringValue(abstention["reason"])
	}
	learning := packet["learning"].(map[string]any)
	matched, _ := learning["matched_local_traces"].(int)
	return stringValue(packet["state"]), len(results), reason, matched
}

// GPK-V0-039, GPK-V0-040. The floor is judged on the ranking's emitted packet,
// before the packet budget narrows it. A budget that drops the only row resting
// on two query words ships the surviving one-word rows as BUDGETED with the
// omission counted, never as a below-relevance-floor withdrawal: the task is
// answerable, and the byte budget is the cause the receipt already names.
func TestEvalQueryRelevanceFloorPrecedesPacketBudget(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/floorbudget\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/a/s.go", "package a\n\nfunc Revocation() bool { return true }\n")
	writeTestFile(t, root, "docs/adr/0001-session.md", "# Session\n\nStatus: accepted\n\nSession.\n")
	// The long path makes the two-word row the one a small budget cannot fit.
	supporting := "internal/" + strings.Repeat("deep", 60) + "/revoke.go"
	writeTestFile(t, root, supporting, "package deep\n\nfunc SessionRevocation() bool { return true }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	budget := 2000
	packet, err := EvalQuery(context.Background(), index, "session revocation", 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range mapsFromAny(packet["results"]) {
		if strings.HasPrefix(stringValue(result["id"]), supporting) {
			t.Fatalf("budget %d kept the supporting row; the fixture no longer exercises the ordering", budget)
		}
	}
	abstention := packet["abstention"].(map[string]any)
	omitted, _ := packet["coverage"].(map[string]any)["omitted_results"].(int)
	if packet["state"] != "BUDGETED" || len(mapsFromAny(packet["results"])) == 0 || omitted == 0 || abstention["reason"] != "none" {
		t.Fatalf("state=%v results=%d omitted=%d abstention=%v, want BUDGETED rows with the omission counted",
			packet["state"], len(mapsFromAny(packet["results"])), omitted, abstention)
	}

	// The surviving rows alone fail the floor: without the supporting file the
	// same query is withdrawn.
	testGit(t, root, "rm", "-q", supporting)
	testGit(t, root, "commit", "-qm", "drop the supporting row")
	unsupported, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if state, count, reason := evalFloorQuery(t, unsupported, "session revocation"); state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without the supporting row: state=%q results=%d reason=%q", state, count, reason)
	}
}

// GPK-V0-066. Records outrank symbols only as answers to the task. Two
// features that each match one query word fill a limit-1 packet that fails the
// floor, while a symbol resting on four query words was never admitted; the
// packet is compiled from the confident symbols instead of being withdrawn.
// Without that symbol the same query is still withdrawn.
func TestEvalQueryUnsupportedRecordsYieldToSupportedSymbols(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/recordfloor\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: plugin-music\n    area: plugins\n    summary: Music library.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: plugin-podcasts\n    area: plugins\n    summary: Podcast feeds.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	supporting := "internal/plugin/trust.go"
	writeTestFile(t, root, supporting, "package plugin\n\ntype PluginTrustRoots struct{}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	query := "validate plugin trust roots at model loader construction"

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, query, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	abstention := packet["abstention"].(map[string]any)
	if len(results) != 1 || results[0]["id"] != supporting+":PluginTrustRoots" || packet["state"] != "READY" || abstention["reason"] != "none" {
		t.Fatalf("state=%v abstention=%v results=%v, want the supporting symbol READY", packet["state"], abstention, results)
	}

	testGit(t, root, "rm", "-q", supporting)
	testGit(t, root, "commit", "-qm", "drop the supporting symbol")
	unsupported, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if state, count, reason := evalFloorQuery(t, unsupported, query); state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without the supporting symbol: state=%q results=%d reason=%q", state, count, reason)
	}
}

// GPK-V0-068. A limit-1 packet that emits one feature while the limit omits a
// competitive feature resting on a query word the emitted one does not rest on
// has chosen between two readings of the task by score alone, so it asks for
// widening instead of claiming READY. Once the limit admits both, the packet is
// READY again.
func TestEvalQueryLimitOmittingCompetingRecordNeedsWidening(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/competing\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: access-request-grant\n    area: auth\n    summary: Per-title access requests and grants.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: hls-transcode\n    area: playback\n    summary: HLS transcode ladder.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	query := "carry hls priority in the signed stream grant instead of the request header"

	for _, tc := range []struct {
		limit          int
		state, reason  string
		active         bool
		wantResultsLen int
	}{
		{limit: 1, state: "NEEDS_WIDENING", reason: "omitted-competing-record", active: true, wantResultsLen: 1},
		{limit: 2, state: "READY", reason: "none", active: false, wantResultsLen: 2},
	} {
		packet, err := EvalQuery(context.Background(), index, query, tc.limit, nil)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		abstention := packet["abstention"].(map[string]any)
		if len(results) != tc.wantResultsLen || results[0]["id"] != "access-request-grant" || packet["state"] != tc.state || abstention["reason"] != tc.reason || abstention["active"] != tc.active {
			t.Fatalf("limit %d: state=%v abstention=%v results=%v, want %s/%s", tc.limit, packet["state"], abstention, results, tc.state, tc.reason)
		}
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). At a limit wide enough to hold a decision that clears
// the floor, the record packet passes GPK-V0-039, so GPK-V0-066 never
// substitutes the symbols: on the accepted contract the one-word features
// precede the wide symbol and their tied scores ask for widening. A symbol
// wider than every record and document now leads: the symbol resting on three
// query words, then the records and the decision in their own class order,
// and the packet is READY. A record at least as wide as every symbol keeps
// precedence.
func TestEvalQueryPrecedenceFollowsSupport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/precedence\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: plugin-music\n    area: plugins\n    summary: Music library.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: plugin-podcasts\n    area: plugins\n    summary: Podcast feeds.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: plugin-filmtv\n    area: plugins\n    summary: Film and TV.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	writeTestFile(t, root, "docs/adr/0170-signed-registry.md", "# Plugin trust registry\n\nStatus: accepted\n\nTrust roots come from the signed registry.\n")
	writeTestFile(t, root, "internal/plugin/trust.go", "package plugin\n\ntype PluginTrustRoots struct{}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"wider symbol leads", "validate plugin trust roots at model loader construction", []string{
			"internal/plugin/trust.go:PluginTrustRoots", "plugin-filmtv", "plugin-music", "plugin-podcasts", "docs/adr/0170-signed-registry.md",
		}},
		{"record at least as wide keeps precedence", "podcast feeds plugin", []string{"plugin-podcasts"}},
		{"equally wide document keeps precedence", "plugin trust", []string{"plugin-filmtv", "plugin-music", "plugin-podcasts", "docs/adr/0170-signed-registry.md"}},
		{"wider document keeps precedence", "plugin trust registry", []string{"plugin-filmtv", "plugin-music", "plugin-podcasts", "docs/adr/0170-signed-registry.md"}},
	} {
		packet, err := EvalQuery(context.Background(), index, tc.query, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		abstention := packet["abstention"].(map[string]any)
		got := make([]string, 0, len(results))
		for _, result := range results {
			got = append(got, stringValue(result["id"]))
		}
		if (packet["state"] != "READY" && (tc.name == "wider symbol leads" || tc.name == "record at least as wide keeps precedence")) || abstention["reason"] != "none" || len(got) < len(tc.want) || !reflect.DeepEqual(got[:len(tc.want)], tc.want) {
			t.Fatalf("%s: state=%v abstention=%v results=%v, want READY leading with %v", tc.name, packet["state"], abstention, got, tc.want)
		}
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). The adopter-shaped
// packet stands on a two-word decision at limit 10, despite five feature records
// matching just "plugin". Both multi-term code symbols must precede them.
func TestEvalQueryWiderSymbolsAtLimitTen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/widersymbols\n\ngo 1.27.0\n")
	features := "features:\n"
	for _, name := range []string{"music", "podcasts", "filmtv", "books", "radio"} {
		features += "  - id: plugin-" + name + "\n    area: plugins\n    summary: Entertainment.\n    adr: []\n    applies: [server]\n    status: shipped\n"
	}
	writeTestFile(t, root, "testing/features.yaml", features)
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	writeTestFile(t, root, "docs/adr/0170-signed-registry.md", "# Plugin trust registry\n\nStatus: accepted\n\nTrust roots come from the signed registry.\n")
	writeTestFile(t, root, "internal/plugin/trust.go", "package plugin\n\ntype PluginTrustRoots struct {\n Validate bool\n Model string\n Loader string\n Construction string\n}\n\nfunc NewModelLoader(roots PluginTrustRoots) bool { return roots.Validate }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"validate plugin trust roots at model loader construction",
		"plugin trust roots validation during model loader construction",
		"model loader construction must validate plugin trust roots",
		"validate trust roots when constructing the plugin model loader",
	} {
		t.Run(query, func(t *testing.T) {
			intent := evalInferIntent(query)
			ordered := evalOrderedTerms(query)
			queryTerms := evalRelevanceTerms(terms(query), intent)
			records, _ := evalRankRecords(index, query, queryTerms, ordered, intent)
			if len(records) != 5 {
				t.Fatalf("fixture has %d matching records, want 5", len(records))
			}
			for _, record := range records {
				if evalSupportWidth(record.support, ordered) != 1 {
					t.Fatalf("fixture record %s score=%d support=%v, want one word", record.id, record.score, record.support)
				}
			}
			packet, err := EvalQuery(context.Background(), index, query, 10, nil)
			if err != nil {
				t.Fatal(err)
			}
			results := mapsFromAny(packet["results"])
			want := map[string]bool{
				"internal/plugin/trust.go:PluginTrustRoots": false,
				"internal/plugin/trust.go:NewModelLoader":   false,
			}
			for _, result := range results[:min(2, len(results))] {
				id := stringValue(result["id"])
				if _, exists := want[id]; !exists || want[id] {
					t.Fatalf("leading results=%v, want both code symbols once", results)
				}
				want[id] = true
			}
			for id, present := range want {
				count := 0
				for _, result := range results {
					if result["id"] == id {
						count++
					}
				}
				if !present || count != 1 {
					t.Fatalf("symbol %s present=%v count=%d, want once in the leading pair", id, present, count)
				}
			}
			if len(results) < 8 || results[2]["kind"] != "feature" || packet["state"] != "READY" {
				t.Fatalf("standing packet lost its records or state: %v", packet)
			}
		})
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). Placement keeps
// the original packet untouched when empty, and removes only duplicate symbols.
func TestEvalLeadWithSymbolsPreservesPacket(t *testing.T) {
	symbol := map[string]any{"kind": "symbol", "id": "same-id", "score": 300}
	feature := map[string]any{"kind": "feature", "id": "same-id", "score": 705}
	carried := map[string]any{"kind": "symbol", "id": "same-id", "score": 900}
	results := []map[string]any{feature, carried}
	unchanged := evalLeadWithSymbols(nil, results, make(map[string]map[string]struct{}))
	if &unchanged[0] != &results[0] || !reflect.DeepEqual(unchanged, results) {
		t.Fatal("placement without wider symbols changed the packet")
	}
	got := evalLeadWithSymbols([]evalSymbolCandidate{{result: symbol}}, results, make(map[string]map[string]struct{}))
	if !reflect.DeepEqual(got, []map[string]any{symbol, feature}) || !reflect.DeepEqual(results, []map[string]any{feature, carried}) {
		t.Fatalf("placement=%v original=%v, want scored symbol then feature without mutation", got, results)
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). Same-named methods
// on different receivers have distinct line-based candidate IDs but render one
// path:name identity. Keep the first, leaving the next result slot for evidence.
func TestEvalLeadWithSymbolsDeduplicatesRenderedIdentity(t *testing.T) {
	first := map[string]any{"kind": "symbol", "id": "internal/plugin/trust.go:Validate", "line": 10}
	second := map[string]any{"kind": "symbol", "id": "internal/plugin/trust.go:Validate", "line": 20}
	feature := map[string]any{"kind": "feature", "id": "internal/plugin/trust.go:Validate"}
	document := map[string]any{"kind": "document", "id": "docs/trust.md"}
	wider := []evalSymbolCandidate{
		{id: "internal/plugin/trust.go:Validate:10", result: first},
		{id: "internal/plugin/trust.go:Validate:20", result: second},
	}
	results := []map[string]any{feature, second, document}
	got := evalLeadWithSymbols(wider, results, make(map[string]map[string]struct{}))
	want := []map[string]any{first, feature, document}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("placement=%v, want first symbol then original evidence once: %v", got, want)
	}
	if !reflect.DeepEqual(results, []map[string]any{feature, second, document}) {
		t.Fatalf("placement mutated the original packet: %v", results)
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). A duplicate's
// support cannot cover an omitted record for the first method's emitted evidence.
func TestEvalQueryDuplicateSymbolSupportPreservesOmittedRecordAbstention(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/duplicatesupport\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: validate-cache\n    area: validation\n    summary: Validate cache.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: beta-delta\n    area: beta\n    summary: Beta delta.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	// Separate context windows; candidate IDs sort line 47 before line 6.
	writeTestFile(t, root, "internal/cache/cache.go", "package cache\n\ntype First struct{}\ntype Second struct{}\n\nfunc (Second) Validate(beta, delta bool) bool { return beta && delta }\n"+
		strings.Repeat("\n", 40)+"func (First) Validate(alpha, gamma bool) bool { return alpha && gamma }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	query := "validate cache alpha gamma beta delta"
	intent := evalInferIntent(query)
	queryTerms := evalRelevanceTerms(terms(query), intent)
	ordered := evalOrderedTerms(query)
	symbols, frequency := evalRankSymbols(index, query, queryTerms, intent)
	confident := evalConfidentSymbols(index, symbols, frequency, query, queryTerms, evalConfidentCeiling)
	if len(confident) != 2 || evalResultIdentity(confident[0].result) != evalResultIdentity(confident[1].result) ||
		!reflect.DeepEqual(confident[0].support, intersectionSet([]string{"validate", "cache", "alpha", "gamma"})) ||
		!reflect.DeepEqual(confident[1].support, intersectionSet([]string{"validate", "cache", "beta", "delta"})) {
		t.Fatalf("fixture must retain first/duplicate support separately: %+v", confident)
	}
	records, _ := evalRankRecords(index, query, queryTerms, ordered, intent)
	if len(records) != 2 || records[0].id != "validate-cache" || records[1].id != "beta-delta" ||
		records[1].score*5 < records[0].score*3 ||
		!reflect.DeepEqual(records[1].support, intersectionSet([]string{"beta", "delta"})) {
		t.Fatalf("fixture must admit an omitted competing beta/delta record: %+v", records)
	}
	packet, err := EvalQuery(context.Background(), index, query, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	if len(results) != 1 || !reflect.DeepEqual(results[0], confident[0].result) {
		t.Fatalf("want only first method's evidence: %v", results)
	}
	abstention := packet["abstention"].(map[string]any)
	if packet["state"] != "NEEDS_WIDENING" || abstention["active"] != true || abstention["reason"] != "omitted-competing-record" {
		t.Fatalf("discarded duplicate covered the omitted record: state=%v abstention=%v", packet["state"], abstention)
	}
}

// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). Wider lexical
// matches are not answers to an override contradicted by project claims. At
// limit 5 their placement would evict the critical scenario and its adjacent
// store invariant; a wider limit must preserve the same negative-claim order.
func TestEvalQueryWiderSymbolsPreserveNegativeClaimAbstention(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/negativeplacement\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: media-delete\n    area: deletion\n    summary: Delete media.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: trash-review\n    area: deletion\n    summary: Trash retention.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios:\n"+
		"  - id: audited-delete-bypass\n    area: deletion\n    summary: Cannot bypass delete audit.\n    features: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: store-invariants\n    area: deletion\n    summary: Store media delete retention.\n    features: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "docs/adr/0051-retention.md", "# Trash retention\n\nStatus: accepted\n\nRetention audit rules.\n")
	writeTestFile(t, root, "internal/store/trash.go", "package store\n\ntype TrashRetention struct {\n Hard bool\n Delete bool\n Media bool\n Immediately bool\n Bypassing bool\n}\n\nfunc TrashMedia(retention TrashRetention) bool { return retention.Delete }\n\nfunc TrashMediaAt(retention TrashRetention) bool { return retention.Media }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"hard delete media immediately bypassing trash retention store audit",
		"hard delete media trash retention store audit",
	} {
		for _, limit := range []int{5, 10} {
			packet, err := EvalQuery(context.Background(), index, query, limit, nil)
			if err != nil {
				t.Fatal(err)
			}
			results := mapsFromAny(packet["results"])
			abstention := packet["abstention"].(map[string]any)
			if len(mapsFromAny(abstention["nearest_claims"])) == 0 {
				t.Fatalf("fixture lost its negative claim: %v", packet)
			}
			if !strings.Contains(query, "immediately") {
				if len(results) == 0 || results[0]["kind"] != "symbol" || abstention["reason"] == "nearest-negative-claim" || limit == 10 && (abstention["reason"] != "none" || packet["state"] != "READY") {
					t.Fatalf("inactive nearest claim blocked placement at limit %d: %v", limit, packet)
				}
				continue
			}
			got := make([]string, 0, len(results))
			for _, result := range results {
				got = append(got, evalResultIdentity(result))
			}
			want := []string{"feature:media-delete", "feature:trash-review", "scenario:store-invariants", "scenario:audited-delete-bypass", "decision:docs/adr/0051-retention.md"}
			if packet["state"] != "NEEDS_WIDENING" || abstention["reason"] != "nearest-negative-claim" || abstention["active"] != true || len(got) < len(want) || !reflect.DeepEqual(got[:len(want)], want) {
				t.Fatalf("limit %d: state=%v abstention=%v selectors=%v, want original negative-claim packet %v", limit, packet["state"], abstention, got, want)
			}
		}
	}
}

// evalScopeRepository is a repository where `command` and `files` are carried
// by most sources, so they describe the repository rather than any task. With
// renderer set it also holds a declaration that answers the task's
// discriminating words.
func evalScopeRepository(t *testing.T, renderer bool) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore":  ".context-corvint/\n",
		"go.mod":      "module example.test/scope\n\ngo 1.27.0\n",
		"cmd/run.go":  "package cmd\n\n// RunCommandFiles runs the command files.\nfunc RunCommandFiles() {}\n",
		"cmd/list.go": "package cmd\n\n// ListCommandFiles lists the command files.\nfunc ListCommandFiles() {}\n",
		"cmd/load.go": "package cmd\n\n// LoadCommandFiles loads the command files.\nfunc LoadCommandFiles() {}\n",
		"cmd/save.go": "package cmd\n\n// SaveCommandFiles saves the command files.\nfunc SaveCommandFiles() {}\n",
	}
	if renderer {
		files["doc/render.go"] = "package doc\n\n// RenderJSONDocs renders command docs as JSON files.\nfunc RenderJSONDocs() {}\n"
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "command files")
	return root
}

// GPK-V0-093 scope admission: a packet resting only on words most sources
// carry is out of scope, while the same task in a repository that answers its
// discriminating words stays READY. The abstention reason is unchanged.
func TestEvalQueryScopeRefusesRepositoryVocabulary(t *testing.T) {
	t.Parallel()
	const task = "render command docs as JSON files"
	for _, testCase := range []struct {
		name, state, reason string
		renderer            bool
	}{
		{name: "GPK-V0-093 common words alone abstain", state: "OUT_OF_SCOPE", reason: "below-relevance-floor"},
		{name: "GPK-V0-093 discriminating support answers", state: "READY", reason: "none", renderer: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			index, err := Build(context.Background(), evalScopeRepository(t, testCase.renderer))
			if err != nil {
				t.Fatal(err)
			}
			state, count, reason := evalFloorQuery(t, index, task)
			if state != testCase.state || reason != testCase.reason {
				t.Fatalf("state=%q reason=%q results=%d, want %q %q", state, reason, count, testCase.state, testCase.reason)
			}
			if testCase.state == "OUT_OF_SCOPE" && count != 0 {
				t.Fatalf("results = %d, want 0", count)
			}
		})
	}
}

// GPK-V0-093 leaves record admission unchanged: in a repository of five
// sources every word the session-revocation feature carries reaches more than
// half of them, and the task's other words (`expiry`, `enforcement`) no source
// carries, yet the ledger row that meets the floor still answers the task. This
// is the cli-parity `query-repository` fixture the scope check once withdrew.
func TestEvalQueryScopeAdmitsLedgerRecordOnCommonWords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	for path, content := range map[string]string{
		".gitignore":                    ".context-corvint/\n",
		"go.mod":                        "module example.test/query\n\ngo 1.27.0\n",
		"internal/auth/session.go":      "package auth\n\nfunc EnforceSessionRevocation() bool { return true }\n",
		"internal/auth/session_test.go": "package auth\n\n// feature:session-revocation scenario:revoked-device\nfunc TestEnforceSessionRevocation() {}\n",
		"testing/features.yaml":         "features:\n  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml":        "scenarios:\n  - id: revoked-device\n    area: auth\n    summary: Expired device session is revoked.\n    features: [session-revocation]\n    applies: [server]\n    status: shipped\n",
	} {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "session revocation")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, "session expiry device revocation enforcement", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	if packet["state"] != "READY" || len(results) == 0 || evalResultIdentity(results[0]) != "feature:session-revocation" {
		t.Fatalf("state=%v abstention=%v results=%v, want READY led by feature:session-revocation", packet["state"], packet["abstention"], results)
	}
}

// GPK-V0-093: in a one-source index every carried word reaches more than half
// of the sources, so without the one-source rule the task's absent word
// (`locate`) would withdraw the declaration the task names.
func TestEvalQueryScopeOneSourceWordDiscriminates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "pkg/add.go", "package fixture\nfunc Add() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "add")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, "locate Add in add.go", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	if packet["state"] != "READY" || len(results) == 0 || evalResultIdentity(results[0]) != "symbol:pkg/add.go:Add" {
		t.Fatalf("state=%v abstention=%v results=%v, want READY led by symbol:pkg/add.go:Add", packet["state"], packet["abstention"], results)
	}
}

// V1-3052 (GPK-V0-093): a declaration the task names exactly, by its indexed
// path and its name, satisfies the scope check once it meets the floor. In the
// qualify-opencode fixture `add` and `go` reach more than half of the sources
// and `locate` none, so the task has an uncommon word and no discriminating
// one, and the scope check withdrew add.go:Add below-relevance-floor. The
// cobra off-topic abstention (TestEvalQueryScopeRefusesRepositoryVocabulary)
// names no path and is unchanged.
func TestEvalQueryScopeAbsentWordDoesNotWithdrawNamedSymbol(t *testing.T) {
	t.Parallel()
	const source = "package qualification\n// before\nfunc Add(a, b int) int { return a + b }\n"
	gomod := "module example.com/qualification\n\ngo 1.27.3\n"
	agents := "# Project instructions\nFor repository workflow changes, inspect add.go and run git diff --check.\n"
	for _, testCase := range []struct {
		name, task string
		files      map[string]string
	}{
		{name: "one source", task: "Locate Add in add.go.", files: map[string]string{"add.go": source}},
		{name: "module and source", task: "Locate Add in add.go.", files: map[string]string{"go.mod": gomod, "add.go": source}},
		{name: "qualify-opencode fixture", task: "Locate Add in add.go.", files: map[string]string{"go.mod": gomod, "add.go": source, "AGENTS.md": agents}},
		{name: "qualify-opencode fixture without the absent word", task: "Add in add.go.", files: map[string]string{"go.mod": gomod, "add.go": source, "AGENTS.md": agents}},
		{name: "qualify-opencode fixture with AGENTS.md not naming add.go", task: "Locate Add in add.go.", files: map[string]string{"go.mod": gomod, "add.go": source, "AGENTS.md": "# Project instructions\nRun git diff --check.\n"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			testGit(t, root, "config", "user.email", "corvint@example.test")
			testGit(t, root, "config", "user.name", "Corvint Test")
			for path, content := range testCase.files {
				writeTestFile(t, root, path, content)
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "fixture")
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := EvalQuery(context.Background(), index, testCase.task, 10, nil)
			if err != nil {
				t.Fatal(err)
			}
			results := mapsFromAny(packet["results"])
			if packet["state"] != "READY" || len(results) == 0 || evalResultIdentity(results[0]) != "symbol:add.go:Add" {
				t.Fatalf("state=%v abstention=%v results=%v, want READY led by symbol:add.go:Add", packet["state"], packet["abstention"], results)
			}
			if _, isGo := testCase.files["go.mod"]; isGo && !slices.Contains(anySlice(packet["verification"]), any("go test ./...")) {
				t.Fatalf("verification = %v, want go test ./...", packet["verification"])
			}
		})
	}
}

// V1-3052: the exact-naming exemption reads the path and the name as two
// pointers. A path whose stem equals the declaration's name does not also name
// the declaration, so "Explain add.py" with `def add` stays under the scope
// check, and neither does another path-like token (`other/add.py`,
// `docs/Add.md`); the `./path` and `path:Name` spellings name it.
func TestEvalScopeNamesExactly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "add.py", "def add():\n    return 1\n")
	writeTestFile(t, root, "pkg/add.go", "package pkg\nfunc Add() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "fixture")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		text, id, name string
		want           bool
	}{
		{text: "Explain add.py", id: "add.py:add", name: "add"},
		{text: "Explain add in add.py", id: "add.py:add", name: "add", want: true},
		{text: "Locate Add in ./pkg/add.go.", id: "pkg/add.go:Add", name: "Add", want: true},
		{text: "Locate `pkg/add.go:Add`", id: "pkg/add.go:Add", name: "Add", want: true},
		{text: "Locate Add in add.go", id: "pkg/add.go:Add", name: "Add"},
		{text: "Locate Add in missing.go", id: "missing.go:Add", name: "Add"},
		{text: "Locate Sub in pkg/add.go", id: "pkg/add.go:Add", name: "Add"},
		{text: "Explain add.py and other/add.py", id: "add.py:add", name: "add"},
		{text: "Locate pkg/add.go per docs/Add.md", id: "pkg/add.go:Add", name: "Add"},
		{text: "Locate Server.Run in pkg/add.go", id: "pkg/add.go:Server.Run", name: "Server.Run", want: true},
	} {
		scope := evalScope{index: index, text: testCase.text}
		result := map[string]any{"kind": "symbol", "id": testCase.id, "name": testCase.name}
		if got := scope.namesExactly(result); got != testCase.want {
			t.Errorf("namesExactly(%q, %s) = %v, want %v", testCase.text, testCase.id, got, testCase.want)
		}
	}
}
